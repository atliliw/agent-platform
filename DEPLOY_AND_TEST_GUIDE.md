# Agent Platform 完整部署和测试指南

## 一、本地修改汇总

### 已修复的问题

| 问题 | 文件 | 修改 |
|------|------|------|
| 端口配置错误 | `services/agent-service/config.yaml` | grpc_port: 50007 → 50006 |
| 端口配置错误 | `services/harness-service/config.yaml` | grpc_port: 50006 → 50007 |
| Gateway 配置错误 | `services/gateway/config.yaml` | 修正 agent/harness 端口 |
| Chat 缺少记忆功能 | `services/chat-service/internal/service/chat_service.go` | 添加长期记忆集成 |
| Chat 缺少 Memory 地址 | `services/chat-service/config.yaml` | 添加 memory: memory-service:50003 |

### 新增文件

| 文件 | 说明 |
|------|------|
| `FIX_REPORT.md` | 端口修复报告 |
| `fix-and-deploy.ps1` | Windows 一键部署脚本 |
| `fix-and-deploy.sh` | Linux 一键部署脚本 |
| `docs/CHAT_MEMORY_INTEGRATION.md` | 记忆功能设计方案 |
| `docs/CHAT_MEMORY_INTEGRATION_COMPLETE.md` | 记忆功能实现报告 |

## 二、服务器部署步骤

### 方式 1: 使用一键脚本 (推荐)

```bash
# SSH 到服务器
ssh user@192.168.10.100

# 进入项目目录
cd /path/to/agent-platform

# 同步代码（如果用 Git）
git pull

# 或者从本地复制修改的文件
# 本地执行:
scp -r services/chat-service user@192.168.10.100:/path/to/agent-platform/services/
scp -r services/agent-service user@192.168.10.100:/path/to/agent-platform/services/
scp -r services/harness-service user@192.168.10.100:/path/to/agent-platform/services/
scp -r services/gateway user@192.168.10.100:/path/to/agent-platform/services/
scp fix-and-deploy.sh user@192.168.10.100:/path/to/agent-platform/

# 服务器执行
chmod +x fix-and-deploy.sh
./fix-and-deploy.sh
```

### 方式 2: 手动部署

```bash
# 1. 停止服务
docker-compose down

# 2. 重新构建
docker-compose build --no-cache

# 3. 启动服务
docker-compose up -d

# 4. 查看状态
docker-compose ps

# 5. 查看日志
docker-compose logs -f
```

## 三、服务健康检查

### 检查所有服务状态

```bash
docker-compose ps
```

期望输出:
```
NAME                STATUS    PORTS
gateway             Up        0.0.0.0:9000->9000/tcp
chat-service        Up        0.0.0.0:50001->50001/tcp
knowledge-service   Up        0.0.0.0:50002->50002/tcp
memory-service      Up        0.0.0.0:50003->50003/tcp
a2a-service         Up        0.0.0.0:50004->50004/tcp
mcp-service         Up        0.0.0.0:50005->50005/tcp
agent-service       Up        0.0.0.0:50006->50006/tcp
harness-service     Up        0.0.0.0:50007->50007/tcp
qdrant              Up        0.0.0.0:6333-6334/tcp
mongodb             Up        0.0.0.0:27017->27017/tcp
redis               Up        0.0.0.0:6379->6379/tcp
```

### 检查各服务连通性

```bash
# Gateway
curl http://192.168.10.100:9000/api/v2/mcp/tools

# Chat Service - 需要 gRPC 客户端，通过 Gateway 测试
curl -X POST http://192.168.10.100:9000/api/v2/chat \
  -H "Content-Type: application/json" \
  -d '{"message":"你好","tenant_id":"test"}'

# Agent Service - 通过 Gateway 测试
curl http://192.168.10.100:9000/api/v2/agents

# Memory Service - 通过 Gateway 测试（如果暴露了 API）
curl http://192.168.10.100:9000/api/v2/memory/session/test
```

## 四、功能测试

### 4.1 基础对话测试

```bash
curl -X POST http://192.168.10.100:9000/api/v2/chat \
  -H "Content-Type: application/json" \
  -d '{
    "message": "你好，请介绍一下你自己",
    "tenant_id": "test-tenant",
    "user_id": "test-user"
  }'
```

期望响应:
```json
{
  "code": 0,
  "data": {
    "session_id": "xxx",
    "content": "你好！我是...",
    "total_tokens": 100,
    "agent_states": []
  }
}
```

### 4.2 工具调用测试

```bash
# 测试计算器
curl -X POST http://192.168.10.100:9000/api/v2/mcp/call \
  -H "Content-Type: application/json" \
  -d '{"name":"calculator","arguments":"{\"expression\":\"10+5*2\"}"}'

# 测试时间工具
curl -X POST http://192.168.10.100:9000/api/v2/mcp/call \
  -H "Content-Type: application/json" \
  -d '{"name":"time","arguments":"{}"}'

# 测试数据分析
curl -X POST http://192.168.10.100:9000/api/v2/mcp/call \
  -H "Content-Type: application/json" \
  -d '{"name":"data_analysis","arguments":"{\"data\":[1,2,3,4,5,6,7,8,9,10]}"}'
```

### 4.3 Agent + 工具组合测试

```bash
curl -X POST http://192.168.10.100:9000/api/v2/chat \
  -H "Content-Type: application/json" \
  -d '{
    "message": "帮我计算 15 * 8 + 25 的结果",
    "tenant_id": "test"
  }'
```

