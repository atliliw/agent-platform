# Agent Platform 修复报告

## 修复时间
2026-06-08

## 发现的问题

### 1. 端口配置不一致 ❌ → ✅ 已修复

| 服务 | docker-compose.yml | 原 config.yaml | 修复后 |
|------|-------------------|----------------|--------|
| agent-service | 50006 | 50007 (错误) | 50006 ✅ |
| harness-service | 50007 | 50006 (错误) | 50007 ✅ |

**问题**：Chat Service 连接 Agent Service 时使用错误的端口 50007，而 Agent Service 实际监听 50006。

### 2. Gateway 配置错误 ❌ → ✅ 已修复

原配置：
```yaml
harness: harness-service:50006  # 错误
agent: agent-service:50007      # 错误
```

修复后：
```yaml
harness: harness-service:50007
agent: agent-service:50006
```

## 修复的文件

1. `services/agent-service/config.yaml` - grpc_port: 50007 → 50006
2. `services/harness-service/config.yaml` - grpc_port: 50006 → 50007
3. `services/gateway/config.yaml` - 修正 agent 和 harness 端口
4. `services/chat-service/config.yaml` - 修正 agent 端口

## 当前端口映射表

| 服务 | Docker 端口 | gRPC 端口 | 状态 |
|------|-------------|-----------|------|
| gateway | 9000 | - | ✅ |
| chat-service | 50001 | 50001 | ✅ |
| knowledge-service | 50002 | 50002 | ✅ |
| memory-service | 50003 | 50003 | ✅ |
| a2a-service | 50004 | 50004 | ✅ |
| mcp-service | 50005 | 50005 | ✅ |
| agent-service | 50006 | 50006 | ✅ 已修复 |
| harness-service | 50007 | 50007 | ✅ 已修复 |
| qdrant | 6333/6334 | - | ✅ |
| mongodb | 27017 | - | ✅ |
| redis | 6379 | - | ✅ |

## 重新部署步骤

### 方式 1: 使用部署脚本

```powershell
# Windows PowerShell
cd D:\BaiduNetdiskDownload\LLM\agent-platform
.\deploy.ps1
```

```bash
# Linux/Mac
cd /path/to/agent-platform
./deploy.sh
```

### 方式 2: 手动部署

```bash
# 1. 停止旧服务
docker-compose down

# 2. 重新构建镜像
docker-compose build --no-cache

# 3. 启动服务
docker-compose up -d

# 4. 查看服务状态
docker-compose ps

# 5. 查看日志
docker-compose logs -f gateway chat-service agent-service
```

## 验证修复

```bash
# 1. 测试 Gateway 健康检查
curl http://localhost:9000/api/v2/mcp/tools

# 2. 测试对话接口
curl -X POST http://localhost:9000/api/v2/chat \
  -H "Content-Type: application/json" \
  -d '{"message":"你好","tenant_id":"test"}'

# 3. 测试 Agent 列表
curl http://localhost:9000/api/v2/agents

# 4. 测试多 Agent 执行
curl -X POST http://localhost:9000/api/v2/agents/execute \
  -H "Content-Type: application/json" \
  -d '{"message":"帮我搜索一下今天北京的天气","entry_agent":"main-agent"}'
```

## 常见问题排查

### 问题 1: 服务无法启动
```bash
# 查看日志
docker-compose logs [service-name]

# 常见原因:
# - 端口被占用: 修改 docker-compose.yml 端口映射
# - 依赖服务未启动: 检查 depends_on 配置
```

### 问题 2: LLM 调用失败
```bash
# 检查 API Key 是否有效
curl -X POST "https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"qwen3.7-max-2026-05-17","messages":[{"role":"user","content":"你好"}]}'
```

### 问题 3: Agent Service 连接失败
```bash
# 检查 Agent Service 是否运行
docker-compose ps agent-service

# 检查 Chat Service 日志中的连接信息
docker-compose logs chat-service | grep "Agent Service"
```

## 前端配置

确保前端 API 地址正确：

```typescript
// frontend/.env
VITE_API_URL=http://192.168.10.100:9000
```

或者修改 `frontend/src/api/client.ts`:

```typescript
const client = axios.create({
  baseURL: 'http://192.168.10.100:9000',  // 你的服务器 IP
  timeout: 60000,
});
```

## 下一步建议

1. **部署前端**: 将前端打包后部署到 Nginx 或 CDN
2. **配置 HTTPS**: 使用 Nginx 反向代理配置 SSL
3. **配置外部工具**: 配置 web_search 和 weather 的 API Key
4. **监控告警**: 添加 Prometheus + Grafana 监控
