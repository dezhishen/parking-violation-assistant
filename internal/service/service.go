package service

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dezhishen/parking-violation-assistant/internal/db"
	"github.com/dezhishen/parking-violation-assistant/internal/models"
)

// CreateRecord 创建新违停记录
func CreateRecord(req models.CreateRecordRequest) (*models.ParkingRecord, error) {
	now := time.Now().Format("2006-01-02 15:04:05")
	res, err := db.DB.Exec(`
		INSERT INTO parking_records (plate_number, image_path, parking_time, status, notes, created_at, updated_at)
		VALUES (?, ?, ?, '待处理', ?, ?, ?)`,
		req.PlateNumber, req.ImagePath, req.ParkingTime, req.Notes, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("插入记录失败: %w", err)
	}
	id, _ := res.LastInsertId()
	return GetRecord(id)
}

// GetRecord 获取单条记录
func GetRecord(id int64) (*models.ParkingRecord, error) {
	row := db.DB.QueryRow(`SELECT id, plate_number, image_path, parking_time, status,
		reminder_time, second_image_path, second_check_time, notes, created_at, updated_at
		FROM parking_records WHERE id = ?`, id)
	return scanRecord(row)
}

// UpdateStatus 更新记录状态
func UpdateStatus(id int64, req models.UpdateStatusRequest) (*models.ParkingRecord, error) {
	if req.Status == "" {
		return nil, errors.New("状态不能为空")
	}

	current, err := GetRecord(id)
	if err != nil {
		return nil, fmt.Errorf("记录不存在: %w", err)
	}
	if err := validateStatusTransition(current.Status, req.Status); err != nil {
		return nil, err
	}

	now := time.Now().Format("2006-01-02 15:04:05")

	setClauses := []string{"status = ?", "updated_at = ?"}
	args := []any{req.Status, now}

	if req.ReminderTime != nil {
		setClauses = append(setClauses, "reminder_time = ?")
		args = append(args, *req.ReminderTime)
	}
	if req.SecondImagePath != nil {
		setClauses = append(setClauses, "second_image_path = ?")
		args = append(args, *req.SecondImagePath)
	}
	if req.SecondCheckTime != nil {
		setClauses = append(setClauses, "second_check_time = ?")
		args = append(args, *req.SecondCheckTime)
	}
	if req.Notes != nil {
		setClauses = append(setClauses, "notes = ?")
		args = append(args, *req.Notes)
	}

	args = append(args, id)
	query := fmt.Sprintf("UPDATE parking_records SET %s WHERE id = ?", strings.Join(setClauses, ", "))

	_, err = db.DB.Exec(query, args...)
	if err != nil {
		return nil, fmt.Errorf("更新状态失败: %w", err)
	}
	return GetRecord(id)
}

// QueryRecords 查询记录（带分页）
func QueryRecords(filters models.QueryFilters) (*models.QueryResult, error) {
	where, args := buildWhere(filters)

	if filters.PageSize <= 0 {
		filters.PageSize = 20
	}
	if filters.Page <= 0 {
		filters.Page = 1
	}
	offset := (filters.Page - 1) * filters.PageSize

	// 总数
	countQuery := "SELECT COUNT(*) FROM parking_records" + where
	var total int
	if err := db.DB.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, err
	}

	// 数据
	dataArgs := append(args, filters.PageSize, offset)
	rows, err := db.DB.Query(`SELECT id, plate_number, image_path, parking_time, status,
		reminder_time, second_image_path, second_check_time, notes, created_at, updated_at
		FROM parking_records`+where+` ORDER BY created_at DESC LIMIT ? OFFSET ?`, dataArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records, err := scanRecords(rows)
	if err != nil {
		return nil, err
	}

	return &models.QueryResult{
		Records:  records,
		Total:    total,
		Page:     filters.Page,
		PageSize: filters.PageSize,
	}, nil
}

// GetPlateStats 按车牌统计违停次数（主界面表格）
func GetPlateStats(filters models.QueryFilters) (*models.PlateStatsResult, error) {
	where, args := buildWhere(filters)
	if filters.Status == "" {
		// 未指定状态时，默认按“违停”统计，保持主界面默认语义。
		where = addWhere(where, "status = '违停'")
	}
	having := ""
	if filters.OverThreeWarning {
		threshold := filters.WarningThreshold
		if threshold <= 0 {
			threshold = 3
		}
		having = fmt.Sprintf(" HAVING COUNT(*) > %d", threshold)
	}

	if filters.PageSize <= 0 {
		filters.PageSize = 20
	}
	if filters.Page <= 0 {
		filters.Page = 1
	}
	offset := (filters.Page - 1) * filters.PageSize

	countQuery := "SELECT COUNT(*) FROM (SELECT plate_number FROM parking_records" + where + " GROUP BY plate_number" + having + ") t"
	var total int
	if err := db.DB.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, err
	}

	dataArgs := append(args, filters.PageSize, offset)
	rows, err := db.DB.Query(`
		SELECT plate_number, COUNT(*) as cnt, MAX(parking_time) as last_violation
		FROM parking_records`+where+`
		GROUP BY plate_number
		`+having+`
		ORDER BY cnt DESC, last_violation DESC
		LIMIT ? OFFSET ?`, dataArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []models.PlateStats
	for rows.Next() {
		var s models.PlateStats
		if err := rows.Scan(&s.PlateNumber, &s.ViolationCount, &s.LastViolation); err != nil {
			return nil, err
		}
		s.IsHighFrequency = s.ViolationCount >= 3
		stats = append(stats, s)
	}
	if stats == nil {
		stats = []models.PlateStats{}
	}

	return &models.PlateStatsResult{
		Stats:    stats,
		Total:    total,
		Page:     filters.Page,
		PageSize: filters.PageSize,
	}, nil
}

