# 停车违停助手 —— 开发 / 构建 / 校验入口
#
# 常用：
#   make dev         启动开发模式（后端 Go + 前端 Vite），日志统一输出到当前控制台
#   make dev-stop    停止开发模式进程
#   make build       完整生产构建（前端 + ONNX 资源 + 后端 + 打包）
#   make help        查看全部命令

SHELL := /bin/bash
.DEFAULT_GOAL := help

GO   ?= go
NPM  ?= npm
NODE ?= node

DIST_DIR    := dist
DEV_DIR     := .dev
BACKEND_BIN := $(DEV_DIR)/backend
# go:embed 要求 frontend/dist 目录必须存在，缺失时生成占位页面
EMBED_STUB  := frontend/dist/index.html

# 开发模式端口（Vite 会把 /api、/uploads 代理到后端端口）
BACKEND_PORT  ?= 8080
FRONTEND_PORT ?= 5173

.PHONY: help dev dev-stop dev-status restart \
        build build-linux build-windows build-frontend build-backend run \
        test vet fmt tidy clean clean-dev

help: ## 显示所有可用命令
	@echo "停车违停助手 —— 可用命令："
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z0-9_.-]+:.*## / {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)
	@echo
	@echo "开发模式端口可通过 BACKEND_PORT / FRONTEND_PORT 覆盖，例如："
	@echo "  make dev BACKEND_PORT=9090 FRONTEND_PORT=5174"

# ---------------- 开发 ----------------

dev: ## 启动开发模式（后端 + 前端），日志输出到当前控制台
	@BACKEND_PORT=$(BACKEND_PORT) FRONTEND_PORT=$(FRONTEND_PORT) ./scripts/dev.sh start

dev-stop: ## 停止 make dev 启动的进程
	@./scripts/dev.sh stop

dev-status: ## 查看开发模式进程状态
	@./scripts/dev.sh status

restart: dev-stop ## 重启开发模式
	@BACKEND_PORT=$(BACKEND_PORT) FRONTEND_PORT=$(FRONTEND_PORT) ./scripts/dev.sh start

# ---------------- 构建 ----------------

build: build-linux ## 默认完整生产构建（Linux）

build-linux: ## 完整生产构建：前端 + 校验 + ONNX 资源 + 后端 + 打包（Linux）
	@./scripts/build_go_linux.sh

build-windows: ## 完整生产构建：前端 + 校验 + ONNX 资源 + 后端 + 打包（Windows）
	@./scripts/build_go_windows.sh

build-frontend: frontend/node_modules ## 构建前端产物到 frontend/dist
	@cd frontend && $(NPM) run build
	@$(NODE) scripts/verify_frontend_dist.mjs

build-backend: mkdist $(EMBED_STUB) ## 仅构建后端二进制（使用当前 frontend/dist）
	@if [ ! -d frontend/dist/assets ]; then \
		echo "警告：frontend/dist 缺少 assets，可能是占位内容，建议先执行 make build-frontend"; \
	fi
	$(GO) build -ldflags="-s -w" -o $(DIST_DIR)/CountCar .

run: mkdev $(EMBED_STUB) ## 本地运行后端（数据落在 .dev/data）
	$(GO) build -o $(BACKEND_BIN) .
	./$(BACKEND_BIN)

mkdist:
	@mkdir -p $(DIST_DIR)

mkdev:
	@mkdir -p $(DEV_DIR)

frontend/node_modules: frontend/package.json frontend/package-lock.json
	@cd frontend && $(NPM) install --no-audit --no-fund

$(EMBED_STUB):
	@mkdir -p $(dir $(EMBED_STUB))
	@printf '%s\n' \
		'<!doctype html>' \
		'<html lang="zh-CN"><head><meta charset="utf-8"><title>停车违停助手</title></head>' \
		'<body>前端产物尚未构建，请执行 <code>make build-frontend</code>。</body></html>' \
		> $(EMBED_STUB)
	@echo "已生成占位 $(EMBED_STUB)（go:embed 需要 frontend/dist 存在）"

# ---------------- 校验 ----------------

test: ## 运行 Go 单元测试
	$(GO) test ./...

vet: ## 运行 go vet 静态检查
	$(GO) vet ./...

fmt: ## 格式化 Go 代码
	$(GO) fmt ./...

tidy: ## 整理 go.mod / go.sum
	$(GO) mod tidy

# ---------------- 清理 ----------------

clean: dev-stop ## 清理构建产物（dist、前端产物、.dev 二进制）
	rm -rf $(DIST_DIR) frontend/dist $(BACKEND_BIN) $(BACKEND_BIN).exe

clean-dev: dev-stop ## 停止开发进程并删除 .dev 目录（含本地测试数据）
	rm -rf $(DEV_DIR)
