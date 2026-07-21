# Configuration

> How Agent Platform services are configured. [中文](../zh-CN/configuration.md)

Each service reads a YAML config file at startup. The real `llm.api_key` lives in `config.yaml`, which is **gitignored**; the committed `config.example.yaml` is the template. Generate the real configs with `scripts/init-config.sh`.

## Config File Layout

Each service has its own config at `services/<service>/config.yaml` (gitignored) plus a committed template at `services/<service>/config.example.yaml`. In Docker, `config.yaml` is mounted read-only into the container at `/app/config.yaml`:

```yaml
# docker-compose volume (root compose)
volumes:
  - ./services/chat-service/config.yaml:/app/config.yaml:ro
```

Template (`services/chat-service/config.example.yaml` - committed):

```yaml
server:
  grpc_port: 50001
  http_port: 8081

database:
  sqlite:
    path: /app/data/chat.db

llm:
  provider: dashscope
  api_key: "<your-api-key>"  # filled in by scripts/init-config.sh
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

## Generating Configs

`scripts/init-config.sh` copies each `config.example.yaml` to `config.yaml` and fills in the key:

```bash
# Fill your DashScope key into every services/*/config.yaml
bash scripts/init-config.sh sk-your-dashscope-key

# Windows PowerShell
pwsh scripts/init-config.ps1 sk-your-dashscope-key

# Without an argument: copies templates with a <your-api-key> placeholder to edit manually
bash scripts/init-config.sh
```

`config.yaml` is **gitignored** - your key is never committed.

## Config Loader

The loader (`pkg/config/config.go`) parses the YAML directly - there is **no** `${VAR}` interpolation and **no** environment-variable override. The YAML is the single source of truth: every value (including `llm.api_key`) is read as-is from `config.yaml`.

To change a value, edit `config.yaml` (or `config.example.yaml` before regenerating). Per-service overrides (e.g. a different model for one service) are just edits to that service's `config.yaml`.

## LLM Configuration

The default provider is Alibaba Cloud DashScope (通义千问) via its OpenAI-compatible endpoint:

| Field | Value |
|-------|-------|
| `llm.provider` | `dashscope` |
| `llm.base_url` | `https://dashscope.aliyuncs.com/compatible-mode/v1` |
| `llm.model` | `qwen3.7-max-2026-06-08` |
| `llm.embedding_model` | `text-embedding-v3` |
| `llm.api_key` | your DashScope key, set in `config.yaml` |

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

- The real key lives only in `config.yaml` (gitignored). `config.example.yaml` carries only a `<your-api-key>` placeholder.
- Never commit a real key. If one was ever committed, rotate it in the provider console and rewrite git history.
- `scripts/init-config.sh` writes keys only to gitignored files.

## Further Reading

- [Deployment](./deployment.md)
- [Architecture](./architecture.md)
