package models

// 违停状态
const (
	StatusPending     = "待处理" // 刚上传，初始状态
	StatusWaitConfirm = "待确认" // 提醒移车后，等待确认
	StatusViolation   = "违停"  // 确认未移车
	StatusMoved       = "已挪车" // 确认已挪车
)

// ParkingRecord 停车违规记录
type ParkingRecord struct {
	ID              int64   `json:"id"`
	PlateNumber     string  `json:"plate_number"`
	ImagePath       string  `json:"image_path"`   // 第一张照片
	ParkingTime     string  `json:"parking_time"` // 停车时间（OCR识别）
	Status          string  `json:"status"`
	ReminderTime    *string `json:"reminder_time"`     // 电话提醒时间
	SecondImagePath *string `json:"second_image_path"` // 第二张照片
	SecondCheckTime *string `json:"second_check_time"` // 第二次检查时间
	Notes           string  `json:"notes"`             // 备注
	CreatedAt       string  `json:"created_at"`
	UpdatedAt       string  `json:"updated_at"`
}

// QueryFilters 查询过滤条件
type QueryFilters struct {
	PlateKeyword     string `json:"plate_keyword"`
	Status           string `json:"status"`
	StartDate        string `json:"start_date"`
	EndDate          string `json:"end_date"`
	Page             int    `json:"page"`
	PageSize         int    `json:"page_size"`
	OverThreeWarning bool   `json:"over_three_warning"`
	WarningThreshold int    `json:"warning_threshold"`
}

// PlateStats 车牌统计（主界面表格用）
type PlateStats struct {
	PlateNumber     string `json:"plate_number"`
	ViolationCount  int    `json:"violation_count"`
	LastViolation   string `json:"last_violation"`
	IsHighFrequency bool   `json:"is_high_frequency"` // 3次以上红色高亮
}

// DashboardStats 首页统计面板
type DashboardStats struct {
	PendingReminder int `json:"pending_reminder"` // 待提醒移车（状态=待处理）
	PendingConfirm  int `json:"pending_confirm"`  // 待确认是否移车（状态=待确认）
}

// QueryResult 分页查询结果
type QueryResult struct {
	Records  []ParkingRecord `json:"records"`
	Total    int             `json:"total"`
	Page     int             `json:"page"`
	PageSize int             `json:"page_size"`
}

// PlateStatsResult 车牌统计查询结果
type PlateStatsResult struct {
	Stats    []PlateStats `json:"stats"`
	Total    int          `json:"total"`
	Page     int          `json:"page"`
	PageSize int          `json:"page_size"`
}

// OCRResult OCR识别结果
type OCRResult struct {
	PlateNumber string `json:"plate_number"`
	ParkingTime string `json:"parking_time"`
	RawText     string `json:"raw_text"`
}

// CreateRecordRequest 创建记录请求
type CreateRecordRequest struct {
	PlateNumber string `json:"plate_number"`
	ImagePath   string `json:"image_path"`
	ParkingTime string `json:"parking_time"`
	Notes       string `json:"notes"`
}

// UpdateStatusRequest 更新状态请求
type UpdateStatusRequest struct {
	Status          string  `json:"status"`
	ReminderTime    *string `json:"reminder_time"`
	SecondImagePath *string `json:"second_image_path"`
	SecondCheckTime *string `json:"second_check_time"`
	Notes           *string `json:"notes"`
}
