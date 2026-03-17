#!/bin/bash
# 前端编译部署脚本 - 包含 Go 重新编译（因为静态文件是 embed 进二进制的）

set -e

cd "$(dirname "$0")"

echo "🔨 编译前端..."
cd vue-app && npm run build
cd ..

echo "🗑️  清理旧静态文件..."
rm -rf static/* dist/static/*
mkdir -p static dist/static

echo "📦 复制静态文件..."
cp -r vue-app/dist/* static/
cp -r vue-app/dist/* dist/static/

echo "🔧 重新编译 Go 后端（嵌入新前端）..."
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o dist/jpy-cloud-darwin-arm64 .

echo "🔄 重启服务..."
ai-hub services restart "集控平台" 2>/dev/null || echo "服务未运行，跳过重启"

echo "✅ 部署完成！请用 Ctrl+Shift+R 强制刷新浏览器"
