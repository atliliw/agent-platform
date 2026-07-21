# 配置

> Agent Platform 各服务的配置方式。 [English](../en/configuration.md)

每个服务在启动时读取一个 YAML 配置文件。真实 `llm.api_key` 写在 `config.yaml`（已 **gitignore**）；提交到仓库的 `config.example.yaml` 是模板。用 `scripts/init-config.sh` 生成真实配置。

## 配置文件布局

每个服务在 `services/<service>/config.yaml`（已 gitignore）有自己的配置，并在 `services/<service>/config.example.yaml` 提交了模板。Docker 中将 `config.yaml` 以只读方式挂载到容器 `/app/config.yaml`：

```yaml
# docker-compose 卷挂载（根 compose）
volumes:
  - ./services/chat-service/config.yaml:/app/config.yaml:ro
```

模板（`services/chat-service/config.example.yaml` - 已提交）：

```yaml
server:
  grpc_port: 50001
  http_port: 8081

database:
  sqlite:
    path: /app/data/chat.db

llm:
  provider: dashscope
  api_key: "<your-api-key>"  # 由 scripts/init-config.sh 填入
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

## 生成配置

`scripts/init-config.sh` 将每个 `config.example.yaml` 复制为 `config.yaml` 并填入 key：

```bash
# 把你的 DashScope key 填入每个 services/*/config.yaml
bash scripts/init-config.sh sk-your-dashscope-key

# Windows PowerShell
pwsh scripts/init-config.ps1 sk-your-dashscope-key

# 不带参数：复制模板（占位 <your-api-key>），手动编辑
bash scripts/init-config.sh
```

`config.yaml` 已被 **gitignore**--真实 key 不会提交。

## 配置加载器

加载器（`pkg/config/config.go`）直接解析 YAML--文件中**没有** `${VAR}` 插值，**也没有**环境变量覆盖。YAML 是唯一真相源：所有值（含 `llm.api_key`）都按 `config.yaml` 中的原值读取。

要改值，编辑 `config.yaml`（或重新生成前编辑 `config.example.yaml`）。按服务覆盖（如某服务用不同模型）就是编辑该服务的 `config.yaml`。

## LLM 配置

默认使用阿里云 DashScope（通义千问）的 OpenAI 兼容接口：

| 字段 | 值 |
|------|-----|
| `llm.provider` | `dashscope` |
| `llm.base_url` | `https://dashscope.aliyuncs.com/compatible-mode/v1` |
| `llm.model` | `qwen3.7-max-2026-06-08` |
| `llm.embedding_model` | `text-embedding-v3` |
| `llm.api_key` | 你的 DashScope key，写在 `config.yaml` |

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

- 真实 key 只存在于 `config.yaml`（已 gitignore）。`config.example.yaml` 仅含 `<your-api-key>` 占位符。
- 切勿提交真实 key。若曾被提交过，请在 provider 控制台轮换并重写 git 历史。
- `scripts/init-config.sh` 只把 key 写入被 gitignore 的文件。

## 延伸阅读

- [部署](./deployment.md)
- [架构](./architecture.md)
