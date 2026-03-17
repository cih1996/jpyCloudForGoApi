.PHONY: build clean generate dist dev dist-all install uninstall

APP_NAME := jpy-cloud
DIST_DIR := dist
VERSION := $(shell date +%Y%m%d)
INSTALL_PATH := /usr/local/bin/$(APP_NAME)

# 开发构建：编译前端 + 后端，复制静态文件，重启服务
dev:
	@echo "=== 构建前端 ==="
	cd vue-app && npm run build
	@echo "=== 复制静态文件 ==="
	rm -rf static/* $(DIST_DIR)/static/*
	mkdir -p static $(DIST_DIR)/static
	cp -r vue-app/dist/* static/
	cp -r vue-app/dist/* $(DIST_DIR)/static/
	@echo "=== 构建后端 (arm64) ==="
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o $(DIST_DIR)/$(APP_NAME)-darwin-arm64 .
	@echo "=== 重启服务 ==="
	-ai-hub services restart 集控平台 2>/dev/null || echo "服务未运行，跳过重启"
	@echo "=== 完成！访问 http://localhost:1001 ==="

# 仅构建后端
build:
	go build -o dist/$(APP_NAME) .

# 安装到系统（需要 sudo）
install: build
	@echo "=== 安装到 $(INSTALL_PATH) ==="
	sudo cp dist/$(APP_NAME) $(INSTALL_PATH)
	sudo chmod +x $(INSTALL_PATH)
	@echo "=== 安装完成！==="
	@echo "运行 'jpy-cloud help' 查看帮助"
	@echo "运行 'jpy-cloud service install' 安装为系统服务"

# 卸载
uninstall:
	@echo "=== 停止并卸载服务 ==="
	-$(INSTALL_PATH) service stop 2>/dev/null || true
	-$(INSTALL_PATH) service uninstall 2>/dev/null || true
	@echo "=== 删除程序 ==="
	sudo rm -f $(INSTALL_PATH)
	@echo "=== 卸载完成！==="

generate:
	./tools/generate_schema.sh

clean:
	rm -rf dist/
	rm -rf $(DIST_DIR)/

# 完整打包：构建前端 + 多平台后端（单文件，前端嵌入）
dist-all:
	@echo "=== 构建前端 ==="
	cd vue-app && npm run build
	@echo "=== 复制静态文件到 static 目录（用于 embed）==="
	rm -rf static/*
	mkdir -p static
	cp -r vue-app/dist/* static/
	@echo "=== 构建多平台二进制（前端已嵌入）==="
	mkdir -p $(DIST_DIR)
	@echo "Building for Linux (amd64)..."
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o $(DIST_DIR)/$(APP_NAME)-linux-amd64 .
	@echo "Building for Linux (arm64)..."
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o $(DIST_DIR)/$(APP_NAME)-linux-arm64 .
	@echo "Building for macOS (amd64)..."
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o $(DIST_DIR)/$(APP_NAME)-darwin-amd64 .
	@echo "Building for macOS (arm64)..."
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o $(DIST_DIR)/$(APP_NAME)-darwin-arm64 .
	@echo "Building for Windows (amd64)..."
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o $(DIST_DIR)/$(APP_NAME)-windows-amd64.exe .
	@echo ""
	@echo "=== 打包完成！==="
	@ls -lh $(DIST_DIR)/$(APP_NAME)-*
	@echo ""
	@echo "单文件运行，无需 static 目录！"

dist-win:
	mkdir -p $(DIST_DIR)
	@echo "Building for Windows (amd64)..."
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o $(DIST_DIR)/$(APP_NAME)-windows-amd64.exe .

dist:
	mkdir -p $(DIST_DIR)
	@echo "Building for Linux (amd64)..."
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o $(DIST_DIR)/$(APP_NAME)-linux-amd64 .
	@echo "Building for Linux (arm64)..."
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o $(DIST_DIR)/$(APP_NAME)-linux-arm64 .
	@echo "Building for macOS (amd64)..."
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o $(DIST_DIR)/$(APP_NAME)-darwin-amd64 .
	@echo "Building for macOS (arm64)..."
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o $(DIST_DIR)/$(APP_NAME)-darwin-arm64 .
	@echo "Building for Windows (amd64)..."
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o $(DIST_DIR)/$(APP_NAME)-windows-amd64.exe .
	@echo "Done! Artifacts are in $(DIST_DIR)/"