// GetPlateRecords 获取某车牌的所有记录
func GetPlateRecords(plateNumber string) ([]models.ParkingRecord, error) {
	rows, err := db.DB.Query(`SELECT id, plate_number, image_path, parking_time, status,
		reminder_time, second_image_path, second_check_time, notes, created_at, updated_at
		FROM parking_records WHERE plate_number = ? ORDER BY created_at DESC`, plateNumber)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRecords(rows)
}

// GetDashboardStats 获取首页统计
func GetDashboardStats() (*models.DashboardStats, error) {
	stats := &models.DashboardStats{}

	row := db.DB.QueryRow("SELECT COUNT(*) FROM parking_records WHERE status = '待处理'")
	if err := row.Scan(&stats.PendingReminder); err != nil {
		return nil, err
	}

	row = db.DB.QueryRow("SELECT COUNT(*) FROM parking_records WHERE status = '待确认'")
	if err := row.Scan(&stats.PendingConfirm); err != nil {
		return nil, err
	}

	return stats, nil
}

// ListAllForExport 导出用：查询所有符合条件的记录
func ListAllForExport(filters models.QueryFilters) ([]models.ParkingRecord, error) {
	where, args := buildWhere(filters)
	rows, err := db.DB.Query(`SELECT id, plate_number, image_path, parking_time, status,
		reminder_time, second_image_path, second_check_time, notes, created_at, updated_at
		FROM parking_records`+where+` ORDER BY plate_number, created_at DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRecords(rows)
}

// ListPlateStatsForExport 导出用：获取所有车牌统计
func ListPlateStatsForExport(filters models.QueryFilters) ([]models.PlateStats, error) {
	where, args := buildWhere(filters)
	if filters.Status == "" {
		where = addWhere(where, "status = '违停'")
	}
	having := ""
	if filters.OverThreeWarning {
		threshold := filters.WarningThreshold
		if threshold <= 0 {
			threshold = 3
		}
		having = fmt.Sprintf(" HAVING COUNT(*) > %d", threshold)
	}
	rows, err := db.DB.Query(`
		SELECT plate_number, COUNT(*) as cnt, MAX(parking_time) as last_violation
		FROM parking_records`+where+`
		GROUP BY plate_number
		`+having+`
		ORDER BY cnt DESC, last_violation DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []models.PlateStats
	for rows.Next() {
		var s models.PlateStats
		if err := rows.Scan(&s.PlateNumber, &s.ViolationCount, &s.LastViolation); err != nil {
			return nil, err
		}
		s.IsHighFrequency = s.ViolationCount >= 3
		stats = append(stats, s)
	}
	return stats, nil
}

// DeleteRecord 删除记录
func DeleteRecord(id int64) error {
	_, err := db.DB.Exec("DELETE FROM parking_records WHERE id = ?", id)
	return err
}

// ---- helpers ----

func buildWhere(f models.QueryFilters) (string, []any) {
	var conditions []string
	var args []any

	if f.PlateKeyword != "" {
		conditions = append(conditions, "plate_number LIKE ?")
		args = append(args, "%"+f.PlateKeyword+"%")
	}
	if f.Status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, f.Status)
	}
	if f.StartDate != "" {
		conditions = append(conditions, "created_at >= ?")
		args = append(args, f.StartDate+" 00:00:00")
	}
	if f.EndDate != "" {
		conditions = append(conditions, "created_at <= ?")
		args = append(args, f.EndDate+" 23:59:59")
	}

	if len(conditions) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conditions, " AND "), args
}

func addWhere(existing, extra string) string {
	if existing == "" {
		return " WHERE " + extra
	}
	return existing + " AND " + extra
}

func scanRecord(row *sql.Row) (*models.ParkingRecord, error) {
	var r models.ParkingRecord
	err := row.Scan(&r.ID, &r.PlateNumber, &r.ImagePath, &r.ParkingTime, &r.Status,
		&r.ReminderTime, &r.SecondImagePath, &r.SecondCheckTime, &r.Notes, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func scanRecords(rows *sql.Rows) ([]models.ParkingRecord, error) {
	var records []models.ParkingRecord
	for rows.Next() {
		var r models.ParkingRecord
		if err := rows.Scan(&r.ID, &r.PlateNumber, &r.ImagePath, &r.ParkingTime, &r.Status,
			&r.ReminderTime, &r.SecondImagePath, &r.SecondCheckTime, &r.Notes, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		records = append(records, r)
	}
	if records == nil {
		records = []models.ParkingRecord{}
	}
	return records, nil
}

func validateStatusTransition(from, to string) error {
	if from == to {
		return nil
	}

	allowed := map[string]map[string]bool{
		models.StatusPending: {
			models.StatusWaitConfirm: true,
		},
		models.StatusWaitConfirm: {
			models.StatusViolation: true,
			models.StatusMoved:     true,
		},
		models.StatusViolation: {},
		models.StatusMoved:     {},
	}

	if next, ok := allowed[from]; ok && next[to] {
		return nil
	}
	return fmt.Errorf("非法状态流转: %s -> %s", from, to)
}
