# 停车违停助手（Go + Vue）

这是一个仅依赖 Go + Vue 的停车违停助手。

- 后端使用 Go 提供 API 与文件服务。
- 前端使用 Vue 3 + Element Plus。
- 上传后由后端 OCR 自动识别车牌与时间，无需手动框选。
- 前端构建产物会被嵌入 Go 二进制，最终可发布为单文件程序。

## 项目标识

- Go Module: `github.com/dezhishen/parking-violation-assistant`
- 推荐仓库地址: `github.com/dezhishen/parking-violation-assistant`
- 前端包名: `parking-violation-assistant-frontend`

## 主要功能

1. 上传违停照片并创建记录。
2. 按车牌、时间范围查询违停数据。
3. 主界面按车牌聚合展示：最后违停时间、违停次数。
4. 高频违停车牌（3 次及以上）高亮。
5. 记录状态流转：待处理 -> 已提醒 -> 待确认移车 -> 违停 / 已移车。
6. 导出 Excel：详细记录、统计记录两类模板。

## 技术栈

- Go（标准库 net/http）
- SQLite（modernc.org/sqlite，纯 Go，无 CGO）
- Vue 3 + Vite
- Element Plus
- ONNX OCR Runner（后端离线 OCR）

## 目录结构

```text
.
├─ frontend/                 # Vue 前端
├─ internal/
│  ├─ api/                   # HTTP handlers
│  ├─ db/                    # SQLite 初始化与迁移
│  ├─ models/                # 数据模型
│  └─ service/               # 业务逻辑
├─ scripts/
│  ├─ build_go_linux.sh
│  └─ build_go_windows.sh
└─ main.go                   # 应用入口（嵌入 frontend/dist）
```

## 开发环境要求

- Go 1.25+
- Node.js 20+

## 本地开发

1. 安装前端依赖

```bash
cd frontend
npm install
```

2. 前端开发模式

```bash
npm run dev
```

3. 后端开发运行

```bash
cd ..
go run .
```

启动后程序会自动尝试打开浏览器。

可选：指定固定端口启动。

```bash
go run . -port 8080
```

或使用环境变量：

```bash
COUNTCAR_PORT=8080 go run .
```

说明：

- 传入 `-port` 时，后端监听固定端口。
- 未传入 `-port` 且未设置 `COUNTCAR_PORT` 时，后端会随机选择一个可用端口（当前默认模式）。

## 生产构建（单文件）

Linux:

```bash
./scripts/build_go_linux.sh
```

Windows:

```bash
./scripts/build_go_windows.sh
```

构建顺序：

1. 先执行前端构建，生成 frontend/dist
2. 自动准备 ONNX OCR 运行时资源（runner + 推荐模型）到 dist/ocr
3. 再执行 go build，将 frontend/dist 嵌入最终可执行文件

输出：

- Linux: dist/CountCar
- Windows: dist/CountCar.exe
- ONNX OCR 资源：dist/ocr/bin + dist/ocr/models
- Linux 压缩包：dist/CountCar-linux-amd64.tar.gz
- Windows 压缩包：dist/CountCar-windows-amd64.zip

## VS Code 调试

仓库已提供调试配置：

- .vscode/launch.json
- .vscode/tasks.json

可直接使用以下调试项：

- Go: Debug Backend
- Frontend: Vite Dev (Terminal)
- Frontend: Open Browser
- Full Stack: Go + Vue（复合调试）

## CI 构建

GitHub Actions 工作流：

- .github/workflows/build-go.yml

会自动构建：

- Windows 可执行文件
- Linux 可执行文件

## 数据文件

- data/records.db：SQLite 数据库
- data/uploads/：上传图片目录

## OCR（后端 ONNX）

后端提供 `/api/ocr` 接口，前端上传图片后直接调用该接口执行离线识别。

`runONNXOCR` 默认按以下推荐路径开箱即用（无需环境变量）：

- runner：`<可执行文件目录>/ocr/bin/onnx_ocr_runner`（Windows 为 `.exe`）
- 检测模型：`<可执行文件目录>/ocr/models/license_det.onnx`
- 识别模型：`<可执行文件目录>/ocr/models/license_rec.onnx`

构建时会自动下载并准备以上资源（Linux/Windows 脚本、VS Code `backend: build`、CI 均已接入）。

构建前置校验：

- 本地脚本、VS Code `backend: build` 任务、CI 工作流都会在 `go build` 前执行 `scripts/verify_frontend_dist.mjs`。
- 前端关键构建产物缺失时，后端构建会直接失败，避免输出不完整二进制。

默认来源：

- runner：RapidAI/RapidOcrOnnx `1.2.2` release
- 推荐模型：RapidOCR `PP-OCRv4` mobile det/rec（ONNX）

可通过以下环境变量覆盖下载来源：

- `COUNTCAR_OCR_RUNNER_TAG`：runner release tag（默认 `1.2.2`）
- `COUNTCAR_OCR_RUNNER_URL`：runner 压缩包直链
- `COUNTCAR_OCR_RUNNER_ARCHIVE_PATH`：runner 在压缩包内的路径
- `COUNTCAR_OCR_DET_MODEL_URL` / `COUNTCAR_OCR_REC_MODEL_URL`：模型下载地址
- `COUNTCAR_OCR_DET_MODEL_SHA256` / `COUNTCAR_OCR_REC_MODEL_SHA256`：模型校验值

启动前需配置以下环境变量：

- `COUNTCAR_ONNX_OCR_RUNNER`：覆盖 runner 路径
- `COUNTCAR_ONNX_DET_MODEL`：覆盖检测模型路径
- `COUNTCAR_ONNX_REC_MODEL`：覆盖识别模型路径

示例：

```bash
COUNTCAR_ONNX_OCR_RUNNER=/opt/ocr/onnx_ocr_runner \
COUNTCAR_ONNX_DET_MODEL=/opt/ocr/models/license_det.onnx \
COUNTCAR_ONNX_REC_MODEL=/opt/ocr/models/license_rec.onnx \
./dist/CountCar
```

runner 建议输出 JSON，字段如下（最少支持 `raw_text` 或 `text`）：

```json
{
	"raw_text": "京A12345 2026-05-14 10:22:00",
	"plate_number": "京A12345",
	"parking_time": "2026-05-14 10:22:00"
}
```

## 许可证

本项目使用 MIT License，详见 `LICENSE`。
