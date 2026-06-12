#!/bin/bash
# Agent Platform 一键修复和部署脚本 (Linux/Mac)

echo "========================================"
echo "Agent Platform 一键修复和部署"
echo "========================================"
echo ""

# 1. 显示修复内容
echo "[1/6] 修复说明:"
echo "  - agent-service 端口: 50007 -> 50006"
echo "  - harness-service 端口: 50006 -> 50007"
echo "  - gateway/chat-service 配置同步更新"
echo ""

# 2. 验证配置文件
echo "[2/6] 验证配置文件..."

if grep -q "grpc_port: 50006" services/agent-service/config.yaml; then
    echo "  ✓ agent-service 端口配置正确 (50006)"
else
    echo "  ! agent-service 端口需要修复"
fi

if grep -q "grpc_port: 50007" services/harness-service/config.yaml; then
    echo "  ✓ harness-service 端口配置正确 (50007)"
else
    echo "  ! harness-service 端口需要修复"
fi

# 3. 检查 Docker
echo ""
echo "[3/6] 检查 Docker..."
if command -v docker &> /dev/null; then
    echo "  Docker 版本: $(docker --version)"
else
    echo "  [ERROR] Docker 未安装"
    exit 1
fi

# 4. 停止旧服务
echo ""
echo "[4/6] 停止旧服务..."
docker compose down --remove-orphans 2>/dev/null || docker-compose down --remove-orphans 2>/dev/null
echo "  ✓ 旧服务已停止"

# 5. 构建镜像
echo ""
echo "[5/6] 构建 Docker 镜像..."
echo "  这可能需要几分钟，请耐心等待..."

if command -v docker-compose &> /dev/null; then
    docker-compose build --no-cache
else
    docker compose build --no-cache
fi

if [ $? -ne 0 ]; then
    echo "  [ERROR] 镜像构建失败"
    exit 1
fi
echo "  ✓ 镜像构建完成"

# 6. 启动服务
echo ""
echo "[6/6] 启动服务..."
if command -v docker-compose &> /dev/null; then
    docker-compose up -d
else
    docker compose up -d
fi

sleep 10

# 显示服务状态
echo ""
echo "========================================"
echo "服务状态"
echo "========================================"
docker compose ps 2>/dev/null || docker-compose ps

# 健康检查
echo ""
echo "========================================"
echo "健康检查"
echo "========================================"

sleep 5

# 检查 Gateway
if curl -s --connect-timeout 5 http://localhost:9000/api/v2/mcp/tools > /dev/null; then
    echo "  ✓ Gateway 正常 (端口 9000)"
else
    echo "  ✗ Gateway 未响应"
fi

echo ""
echo "========================================"
echo "部署完成!"
echo "========================================"
echo ""
echo "访问地址:"
echo "  - Gateway API:    http://localhost:9000"
echo "  - 服务器访问:     http://192.168.10.100:9000"
echo ""
echo "测试命令:"
echo "  curl http://localhost:9000/api/v2/mcp/tools"
echo "  curl -X POST http://localhost:9000/api/v2/chat -H 'Content-Type: application/json' -d '{\"message\":\"你好\"}'"
echo ""
echo "查看日志:"
echo "  docker compose logs -f [service-name]"
echo ""
echo "停止服务:"
echo "  docker compose down"
echo ""