# Makefile for NodeAccessManager

APP_NAME := nam
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "v0.1.0-dev")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS := -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)

.PHONY: all build build-linux build-all clean test fmt lint help install

# 默认目标
all: build

# 构建当前平台
build:
	@echo "Building $(APP_NAME) for current platform..."
	go build -ldflags "$(LDFLAGS)" -o bin/$(APP_NAME) ./cmd/nam

# 构建 Linux 版本（静态链接）
build-linux:
	@echo "Building $(APP_NAME) for Linux (static)..."
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 \
	go build -ldflags "$(LDFLAGS) -extldflags '-static'" \
	-tags 'sqlite_omit_load_extension' \
	-o bin/$(APP_NAME)-linux-amd64 ./cmd/nam

# 构建所有平台
build-all: build-linux
	@echo "Building $(APP_NAME) for all platforms..."
	GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" \
		-o bin/$(APP_NAME)-darwin-amd64 ./cmd/nam
	GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" \
		-o bin/$(APP_NAME)-darwin-arm64 ./cmd/nam
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" \
		-o bin/$(APP_NAME)-windows-amd64.exe ./cmd/nam

# 安装依赖
deps:
	@echo "Downloading dependencies..."
	go mod download
	go mod verify

# 运行测试
test:
	@echo "Running tests..."
	go test -v -race ./...

# 代码格式化
fmt:
	@echo "Formatting code..."
	go fmt ./...

# 静态检查
lint:
	@echo "Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed, skipping..."; \
	fi

# 清理构建产物
clean:
	@echo "Cleaning..."
	rm -rf bin/
	rm -f $(APP_NAME)

# 安装到系统（需要 root）
install: build
	@echo "Installing to /usr/local/bin/..."
	@sudo cp bin/$(APP_NAME) /usr/local/bin/$(APP_NAME)
	@sudo chmod +x /usr/local/bin/$(APP_NAME)
	@echo "Installation complete!"

# 帮助信息
help:
	@echo "NodeAccessManager Makefile"
	@echo ""
	@echo "Available targets:"
	@echo "  make build       - Build for current platform"
	@echo "  make build-linux - Build for Linux (static)"
	@echo "  make build-all   - Build for all platforms"
	@echo "  make deps        - Download dependencies"
	@echo "  make test        - Run tests"
	@echo "  make fmt         - Format code"
	@echo "  make lint        - Run linter"
	@echo "  make clean       - Clean build artifacts"
	@echo "  make install     - Install to system (requires sudo)"
	@echo "  make help        - Show this help"
