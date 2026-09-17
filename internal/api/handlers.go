package api

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/dezhishen/parking-violation-assistant/internal/models"
	"github.com/dezhishen/parking-violation-assistant/internal/service"

	"github.com/xuri/excelize/v2"
)

var uploadDir string

const (
	// maxUploadBytes 单个上传文件（含 multipart 开销）的最大字节数。
	maxUploadBytes = 20 << 20
	// maxOCRJSONBytes JSON 形式的 OCR 请求体上限。
	maxOCRJSONBytes = 1 << 20
)

// allowedImageExt 允许上传的图片扩展名。
var allowedImageExt = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".bmp":  true,
	".gif":  true,
}

// allowedImageMIME 允许上传的图片内容类型（依据文件头字节判断）。
var allowedImageMIME = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/bmp":  true,
	"image/gif":  true,
}

// SetUploadDir 设置上传目录
func SetUploadDir(dir string) {
	uploadDir = dir
}

// RegisterRoutes 注册所有 API 路由
func RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/records", handleRecords)
	mux.HandleFunc("/api/records/", handleRecordByID)
	mux.HandleFunc("/api/plates", handlePlates)
	mux.HandleFunc("/api/plates/", handlePlateDetail)
	mux.HandleFunc("/api/dashboard", handleDashboard)
	mux.HandleFunc("/api/upload", handleUpload)
	mux.HandleFunc("/api/uploads/discard", handleDiscardUpload)
	mux.HandleFunc("/api/ocr", handleOCR)
	mux.HandleFunc("/api/export/detail", handleExportDetail)
	mux.HandleFunc("/api/export/summary", handleExportSummary)
	mux.HandleFunc("/uploads/", handleServeFile)
}

// handleRecords GET /api/records?... POST /api/records
func handleRecords(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		filters := parseFilters(r)
		result, err := service.QueryRecords(filters)
		if err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		jsonOK(w, result)

	case http.MethodPost:
		var req models.CreateRecordRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, "请求格式错误", http.StatusBadRequest)
			return
		}
		if req.PlateNumber == "" {
			jsonError(w, "车牌号不能为空", http.StatusBadRequest)
			return
		}
		record, err := service.CreateRecord(req)
		if err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		jsonOK(w, record)

	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

// handleRecordByID GET/PUT/DELETE /api/records/:id
func handleRecordByID(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/api/records/")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		jsonError(w, "无效的记录ID", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		record, err := service.GetRecord(id)
		if err != nil {
			jsonError(w, "记录不存在", http.StatusNotFound)
			return
		}
		jsonOK(w, record)

	case http.MethodPut:
		var req models.UpdateStatusRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, "请求格式错误", http.StatusBadRequest)
			return
		}
		record, err := service.UpdateStatus(id, req)
		if err != nil {
			serviceError(w, err)
			return
		}
		jsonOK(w, record)

	case http.MethodDelete:
		record, err := service.DeleteRecord(id)
		if err != nil {
			serviceError(w, err)
			return
		}
		cleanupRecordFiles(record)
		jsonOK(w, map[string]string{"message": "删除成功"})

	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

// handlePlates GET /api/plates?... — 主界面车牌统计表格
func handlePlates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	filters := parseFilters(r)
	result, err := service.GetPlateStats(filters)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, result)
}

// handlePlateDetail GET /api/plates/:plate — 某车牌的记录（分页）
func handlePlateDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	plate := strings.TrimPrefix(r.URL.Path, "/api/plates/")
	plate = strings.TrimSpace(plate)
	if plate == "" {
		jsonError(w, "车牌号不能为空", http.StatusBadRequest)
		return
	}
	filters := parseFilters(r)
	result, err := service.GetPlateRecords(plate, filters.Page, filters.PageSize)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, result)
}

// handleDashboard GET /api/dashboard — 首页统计
func handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	stats, err := service.GetDashboardStats()
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, stats)
}

