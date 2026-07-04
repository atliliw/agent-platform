.PHONY: proto build run-dev run-prod stop clean test docker-build docker-push k8s-deploy k8s-delete help

# ============================================================
# Agent Platform Makefile
# ============================================================

# 变量定义
PROTO_DIR := proto
PB_DIR := pkg/pb
SERVICES := gateway chat-service knowledge-service memory-service a2a-service mcp-service harness-service
BIN_DIR := bin
DOCKER_COMPOSE_DEV := docker/docker-compose.dev.yaml
DOCKER_COMPOSE_PROD := docker/docker-compose.yaml

# 默认目标
.DEFAULT_GOAL := help

# ============================================================
# Protobuf 生成
# ============================================================

proto: proto-common proto-chat proto-knowledge proto-memory proto-a2a proto-mcp proto-harness
	@echo "✅ Protobuf generation completed"

proto-common:
	@echo "Generating common protobuf..."
	@protoc --go_out=$(PB_DIR) --go-grpc_out=$(PB_DIR) \
		--go_opt=paths=source_relative \
		--go-grpc_opt=paths=source_relative \
		-I . \
		$(PROTO_DIR)/common/*.proto

proto-chat:
	@echo "Generating chat protobuf..."
	@protoc --go_out=$(PB_DIR) --go-grpc_out=$(PB_DIR) \
		--go_opt=paths=source_relative \
		--go-grpc_opt=paths=source_relative \
		-I . \
		$(PROTO_DIR)/chat/*.proto

proto-knowledge:
	@echo "Generating knowledge protobuf..."
	@protoc --go_out=$(PB_DIR) --go-grpc_out=$(PB_DIR) \
		--go_opt=paths=source_relative \
		--go-grpc_opt=paths=source_relative \
		-I . \
		$(PROTO_DIR)/knowledge/*.proto

proto-memory:
	@echo "Generating memory protobuf..."
	@protoc --go_out=$(PB_DIR) --go-grpc_out=$(PB_DIR) \
		--go_opt=paths=source_relative \
		--go-grpc_opt=paths=source_relative \
		-I . \
		$(PROTO_DIR)/memory/*.proto

proto-a2a:
	@echo "Generating a2a protobuf..."
	@protoc --go_out=$(PB_DIR) --go-grpc_out=$(PB_DIR) \
		--go_opt=paths=source_relative \
		--go-grpc_opt=paths=source_relative \
		-I . \
		$(PROTO_DIR)/a2a/*.proto

proto-mcp:
	@echo "Generating mcp protobuf..."
	@protoc --go_out=$(PB_DIR) --go-grpc_out=$(PB_DIR) \
		--go_opt=paths=source_relative \
		--go-grpc_opt=paths=source_relative \
		-I . \
		$(PROTO_DIR)/mcp/*.proto

proto-harness:
	@echo "Generating harness protobuf..."
	@protoc --go_out=$(PB_DIR) --go-grpc_out=$(PB_DIR) \
		--go_opt=paths=source_relative \
		--go-grpc_opt=paths=source_relative \
		-I . \
		$(PROTO_DIR)/harness/*.proto

# ============================================================
# 构建
# ============================================================

build: proto
	@echo "Building all services..."
	@mkdir -p $(BIN_DIR)
	@for service in $(SERVICES); do \
		echo "Building $$service..."; \
		go build -o $(BIN_DIR)/$$service ./services/$$service/cmd; \
	done
	@echo "✅ Build completed"

build-%:
	@echo "Building $*..."
	@mkdir -p $(BIN_DIR)
	@go build -o $(BIN_DIR)/$* ./services/$*/cmd

# ============================================================
# 运行
# ============================================================

run-dev:
	@echo "Starting development environment..."
	@docker-compose -f $(DOCKER_COMPOSE_DEV) up -d
	@echo "✅ Development environment started"
	@echo "Gateway: http://localhost:8080"
	@echo "Frontend: http://localhost:5173"

run-prod:
	@echo "Starting production environment..."
	@docker-compose -f $(DOCKER_COMPOSE_PROD) up -d
	@echo "✅ Production environment started"

stop:
	@echo "Stopping all environments..."
	@docker-compose -f $(DOCKER_COMPOSE_DEV) down 2>/dev/null || true
	@docker-compose -f $(DOCKER_COMPOSE_PROD) down 2>/dev/null || true
	@echo "✅ All environments stopped"

# ============================================================
# 清理
# ============================================================

clean:
	@echo "Cleaning..."
	@rm -rf $(BIN_DIR)
	@rm -rf $(PB_DIR)
	@docker-compose -f $(DOCKER_COMPOSE_DEV) down -v 2>/dev/null || true
	@echo "✅ Clean completed"

# ============================================================
# 测试
# ============================================================

test:
	@echo "Running tests..."
	@go test -v -race ./...

test-%:
	@echo "Running tests for $*..."
	@go test -v -race ./services/$*/...

test-coverage:
	@echo "Running tests with coverage..."
	@go test -v -race -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# ============================================================
# Docker
# ============================================================

docker-build:
	@echo "Building Docker images..."
	@docker-compose -f $(DOCKER_COMPOSE_PROD) build
	@echo "✅ Docker images built"

docker-push:
	@echo "Pushing Docker images..."
	@docker-compose -f $(DOCKER_COMPOSE_PROD) push
	@echo "✅ Docker images pushed"

docker-logs:
	@docker-compose -f $(DOCKER_COMPOSE_DEV) logs -f

# ============================================================
# Kubernetes
# ============================================================

k8s-deploy:
	@echo "Deploying to Kubernetes..."
	@kubectl apply -f k8s/
	@echo "✅ Kubernetes deployment completed"

k8s-delete:
	@echo "Deleting from Kubernetes..."
	@kubectl delete -f k8s/
	@echo "✅ Kubernetes resources deleted"

k8s-status:
	@kubectl get pods -n agent-platform

# ============================================================
# 开发工具
# ============================================================

lint:
	@echo "Running linters..."
	@golangci-lint run ./...

fmt:
	@echo "Formatting code..."
	@go fmt ./...
	@echo "✅ Code formatted"

tidy:
	@go mod tidy

# ============================================================
# 帮助
# ============================================================

help:
	@echo "Agent Platform Makefile"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  proto          Generate protobuf code"
	@echo "  build          Build all services"
	@echo "  build-<service> Build specific service"
	@echo "  run-dev        Start development environment"
	@echo "  run-prod       Start production environment"
	@echo "  stop           Stop all environments"
	@echo "  clean          Clean build artifacts"
	@echo "  test           Run all tests"
	@echo "  test-<service> Run tests for specific service"
	@echo "  test-coverage  Run tests with coverage"
	@echo "  docker-build   Build Docker images"
	@echo "  docker-push    Push Docker images"
	@echo "  docker-logs    View Docker logs"
	@echo "  k8s-deploy     Deploy to Kubernetes"
	@echo "  k8s-delete     Delete from Kubernetes"
	@echo "  k8s-status     Show Kubernetes status"
	@echo "  lint           Run linters"
	@echo "  fmt            Format code"
	@echo "  tidy           Tidy go modules"
