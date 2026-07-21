# Deployment

> Complete deployment guide. Clone the repo and run - no local Go/protoc/Node toolchain required for the Docker path. [中文](../zh-CN/deployment.md)

There are two ways to run Agent Platform:

- **Option A - Docker Compose (recommended):** builds every service image from source inside Docker. You only need Docker installed. This is the clone-and-deploy path.
- **Option B - Local build:** build binaries with `make build` and run them directly (or via Compose with pre-built images). Needs Go and optionally Node.

---

## Option A - Docker Compose (recommended)

### 1. Prerequisites

Install Docker and Docker Compose only:

| Tool | Version | Install |
|------|---------|---------|
| Docker | ≥ 20.10 | <https://docs.docker.com/get-docker/> |
| Docker Compose | v2 (bundled with Docker Desktop) | <https://docs.docker.com/compose/install/> |

Verify:

```bash
docker --version
docker compose version
```

> No Go, protoc, or Node.js is required for this path - Docker builds everything from source.

### 2. Clone

```bash
git clone https://github.com/atliliw/agent-platform.git
cd agent-platform
```

### 3. Generate service configs (required)

Each service reads its own `config.yaml` (gitignored - it holds your real key). Generate them from the committed templates and inject your DashScope key in one step:

```bash
bash scripts/init-config.sh sk-your-dashscope-key
```

This creates `services/*/config.yaml` from `services/*/config.example.yaml` and fills in `llm.api_key`. Get a DashScope (Qwen) key at <https://dashscope.console.aliyun.com/>. Without a valid key, LLM-dependent services fail.

> On Windows PowerShell: `pwsh scripts/init-config.ps1 sk-your-dashscope-key`
> Without an argument the script copies templates with a `<your-api-key>` placeholder for you to edit manually. `config.yaml` is gitignored, so your key is never committed.

### 4. Choose a compose file

| File | Stack | Use when |
|------|-------|----------|
| `docker/docker-compose.yaml` | Full: all services + Qdrant + MongoDB + Redis + Obscura stealth browser + OpenTelemetry Collector + frontend | **Default - recommended** |
| `docker/docker-compose.simple.yaml` | Minimal: services + Qdrant + MongoDB + Redis only (no otel/obscura/frontend) | Quick local run without observability/browser engine |

### 5. Build & start

```bash
# Full stack (recommended)
docker compose -f docker/docker-compose.yaml up -d --build

# Or minimal stack
# docker compose -f docker/docker-compose.simple.yaml up -d --build
```

The first build compiles every Go service (multi-stage build) and the Vue frontend. It uses China-friendly mirrors (`goproxy.cn`, Aliyun APK, npmmirror) so it works well on mainland networks. Expect several minutes on first run; subsequent starts are fast.

### 6. Verify

```bash
# Health check
curl http://localhost:9000/health
# {"status":"healthy","services":{...}}

# Readiness
curl http://localhost:9000/ready
# {"ready":true}
```

Access:

| Surface | URL |
|---------|-----|
| Gateway API | <http://localhost:9000> |
| Frontend (full stack) | <http://localhost:8888> |
| Qdrant dashboard | <http://localhost:6333/dashboard> |
| MongoDB | `localhost:27017` |
| Redis | `localhost:6379` |

### 7. Operations

```bash
# View logs (follow)
docker compose -f docker/docker-compose.yaml logs -f
# One service
docker compose -f docker/docker-compose.yaml logs -f chat-service

# Status
docker compose -f docker/docker-compose.yaml ps

# Stop
docker compose -f docker/docker-compose.yaml down

# Rebuild a single service after code changes
docker compose -f docker/docker-compose.yaml up -d --build chat-service

# Stop and remove volumes (full reset)
docker compose -f docker/docker-compose.yaml down -v
```

---

## Option B - Local build

Use this if you want to run binaries directly or develop without rebuilding Docker images.

### 1. Prerequisites

| Tool | Version | Notes |
|------|---------|-------|
| Go | ≥ 1.22 | <https://go.dev/dl/> |
| protoc + plugins | latest | Only needed if you edit `.proto` files (generated code is committed in `pkg/pb/`) |
| Node.js | ≥ 18 | Only for frontend dev |
| Docker | ≥ 20.10 | Still needed for Qdrant/MongoDB/Redis (or run them separately) |

### 2. Generate service configs

```bash
bash scripts/init-config.sh sk-...
```

Edit per-service overrides in `services/<service>/config.yaml` if needed (e.g. a different model).

### 3. Generate & build

```bash
make proto     # optional: regenerate protobuf (committed code already exists)
make build     # build all services -> bin/
```

### 4. Start infrastructure + services

```bash
# Start data stores via Compose (minimal stack minus app services), or run your own
docker compose -f docker/docker-compose.simple.yaml up -d qdrant mongodb redis

# Run a service binary (example: chat-service) - config.yaml already holds the key
./bin/chat-service
```

---

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
| Frontend (full stack) | 8888 |

## Configuration

- Each service reads `services/<service>/config.yaml` (mounted read-only to `/app/config.yaml`).
- `config.yaml` is gitignored and holds the real `llm.api_key`; `config.example.yaml` is the committed template. See [Configuration](./configuration.md).

## Troubleshooting

| Problem | Fix |
|---------|-----|
| LLM calls fail / services crash | Run `bash scripts/init-config.sh sk-...` so `services/*/config.yaml` holds a valid DashScope key |
| `make run-dev` fails | It references `docker/docker-compose.dev.yaml` which does not exist. Use `docker compose -f docker/docker-compose.yaml up -d` instead |
| Port already in use | Stop the conflicting process or remap the host port in the compose file |
| Docker build fails on Go modules | The Dockerfile sets `GOPROXY=https://goproxy.cn,direct`; if blocked, edit the Dockerfile to use a reachable proxy |
| Frontend build fails | Ensure `frontend/nginx.conf` exists (it does in the repo); run `cd frontend && npm ci && npm run build` to debug locally |
| `configs/agents` mount warnings | The directory is created automatically; agent-service does not require files there at startup |

## Kubernetes (optional)

The Makefile exposes k8s targets (expects manifests under `k8s/`):

```bash
make k8s-deploy
make k8s-status
make k8s-delete
```

## Further Reading

- [Configuration](./configuration.md)
- [Development](./development.md)
- [Architecture](./architecture.md)
