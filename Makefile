# ==============================================================================
# fqstory - 番茄小说终端摸鱼阅读器 Makefile
# ==============================================================================

BINARY_NAME   := fqstory
MAIN_SRC      := ./cmd/fqstory
BIN_DIR       := bin
VERSION       := $(shell git describe --tags --always --dirty 2>/dev/null || echo "v1.0.0")
COMMIT        := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
BUILD_TIME    := $(shell date "+%Y-%m-%d_%H:%M:%S")

LDFLAGS       := -s -w \
                 -X 'main.Version=$(VERSION)' \
                 -X 'main.Commit=$(COMMIT)' \
                 -X 'main.BuildTime=$(BUILD_TIME)'

# Go parameters
GOCMD         := go
GOBUILD       := CGO_ENABLED=0 $(GOCMD) build
GOTEST        := $(GOCMD) test
GOMOD         := $(GOCMD) mod
GOFMT         := $(GOCMD) fmt
GOVET         := $(GOCMD) vet

# Colors for terminal output
GREEN         := $(shell printf '\033[32m')
CYAN          := $(shell printf '\033[36m')
YELLOW        := $(shell printf '\033[33m')
RESET         := $(shell printf '\033[0m')

.PHONY: all help build run run-book install clean test test-cover tidy fmt vet build-all

all: build

## help: 显示所有可用命令说明
help:
	@printf "\n  $(GREEN)🍅 fqstory - 番茄小说终端摸鱼阅读器$(RESET)\n\n"
	@printf "  $(CYAN)Usage:$(RESET)\n"
	@printf "    make $(YELLOW)<target>$(RESET)\n\n"
	@printf "  $(CYAN)Targets:$(RESET)\n"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/   /'
	@printf "\n"

## build: 编译当前平台的二进制文件至 bin/fqstory
build:
	@printf "$(CYAN)🔨 正在构建 $(BINARY_NAME) ($(VERSION))...$(RESET)\n"
	@mkdir -p $(BIN_DIR)
	$(GOBUILD) -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY_NAME) $(MAIN_SRC)
	@printf "$(GREEN)✅ 构建成功: $(BIN_DIR)/$(BINARY_NAME)$(RESET)\n"

## run: 直接以源码模式启动 TUI 书架
run:
	$(GOCMD) run $(MAIN_SRC)

## run-book: 直接启动指定小说 (例: make run-book BOOK=6883748331202284558)
run-book:
	@if [ -z "$(BOOK)" ]; then \
		echo "$(YELLOW)⚠️ 请指定 BOOK 参数，例如: make run-book BOOK=6883748331202284558$(RESET)"; \
	else \
		$(GOCMD) run $(MAIN_SRC) -book $(BOOK); \
	fi

## install: 安装二进制至系统 GOPATH/bin (可全局运行 fqstory)
install:
	@echo "$(CYAN)📦 正在安装至 \$$GOPATH/bin...$(RESET)"
	$(GOBUILD) -ldflags "$(LDFLAGS)" -o $(shell $(GOCMD) env GOPATH)/bin/$(BINARY_NAME) $(MAIN_SRC)
	@echo "$(GREEN)✅ 安装成功！可以在任意终端直接输入 $(BINARY_NAME) 启动。$(RESET)"

## test: 运行所有单元测试与集成测试
test:
	@echo "$(CYAN)🧪 正在运行测试...$(RESET)"
	$(GOTEST) -v ./...

## test-cover: 运行测试并生成代码覆盖率报告
test-cover:
	@mkdir -p coverage
	$(GOTEST) -coverprofile=coverage/cover.out ./...
	$(GOCMD) tool cover -func=coverage/cover.out
	@echo "$(GREEN)📊 报告生成完成: coverage/cover.out$(RESET)"

## tidy: 整理并下载 go.mod 依赖
tidy:
	$(GOMOD) tidy

## fmt: 自动格式化 Go 代码
fmt:
	$(GOFMT) ./...

## vet: 运行 Go 静态代码语法检查
vet:
	$(GOVET) ./...

## clean: 清理编译生成的目标目录与临时文件
clean:
	@echo "$(YELLOW)🧹 正在清理编译产物...$(RESET)"
	@rm -rf $(BIN_DIR) coverage
	@echo "$(GREEN)✅ 清理完成$(RESET)"

## build-all: 纯 Go 免 CGO 跨平台交叉编译 (Linux, macOS, Windows)
build-all: clean
	@mkdir -p $(BIN_DIR)
	@echo "$(CYAN)🚀 正在交叉编译多平台架构...$(RESET)"
	@echo "  -> Darwin ARM64 (macOS Apple Silicon)..."
	@GOOS=darwin GOARCH=arm64 $(GOBUILD) -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY_NAME)-darwin-arm64 $(MAIN_SRC)
	@echo "  -> Darwin AMD64 (macOS Intel)..."
	@GOOS=darwin GOARCH=amd64 $(GOBUILD) -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY_NAME)-darwin-amd64 $(MAIN_SRC)
	@echo "  -> Linux AMD64..."
	@GOOS=linux GOARCH=amd64 $(GOBUILD) -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY_NAME)-linux-amd64 $(MAIN_SRC)
	@echo "  -> Linux ARM64..."
	@GOOS=linux GOARCH=arm64 $(GOBUILD) -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY_NAME)-linux-arm64 $(MAIN_SRC)
	@echo "  -> Windows AMD64..."
	@GOOS=windows GOARCH=amd64 $(GOBUILD) -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY_NAME)-windows-amd64.exe $(MAIN_SRC)
	@echo "$(GREEN)🎉 跨平台编译完成！产物位于 bin/ 目录下：$(RESET)"
	@ls -lh $(BIN_DIR)
