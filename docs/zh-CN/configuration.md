# 配置

> Agent Platform 各服务的配置方式。 [English](../en/configuration.md)

每个服务在启动时读取一个 YAML 配置文件。敏感值（尤其是 LLM API Key）**不**保存在已提交的配置中，而是通过环境变量注入。

## 配置文件布局

每个服务在 `services/<service>/config.yaml` 有自己的配置。Docker 中以只读方式挂载到容器 `/app/config.yaml`：

```yaml
# docker-compose 卷挂载（根 compose）
volumes:
  - ./services/chat-service/config.yaml:/app/config.yaml:ro
```

示例（`services/chat-service/config.yaml`）：

```yaml
server:
  grpc_port: 50001
  http_port: 8081

database:
  sqlite:
    path: /app/data/chat.db

llm:
  provider: dashscope
  api_key: ""  # 通过 OPENAI_API_KEY 环境变量注入
  base_url: "https://dashscope.aliyuncs.com/compatible-mode/v1"
  model: "qwen3.7-max-2026-06-08"
  embedding_model: "text-embedding-v3"
  max_tokens: 4096

qdrant:
  url: http://qdrant:6333
  collection: documents

redis:
  url: redis://redis:6379

services:
  mcp: mcp-service:50005
  agent: agent-service:50006
  memory: memory-service:50003
  harness: harness-service:50007

logging:
  level: info
  format: json
```

## 配置加载器与环境变量覆盖

加载器（`pkg/config/config.go`）直接解析 YAML--文件中**没有** `${VAR}` 插值。解析后通过 `applyEnvOverrides()` 应用环境变量覆盖：

| 环境变量 | 覆盖 | 是否必填 |
|---------|------|---------|
| `OPENAI_API_KEY` | `llm.api_key` | **是**（无则 LLM 调用失败） |
| `LLM_PROVIDER` | `llm.provider` | 否 |
| `LLM_MODEL` | `llm.model` | 否 |
| `QDRANT_URL` | `qdrant.url` | 否 |
| `MONGODB_URL` | `mongodb.url` | 否 |
| `REDIS_URL` | `redis.url` | 否 |

因此提交的 `llm.api_key` 故意为空，真实 Key 通过 `OPENAI_API_KEY` 提供。

## `.env` 机制

Docker Compose 通过 `env_file` 将仓库根目录的 `.env` 加载进每个使用 LLM 的服务：

```yaml
# docker/docker-compose.simple.yaml
chat-service:
  build: { ... }
  env_file: .env          # <-- 注入 OPENAI_API_KEY 等
  volumes:
    - ./services/chat-service/config.yaml:/app/config.yaml:ro
```

生产 compose（`docker/docker-compose.yaml`）使用 `env_file: ../.env`（相对 `docker/` 目录）。

`.env` 已被 **gitignore**--切勿提交真实 Key。提供模板：

```bash
cp .env.example .env
# 然后编辑 .env，设置 OPENAI_API_KEY=sk-...
```

`.env.example`：

```ini
# LLM API Key（必填）- 阿里云 DashScope（通义千问）
OPENAI_API_KEY=your-dashscope-api-key-here

# 可选覆盖（加载器会用环境变量覆盖 config.yaml）
# LLM_PROVIDER=dashscope
# LLM_MODEL=qwen3.7-max-2026-06-08
# QDRANT_URL=http://qdrant:6333
# MONGODB_URL=mongodb://mongo:27017
# REDIS_URL=redis://redis:6379
```

## LLM 配置

默认使用阿里云 DashScope（通义千问）的 OpenAI 兼容接口：

| 字段 | 值 |
|------|-----|
| `llm.provider` | `dashscope` |
| `llm.base_url` | `https://dashscope.aliyuncs.com/compatible-mode/v1` |
| `llm.model` | `qwen3.7-max-2026-06-08` |
| `llm.embedding_model` | `text-embedding-v3` |
| `llm.api_key` | 通过 `OPENAI_API_KEY` |

在 <https://dashscope.console.aliyun.com/> 获取 DashScope Key。

## 各服务端口

| 服务 | `grpc_port` |
|------|-------------|
| chat-service | 50001 |
| knowledge-service | 50002 |
| memory-service | 50003 |
| a2a-service | 50004 |
| mcp-service | 50005 |
| agent-service | 50006 |
| harness-service | 50007 |
| gateway | HTTP 9000 |

## 安全注意

- 切勿在 `config.yaml` 中硬编码 `api_key`。保持为空，使用 `OPENAI_API_KEY`。
- `.env` 已 gitignore；`.env.example` 仅含占位符。
- 若 Key 曾被提交过，请在 provider 控制台轮换。

## 延伸阅读

- [部署](./deployment.md)
- [架构](./architecture.md)
