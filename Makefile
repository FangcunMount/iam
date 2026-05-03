# ============================================================================
# Makefile for IAM
# ============================================================================
# 项目：iam - IAM 身份与访问管理系统
# 架构：六边形架构 + DDD + CQRS
# ============================================================================

.DEFAULT_GOAL := help

# ============================================================================
# 变量定义
# ============================================================================

# 项目信息
PROJECT_NAME := iam
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "v0.0.0-dev")
BUILD_TIME ?= $(shell date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GIT_BRANCH ?= $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown")

# Docker 镜像信息（可通过环境变量覆盖）
DOCKER_REGISTRY ?= ghcr.io
DOCKER_REPOSITORY ?= fangcunmount
DOCKER_IMAGE := $(DOCKER_REGISTRY)/$(DOCKER_REPOSITORY)/$(PROJECT_NAME)
DOCKER_TAG ?= latest
RUN_UID ?= 2000
RUN_GID ?= 2000
WWW_UID ?= $(RUN_UID)
WWW_GID ?= $(RUN_GID)
LOG_DIR_HOST ?= /data/logs/iam
TLS_CERT_HOST ?= /data/ssl/certs/fangcunmount.cn.crt
TLS_KEY_HOST ?= /data/ssl/private/fangcunmount.cn.key
TLS_CERT_DEST ?= /etc/iam/ssl/fangcunmount.cn.crt
TLS_KEY_DEST ?= /etc/iam/ssl/fangcunmount.cn.key
DOCKER_NETWORK ?= infra-network
SERVICE ?= apiserver
DEPLOY_REF ?= $(GIT_BRANCH)
DEPLOY_SHA ?= $(GIT_COMMIT)
IMAGE_TAG ?= $(DEPLOY_SHA)

# Go 相关
GO := env -u GOROOT go
GO_BUILD := $(GO) build
GO_TEST := $(GO) test
GO_LDFLAGS := -ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.GitCommit=$(GIT_COMMIT)"

# 目录结构
BIN_DIR := bin
TMP_DIR := tmp
PID_DIR := $(TMP_DIR)/pids
LOG_DIR := logs
COVERAGE_DIR := coverage
SPECTRAL_IMAGE ?= stoplight/spectral:latest
CD_SCRIPT_DIR := scripts/cd

# 服务配置
APISERVER_BIN := $(BIN_DIR)/apiserver
APISERVER_CONFIG := configs/apiserver.prod.yaml
APISERVER_DEV_CONFIG := configs/apiserver.dev.yaml
APISERVER_PORT := 8080
APISERVER_SSL_PORT := 8443

# 颜色输出
COLOR_RESET := \033[0m
COLOR_BOLD := \033[1m
COLOR_GREEN := \033[32m
COLOR_YELLOW := \033[33m
COLOR_BLUE := \033[34m
COLOR_CYAN := \033[36m
COLOR_RED := \033[31m

# ============================================================================
# .PHONY 声明
# ============================================================================

.PHONY: help version debug
.PHONY: build build-apiserver clean
.PHONY: run run-apiserver stop stop-apiserver restart restart-apiserver
.PHONY: status status-apiserver logs logs-apiserver health health-check
.PHONY: dev dev-apiserver dev-stop dev-status dev-logs
.PHONY: test test-unit test-coverage test-race test-bench
.PHONY: lint fmt fmt-check
.PHONY: deps deps-download deps-tidy deps-verify deps-update deps-update-all deps-check
.PHONY: proto proto-gen
.PHONY: install install-tools create-dirs
.PHONY: up down re st log
.PHONY: api-validate docs-hygiene
.PHONY: db-bootstrap db-connect db-status db-backup
.PHONY: docker-mysql-up docker-mysql-down docker-mysql-clean docker-mysql-logs
.PHONY: cert-gen cert-test cert-verify test-dev-config
.PHONY: grpc-cert-verify grpc-cert-info
.PHONY: docker-dev-up docker-dev-down docker-dev-restart docker-dev-logs docker-dev-clean
.PHONY: docker-compose-build docker-compose-up docker-compose-down docker-compose-restart docker-compose-logs
.PHONY: deploy deploy-local deploy-prod deploy-nginx deploy-systemd
.PHONY: cd-validate cd-image cd-package cd-remote-deploy

# ============================================================================
# 帮助信息
# ============================================================================

help: ## 显示帮助信息
	@echo "$(COLOR_BOLD)$(COLOR_CYAN)======================================"
	@echo "  IAM - 构建和管理工具"
	@echo "======================================$(COLOR_RESET)"
	@echo ""
	@echo "$(COLOR_BOLD)项目信息:$(COLOR_RESET)"
	@echo "  版本:     $(COLOR_GREEN)$(VERSION)$(COLOR_RESET)"
	@echo "  分支:     $(COLOR_GREEN)$(GIT_BRANCH)$(COLOR_RESET)"
	@echo "  提交:     $(COLOR_GREEN)$(GIT_COMMIT)$(COLOR_RESET)"
	@echo ""
	@echo "$(COLOR_BOLD)📦 构建命令:$(COLOR_RESET)"
	@grep -E '^build.*:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(COLOR_CYAN)%-20s$(COLOR_RESET) %s\n", $$1, $$2}'
	@echo ""
	@echo "$(COLOR_BOLD)🚀 服务管理:$(COLOR_RESET)"
	@grep -E '^(run|start|stop|restart|status|logs|health).*:.*?## .*$$' $(MAKEFILE_LIST) | grep -v "dev" | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(COLOR_CYAN)%-20s$(COLOR_RESET) %s\n", $$1, $$2}'
	@echo ""
	@echo "$(COLOR_BOLD)🛠️  开发工具:$(COLOR_RESET)"
	@grep -E '^(dev|test|lint|fmt|cert|api-validate|docs-hygiene).*:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(COLOR_CYAN)%-20s$(COLOR_RESET) %s\n", $$1, $$2}'
	@echo ""
	@echo "$(COLOR_BOLD)🗄️  数据库管理:$(COLOR_RESET)"
	@grep -E '^(db-|docker-mysql-).*:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(COLOR_CYAN)%-20s$(COLOR_RESET) %s\n", $$1, $$2}'
	@echo ""
	@echo "$(COLOR_BOLD)🐳 Docker 开发环境:$(COLOR_RESET)"
	@grep -E '^docker-dev-.*:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(COLOR_CYAN)%-20s$(COLOR_RESET) %s\n", $$1, $$2}'
	@echo ""
	@echo "$(COLOR_BOLD)🐳 Docker 生产部署:$(COLOR_RESET)"
	@grep -E '^docker-compose-.*:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(COLOR_CYAN)%-20s$(COLOR_RESET) %s\n", $$1, $$2}'
	@echo ""
	@echo "$(COLOR_BOLD)🚢 CD 发布入口:$(COLOR_RESET)"
	@grep -E '^cd-.*:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(COLOR_CYAN)%-20s$(COLOR_RESET) %s\n", $$1, $$2}'
	@echo ""
	@echo "$(COLOR_BOLD)📚 其他命令:$(COLOR_RESET)"
	@grep -E '^(deps|proto|install|clean|version|debug|up|down|st).*:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(COLOR_CYAN)%-20s$(COLOR_RESET) %s\n", $$1, $$2}'
	@echo ""

api-validate: ## Lint OpenAPI (spectral) + compare swagger vs api/rest
	./scripts/validate-openapi.sh

docs-hygiene: ## Check active Markdown links and retired documentation references
	python scripts/check-docs-links.py

docs-swagger: ## Regenerate swagger (internal/apiserver/docs)
	swag init -g cmd/apiserver/apiserver.go -o internal/apiserver/docs --parseDependency --parseInternal

docs-reset: ## Reset api/rest paths from swagger (split by prefix rules)
	python scripts/reset-openapi-from-swagger.py

version: ## 显示版本信息
	@echo "$(COLOR_BOLD)版本信息:$(COLOR_RESET)"
	@echo "  版本:     $(COLOR_GREEN)$(VERSION)$(COLOR_RESET)"
	@echo "  构建时间: $(BUILD_TIME)"
	@echo "  Git 提交: $(GIT_COMMIT)"
	@echo "  Git 分支: $(GIT_BRANCH)"
	@echo "  Go 版本:  $(shell $(GO) version)"

# ============================================================================
# 构建命令
# ============================================================================

build: build-apiserver ## 构建所有服务

build-apiserver: ## 构建 API 服务器
	@echo "$(COLOR_BOLD)$(COLOR_BLUE)🔨 构建 API 服务器...$(COLOR_RESET)"
	@$(MAKE) create-dirs
	@$(GO_BUILD) $(GO_LDFLAGS) -o $(APISERVER_BIN) ./cmd/apiserver/
	@echo "$(COLOR_GREEN)✅ API 服务器构建完成: $(APISERVER_BIN)$(COLOR_RESET)"

# =============================================================================
# 服务运行管理
# =============================================================================

run: run-apiserver ## 启动服务

stop: stop-apiserver ## 停止服务

restart: restart-apiserver ## 重启服务

status: status-apiserver ## 查看服务状态

logs: logs-apiserver ## 查看服务日志

health: health-check ## 健康检查

run-apiserver: ## 启动 API 服务器
	@echo "🚀 启动 iam..."
	@$(MAKE) create-dirs
	@if [ -f $(PID_DIR)/apiserver.pid ]; then \
			echo "⚠️  iam 可能已在运行 (PID: $$(cat $(PID_DIR)/apiserver.pid))"; \
		if ! kill -0 $$(cat $(PID_DIR)/apiserver.pid) 2>/dev/null; then \
			echo "🧹 清理无效的 PID 文件"; \
			rm -f $(PID_DIR)/apiserver.pid; \
		else \
			echo "❌ iam 已在运行，请先停止"; \
			exit 1; \
		fi; \
	fi
	@nohup ./$(APISERVER_BIN) --config=$(APISERVER_CONFIG) > $(LOG_DIR)/apiserver.log 2>&1 & echo $$! > $(PID_DIR)/apiserver.pid
	@echo "✅ iam 已启动 (PID: $$(cat $(PID_DIR)/apiserver.pid))"

# =============================================================================
# 服务停止管理
# =============================================================================

stop-apiserver: ## 停止 API 服务器
	@echo "⏹️  停止 iam..."
	@if [ -f $(PID_DIR)/apiserver.pid ]; then \
		PID=$$(cat $(PID_DIR)/apiserver.pid); \
		if kill -0 $$PID 2>/dev/null; then \
			kill $$PID && echo "✅ iam 已停止 (PID: $$PID)"; \
			rm -f $(PID_DIR)/apiserver.pid; \
		else \
			echo "⚠️  iam 进程不存在，清理 PID 文件"; \
			rm -f $(PID_DIR)/apiserver.pid; \
		fi; \
	else \
			echo "ℹ️  iam 未运行"; \
	fi

# =============================================================================
# 服务重启管理
# =============================================================================

restart-apiserver: ## 重启 API 服务器
	@echo "🔄 重启 iam..."
	@$(MAKE) stop-apiserver
	@sleep 1
	@$(MAKE) run-apiserver

# =============================================================================
# 服务状态和日志
# =============================================================================

status-apiserver: ## 查看 API 服务器状态
	@if [ -f $(PID_DIR)/apiserver.pid ]; then \
		PID=$$(cat $(PID_DIR)/apiserver.pid); \
		if kill -0 $$PID 2>/dev/null; then \
			echo "✅ iam      - 运行中 (PID: $$PID, Port: $(APISERVER_PORT))"; \
		else \
			echo "❌ iam      - 已停止 (PID 文件存在但进程不存在)"; \
		fi; \
	else \
			echo "⚪ iam      - 未运行"; \
	fi

logs-apiserver: ## 查看 API 服务器日志
	@echo "📋 查看 iam 日志..."
	@tail -f $(LOG_DIR)/apiserver.log

# =============================================================================
# 健康检查
# =============================================================================

health-check: ## 检查所有服务健康状态
	@echo "🔍 健康检查:"
	@echo "============"
	@echo -n "HTTP  ($(APISERVER_PORT)):  "; curl -s http://localhost:$(APISERVER_PORT)/healthz || echo "❌ 无响应"
	@echo -n "HTTPS ($(APISERVER_SSL_PORT)): "; curl -s -k https://localhost:$(APISERVER_SSL_PORT)/healthz || echo "❌ 无响应"

# =============================================================================
# 测试工具
# =============================================================================

test-message-queue: ## 测试消息队列系统
	@echo "📨 测试消息队列系统..."
	@if [ ! -x "./test-message-queue.sh" ]; then \
		echo "❌ 测试脚本不存在或不可执行"; \
		exit 1; \
	fi
	@./test-message-queue.sh

test-submit: ## 测试答卷提交
	@echo "📝 测试答卷提交..."
	@if [ ! -x "./test-answersheet-submit.sh" ]; then \
		echo "❌ 测试脚本不存在或不可执行"; \
		exit 1; \
	fi
	@./test-answersheet-submit.sh

# =============================================================================
# 开发工具
# =============================================================================

dev: ## 启动开发环境（热更新）
	@echo "🚀 启动开发环境..."
	@mkdir -p tmp/pids
	@echo "启动 iam..."
	@air -c .air-apiserver.toml & echo $$! > tmp/pids/air-apiserver.pid
	@sleep 2
	@echo "✅ 所有服务已启动（热更新模式）"
	@echo "提示：使用 Ctrl+C 停止所有服务"
	@echo "      或使用 make dev-stop 停止服务"

dev-apiserver: ## 独立启动 API 服务器（热更新）
	@echo "🚀 启动 apiserver 开发环境..."
	@mkdir -p tmp/pids
	@air -c .air-apiserver.toml

dev-stop: ## 停止开发环境
	@echo "⏹️  停止开发环境..."
	@if [ -f tmp/pids/air-apiserver.pid ]; then \
		kill $$(cat tmp/pids/air-apiserver.pid) 2>/dev/null || true; \
		rm -f tmp/pids/air-apiserver.pid; \
	fi
	@echo "✅ 开发环境已停止"

dev-status: ## 查看开发环境状态
	@echo "📊 开发环境状态:"
	@echo "=============="
	@if [ -f tmp/pids/air-apiserver.pid ] && kill -0 $$(cat tmp/pids/air-apiserver.pid) 2>/dev/null; then \
			echo "✅ iam      - 运行中 (PID: $$(cat tmp/pids/air-apiserver.pid))"; \
	else \
			echo "⚪ iam      - 未运行"; \
	fi

dev-logs: ## 查看开发环境日志
	@echo "📋 开发环境日志:"
	@echo "=============="
	@tail -f tmp/build-errors-*.log

# =============================================================================
# 证书管理（开发/测试环境）
# =============================================================================

cert-gen: ## 生成开发环境自签名证书
	@echo "$(COLOR_CYAN)🔐 生成开发环境证书...$(COLOR_RESET)"
	@chmod +x scripts/cert/generate-dev-cert.sh
	@./scripts/cert/generate-dev-cert.sh

cert-test: ## 测试证书配置
	@echo "$(COLOR_CYAN)🧪 测试证书配置...$(COLOR_RESET)"
	@chmod +x scripts/cert/test-cert.sh
	@./scripts/cert/test-cert.sh

cert-verify: ## 验证证书文件
	@echo "$(COLOR_CYAN)🔍 验证证书文件...$(COLOR_RESET)"
	@if [ -f configs/cert/web-apiserver.crt ]; then \
		openssl x509 -in configs/cert/web-apiserver.crt -noout -text | grep -E "(Subject:|Issuer:|Not Before|Not After|DNS:)"; \
		echo "$(COLOR_GREEN)✅ 证书文件有效$(COLOR_RESET)"; \
	else \
		echo "$(COLOR_RED)❌ 证书文件不存在，请运行: make cert-gen$(COLOR_RESET)"; \
		exit 1; \
	fi

# =============================================================================
# gRPC mTLS 证书管理
# =============================================================================

grpc-cert-verify: ## 验证 gRPC mTLS 证书（infra 统一管理）
	@echo "$(COLOR_CYAN)🔍 验证 gRPC mTLS 证书...$(COLOR_RESET)"
	@if [ ! -f /data/infra/ssl/grpc/ca/ca-chain.crt ]; then \
		echo "$(COLOR_RED)❌ CA 证书不存在: /data/infra/ssl/grpc/ca/ca-chain.crt$(COLOR_RESET)"; \
		echo "$(COLOR_YELLOW)请先在 infra 项目中运行: ./scripts/cert/generate-grpc-certs.sh generate-ca$(COLOR_RESET)"; \
		exit 1; \
	fi
	@if [ ! -f /data/infra/ssl/grpc/server/iam-apiserver.crt ]; then \
		echo "$(COLOR_RED)❌ IAM 证书不存在: /data/infra/ssl/grpc/server/iam-apiserver.crt$(COLOR_RESET)"; \
		echo "$(COLOR_YELLOW)请先在 infra 项目中生成 iam-apiserver 服务端证书$(COLOR_RESET)"; \
		exit 1; \
	fi
	@openssl verify -CAfile /data/infra/ssl/grpc/ca/ca-chain.crt /data/infra/ssl/grpc/server/iam-apiserver.crt
	@echo "$(COLOR_GREEN)✅ 证书验证成功$(COLOR_RESET)"

grpc-cert-info: ## 显示 gRPC 证书详细信息
	@echo "$(COLOR_CYAN)📋 显示 gRPC 证书信息...$(COLOR_RESET)"
	@if [ -f /data/infra/ssl/grpc/server/iam-apiserver.crt ]; then \
		openssl x509 -in /data/infra/ssl/grpc/server/iam-apiserver.crt -noout -subject -issuer -dates -ext subjectAltName; \
	else \
		echo "$(COLOR_RED)❌ 证书不存在: /data/infra/ssl/grpc/server/iam-apiserver.crt$(COLOR_RESET)"; \
		exit 1; \
	fi

test-dev-config: ## 测试开发环境配置
	@echo "$(COLOR_CYAN)🧪 测试开发环境配置...$(COLOR_RESET)"
	@chmod +x scripts/test-dev-config.sh
	@./scripts/test-dev-config.sh

# =============================================================================
# 测试
# =============================================================================

test: ## 运行测试
	@echo "🧪 运行测试..."
	@$(GO_TEST) ./...

clean: ## 清理构建文件和进程
	@echo "🧹 清理构建文件和进程..."
	@$(MAKE) stop-apiserver
	@rm -rf tmp bin $(LOG_DIR)/*.log
	@rm -f $(APISERVER_BIN)
	@$(GO) clean
	@echo "✅ 清理完成"

create-dirs: ## 创建必要的目录
	@mkdir -p $(PID_DIR) $(LOG_DIR) 
# ============================================================================
# 测试（扩展）
# ============================================================================

test-unit: ## 运行单元测试
	@echo "$(COLOR_CYAN)🧪 运行单元测试...$(COLOR_RESET)"
	@$(GO_TEST) -v -short ./...

test-coverage: create-dirs ## 生成测试覆盖率报告
	@echo "$(COLOR_CYAN)🧪 生成测试覆盖率报告...$(COLOR_RESET)"
	@mkdir -p $(COVERAGE_DIR)
	@$(GO_TEST) -v -coverprofile=$(COVERAGE_DIR)/coverage.out ./...
	@$(GO) tool cover -html=$(COVERAGE_DIR)/coverage.out -o $(COVERAGE_DIR)/coverage.html
	@echo "$(COLOR_GREEN)✅ 覆盖率报告: $(COVERAGE_DIR)/coverage.html$(COLOR_RESET)"
	@$(GO) tool cover -func=$(COVERAGE_DIR)/coverage.out | tail -n 1

test-race: ## 运行竞态检测
	@echo "$(COLOR_CYAN)🧪 运行竞态检测...$(COLOR_RESET)"
	@$(GO_TEST) -v -race ./...

test-bench: ## 运行基准测试
	@echo "$(COLOR_CYAN)🧪 运行基准测试...$(COLOR_RESET)"
	@$(GO_TEST) -v -bench=. -benchmem ./...

# ============================================================================
# 代码质量
# ============================================================================

lint: ## 运行代码检查
	@echo "$(COLOR_CYAN)🔍 运行代码检查...$(COLOR_RESET)"
	@if command -v golangci-lint > /dev/null 2>&1; then \
		golangci-lint run --timeout=5m ./...; \
	else \
		echo "$(COLOR_YELLOW)⚠️  golangci-lint 未安装，使用 go vet$(COLOR_RESET)"; \
		$(GO) vet ./...; \
	fi

fmt: ## 格式化代码
	@echo "$(COLOR_CYAN)✨ 格式化代码...$(COLOR_RESET)"
	@$(GO) fmt ./...
	@gofmt -s -w .
	@echo "$(COLOR_GREEN)✅ 代码格式化完成$(COLOR_RESET)"

fmt-check: ## 检查代码格式
	@echo "$(COLOR_CYAN)🔍 检查代码格式...$(COLOR_RESET)"
	@if [ -n "$$(gofmt -l .)" ]; then \
		echo "$(COLOR_RED)❌ 以下文件需要格式化:$(COLOR_RESET)"; \
		gofmt -l .; \
		exit 1; \
	else \
		echo "$(COLOR_GREEN)✅ 代码格式正确$(COLOR_RESET)"; \
	fi

# ============================================================================
# 依赖管理
# ============================================================================

deps: deps-download ## 下载依赖

deps-download: ## 下载所有依赖
	@echo "$(COLOR_CYAN)📦 下载依赖...$(COLOR_RESET)"
	@$(GO) mod download
	@echo "$(COLOR_GREEN)✅ 依赖下载完成$(COLOR_RESET)"

deps-tidy: ## 整理依赖
	@echo "$(COLOR_CYAN)🧹 整理依赖...$(COLOR_RESET)"
	@$(GO) mod tidy
	@echo "$(COLOR_GREEN)✅ 依赖整理完成$(COLOR_RESET)"

deps-verify: ## 验证依赖
	@echo "$(COLOR_CYAN)🔍 验证依赖...$(COLOR_RESET)"
	@$(GO) mod verify
	@echo "$(COLOR_GREEN)✅ 依赖验证通过$(COLOR_RESET)"

deps-update: ## 更新 component-base 到最新版本
	@echo "$(COLOR_CYAN)🔄 更新 component-base...$(COLOR_RESET)"
	@$(GO) get -u github.com/FangcunMount/component-base@latest
	@$(GO) mod tidy
	@echo "$(COLOR_GREEN)✅ component-base 已更新$(COLOR_RESET)"
	@$(GO) list -m github.com/FangcunMount/component-base

deps-update-all: ## 更新所有依赖到最新版本
	@echo "$(COLOR_CYAN)🔄 更新所有依赖...$(COLOR_RESET)"
	@$(GO) get -u ./...
	@$(GO) mod tidy
	@$(GO) mod verify
	@echo "$(COLOR_GREEN)✅ 所有依赖已更新$(COLOR_RESET)"

deps-check: ## 检查可更新的依赖
	@echo "$(COLOR_CYAN)🔍 检查依赖状态...$(COLOR_RESET)"
	@$(GO) list -u -m all | grep -v indirect || true
	@echo ""
	@echo "$(COLOR_YELLOW)说明: 后面有方括号 [...] 的表示有更新可用$(COLOR_RESET)"

# ============================================================================
# Protocol Buffers
# ============================================================================

proto-gen: ## 生成 protobuf 代码
	@echo "$(COLOR_CYAN)🔨 生成 protobuf 代码...$(COLOR_RESET)"
	@if [ -f scripts/proto/generate.sh ]; then \
		bash scripts/proto/generate.sh; \
		echo "$(COLOR_GREEN)✅ Protobuf 代码生成完成$(COLOR_RESET)"; \
	else \
		echo "$(COLOR_YELLOW)⚠️  脚本不存在: scripts/proto/generate.sh$(COLOR_RESET)"; \
	fi

# ============================================================================
# 工具安装
# ============================================================================

install-tools: ## 安装开发工具
	@echo "$(COLOR_CYAN)📦 安装开发工具...$(COLOR_RESET)"
	@echo "安装 Air (热更新)..."
	@$(GO) install github.com/air-verse/air@latest
	@echo "安装 mockgen..."
	@$(GO) install go.uber.org/mock/mockgen@latest
	@echo "$(COLOR_GREEN)✅ 工具安装完成$(COLOR_RESET)"

# ============================================================================
# 调试和诊断
# ============================================================================

debug: ## 显示调试信息
	@echo "$(COLOR_BOLD)$(COLOR_CYAN)🔍 调试信息:$(COLOR_RESET)"
	@echo "$(COLOR_BOLD)━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━$(COLOR_RESET)"
	@echo "项目名称:     $(PROJECT_NAME)"
	@echo "版本:         $(VERSION)"
	@echo "Git 提交:     $(GIT_COMMIT)"
	@echo "Git 分支:     $(GIT_BRANCH)"
	@echo "构建时间:     $(BUILD_TIME)"
	@echo "Go 版本:      $(shell $(GO) version)"
	@echo "GOPATH:       $(shell go env GOPATH)"
	@echo "GOOS:         $(shell go env GOOS)"
	@echo "GOARCH:       $(shell go env GOARCH)"
	@echo "$(COLOR_BOLD)━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━$(COLOR_RESET)"

ps: ## 显示相关进程
	@echo "$(COLOR_CYAN)🔍 相关进程:$(COLOR_RESET)"
	@ps aux | grep -E "(apiserver|air)" | grep -v grep || echo "$(COLOR_YELLOW)未找到相关进程$(COLOR_RESET)"

ports: ## 检查端口占用
	@echo "$(COLOR_CYAN)🔍 端口占用:$(COLOR_RESET)"
	@lsof -i :$(APISERVER_PORT) 2>/dev/null || echo "$(COLOR_GREEN)端口 $(APISERVER_PORT) 未被占用$(COLOR_RESET)"

# ============================================================================
# CI/CD
# ============================================================================

ci: deps-verify fmt-check lint test ## CI 流程
	@echo "$(COLOR_GREEN)✅ CI 检查通过$(COLOR_RESET)"

release: clean build ## 发布版本
	@echo "$(COLOR_GREEN)✅ 版本 $(VERSION) 发布准备完成$(COLOR_RESET)"

cd-validate: ## 校验 CD 服务元数据和脚本入口 (SERVICE=apiserver)
	@SERVICE="$(SERVICE)" IMAGE_METADATA_PRINT=1 "$(CD_SCRIPT_DIR)/image-metadata.sh" >/dev/null
	@test -x "$(CD_SCRIPT_DIR)/build-image.sh"
	@test -x "$(CD_SCRIPT_DIR)/push-dockerhub.sh"
	@test -x "$(CD_SCRIPT_DIR)/prepare-package.sh"
	@test -x "$(CD_SCRIPT_DIR)/remote-deploy.sh"
	@echo "$(COLOR_GREEN)✅ CD metadata validated for SERVICE=$(SERVICE)$(COLOR_RESET)"

cd-image: cd-validate ## 构建并发布 IAM 镜像到 GHCR 和 Docker Hub
	@SERVICE="$(SERVICE)" DOCKER_REGISTRY="$(DOCKER_REGISTRY)" DOCKER_REPOSITORY="$(DOCKER_REPOSITORY)" DEPLOY_REF="$(DEPLOY_REF)" DEPLOY_SHA="$(DEPLOY_SHA)" BUILD_TIME="$(BUILD_TIME)" BUILD_CACHE_REF="$(BUILD_CACHE_REF)" WWW_UID="$(WWW_UID)" WWW_GID="$(WWW_GID)" "$(CD_SCRIPT_DIR)/build-image.sh"
	@SERVICE="$(SERVICE)" DOCKER_REGISTRY="$(DOCKER_REGISTRY)" DOCKER_REPOSITORY="$(DOCKER_REPOSITORY)" DOCKERHUB_USERNAME="$(DOCKERHUB_USERNAME)" DEPLOY_SHA="$(DEPLOY_SHA)" "$(CD_SCRIPT_DIR)/push-dockerhub.sh"

cd-package: cd-validate ## 生成 IAM 生产部署包
	@SERVICE="$(SERVICE)" "$(CD_SCRIPT_DIR)/prepare-package.sh"

cd-remote-deploy: cd-validate ## 在目标机执行远端部署脚本
	@SERVICE="$(SERVICE)" IMAGE_TAG="$(IMAGE_TAG)" "$(CD_SCRIPT_DIR)/remote-deploy.sh"

# ============================================================================
# 快捷命令
# ============================================================================

up: run ## 启动服务（别名）
down: stop ## 停止服务（别名）
re: restart ## 重启服务（别名）
st: status ## 查看状态（别名）
log: logs ## 查看日志（别名）

# ============================================================================
# 数据库管理
# ============================================================================

# 数据库配置
DB_HOST ?= 127.0.0.1
DB_PORT ?= 3306
DB_USER ?= root
DB_PASSWORD ?=
DB_NAME ?= iam

# 注意: db-init, db-migrate, db-reset 已弃用
# 请使用以下新命令:
# - 数据库迁移: 应用程序启动时自动执行 (internal/pkg/migration)
# - 系统 bootstrap 基线数据: migration 000005 或 make db-bootstrap

db-bootstrap: ## 重新应用系统 bootstrap 基线数据（幂等 SQL）
	@echo "$(COLOR_BOLD)$(COLOR_BLUE)🧱 应用系统 bootstrap 数据...$(COLOR_RESET)"
	@if [ -n "$(DB_PASSWORD)" ]; then \
		mysql -h$(DB_HOST) -P$(DB_PORT) -u$(DB_USER) -p$(DB_PASSWORD) $(DB_NAME) < configs/mysql/bootstrap.sql; \
	else \
		mysql -h$(DB_HOST) -P$(DB_PORT) -u$(DB_USER) $(DB_NAME) < configs/mysql/bootstrap.sql; \
	fi
	@echo "$(COLOR_GREEN)✅ 系统 bootstrap 数据已应用$(COLOR_RESET)"

db-connect: ## 连接到数据库
	@echo "$(COLOR_CYAN)🔌 连接到数据库 $(DB_NAME)...$(COLOR_RESET)"
	@if [ -n "$(DB_PASSWORD)" ]; then \
		mysql -h$(DB_HOST) -P$(DB_PORT) -u$(DB_USER) -p$(DB_PASSWORD) $(DB_NAME); \
	else \
		mysql -h$(DB_HOST) -P$(DB_PORT) -u$(DB_USER) $(DB_NAME); \
	fi

db-status: ## 查看数据库状态
	@echo "$(COLOR_CYAN)🔍 数据库状态:$(COLOR_RESET)"
	@if [ -n "$(DB_PASSWORD)" ]; then \
		mysql -h$(DB_HOST) -P$(DB_PORT) -u$(DB_USER) -p$(DB_PASSWORD) -e "\
			SELECT TABLE_NAME AS '表名', TABLE_ROWS AS '行数', TABLE_COMMENT AS '说明' \
			FROM information_schema.TABLES \
			WHERE TABLE_SCHEMA = '$(DB_NAME)' \
			ORDER BY TABLE_NAME;" 2>/dev/null || echo "$(COLOR_YELLOW)⚠️  无法连接到数据库$(COLOR_RESET)"; \
	else \
		mysql -h$(DB_HOST) -P$(DB_PORT) -u$(DB_USER) -e "\
			SELECT TABLE_NAME AS '表名', TABLE_ROWS AS '行数', TABLE_COMMENT AS '说明' \
			FROM information_schema.TABLES \
			WHERE TABLE_SCHEMA = '$(DB_NAME)' \
			ORDER BY TABLE_NAME;" 2>/dev/null || echo "$(COLOR_YELLOW)⚠️  无法连接到数据库$(COLOR_RESET)"; \
	fi

db-backup: ## 备份数据库
	@echo "$(COLOR_CYAN)💾 备份数据库...$(COLOR_RESET)"
	@BACKUP_FILE="backups/$(DB_NAME)_$(shell date +%Y%m%d_%H%M%S).sql"; \
	mkdir -p backups; \
	if [ -n "$(DB_PASSWORD)" ]; then \
		mysqldump -h$(DB_HOST) -P$(DB_PORT) -u$(DB_USER) -p$(DB_PASSWORD) $(DB_NAME) > $$BACKUP_FILE; \
	else \
		mysqldump -h$(DB_HOST) -P$(DB_PORT) -u$(DB_USER) $(DB_NAME) > $$BACKUP_FILE; \
	fi; \
	echo "$(COLOR_GREEN)✅ 数据库已备份到: $$BACKUP_FILE$(COLOR_RESET)"

# ============================================================================
# Docker MySQL 管理（开发环境）
# ============================================================================

docker-mysql-up: ## 启动 Docker MySQL 容器（开发环境）
	@echo "$(COLOR_CYAN)🐳 启动 Docker MySQL 容器...$(COLOR_RESET)"
	@docker run -d \
		--name iam-mysql \
		-e MYSQL_ROOT_PASSWORD=root \
		-e MYSQL_DATABASE=$(DB_NAME) \
		-p $(DB_PORT):3306 \
		-v iam-mysql-data:/var/lib/mysql \
		mysql:8.0 \
		--character-set-server=utf8mb4 \
		--collation-server=utf8mb4_unicode_ci
	@echo "$(COLOR_GREEN)✅ MySQL 容器已启动$(COLOR_RESET)"
	@echo "$(COLOR_YELLOW)⏳ 等待 MySQL 启动完成（约 10 秒）...$(COLOR_RESET)"
	@sleep 10
	@echo "$(COLOR_GREEN)✅ MySQL 已就绪，可以执行初始化: make db-init DB_PASSWORD=root$(COLOR_RESET)"

docker-mysql-down: ## 停止并删除 Docker MySQL 容器
	@echo "$(COLOR_CYAN)🐳 停止 Docker MySQL 容器...$(COLOR_RESET)"
	@docker stop iam-mysql 2>/dev/null || true
	@docker rm iam-mysql 2>/dev/null || true
	@echo "$(COLOR_GREEN)✅ MySQL 容器已停止$(COLOR_RESET)"

docker-mysql-clean: ## 清理 Docker MySQL 数据（删除容器和数据卷）
	@echo "$(COLOR_RED)⚠️  清理 Docker MySQL 数据...$(COLOR_RESET)"
	@docker stop iam-mysql 2>/dev/null || true
	@docker rm iam-mysql 2>/dev/null || true
	@docker volume rm iam-mysql-data 2>/dev/null || true
	@echo "$(COLOR_GREEN)✅ MySQL 数据已清理$(COLOR_RESET)"

docker-mysql-logs: ## 查看 Docker MySQL 日志
	@docker logs -f iam-mysql

# ============================================================================
# Docker 构建和部署
# ============================================================================

.PHONY: docker-build docker-run docker-stop docker-clean docker-push
.PHONY: docker-compose-up docker-compose-down docker-compose-restart

docker-build: ## 构建 Docker 镜像
	@echo "$(COLOR_BOLD)$(COLOR_BLUE)🐳 构建 Docker 镜像...$(COLOR_RESET)"
	@docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg BUILD_TIME=$(BUILD_TIME) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		--build-arg RUN_UID=$(RUN_UID) \
		--build-arg RUN_GID=$(RUN_GID) \
		-f build/docker/Dockerfile \
		-t $(DOCKER_IMAGE):$(VERSION) \
		-t $(DOCKER_IMAGE):latest \
		.
	@echo "$(COLOR_GREEN)✅ Docker 镜像构建完成$(COLOR_RESET)"
	@docker images $(DOCKER_IMAGE)

docker-run: ## 运行 Docker 容器
	@echo "$(COLOR_BLUE)🐳 运行 Docker 容器...$(COLOR_RESET)"
	@# 确保存在基础设施网络（优先 overlay attachable，不可用则退回 bridge）
	@if ! docker network ls --format '{{.Name}}' | grep -w $(DOCKER_NETWORK) >/dev/null 2>&1; then \
		if docker info --format '{{.Swarm.LocalNodeState}}' 2>/dev/null | grep -Eq '(active|pending)'; then \
			echo "Creating overlay network $(DOCKER_NETWORK)..."; \
			docker network create --driver overlay --attachable $(DOCKER_NETWORK); \
		else \
			echo "Creating bridge network $(DOCKER_NETWORK) (Swarm not initialized)..."; \
			docker network create $(DOCKER_NETWORK); \
		fi; \
	fi
	@mkdir -p $(LOG_DIR_HOST) >/dev/null 2>&1 || true
	@TLS_MOUNTS=""; \
	if [ -f "$(TLS_CERT_HOST)" ]; then \
		TLS_MOUNTS="$$TLS_MOUNTS -v $(TLS_CERT_HOST):$(TLS_CERT_DEST):ro"; \
	else \
		echo "$(COLOR_YELLOW)⚠️  未找到 TLS 证书文件: $(TLS_CERT_HOST)$(COLOR_RESET)"; \
	fi; \
	if [ -f "$(TLS_KEY_HOST)" ]; then \
		TLS_MOUNTS="$$TLS_MOUNTS -v $(TLS_KEY_HOST):$(TLS_KEY_DEST):ro"; \
	else \
		echo "$(COLOR_YELLOW)⚠️  未找到 TLS 私钥文件: $(TLS_KEY_HOST)$(COLOR_RESET)"; \
	fi; \
	docker run -d \
		--name $(PROJECT_NAME) \
		--network $(DOCKER_NETWORK) \
		--cpus 0.25 \
		--memory 384m --memory-swap 384m \
		-v $(PWD)/configs:/app/configs:ro \
		-v $(LOG_DIR_HOST):/var/log/iam \
		$$TLS_MOUNTS \
		$(DOCKER_IMAGE):$(DOCKER_TAG)
	@echo "$(COLOR_GREEN)✅ Docker 容器已启动$(COLOR_RESET)"

docker-stop: ## 停止 Docker 容器
	@echo "$(COLOR_YELLOW)⏹️  停止 Docker 容器...$(COLOR_RESET)"
	@docker stop $(PROJECT_NAME) 2>/dev/null || true
	@docker rm $(PROJECT_NAME) 2>/dev/null || true
	@echo "$(COLOR_GREEN)✅ Docker 容器已停止$(COLOR_RESET)"

docker-clean: ## 清理 Docker 镜像和容器
	@echo "$(COLOR_RED)🧹 清理 Docker 资源...$(COLOR_RESET)"
	@docker stop $(PROJECT_NAME) 2>/dev/null || true
	@docker rm $(PROJECT_NAME) 2>/dev/null || true
	@docker rmi $(DOCKER_IMAGE):latest 2>/dev/null || true
	@echo "$(COLOR_GREEN)✅ Docker 资源已清理$(COLOR_RESET)"

docker-push: ## 推送 Docker 镜像到仓库
	@echo "$(COLOR_BLUE)📤 推送 Docker 镜像...$(COLOR_RESET)"
	@docker push $(DOCKER_IMAGE):$(VERSION)
	@docker push $(DOCKER_IMAGE):latest
	@echo "$(COLOR_GREEN)✅ Docker 镜像已推送$(COLOR_RESET)"

docker-compose-up: ## 使用 docker-compose 启动所有服务
	@echo "$(COLOR_BLUE)🐳 启动 Docker Compose 服务...$(COLOR_RESET)"
	@docker-compose -f build/docker/docker-compose.prod.yml up -d
	@echo "$(COLOR_GREEN)✅ 服务已启动$(COLOR_RESET)"
	@docker-compose -f build/docker/docker-compose.prod.yml ps

docker-compose-down: ## 停止 docker-compose 服务
	@echo "$(COLOR_YELLOW)⏹️  停止 Docker Compose 服务...$(COLOR_RESET)"
	@docker-compose -f build/docker/docker-compose.prod.yml down
	@echo "$(COLOR_GREEN)✅ 服务已停止$(COLOR_RESET)"

docker-compose-restart: ## 重启 docker-compose 服务
	@echo "$(COLOR_BLUE)🔄 重启 Docker Compose 服务...$(COLOR_RESET)"
	@docker-compose -f build/docker/docker-compose.prod.yml restart
	@echo "$(COLOR_GREEN)✅ 服务已重启$(COLOR_RESET)"

docker-compose-logs: ## 查看 docker-compose 日志
	@docker-compose -f build/docker/docker-compose.prod.yml logs -f

# ============================================================================
# Docker 开发环境管理
# ============================================================================

docker-dev-up: cert-gen ## 启动 Docker 开发环境
	@echo "$(COLOR_BLUE)🐳 启动 Docker 开发环境...$(COLOR_RESET)"
	@docker-compose -f build/docker/docker-compose.dev.yml up -d
	@echo "$(COLOR_GREEN)✅ 开发环境已启动$(COLOR_RESET)"
	@echo ""
	@echo "$(COLOR_CYAN)📊 服务状态:$(COLOR_RESET)"
	@docker-compose -f build/docker/docker-compose.dev.yml ps
	@echo ""
	@echo "$(COLOR_YELLOW)💡 提示:$(COLOR_RESET)"
	@echo "  查看日志: make docker-dev-logs"
	@echo "  停止服务: make docker-dev-down"
	@echo "  重启服务: make docker-dev-restart"

docker-dev-down: ## 停止 Docker 开发环境
	@echo "$(COLOR_YELLOW)⏹️  停止 Docker 开发环境...$(COLOR_RESET)"
	@docker-compose -f build/docker/docker-compose.dev.yml down
	@echo "$(COLOR_GREEN)✅ 开发环境已停止$(COLOR_RESET)"

docker-dev-restart: ## 重启 Docker 开发环境
	@echo "$(COLOR_BLUE)🔄 重启 Docker 开发环境...$(COLOR_RESET)"
	@docker-compose -f build/docker/docker-compose.dev.yml restart
	@echo "$(COLOR_GREEN)✅ 开发环境已重启$(COLOR_RESET)"

docker-dev-logs: ## 查看 Docker 开发环境日志
	@docker-compose -f build/docker/docker-compose.dev.yml logs -f

docker-dev-clean: ## 清理 Docker 开发环境（包括数据卷）
	@echo "$(COLOR_RED)⚠️  清理 Docker 开发环境...$(COLOR_RESET)"
	@docker-compose -f build/docker/docker-compose.dev.yml down -v
	@echo "$(COLOR_GREEN)✅ 开发环境已清理$(COLOR_RESET)"

# ============================================================================
# 部署相关
# ============================================================================

.PHONY: deploy deploy-prepare deploy-check

deploy-prepare: ## 准备部署文件 (已废弃，现使用 Docker 部署)
	@echo "$(COLOR_YELLOW)⚠️  此命令已废弃，现在使用 Docker 部署$(COLOR_RESET)"
	@echo "$(COLOR_BLUE)请使用: git push origin main (自动触发 CI + CD)$(COLOR_RESET)"
	@echo "$(COLOR_BLUE)或查看: .github/workflows/cd.yml$(COLOR_RESET)"

deploy-check: ## 检查部署环境
	@echo "$(COLOR_BLUE)🔍 检查部署环境...$(COLOR_RESET)"
	@echo "部署主机: $(DEPLOY_HOST)"
	@echo "部署路径: $(DEPLOY_PATH)"
	@echo "SSH 用户: $(DEPLOY_USER)"
	@echo ""
	@echo "测试 SSH 连接..."
	@ssh -o ConnectTimeout=5 $(DEPLOY_USER)@$(DEPLOY_HOST) "echo '✅ SSH 连接成功'" || \
		echo "$(COLOR_RED)❌ SSH 连接失败$(COLOR_RESET)"
