# 配置总结

## 已完成的配置更新

### 1. 所有服务配置文件已更新

所有服务的 `config.yaml` 都已更新为使用 DashScope：

| 服务 | 配置文件 | 状态 |
|------|----------|------|
| Gateway | `services/gateway/config.yaml` | ✅ 已配置 |
| Chat Service | `services/chat-service/config.yaml` | ✅ 已配置 |
| Knowledge Service | `services/knowledge-service/config.yaml` | ✅ 已配置 |
| Memory Service | `services/memory-service/config.yaml` | ✅ 已配置 |
| MCP Service | `services/mcp-service/config.yaml` | ✅ 已配置 |
| Agent Service | `services/agent-service/config.yaml` | ✅ 已创建 |

### 2. DashScope 客户端已完整实现

**文件**: `pkg/llm/dashscope.go`

支持：
- Chat 对话
- ChatStream 流式对话
- Embed / EmbedBatch 向量嵌入

### 3. 配置结构已更新

**文件**: `pkg/config/config.go`

新增配置项：
- `llm.embedding_model` - 向量嵌入模型
- `tools.web_search` - 网络搜索工具配置
- `tools.weather` - 天气工具配置
- `forgetting` - 记忆遗忘配置
- `engine` - Agent 引擎配置

### 4. 你的 API Key 配置

```yaml
llm:
  provider: dashscope
  api_key: ""  # 通过 OPENAI_API_KEY 环境变量注入，勿提交真实 key
  base_url: "https://dashscope.aliyuncs.com/compatible-mode/v1"
  model: "qwen3.7-max-2026-05-17"
  embedding_model: "text-embedding-v3"
  max_tokens: 4096
```

---

## 使用方法

### 启动服务

```bash
# 使用 Docker Compose
docker-compose up -d

# 或使用 Makefile
make run-dev
```

### 调用 API

```bash
# 对话
curl -X POST http://localhost:9000/api/v2/chat \
  -H "Content-Type: application/json" \
  -d '{
    "message": "你好",
    "tenant_id": "test"
  }'
```

---

## 可选：配置外部工具 API

在 `services/mcp-service/config.yaml` 中配置：

```yaml
tools:
  web_search:
    api_key: "your-serpapi-key"
    provider: "serpapi"
  weather:
    api_key: "your-openweathermap-key"
    provider: "openweathermap"
```

---

## 文件变更清单

### 新建文件
1. `pkg/llm/dashscope.go` - DashScope 客户端完整实现
2. `services/agent-service/config.yaml` - Agent 服务配置
3. `configs/config.example.yaml` - 示例配置文件（带注释）
4. `services/mcp-service/internal/tools/real_tools.go` - 真实工具实现
5. `services/mcp-service/internal/tools/knowledge_search_tool.go` - 知识库搜索工具
6. `services/knowledge-service/internal/search/bm25.go` - BM25 搜索算法
7. `services/memory-service/internal/service/memory_service_with_forgetting.go` - 记忆遗忘服务

### 修改文件
1. `pkg/llm/client.go` - 添加 Streaming 实现，更新 NewClient
2. `pkg/config/config.go` - 添加新配置结构
3. 所有 `services/*/config.yaml` - 更新为 DashScope 配置
4. `services/mcp-service/internal/service/mcp_service.go` - 使用配置中的 API Key
5. `services/a2a-service/internal/service/a2a_service.go` - 真实任务执行
6. `services/knowledge-service/internal/repository/document_repository.go` - 使用真正的 BM25

---

## 下一步

1. **编译验证**: `make build`
2. **启动服务**: `docker-compose up -d` 或 `make run-dev`
3. **测试 API**: 调用对话接口验证 DashScope 是否正常工作