// handleUpload POST /api/upload — 上传图片
func handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		jsonError(w, "文件过大", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		jsonError(w, "未找到文件", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// 校验文件类型：先看扩展名，再校验文件头，避免改后缀上传任意内容。
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !allowedImageExt[ext] {
		jsonError(w, "不支持的文件类型，仅支持图片", http.StatusBadRequest)
		return
	}
	if err := verifyImageContent(file); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	newName, err := newUploadName(ext)
	if err != nil {
		jsonError(w, "生成文件名失败", http.StatusInternalServerError)
		return
	}
	savePath := filepath.Join(uploadDir, newName)

	// 先写临时文件，成功后再重命名，避免失败时留下半截文件。
	tmp, err := os.CreateTemp(uploadDir, "upload_*.tmp")
	if err != nil {
		jsonError(w, "保存文件失败", http.StatusInternalServerError)
		return
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := io.Copy(tmp, file); err != nil {
		tmp.Close()
		jsonError(w, "写入文件失败", http.StatusInternalServerError)
		return
	}
	if err := tmp.Close(); err != nil {
		jsonError(w, "写入文件失败", http.StatusInternalServerError)
		return
	}
	if err := os.Rename(tmpName, savePath); err != nil {
		jsonError(w, "保存文件失败", http.StatusInternalServerError)
		return
	}

	urlPath := "/uploads/" + newName
	log.Printf("上传文件: %s", savePath)
	jsonOK(w, map[string]string{
		"url":  urlPath,
		"name": newName,
	})
}

// verifyImageContent 读取文件头判断真实内容类型，并把读取位置复位。
func verifyImageContent(file io.ReadSeeker) error {
	head := make([]byte, 512)
	n, err := io.ReadFull(file, head)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return fmt.Errorf("读取文件失败")
	}
	if !allowedImageMIME[http.DetectContentType(head[:n])] {
		return fmt.Errorf("文件内容不是受支持的图片格式")
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("读取文件失败")
	}
	return nil
}

// newUploadName 生成唯一的上传文件名（时间戳 + 随机串）。
func newUploadName(ext string) (string, error) {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s_%s%s", time.Now().Format("20060102_150405"), hex.EncodeToString(buf[:]), ext), nil
}

// handleDiscardUpload POST /api/uploads/discard — 丢弃未被任何记录引用的上传图片
// 用于「上传后取消创建记录」的场景，避免在磁盘上留下孤儿文件。
func handleDiscardUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxOCRJSONBytes)
	var req struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "请求格式错误", http.StatusBadRequest)
		return
	}

	stored := strings.TrimSpace(req.URL)
	if stored == "" {
		jsonError(w, "图片地址不能为空", http.StatusBadRequest)
		return
	}

	filePath, err := resolveImageFilePath(stored)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if !isInsideDir(uploadDir, filePath) {
		jsonError(w, "只能丢弃上传目录内的文件", http.StatusBadRequest)
		return
	}

	referenced, err := service.IsImagePathReferenced(stored, 0)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if referenced {
		jsonOK(w, map[string]string{"message": "图片已被记录引用，跳过删除"})
		return
	}

	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		jsonError(w, "删除图片失败", http.StatusInternalServerError)
		return
	}
	log.Printf("丢弃未使用的上传文件: %s", filePath)
	jsonOK(w, map[string]string{"message": "已丢弃"})
}

