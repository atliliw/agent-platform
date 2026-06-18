# Agent Platform 部署指南

## 一、服务器信息

| 项目 | 值 |
|------|------|
| 服务器 IP | 192.168.10.100 |
| SSH 密钥 | ~/.ssh/demo_deploy_key |
| 部署目录 | /opt/agent-platform |
| Docker Compose 目录 | /opt/agent-platform/docker |

---

## 二、一键部署

### 本地编译验证

```powershell
# Windows PowerShell
$env:PATH = "C:\Program Files\Go\bin;$env:PATH"
cd D:\BaiduNetdiskDownload\LLM\agent-platform
go build ./pkg/... ./services/...
```

### 同步代码到服务器

```bash
scp -i ~/.ssh/demo_deploy_key -r \
  pkg services docker go.mod go.sum \
  root@192.168.10.100:/opt/agent-platform/
```

### 服务器构建并启动

```bash
ssh -i ~/.ssh/demo_deploy_key root@192.168.10.100 << 'EOF'
cd /opt/agent-platform/docker
docker network create agent-network 2>/dev/null || true
docker compose build
docker compose up -d
docker compose ps
EOF
```

---

## 三、单独重建某个服务（更快）

```bash
# MCP Service（包含浏览器 Agent，构建约 10 分钟）
ssh -i ~/.ssh/demo_deploy_key root@192.168.10.100 \
  "cd /opt/agent-platform/docker && docker compose build mcp-service && docker compose up -d mcp-service"

# Chat Service
ssh -i ~/.ssh/demo_deploy_key root@192.168.10.100 \
  "cd /opt/agent-platform/docker && docker compose build chat-service && docker compose up -d chat-service"

# Agent Service
ssh -i ~/.ssh/demo_deploy_key root@192.168.10.100 \
  "cd /opt/agent-platform/docker && docker compose build agent-service && docker compose up -d agent-service"

# Gateway
ssh -i ~/.ssh/demo_deploy_key root@192.168.10.100 \
  "cd /opt/agent-platform/docker && docker compose build gateway && docker compose up -d gateway"

# Frontend
ssh -i ~/.ssh/demo_deploy_key root@192.168.10.100 \
  "cd /opt/agent-platform/docker && docker compose build frontend && docker compose up -d frontend"
```

---

## 四、常用运维命令

### 查看服务状态

```bash
ssh -i ~/.ssh/demo_deploy_key root@192.168.10.100 \
  "cd /opt/agent-platform/docker && docker compose ps"
```

### 查看日志

```bash
# 所有服务日志
ssh -i ~/.ssh/demo_deploy_key root@192.168.10.100 \
  "cd /opt/agent-platform/docker && docker compose logs --tail=100"

# 单个服务日志
ssh -i ~/.ssh/demo_deploy_key root@192.168.10.100 \
  "cd /opt/agent-platform/docker && docker compose logs --tail=100 -f mcp-service"
```

### 重启服务

```bash
# 重启所有
ssh -i ~/.ssh/demo_deploy_key root@192.168.10.100 \
  "cd /opt/agent-platform/docker && docker compose restart"

# 重启单个
ssh -i ~/.ssh/demo_deploy_key root@192.168.10.100 \
  "cd /opt/agent-platform/docker && docker compose restart mcp-service"
```

### 停止服务

```bash
ssh -i ~/.ssh/demo_deploy_key root@192.168.10.100 \
  "cd /opt/agent-platform/docker && docker compose down"
```

---

## 五、服务端口

| 服务 | 端口 | 说明 |
|------|------|------|
| Gateway | 9000 | API 网关 |
| Frontend | 8888 | Web 前端 |
| Chat Service | 50001 | 对话服务 |
| Knowledge Service | 50002 | 知识库服务 |
| Memory Service | 50003 | 记忆服务 |
| A2A Service | 50004 | 跨服务通信 |
| MCP Service | 50005 | 工具协议（浏览器 Agent） |
| Agent Service | 50006 | Agent 编排 |
| Harness Service | 50007 | 运维治理 |
| Qdrant | 6333 | 向量数据库 |
| MongoDB | 27017 | 文档数据库 |
| Redis | 6379 | 缓存 |

---

## 六、测试 API

```bash
# 健康检查
curl http://192.168.10.100:9000/health

# 测试 Chat
curl -X POST http://192.168.10.100:9000/api/v2/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "你好", "tenant_id": "test"}'

# MCP 工具列表
curl http://192.168.10.100:9000/api/v2/mcp/tools
```

---

## 七、部署检查清单

**部署前：**
- [ ] `go build ./pkg/... ./services/...` 编译通过
- [ ] config.yaml 配置正确
- [ ] SSH 密钥可用

**部署后：**
- [ ] `docker compose ps` 所有服务 Up
- [ ] `/health` 返回 healthy
- [ ] Chat API 可调用

---

## 八、故障排查

### 服务无法启动

```bash
# 查看日志
docker logs --tail 100 docker-mcp-service-1

# 检查端口
netstat -tlnp | grep 9000
```

### 容器频繁重启

```bash
docker inspect docker-mcp-service-1 | grep -A 5 "State"
```

### 网络问题

```bash
docker network ls
docker network inspect agent-network
```

---

## 九、注意事项

1. **修改代码后** → 必须 `docker compose build` + `docker compose up -d`
2. **修改配置后** → 只需 `docker compose restart`
3. **MaxSteps = 50** → 浏览器 Agent 最大步数已优化
4. **MCP Service 镜像 770MB** → 包含 Chromium，首次构建约 15 分钟

---

## 十、快速命令汇总

```bash
# 部署全部（约 15 分钟）
ssh -i ~/.ssh/demo_deploy_key root@192.168.10.100 "cd /opt/agent-platform/docker && docker compose build && docker compose up -d"

# 部署单个服务（约 2 分钟，不含 mcp）
ssh -i ~/.ssh/demo_deploy_key root@192.168.10.100 "cd /opt/agent-platform/docker && docker compose build chat-service && docker compose up -d chat-service"

# 查看状态
ssh -i ~/.ssh/demo_deploy_key root@192.168.10.100 "docker ps"

# 查看日志
ssh -i ~/.ssh/demo_deploy_key root@192.168.10.100 "docker logs -f docker-mcp-service-1"

# 重启
ssh -i ~/.ssh/demo_deploy_key root@192.168.10.100 "cd /opt/agent-platform/docker && docker compose restart"

# 健康检查
curl http://192.168.10.100:9000/health
```

---

## 十一、访问地址

| 类型 | 地址 |
|------|------|
| 前端 | http://192.168.10.100:8888 |
| API | http://192.168.10.100:9000 |
| Health | http://192.168.10.100:9000/health |
| MCP Tools | http://192.168.10.100:9000/api/v2/mcp/tools |