期望: AI 会调用 calculator 工具，返回正确结果。

### 4.4 记忆功能测试 ★ 新增

```bash
# 第一轮：告诉 AI 你的信息
curl -X POST http://192.168.10.100:9000/api/v2/chat \
  -H "Content-Type: application/json" \
  -d '{
    "message": "我叫张三，我喜欢编程和看电影",
    "tenant_id": "memory-test",
    "user_id": "user-001"
  }'

# 记录返回的 session_id，假设是 "sess-123"

# 第二轮：在同一 session 继续对话（短期记忆）
curl -X POST http://192.168.10.100:9000/api/v2/chat \
  -H "Content-Type: application/json" \
  -d '{
    "session_id": "sess-123",
    "message": "我叫什么？",
    "tenant_id": "memory-test"
  }'

# 第三轮：新 session 测试长期记忆
curl -X POST http://192.168.10.100:9000/api/v2/chat \
  -H "Content-Type: application/json" \
  -d '{
    "message": "你知道我喜欢什么吗？",
    "tenant_id": "memory-test",
    "user_id": "user-001"
  }'
```

期望结果:
- 第二轮应该直接回答（短期记忆）
- 第三轮应该能回忆起信息（长期记忆，如果 Memory Service 正常）

### 4.5 多 Agent 测试

```bash
curl -X POST http://192.168.10.100:9000/api/v2/agents/execute \
  -H "Content-Type: application/json" \
  -d '{
    "message": "帮我搜索今天北京的天气，并分析是否适合出行",
    "entry_agent": "main-agent",
    "tenant_id": "test"
  }'
```

## 五、常见问题排查

### 问题 1: 服务启动失败

```bash
# 查看具体服务日志
docker-compose logs chat-service

# 常见错误:
# - "connection refused" → 依赖服务未启动
# - "port already in use" → 端口冲突
# - "API key invalid" → LLM 配置错误
```

### 问题 2: 对话返回错误

```bash
# 检查 LLM 配置
docker-compose exec chat-service cat /app/config.yaml

# 测试 DashScope API
curl -X POST "https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"qwen-max","messages":[{"role":"user","content":"你好"}]}'
```

### 问题 3: Agent Service 连接失败

```bash
# 检查端口配置
docker-compose exec chat-service cat /app/config.yaml | grep agent

# 应该显示: agent: agent-service:50006

# 检查 Agent Service 是否运行
docker-compose ps agent-service
```

### 问题 4: Memory Service 未工作

```bash
# 检查 Memory Service 日志
docker-compose logs memory-service

# 检查 Chat Service 是否连接 Memory
docker-compose logs chat-service | grep "Memory Service"
```

## 六、前端部署

### 构建前端

```bash
cd frontend

# 安装依赖
npm install

# 配置 API 地址
# 修改 .env 或 vite.config.ts
VITE_API_URL=http://192.168.10.100:9000

# 构建
npm run build

# 产物在 frontend/dist/
```

### 部署前端

**方式 1: Nginx**

```nginx
server {
    listen 80;
    server_name 192.168.10.100;

    # 前端静态文件
    location / {
        root /path/to/frontend/dist;
        try_files $uri $uri/ /index.html;
    }

    # API 反向代理
    location /api/ {
        proxy_pass http://gateway:9000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

**方式 2: 直接通过 Gateway**

Gateway 可以配置静态文件服务，直接托管前端。

## 七、监控和维护

### 日志查看

```bash
# 实时日志
docker-compose logs -f

# 特定服务
docker-compose logs -f chat-service

# 最近 100 行
docker-compose logs --tail=100 chat-service
```

### 数据备份

```bash
# MongoDB
docker-compose exec mongodb mongodump --out /backup

# Qdrant
docker-compose exec qdrant curl -X POST http://localhost:6333/collections/documents/snapshots

# Redis (可选)
docker-compose exec redis redis-cli SAVE
```

### 性能监控

```bash
# 容器资源使用
docker stats

# 服务健康
docker-compose ps
```

## 八、API 端点汇总

| 端点 | 方法 | 说明 |
|------|------|------|
| `/api/v2/chat` | POST | 对话接口 |
| `/api/v2/chat/stream` | POST | 流式对话 |
| `/api/v2/sessions` | GET | 会话列表 |
| `/api/v2/sessions/:id` | GET | 获取会话 |
| `/api/v2/sessions/:id` | DELETE | 删除会话 |
| `/api/v2/mcp/tools` | GET | 工具列表 |
| `/api/v2/mcp/call` | POST | 调用工具 |
| `/api/v2/agents` | GET | Agent 列表 |
| `/api/v2/agents/:id` | GET | 获取 Agent |
| `/api/v2/agents/execute` | POST | 执行多 Agent |
| `/api/v2/knowledge/search` | POST | 知识库搜索 |
| `/api/v2/multi-agent/chat` | POST | 多 Agent 对话 |

## 九、下一步建议

1. **配置外部工具 API**: web_search 和 weather 需要配置 API Key
2. **部署前端**: 构建并部署 Web UI
3. **配置 HTTPS**: 使用 Nginx + SSL
4. **添加监控**: Prometheus + Grafana
5. **配置告警**: 服务异常时自动通知

---

## 联系支持

如有问题，请查看:
- 日志文件: `docker-compose logs`
- 配置文件: `services/*/config.yaml`
- 文档目录: `docs/`