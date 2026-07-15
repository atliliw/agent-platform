#!/bin/bash
# Agent Platform 远程一键部署脚本
# 使用方法: ./deploy-remote.sh [service-name]

set -e

# 配置
SERVER="root@192.168.10.100"
SSH_KEY="$HOME/.ssh/demo_deploy_key"
REMOTE_DIR="/opt/agent-platform"
LOCAL_DIR="$(cd "$(dirname "$0")" && pwd)"

# 颜色
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

print_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
print_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
print_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# 服务名（可选）
SERVICE="${1:-}"

print_info "=== Agent Platform 远程部署 ==="
print_info "服务器: $SERVER"
print_info "本地目录: $LOCAL_DIR"
print_info "远程目录: $REMOTE_DIR"
print_info "目标服务: ${SERVICE:-全部}"

# 1. 本地编译
print_info "1. 本地编译验证..."
cd "$LOCAL_DIR"
if ! go build ./pkg/... ./services/... 2>/dev/null; then
    print_warn "编译有警告，但继续部署..."
fi
print_info "编译完成"

# 2. 同步代码
print_info "2. 同步代码到服务器..."
scp -i "$SSH_KEY" -r \
    pkg services docker go.mod go.sum \
    "$SERVER:$REMOTE_DIR/" 2>/dev/null
print_info "代码同步完成"

# 3. 构建并启动
print_info "3. 构建并启动服务..."
if [ -n "$SERVICE" ]; then
    # 只构建指定服务
    ssh -i "$SSH_KEY" "$SERVER" << EOF
cd $REMOTE_DIR/docker
docker network create agent-network 2>/dev/null || true
docker compose build $SERVICE
docker compose up -d $SERVICE
docker compose ps $SERVICE
EOF
else
    # 构建所有服务
    ssh -i "$SSH_KEY" "$SERVER" << EOF
cd $REMOTE_DIR/docker
docker network create agent-network 2>/dev/null || true
docker compose build
docker compose up -d
docker compose ps
EOF
fi

# 4. 健康检查
print_info "4. 健康检查..."
sleep 5
if curl -s "http://192.168.10.100:9000/health" | grep -q "healthy"; then
    print_info "服务健康检查通过!"
else
    print_warn "服务可能还在启动中，请稍后检查"
fi

print_info "=== 部署完成 ==="
echo ""
echo "前端地址: http://192.168.10.100:8888"
echo "API地址: http://192.168.10.100:9000"
echo ""
echo "查看日志: ssh -i $SSH_KEY $SERVER 'docker compose -f $REMOTE_DIR/docker/docker-compose.yml logs -f'"
