.PHONY: build clean generate dist

APP_NAME := jpy-server
DIST_DIR := dist

build:
	go build -o dist/$(APP_NAME) .

generate:
	./tools/generate_schema.sh

clean:
	rm -rf dist/
	rm -rf $(DIST_DIR)/

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