// handleServeFile GET /uploads/:filename — 提供上传的图片访问
func handleServeFile(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/uploads/"))
	// 安全校验：不允许路径穿越，只接受纯文件名
	if name == "" || name == "." || name == ".." ||
		strings.ContainsAny(name, `/\`) || name != filepath.Base(name) {
		http.NotFound(w, r)
		return
	}
	// 上传文件内容已做类型校验，额外禁止浏览器嗅探内容类型。
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "private, max-age=86400")
	http.ServeFile(w, r, filepath.Join(uploadDir, name))
}

// handleOCR POST /api/ocr — 对已上传图片进行OCR并提取车牌和停车时间
func handleOCR(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	imagePath := ""
	var fileToRemove string
	if strings.Contains(r.Header.Get("Content-Type"), "multipart/form-data") {
		r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
		if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
			jsonError(w, "文件过大", http.StatusBadRequest)
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			jsonError(w, "未找到OCR文件", http.StatusBadRequest)
			return
		}
		defer file.Close()

		ext := strings.ToLower(filepath.Ext(header.Filename))
		if ext == "" {
			ext = ".png"
		}

		tempFile, err := os.CreateTemp(uploadDir, "ocr_*."+strings.TrimPrefix(ext, "."))
		if err != nil {
			jsonError(w, "保存OCR临时文件失败", http.StatusInternalServerError)
			return
		}
		fileToRemove = tempFile.Name()
		if _, err := io.Copy(tempFile, file); err != nil {
			tempFile.Close()
			os.Remove(tempFile.Name())
			jsonError(w, "写入OCR临时文件失败", http.StatusInternalServerError)
			return
		}
		tempFile.Close()
		imagePath = tempFile.Name()
	} else {
		r.Body = http.MaxBytesReader(w, r.Body, maxOCRJSONBytes)
		var req struct {
			ImagePath string `json:"image_path"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, "请求格式错误", http.StatusBadRequest)
			return
		}
		imagePath = req.ImagePath
	}

	if fileToRemove != "" {
		defer os.Remove(fileToRemove)
	}

	resolved, err := resolveImageFilePath(imagePath)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if _, err := os.Stat(resolved); err != nil {
		jsonError(w, "图片不存在", http.StatusBadRequest)
		return
	}

	result, err := runONNXOCR(resolved)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, result)
}

// handleExportDetail GET /api/export/detail — 导出详细记录Excel
func handleExportDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	filters := parseFilters(r)
	records, err := service.ListAllForExport(filters)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	f := excelize.NewFile()
	sheet := "详细记录"
	f.SetSheetName("Sheet1", sheet)
	// excelize 会为工作簿创建临时文件，必须显式关闭，否则反复导出会泄漏磁盘与句柄。
	defer closeExportFile(f)

	headers := []string{"ID", "车牌号", "第一张照片", "停车时间", "状态", "提醒时间", "第二次照片", "第二次检查时间", "备注", "创建时间"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
	}

	// 为图片列预留更友好的尺寸，避免图片被压缩到不可见。
	f.SetColWidth(sheet, "C", "C", 16)
	f.SetColWidth(sheet, "G", "G", 16)

	for row, rec := range records {
		rowNum := row + 2
		values := []any{rec.ID, rec.PlateNumber, rec.ParkingTime, rec.Status,
			nilStr(rec.ReminderTime), nilStr(rec.SecondCheckTime), rec.Notes, rec.CreatedAt}
		for col, v := range values {
			targetCol := col + 1
			if targetCol >= 3 {
				targetCol += 1
			}
			if targetCol >= 7 {
				targetCol += 1
			}
			cell, _ := excelize.CoordinatesToCellName(targetCol, rowNum)
			f.SetCellValue(sheet, cell, v)
		}

		// 有图片时抬高行高，保证导出文件中图片可见。
		if strings.TrimSpace(rec.ImagePath) != "" || (rec.SecondImagePath != nil && strings.TrimSpace(*rec.SecondImagePath) != "") {
			f.SetRowHeight(sheet, rowNum, 72)
		}

		firstCell, _ := excelize.CoordinatesToCellName(3, rowNum)
		if err := addImageToCell(f, sheet, firstCell, rec.ImagePath); err != nil {
			f.SetCellValue(sheet, firstCell, err.Error())
		}

		secondCell, _ := excelize.CoordinatesToCellName(7, rowNum)
		if err := addImageToCell(f, sheet, secondCell, nilStr(rec.SecondImagePath)); err != nil {
			f.SetCellValue(sheet, secondCell, err.Error())
		}
	}

	filename := fmt.Sprintf("违停详细记录_%s.xlsx", time.Now().Format("20060102_150405"))
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename*=UTF-8''%s`, encodeFilename(filename)))
	if err := f.Write(w); err != nil {
		log.Printf("导出详细记录失败: %v", err)
	}
}

// handleExportSummary GET /api/export/summary — 导出统计记录Excel
func handleExportSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	filters := parseFilters(r)
	stats, err := service.ListPlateStatsForExport(filters)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	f := excelize.NewFile()
	sheet := "统计记录"
	f.SetSheetName("Sheet1", sheet)
	// excelize 会为工作簿创建临时文件，必须显式关闭，否则反复导出会泄漏磁盘与句柄。
	defer closeExportFile(f)

	headers := []string{"车牌号", "违停次数", "最后违停时间", "是否高频(3次以上)"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
	}

	for row, s := range stats {
		rowNum := row + 2
		highFreq := "否"
		if s.IsHighFrequency {
			highFreq = "是"
		}
		values := []any{s.PlateNumber, s.ViolationCount, s.LastViolation, highFreq}
		for col, v := range values {
			cell, _ := excelize.CoordinatesToCellName(col+1, rowNum)
			f.SetCellValue(sheet, cell, v)
		}
	}

	filename := fmt.Sprintf("违停统计记录_%s.xlsx", time.Now().Format("20060102_150405"))
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename*=UTF-8''%s`, encodeFilename(filename)))
	if err := f.Write(w); err != nil {
		log.Printf("导出统计记录失败: %v", err)
	}
}

