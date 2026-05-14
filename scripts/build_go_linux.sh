#!/bin/bash
# 构建脚本：先构建前端，再构建 Go 二进制（包含嵌入的前端）
set -e

echo "=== 构建前端 ==="
cd frontend
npm install
npm run build
cd ..

echo "=== 校验前端产物完整性 ==="
node scripts/verify_frontend_dist.mjs

echo "=== 准备 ONNX OCR 运行时资源 ==="
node scripts/prepare_onnx_ocr_assets.mjs --os linux --dist dist

echo "=== 构建 Go 二进制 (Linux) ==="
go build -ldflags="-s -w" -o dist/CountCar .

echo "=== 打包发布物 (Linux) ==="
tar -czf dist/CountCar-linux-amd64.tar.gz -C dist CountCar ocr

echo "=== 构建完成 ==="
ls -lh dist/CountCar
ls -lh dist/ocr/bin dist/ocr/models
ls -lh dist/CountCar-linux-amd64.tar.gz
echo "运行: ./dist/CountCar"
