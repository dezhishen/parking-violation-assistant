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

// 领域错误，供 HTTP 层映射状态码。
var (
	// ErrRecordNotFound 记录不存在
	ErrRecordNotFound = errors.New("记录不存在")
	// ErrInvalidTransition 非法状态流转
	ErrInvalidTransition = errors.New("非法状态流转")
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
	record, err := scanRecord(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrRecordNotFound
		}
		return nil, err
	}
	return record, nil
}

// UpdateStatus 更新记录状态
func UpdateStatus(id int64, req models.UpdateStatusRequest) (*models.ParkingRecord, error) {
	if req.Status == "" {
		return nil, errors.New("状态不能为空")
	}

	current, err := GetRecord(id)
	if err != nil {
		return nil, err
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
	dataArgs := append(append([]any{}, args...), filters.PageSize, offset)
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

	threshold := warningThreshold(filters)
	if filters.PageSize <= 0 {
		filters.PageSize = 20
	}
	if filters.Page <= 0 {
		filters.Page = 1
	}
	offset := (filters.Page - 1) * filters.PageSize

	countQuery := "SELECT COUNT(*) FROM (SELECT plate_number FROM parking_records" + where + " GROUP BY plate_number" + havingClause(filters, threshold) + ") t"
	var total int
	if err := db.DB.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, err
	}

	dataArgs := append(append([]any{}, args...), filters.PageSize, offset)
	rows, err := db.DB.Query(`
		SELECT plate_number, COUNT(*) as cnt, MAX(parking_time) as last_violation
		FROM parking_records`+where+`
		GROUP BY plate_number
		`+havingClause(filters, threshold)+`
		ORDER BY cnt DESC, last_violation DESC
		LIMIT ? OFFSET ?`, dataArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats, err := scanPlateStats(rows, threshold)
	if err != nil {
		return nil, err
	}

	return &models.PlateStatsResult{
		Stats:    stats,
		Total:    total,
		Page:     filters.Page,
		PageSize: filters.PageSize,
	}, nil
}

// GetPlateRecords 获取某车牌的记录（分页）
func GetPlateRecords(plateNumber string, page, pageSize int) (*models.PlateRecordsResult, error) {
	if pageSize <= 0 {
		pageSize = 20
	}
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * pageSize

	var total int
	if err := db.DB.QueryRow(`SELECT COUNT(*) FROM parking_records WHERE plate_number = ?`, plateNumber).Scan(&total); err != nil {
		return nil, err
	}

	rows, err := db.DB.Query(`SELECT id, plate_number, image_path, parking_time, status,
		reminder_time, second_image_path, second_check_time, notes, created_at, updated_at
		FROM parking_records WHERE plate_number = ? ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		plateNumber, pageSize, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records, err := scanRecords(rows)
	if err != nil {
		return nil, err
	}

	return &models.PlateRecordsResult{
		Records:  records,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
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
	threshold := warningThreshold(filters)
	rows, err := db.DB.Query(`
		SELECT plate_number, COUNT(*) as cnt, MAX(parking_time) as last_violation
		FROM parking_records`+where+`
		GROUP BY plate_number
		`+havingClause(filters, threshold)+`
		ORDER BY cnt DESC, last_violation DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats, err := scanPlateStats(rows, threshold)
	if err != nil {
		return nil, err
	}
	if stats == nil {
		stats = []models.PlateStats{}
	}
	return stats, nil
}

// DeleteRecord 删除记录，并返回被删除的记录（供调用方清理关联文件）
func DeleteRecord(id int64) (*models.ParkingRecord, error) {
	record, err := GetRecord(id)
	if err != nil {
		return nil, err
	}
	if _, err := db.DB.Exec("DELETE FROM parking_records WHERE id = ?", id); err != nil {
		return nil, err
	}
	return record, nil
}

// IsImagePathReferenced 判断图片路径是否仍被其他记录引用
func IsImagePathReferenced(imagePath string, excludeID int64) (bool, error) {
	var count int
	err := db.DB.QueryRow(`SELECT COUNT(*) FROM parking_records
		WHERE id <> ? AND (image_path = ? OR second_image_path = ?)`,
		excludeID, imagePath, imagePath).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// ---- helpers ----

// warningThreshold 返回生效的预警阈值
func warningThreshold(f models.QueryFilters) int {
	if f.WarningThreshold <= 0 {
		return models.DefaultWarningThreshold
	}
	return f.WarningThreshold
}

// havingClause 构造“仅看 N 次及以上”的 HAVING 子句（阈值直接取自查询条件）
func havingClause(f models.QueryFilters, threshold int) string {
	if !f.OverThreeWarning {
		return ""
	}
	return fmt.Sprintf(" HAVING COUNT(*) >= %d", threshold)
}

func scanPlateStats(rows *sql.Rows, threshold int) ([]models.PlateStats, error) {
	stats := make([]models.PlateStats, 0)
	for rows.Next() {
		var s models.PlateStats
		if err := rows.Scan(&s.PlateNumber, &s.ViolationCount, &s.LastViolation); err != nil {
			return nil, err
		}
		// 高亮阈值与查询阈值保持一致，避免“筛选 5 次、仍高亮 3 次”。
		s.IsHighFrequency = s.ViolationCount >= threshold
		stats = append(stats, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return stats, nil
}

func buildWhere(f models.QueryFilters) (string, []any) {
	var conditions []string
	var args []any

	if f.PlateKeyword != "" {
		conditions = append(conditions, "plate_number LIKE ?")
		args = append(args, "%"+f.PlateKeyword+"%")
	}
	// StatusAll 表示不按状态筛选
	if f.Status != "" && f.Status != models.StatusAll {
		conditions = append(conditions, "status = ?")
		args = append(args, f.Status)
	}
	if f.StartDate != "" {
		conditions = append(conditions, "created_at >= ?")
		args = append(args, f.StartDate+" 00:00:00")
	}
	if f.EndDate != "" {
		// 用“小于次日 00:00:00”代替“小于等于当日 23:59:59”，避免漏掉带小数秒的时间戳。
		if next, ok := nextDay(f.EndDate); ok {
			conditions = append(conditions, "created_at < ?")
			args = append(args, next+" 00:00:00")
		} else {
			conditions = append(conditions, "created_at <= ?")
			args = append(args, f.EndDate+" 23:59:59")
		}
	}

	if len(conditions) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conditions, " AND "), args
}

// nextDay 返回 date 的次日日期字符串
func nextDay(date string) (string, bool) {
	t, err := time.ParseInLocation("2006-01-02", date, time.Local)
	if err != nil {
		return "", false
	}
	return t.AddDate(0, 0, 1).Format("2006-01-02"), true
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
	records := make([]models.ParkingRecord, 0)
	for rows.Next() {
		var r models.ParkingRecord
		if err := rows.Scan(&r.ID, &r.PlateNumber, &r.ImagePath, &r.ParkingTime, &r.Status,
			&r.ReminderTime, &r.SecondImagePath, &r.SecondCheckTime, &r.Notes, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		records = append(records, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return records, nil
}

// allowedTransitions 状态流转白名单（包级常量，避免每次调用重建）
var allowedTransitions = map[string]map[string]bool{
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

func validateStatusTransition(from, to string) error {
	if from == to {
		return nil
	}

	if next, ok := allowedTransitions[from]; ok && next[to] {
		return nil
	}
	return fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, from, to)
}
