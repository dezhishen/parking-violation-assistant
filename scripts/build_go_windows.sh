#!/usr/bin/env bash
# 构建脚本：先构建前端，再构建 Windows Go 二进制（包含嵌入的前端）
set -euo pipefail

echo "=== 构建前端 ==="
cd frontend
npm install
npm run build
cd ..

echo "=== 校验前端产物完整性 ==="
node scripts/verify_frontend_dist.mjs

echo "=== 准备 ONNX OCR 运行时资源 (Windows) ==="
node scripts/prepare_onnx_ocr_assets.mjs --os windows --dist dist

echo "=== 构建 Go 二进制 (Windows) ==="
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o dist/CountCar.exe .

echo "=== 打包发布物 (Windows) ==="
if command -v zip >/dev/null 2>&1; then
  rm -f dist/CountCar-windows-amd64.zip
  zip -r -q dist/CountCar-windows-amd64.zip dist/CountCar.exe dist/ocr
else
  echo "未找到 zip 命令，请安装后重试（例如: apt install zip）"
  exit 1
fi

echo "=== 构建完成 ==="
ls -lh dist/CountCar.exe
ls -lh dist/ocr/bin dist/ocr/models
ls -lh dist/CountCar-windows-amd64.zip
echo "运行: dist/CountCar.exe"
