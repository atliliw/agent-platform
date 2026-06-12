#!/bin/bash

# ========================================
# Agent Platform 部署脚本
# ========================================

set -e

echo "========================================"
echo "Agent Platform 部署脚本"
echo "========================================"

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 打印函数
print_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 检查 Docker
check_docker() {
    print_info "检查 Docker..."
    if ! command -v docker &> /dev/null; then
        print_error "Docker 未安装，请先安装 Docker"
        exit 1
    fi

    if ! command -v docker-compose &> /dev/null && ! docker compose version &> /dev/null; then
        print_error "Docker Compose 未安装"
        exit 1
    fi

    print_info "Docker 版本: $(docker --version)"
}

# 检查配置文件
check_configs() {
    print_info "检查配置文件..."

    local configs=(
        "services/gateway/config.yaml"
        "services/chat-service/config.yaml"
        "services/knowledge-service/config.yaml"
        "services/memory-service/config.yaml"
        "services/mcp-service/config.yaml"
        "services/agent-service/config.yaml"
    )

    for config in "${configs[@]}"; do
        if [ ! -f "$config" ]; then
            print_error "配置文件不存在: $config"
            exit 1
        fi
    done

    print_info "所有配置文件检查通过"
}

# 停止旧容器
stop_old_containers() {
    print_info "停止旧容器..."
    docker-compose down --remove-orphans 2>/dev/null || true
}

# 构建镜像
build_images() {
    print_info "构建 Docker 镜像 (这可能需要几分钟)..."
    docker-compose build --no-cache
}

# 启动服务
start_services() {
    print_info "启动服务..."
    docker-compose up -d
}

# 等待服务就绪
wait_for_services() {
    print_info "等待服务启动..."
    sleep 10

    # 检查服务健康状态
    local services=("gateway:9000" "qdrant:6333" "mongodb:27017" "redis:6379")

    for service in "${services[@]}"; do
        local name=$(echo $service | cut -d: -f1)
        local port=$(echo $service | cut -d: -f2)

        print_info "检查 $name (端口 $port)..."
        if curl -s "http://localhost:$port" > /dev/null 2>&1 || nc -z localhost $port 2>/dev/null; then
            print_info "$name 已就绪"
        else
            print_warn "$name 可能还在启动中"
        fi
    done
}

# 显示服务状态
show_status() {
    echo ""
    print_info "服务状态:"
    docker-compose ps

    echo ""
    echo "========================================"
    echo "部署完成!"
    echo "========================================"
    echo ""
    echo "服务地址:"
    echo "  - Gateway API:    http://localhost:9000"
    echo "  - Qdrant:         http://localhost:6333"
    echo "  - MongoDB:        localhost:27017"
    echo "  - Redis:          localhost:6379"
    echo ""
    echo "查看日志:"
    echo "  docker-compose logs -f [service-name]"
    echo ""
    echo "停止服务:"
    echo "  docker-compose down"
    echo ""
}

# 主函数
main() {
    check_docker
    check_configs
    stop_old_containers
    build_images
    start_services
    wait_for_services
    show_status
}

# 运行
main "$@"
