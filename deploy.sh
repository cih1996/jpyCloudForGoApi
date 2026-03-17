#!/bin/bash
# 集控平台前端部署脚本
# 用法: ./deploy.sh [--restart]

set -e

PROJECT_DIR="/Users/cih1996/work/jpy-zyd/ts-sdk/go-port-trans"
VUE_APP_DIR="$PROJECT_DIR/vue-app"
STATIC_DIR="$PROJECT_DIR/static"

echo "📦 编译前端..."
cd "$VUE_APP_DIR"
npm run build

echo "🗑️  清理旧文件..."
rm -rf "$STATIC_DIR/assets"
rm -f "$STATIC_DIR/index.html"

echo "📋 复制新文件..."
cp -r "$VUE_APP_DIR/dist/"* "$STATIC_DIR/"

echo "✅ 部署完成!"

# 如果传入 --restart 参数，重启服务
if [ "$1" = "--restart" ]; then
    echo "🔄 重启服务..."
    ai-hub services restart 10
fi

echo "🎉 完成!"