// ---- helpers ----

func parseFilters(r *http.Request) models.QueryFilters {
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	pageSize, _ := strconv.Atoi(q.Get("page_size"))
	warningThreshold, _ := strconv.Atoi(q.Get("warning_threshold"))
	if warningThreshold <= 0 {
		warningThreshold = models.DefaultWarningThreshold
	}
	overThreeWarning := q.Get("over_three_warning") == "1" || strings.EqualFold(q.Get("over_three_warning"), "true")
	return models.QueryFilters{
		PlateKeyword:     q.Get("plate"),
		Status:           q.Get("status"),
		StartDate:        q.Get("start_date"),
		EndDate:          q.Get("end_date"),
		OverThreeWarning: overThreeWarning,
		WarningThreshold: warningThreshold,
		Page:             page,
		PageSize:         pageSize,
	}
}

func jsonOK(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": data})
}

func jsonError(w http.ResponseWriter, msg string, status int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]any{"code": -1, "message": msg})
}

// closeExportFile 关闭导出工作簿并释放其临时文件。
func closeExportFile(f *excelize.File) {
	if err := f.Close(); err != nil {
		log.Printf("清理导出临时文件失败: %v", err)
	}
}

// serviceError 把 service 层的领域错误映射为合适的 HTTP 状态码。
func serviceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrRecordNotFound):
		jsonError(w, err.Error(), http.StatusNotFound)
	case errors.Is(err, service.ErrInvalidTransition):
		jsonError(w, err.Error(), http.StatusBadRequest)
	default:
		jsonError(w, err.Error(), http.StatusInternalServerError)
	}
}

// cleanupRecordFiles 删除记录后，清理不再被其他记录引用的图片文件。
func cleanupRecordFiles(record *models.ParkingRecord) {
	if record == nil {
		return
	}

	paths := []string{record.ImagePath}
	if record.SecondImagePath != nil {
		paths = append(paths, *record.SecondImagePath)
	}

	for _, stored := range paths {
		stored = strings.TrimSpace(stored)
		if stored == "" {
			continue
		}

		filePath, err := resolveImageFilePath(stored)
		if err != nil {
			log.Printf("清理图片跳过(路径无法解析): %s (%v)", stored, err)
			continue
		}
		// 只删除上传目录内的文件，避免误删历史数据中的外部路径。
		if !isInsideDir(uploadDir, filePath) {
			continue
		}

		referenced, err := service.IsImagePathReferenced(stored, record.ID)
		if err != nil {
			log.Printf("清理图片跳过(引用检查失败): %s (%v)", stored, err)
			continue
		}
		if referenced {
			continue
		}

		if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
			log.Printf("清理图片失败: %s (%v)", filePath, err)
		}
	}
}

