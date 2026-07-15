# Deployment

> Deploying Agent Platform with Docker Compose. [中文](../zh-CN/deployment.md)

## Prerequisites

- Go ≥ 1.22 (for building from source)
- Docker + Docker Compose
- `protoc`, `protoc-gen-go`, `protoc-gen-go-grpc` (for `make proto`)
- Node.js ≥ 18 (frontend development only)
- A DashScope (Qwen) API key

## Compose Files

There are two working compose files:

| File | Stack | env_file |
|------|-------|----------|
| `docker/docker-compose.simple.yaml` | Minimal: all services + Qdrant, MongoDB, Redis | `../.env` |
| `docker/docker-compose.yaml` | Full: above + Obscura stealth browser + OpenTelemetry Collector + frontend | `../.env` |

> ⚠️ The Makefile target `make run-dev` references `docker/docker-compose.dev.yaml`, which **does not exist** in the repo. Use `make run-prod` (the `docker/docker-compose.yaml` stack) or `docker/docker-compose.simple.yaml` instead.

## Services & Ports

| Service | Container Port | Host Port |
|---------|----------------|-----------|
| gateway | 9000 | 9000 |
| chat-service | 50001 | 50001 |
| knowledge-service | 50002 | 50002 |
| memory-service | 50003 | 50003 |
| a2a-service | 50004 | 50004 |
| mcp-service | 50005 | 50005 |
| agent-service | 50006 | 50006 |
| harness-service | 50007 | 50007 |
| mcp-demo-server | 50009 | 50009 |

## Infrastructure Ports

| Component | Host Port |
|-----------|-----------|
| Qdrant (HTTP / gRPC) | 6333 / 6334 |
| MongoDB | 27017 |
| Redis | 6379 |
| Obscura stealth browser (CDP) | 9222 |
| OpenTelemetry Collector (gRPC / HTTP) | 4317 / 4318 |
| Frontend (prod compose) | 8888 |

## Steps

```bash
# 1. Configure environment
cp .env.example .env
#   edit .env -> OPENAI_API_KEY=sk-...

# 2. (Optional) regenerate protobuf & build locally
make proto
make build

# 3. Start the stack
make run-prod            # uses docker/docker-compose.yaml
# or, for the minimal stack:
# docker compose -f docker/docker-compose.simple.yaml up -d
```

After startup:

- Gateway API: `http://localhost:9000`
- Frontend (prod compose): `http://localhost:8888`
- Health check: `GET http://localhost:9000/health`
- Readiness: `GET http://localhost:9000/ready`

## Health Checks

```bash
curl http://localhost:9000/health
# {"status":"healthy","services":{...}}
```

## Stop / Rebuild

```bash
make stop            # stop all compose environments
make docker-build    # rebuild images (prod compose)
make clean           # remove bin/, generated pb/, and dev volumes
```

## Image Build & Push

```bash
make docker-build    # build all service images
make docker-push     # push to registry (configure registry in compose)
```

## Kubernetes (optional)

The Makefile exposes k8s targets (expects manifests under `k8s/`):

```bash
make k8s-deploy
make k8s-status
make k8s-delete
```

## Notes

- LLM services load `OPENAI_API_KEY` from `.env` via `env_file`. Without a valid key, LLM-dependent calls will fail.
- The production compose includes the Obscura stealth browser (`h4ckf0r0day/obscura`) used by the MCP browser/XHS tools, wired at `ws://obscura:9222/devtools/browser`.
- OpenTelemetry traces/metrics are sent to the collector at `otel-collector:4317`.

## Further Reading

- [Configuration](./configuration.md)
- [Development](./development.md)
- [Architecture](./architecture.md)
