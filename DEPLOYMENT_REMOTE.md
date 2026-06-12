# Agent Platform 远程部署指南

## 服务器信息

| 项目 | 值 |
|------|------|
| 服务器 IP | 192.168.10.100 |
| SSH 密钥 | ~/.ssh/demo_deploy_key |
| 部署目录 | /opt/agent-platform |
| Docker Compose 目录 | /opt/agent-platform/docker |

## 一键部署命令

### 1. 本地编译验证

```powershell
# Windows PowerShell
$env:PATH = "C:\Program Files\Go\bin;$env:PATH"
cd D:\BaiduNetdiskDownload\LLM\agent-platform
go build ./pkg/... ./services/...
```

### 2. 同步代码到服务器

```bash
# 使用 scp 批量同步（排除不需要的文件）
scp -i ~/.ssh/demo_deploy_key -r \
  pkg services docker go.mod go.sum configs \
  root@192.168.10.100:/opt/agent-platform/
```

### 3. 服务器构建并启动

```bash
ssh -i ~/.ssh/demo_deploy_key root@192.168.10.100 << 'EOF'
cd /opt/agent-platform/docker

# 创建网络（如果不存在）
docker network create agent-network 2>/dev/null || true

# 构建并启动所有服务
docker compose build
docker compose up -d

# 查看状态
docker compose ps
EOF
```

## 快速部署脚本

### deploy-remote.sh

```bash
#!/bin/bash
# Agent Platform 远程部署脚本

SERVER="root@192.168.10.100"
SSH_KEY="~/.ssh/demo_deploy_key"
REMOTE_DIR="/opt/agent-platform"
LOCAL_DIR="D:/BaiduNetdiskDownload/LLM/agent-platform"

echo "=== 1. 本地编译验证 ==="
cd $LOCAL_DIR
go build ./pkg/... ./services/...

echo "=== 2. 同步代码到服务器 ==="
scp -i $SSH_KEY -r \
  pkg services docker go.mod go.sum \
  $SERVER:$REMOTE_DIR/

echo "=== 3. 服务器构建并启动 ==="
ssh -i $SSH_KEY $SERVER << 'EOF'
cd /opt/agent-platform/docker
docker network create agent-network 2>/dev/null || true
docker compose build --no-cache
docker compose up -d
docker compose ps
EOF

echo "=== 4. 健康检查 ==="
sleep 10
curl -s http://192.168.10.100:9000/health

echo ""
echo "部署完成！"
echo "前端地址: http://192.168.10.100:8888"
echo "API地址: http://192.168.10.100:9000"
```

## 单独重建某个服务

```bash
# 只重建 mcp-service
ssh -i ~/.ssh/demo_deploy_key root@192.168.10.100 \
  "cd /opt/agent-platform/docker && docker compose build mcp-service && docker compose up -d mcp-service"

# 只重建 chat-service
ssh -i ~/.ssh/demo_deploy_key root@192.168.10.100 \
  "cd /opt/agent-platform/docker && docker compose build chat-service && docker compose up -d chat-service"

# 只重建 agent-service
ssh -i ~/.ssh/demo_deploy_key root@192.168.10.100 \
  "cd /opt/agent-platform/docker && docker compose build agent-service && docker compose up -d agent-service"
```

## 常用运维命令

### 查看服务状态

```bash
ssh -i ~/.ssh/demo_deploy_key root@192.168.10.100 "docker compose -f /opt/agent-platform/docker/docker-compose.yml ps"
```

### 查看服务日志

```bash
# 查看所有日志
ssh -i ~/.ssh/demo_deploy_key root@192.168.10.100 "docker compose -f /opt/agent-platform/docker/docker-compose.yml logs --tail=100"

# 查看单个服务日志
ssh -i ~/.ssh/demo_deploy_key root@192.168.10.100 "docker compose -f /opt/agent-platform/docker/docker-compose.yml logs --tail=100 mcp-service"
```

### 重启服务

```bash
# 重启所有服务
ssh -i ~/.ssh/demo_deploy_key root@192.168.10.100 "docker compose -f /opt/agent-platform/docker/docker-compose.yml restart"

# 重启单个服务
ssh -i ~/.ssh/demo_deploy_key root@192.168.10.100 "docker compose -f /opt/agent-platform/docker/docker-compose.yml restart mcp-service"
```

### 停止服务

```bash
ssh -i ~/.ssh/demo_deploy_key root@192.168.10.100 "docker compose -f /opt/agent-platform/docker/docker-compose.yml down"
```

## 服务端口

| 服务 | 容器端口 | 主机端口 |
|------|---------|---------|
| Gateway | 9000 | 9000 |
| Frontend | 80 | 8888 |
| Chat Service | 50001 | 50001 |
| Knowledge Service | 50002 | 50002 |
| Memory Service | 50003 | 50003 |
| A2A Service | 50004 | 50004 |
| MCP Service | 50005 | 50005 |
| Agent Service | 50006 | 50006 |
| Harness Service | 50007 | 50007 |
| Qdrant | 6333 | 6333 |
| MongoDB | 27017 | 27017 |
| Redis | 6379 | 6379 |

## 测试 API

```bash
# 健康检查
curl http://192.168.10.100:9000/health

# 测试 Chat
curl -X POST http://192.168.10.100:9000/api/v2/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "你好", "tenant_id": "test"}'

# 测试 MCP 工具列表
curl http://192.168.10.100:9000/api/v2/mcp/tools
```

## 部署检查清单

部署前检查：
- [ ] `go build ./pkg/... ./services/...` 编译通过
- [ ] config.yaml 配置文件存在
- [ ] docker-compose.yaml 格式正确

部署后检查：
- [ ] `docker compose ps` 所有服务 Up
- [ ] `curl http://192.168.10.100:9000/health` 返回 healthy
- [ ] Chat API 可正常调用

## 故障排查

### 服务无法启动

```bash
# 查看日志
docker compose logs mcp-service

# 检查端口占用
netstat -tlnp | grep 9000
```

### 容器频繁重启

```bash
# 查看容器状态
docker inspect docker-mcp-service-1 | grep -A 5 "State"

# 查看退出日志
docker logs --tail 100 docker-mcp-service-1
```

### 网络问题

```bash
# 检查网络
docker network ls
docker network inspect agent-network
```

## 注意事项

1. **代码修改后需要重建镜像** - 修改 Go 代码后必须 `docker compose build` 再 `docker compose up -d`
2. **配置修改只需重启** - 修改 config.yaml 只需 `docker compose restart`
3. **MaxSteps 已改为 50** - 浏览器 Agent 最大步数
4. **MCP Service 镜像较大** (770MB) - 包含 Chromium，构建时间约 10 分钟
