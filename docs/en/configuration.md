# Configuration

> How Agent Platform services are configured. [中文](../zh-CN/configuration.md)

Each service reads a YAML config file at startup. Sensitive values (notably the LLM API key) are **not** stored in the committed config; they are injected via environment variables.

## Config File Layout

Each service has its own config at `services/<service>/config.yaml`. In Docker, it is mounted read-only into the container at `/app/config.yaml`:

```yaml
# docker-compose volume (root compose)
volumes:
  - ./services/chat-service/config.yaml:/app/config.yaml:ro
```

Example (`services/chat-service/config.yaml`):

```yaml
server:
  grpc_port: 50001
  http_port: 8081

database:
  sqlite:
    path: /app/data/chat.db

llm:
  provider: dashscope
  api_key: ""  # injected via OPENAI_API_KEY env var
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

## Config Loader & Env Overrides

The loader (`pkg/config/config.go`) parses the YAML directly — there is **no** `${VAR}` interpolation in the file. After parsing, it applies environment-variable overrides via `applyEnvOverrides()`:

| Environment Variable | Overrides | Required |
|----------------------|-----------|----------|
| `OPENAI_API_KEY` | `llm.api_key` | **Yes** (LLM calls fail without it) |
| `LLM_PROVIDER` | `llm.provider` | No |
| `LLM_MODEL` | `llm.model` | No |
| `QDRANT_URL` | `qdrant.url` | No |
| `MONGODB_URL` | `mongodb.url` | No |
| `REDIS_URL` | `redis.url` | No |

So the committed `llm.api_key` is intentionally blank; the real key is supplied through `OPENAI_API_KEY`.

## `.env` Mechanism

Docker Compose loads a `.env` file at the repository root into each LLM-using service via `env_file`:

```yaml
# docker/docker-compose.simple.yaml
chat-service:
  build: { ... }
  env_file: .env          # <-- injects OPENAI_API_KEY etc.
  volumes:
    - ./services/chat-service/config.yaml:/app/config.yaml:ro
```

The production compose (`docker/docker-compose.yaml`) uses `env_file: ../.env` (relative to the `docker/` directory).

`.env` is **gitignored** — never commit a real key. A template is provided:

```bash
cp .env.example .env
# then edit .env and set OPENAI_API_KEY=sk-...
```

`.env.example`:

```ini
# LLM API Key (required) - Alibaba DashScope (Qwen)
OPENAI_API_KEY=your-dashscope-api-key-here

# Optional overrides (config loader applies these over config.yaml)
# LLM_PROVIDER=dashscope
# LLM_MODEL=qwen3.7-max-2026-06-08
# QDRANT_URL=http://qdrant:6333
# MONGODB_URL=mongodb://mongo:27017
# REDIS_URL=redis://redis:6379
```

## LLM Configuration

The default provider is Alibaba Cloud DashScope (通义千问) via its OpenAI-compatible endpoint:

| Field | Value |
|-------|-------|
| `llm.provider` | `dashscope` |
| `llm.base_url` | `https://dashscope.aliyuncs.com/compatible-mode/v1` |
| `llm.model` | `qwen3.7-max-2026-06-08` |
| `llm.embedding_model` | `text-embedding-v3` |
| `llm.api_key` | via `OPENAI_API_KEY` |

Get a DashScope key at <https://dashscope.console.aliyun.com/>.

## Per-Service Ports

| Service | `grpc_port` |
|---------|-------------|
| chat-service | 50001 |
| knowledge-service | 50002 |
| memory-service | 50003 |
| a2a-service | 50004 |
| mcp-service | 50005 |
| agent-service | 50006 |
| harness-service | 50007 |
| gateway | HTTP 9000 |

## Security Notes

- Never hardcode `api_key` in `config.yaml`. Keep it blank and use `OPENAI_API_KEY`.
- `.env` is gitignored; `.env.example` carries only placeholders.
- If a key was ever committed, rotate it in the provider console.

## Further Reading

- [Deployment](./deployment.md)
- [Architecture](./architecture.md)