// isInsideDir 判断 path 是否位于 dir 内（含子目录）。
func isInsideDir(dir, path string) bool {
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func nilStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func encodeFilename(name string) string {
	// RFC 5987 encoding (simple percent-encoding for CJK)
	var sb strings.Builder
	for _, b := range []byte(name) {
		if (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') ||
			b == '-' || b == '_' || b == '.' {
			sb.WriteByte(b)
		} else {
			fmt.Fprintf(&sb, "%%%02X", b)
		}
	}
	return sb.String()
}

func addImageToCell(f *excelize.File, sheet, cell, storedPath string) error {
	storedPath = strings.TrimSpace(storedPath)
	if storedPath == "" {
		return nil
	}

	filePath, err := resolveImageFilePath(storedPath)
	if err != nil {
		return err
	}

	if _, err := os.Stat(filePath); err != nil {
		return fmt.Errorf("图片不存在: %s", storedPath)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("读取图片失败: %s (%v)", storedPath, err)
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	if ext == "" {
		ext = strings.ToLower(filepath.Ext(storedPath))
	}
	if ext == "" {
		ext = ".png"
	}

	// 将图片限制在统一视觉框内，避免大图撑坏导出的布局。
	scale := 0.22
	if cfg, _, err := image.DecodeConfig(bytes.NewReader(data)); err == nil && cfg.Width > 0 && cfg.Height > 0 {
		const maxWidth = 150.0
		const maxHeight = 90.0
		scaleX := maxWidth / float64(cfg.Width)
		scaleY := maxHeight / float64(cfg.Height)
		if scaleX > 1 {
			scaleX = 1
		}
		if scaleY > 1 {
			scaleY = 1
		}
		if scaleX < scaleY {
			scale = scaleX
		} else {
			scale = scaleY
		}
	}

	if err := f.AddPictureFromBytes(sheet, cell, &excelize.Picture{
		Extension: ext,
		File:      data,
		Format: &excelize.GraphicOptions{
			ScaleX:          scale,
			ScaleY:          scale,
			LockAspectRatio: true,
			Positioning:     "oneCell",
			AltText:         filepath.Base(filePath),
		},
	}); err != nil {
		return fmt.Errorf("插入图片失败: %s (%v)", storedPath, err)
	}

	return nil
}

func resolveImageFilePath(storedPath string) (string, error) {
	storedPath = strings.TrimSpace(storedPath)
	if storedPath == "" {
		return "", fmt.Errorf("图片路径为空")
	}

	if strings.HasPrefix(storedPath, "http://") || strings.HasPrefix(storedPath, "https://") {
		u, err := url.Parse(storedPath)
		if err != nil {
			return "", fmt.Errorf("图片地址格式错误: %s", storedPath)
		}
		storedPath = u.Path
	}

	// 先判断前缀再取文件名，避免 "/uploads/../x" 被 Clean 成上传目录之外的路径。
	if strings.HasPrefix(storedPath, "/uploads/") || strings.HasPrefix(storedPath, "uploads/") {
		name := filepath.Base(storedPath)
		if !isValidUploadName(name) {
			return "", fmt.Errorf("图片路径非法: %s", storedPath)
		}
		return filepath.Join(uploadDir, name), nil
	}

	if filepath.IsAbs(storedPath) {
		cleaned := filepath.Clean(storedPath)
		// 只允许读取上传目录内的绝对路径，防止通过 image_path 读取任意文件。
		if !isInsideDir(uploadDir, cleaned) {
			return "", fmt.Errorf("图片路径不在上传目录内: %s", storedPath)
		}
		return cleaned, nil
	}

	// 兜底：历史数据可能只保存了文件名。
	name := filepath.Base(filepath.Clean(storedPath))
	if !isValidUploadName(name) {
		return "", fmt.Errorf("图片路径非法: %s", storedPath)
	}
	return filepath.Join(uploadDir, name), nil
}

// isValidUploadName 判断是否为合法的上传文件名（不含路径成分）
func isValidUploadName(name string) bool {
	return name != "" && name != "." && name != ".." &&
		!strings.ContainsAny(name, `/\`) && name == filepath.Base(name)
}

func runONNXOCR(filePath string) (*models.OCRResult, error) {
	runner, detModel, recModel, err := resolveONNXRuntimeConfig()
	if err != nil {
		return nil, err
	}

	modelsDir := filepath.Dir(detModel)
	imageArg := filePath
	if runtime.GOOS == "windows" {
		modelsDir = filepath.ToSlash(modelsDir)
		imageArg = filepath.ToSlash(imageArg)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, runner,
		"-d", modelsDir,
		"-1", filepath.Base(detModel),
		"-2", filepath.Base(filepath.Join(modelsDir, "license_cls.onnx")),
		"-3", filepath.Base(recModel),
		"-4", "ppocr_keys_v1.txt",
		"-i", imageArg,
		"-t", strconv.Itoa(runtime.NumCPU()),
		"-p", "50",
		"-s", "1024",
		"-b", "0.5",
		"-o", "0.3",
		"-u", "1.6",
		"-a", "1",
		"-A", "1",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("ONNX OCR超时：%s", msg)
		}
		return nil, fmt.Errorf("ONNX OCR执行失败：%s", msg)
	}

	var resp struct {
		RawText     string `json:"raw_text"`
		PlateNumber string `json:"plate_number"`
		ParkingTime string `json:"parking_time"`
		Text        string `json:"text"`
	}
	output := strings.TrimSpace(string(out))
	if err := json.Unmarshal(out, &resp); err != nil {
		rawText := extractRunnerRawText(output)
		// 兼容runner仅输出纯文本的场景。
		return &models.OCRResult{
			PlateNumber: extractPlateNumber(rawText),
			ParkingTime: extractParkingTime(rawText),
			RawText:     rawText,
		}, nil
	}

	rawText := strings.TrimSpace(resp.RawText)
	if rawText == "" {
		rawText = strings.TrimSpace(resp.Text)
	}

	plate := strings.TrimSpace(resp.PlateNumber)
	if plate == "" {
		plate = extractPlateNumber(rawText)
	}
	timeText := strings.TrimSpace(resp.ParkingTime)
	if timeText == "" {
		timeText = extractParkingTime(rawText)
	}

	return &models.OCRResult{
		PlateNumber: plate,
		ParkingTime: timeText,
		RawText:     rawText,
	}, nil
}

func extractRunnerRawText(output string) string {
	lines := strings.Split(strings.ReplaceAll(output, "\r\n", "\n"), "\n")
	collecting := false
	content := make([]string, 0, len(lines))
	for _, line := range lines {
		text := strings.TrimSpace(line)
		if text == "" {
			continue
		}
		if strings.Contains(text, "=====End detect=====") {
			collecting = true
			continue
		}
		if collecting {
			if isRunnerLogLine(text) {
				continue
			}
			content = append(content, text)
		}
	}
	if len(content) > 0 {
		return strings.Join(content, "\n")
	}

	for i := len(lines) - 1; i >= 0; i-- {
		text := strings.TrimSpace(lines[i])
		if text == "" {
			continue
		}
		if !isRunnerLogLine(text) {
			return text
		}
	}
	return ""
}

func isRunnerLogLine(text string) bool {
	switch {
	case strings.HasPrefix(text, "modelsPath="):
		return true
	case strings.HasPrefix(text, "model det path="):
		return true
	case strings.HasPrefix(text, "model cls path="):
		return true
	case strings.HasPrefix(text, "model rec path="):
		return true
	case strings.HasPrefix(text, "keys path="):
		return true
	case strings.HasPrefix(text, "imgDir="):
		return true
	case strings.HasPrefix(text, "resultTxtPath("):
		return true
	case strings.HasPrefix(text, "====="):
		return true
	case strings.HasPrefix(text, "-----"):
		return true
	case strings.HasPrefix(text, "--- "):
		return true
	case strings.HasPrefix(text, "ScaleParam("):
		return true
	case strings.HasPrefix(text, "TextBox["):
		return true
	case strings.HasPrefix(text, "angle["):
		return true
	case strings.HasPrefix(text, "dbNetTime("):
		return true
	case strings.HasPrefix(text, "crnnTime["):
		return true
	case strings.HasPrefix(text, "FullDetectTime("):
		return true
	case strings.HasPrefix(text, "numThread("):
		return true
	case strings.HasPrefix(text, "other option "):
		return true
	case strings.Contains(text, " path("):
		return true
	default:
		return false
	}
}

func resolveONNXRuntimeConfig() (runner string, detModel string, recModel string, err error) {
	exeDir := "."
	if exePath, e := os.Executable(); e == nil {
		exeDir = filepath.Dir(exePath)
	}

	runnerName := "onnx_ocr_runner"
	if runtime.GOOS == "windows" {
		runnerName += ".exe"
	}

	if v := strings.TrimSpace(os.Getenv("COUNTCAR_ONNX_OCR_RUNNER")); v != "" {
		runner = v
	} else {
		candidates := []string{
			filepath.Join(exeDir, "ocr", "bin", runnerName),
			filepath.Join(exeDir, runnerName),
		}
		for _, c := range candidates {
			if _, e := os.Stat(c); e == nil {
				runner = c
				break
			}
		}
		if runner == "" {
			runner = runnerName
		}
	}

	if _, e := exec.LookPath(runner); e != nil {
		return "", "", "", fmt.Errorf("ONNX OCR不可用：未找到runner(%s)。建议放在 %s 或设置 COUNTCAR_ONNX_OCR_RUNNER", runnerName, filepath.Join(exeDir, "ocr", "bin"))
	}

	if v := strings.TrimSpace(os.Getenv("COUNTCAR_ONNX_DET_MODEL")); v != "" {
		detModel = v
	} else {
		detModel = filepath.Join(exeDir, "ocr", "models", "license_det.onnx")
	}

	if v := strings.TrimSpace(os.Getenv("COUNTCAR_ONNX_REC_MODEL")); v != "" {
		recModel = v
	} else {
		recModel = filepath.Join(exeDir, "ocr", "models", "license_rec.onnx")
	}

	if _, e := os.Stat(detModel); e != nil {
		return "", "", "", fmt.Errorf("ONNX OCR缺少检测模型：%s", detModel)
	}
	if _, e := os.Stat(recModel); e != nil {
		return "", "", "", fmt.Errorf("ONNX OCR缺少识别模型：%s", recModel)
	}

	return runner, detModel, recModel, nil
}

// 预编译正则：这些正则在每次 OCR 结果解析时都会用到，避免重复编译。
// 号牌后必须是非字母数字边界，否则会把下一行的日期数字粘进号牌（如「京A12345」+「2026-05-14」）。
var (
	plateNumberRe = regexp.MustCompile(`([京津沪渝冀豫云辽黑湘皖鲁新苏浙赣鄂桂甘晋蒙陕吉闽贵粤青藏川宁琼][A-Z][A-Z0-9]{5,6})(?:[^A-Z0-9]|$)`)
	parkingTimeRe = regexp.MustCompile(`(?m)(20\d{2})[-/年\.](\d{1,2})[-/月\.](\d{1,2})(?:日)?\s+([01]?\d|2[0-3])[:时](\d{1,2})(?:[:分](\d{1,2}))?`)
)

func extractPlateNumber(text string) string {
	cleaned := strings.ReplaceAll(text, "\r", "")
	for _, line := range strings.Split(cleaned, "\n") {
		upper := strings.ToUpper(line)
		// 先按原样匹配：保留空格作为号牌与后续数字的分隔
		if m := plateNumberRe.FindStringSubmatch(upper); len(m) >= 2 {
			return m[1]
		}
		// 再容忍 OCR 在号牌内部插入空格的情况（如「粤 A 12345」）
		if m := plateNumberRe.FindStringSubmatch(strings.ReplaceAll(upper, " ", "")); len(m) >= 2 {
			return m[1]
		}
	}
	return ""
}

func extractParkingTime(text string) string {
	match := parkingTimeRe.FindStringSubmatch(text)
	if len(match) == 0 {
		return ""
	}

	year, _ := strconv.Atoi(match[1])
	month, _ := strconv.Atoi(match[2])
	day, _ := strconv.Atoi(match[3])
	hour, _ := strconv.Atoi(match[4])
	minute, _ := strconv.Atoi(match[5])
	second := 0
	if len(match) >= 7 && match[6] != "" {
		second, _ = strconv.Atoi(match[6])
	}

	ts := time.Date(year, time.Month(month), day, hour, minute, second, 0, time.Local)
	if ts.Year() != year || int(ts.Month()) != month || ts.Day() != day || ts.Hour() != hour || ts.Minute() != minute {
		return ""
	}
	return ts.Format("2006-01-02 15:04:05")
}
