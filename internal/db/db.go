package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

var DB *sql.DB

// Init 初始化数据库连接和表结构
func Init(dataDir string) error {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("创建数据目录失败: %w", err)
	}

	dbPath := filepath.Join(dataDir, "records.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("打开数据库失败: %w", err)
	}

	db.SetMaxOpenConns(1) // SQLite 单连接

	if err := migrate(db); err != nil {
		return fmt.Errorf("数据库迁移失败: %w", err)
	}

	DB = db
	log.Printf("数据库已连接: %s", dbPath)
	return nil
}

// migrate 执行数据库迁移
func migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS parking_records (
			id               INTEGER PRIMARY KEY AUTOINCREMENT,
			plate_number     TEXT    NOT NULL,
			image_path       TEXT    NOT NULL,
			parking_time     TEXT    NOT NULL DEFAULT '',
			status           TEXT    NOT NULL DEFAULT '待处理',
			reminder_time    TEXT,
			second_image_path TEXT,
			second_check_time TEXT,
			notes            TEXT    NOT NULL DEFAULT '',
			created_at       TEXT    NOT NULL DEFAULT (datetime('now', 'localtime')),
			updated_at       TEXT    NOT NULL DEFAULT (datetime('now', 'localtime'))
		);

		CREATE INDEX IF NOT EXISTS idx_plate ON parking_records(plate_number);
		CREATE INDEX IF NOT EXISTS idx_status ON parking_records(status);
		CREATE INDEX IF NOT EXISTS idx_created ON parking_records(created_at DESC);
	`)
	if err != nil {
		return err
	}

	// 兼容旧版本状态值，统一到新流转：待处理 -> 待确认 -> 违停/已挪车
	_, err = db.Exec(`
		UPDATE parking_records SET status = '待确认' WHERE status IN ('已提醒', '待确认移车');
		UPDATE parking_records SET status = '已挪车' WHERE status = '已移车';
	`)
	return err
}

// Close 关闭数据库连接
func Close() {
	if DB != nil {
		DB.Close()
	}
}
