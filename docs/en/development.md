# Development

> Local development guide for Agent Platform. [中文](../zh-CN/development.md)

## Prerequisites

Install the toolchain for your platform:

| Tool | Version | Install |
|------|---------|---------|
| Go | ≥ 1.22 | <https://go.dev/dl/> (verify: `go version`) |
| protoc + plugins | latest | Only if you edit `.proto` files - generated code is committed in `pkg/pb/`. Install protoc from <https://grpc.io/docs/protoc-installation/>, then `go install google.golang.org/protobuf/cmd/protoc-gen-go@latest` and `go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest` |
| Docker + Compose | ≥ 20.10 | <https://docs.docker.com/get-docker/> (used for Qdrant/MongoDB/Redis and the full stack) |
| Node.js | ≥ 18 | <https://nodejs.org/> (frontend dev only) |
| golangci-lint | latest | Optional, for `make lint`: <https://golangci-lint.run/usage/install/> |

Tip (mainland China): set `GOPROXY=https://goproxy.cn,direct` and `npm config set registry https://registry.npmmirror.com` for faster downloads.

## Project Structure

```
agent-platform/
├── docs/                   # Documentation (en/ + zh-CN/)
├── proto/                  # Protobuf definitions (common, chat, knowledge, memory, a2a, mcp, harness)
├── pkg/                    # Shared libraries
│   ├── llm/                # LLM client (OpenAI-compatible)
│   ├── qdrant/             # Qdrant client
│   ├── mongodb/            # MongoDB client
│   ├── redis/              # Redis client
│   ├── config/             # Config loading + env overrides
│   ├── agent/              # Agent engine primitives (handoff, store, etc.)
│   ├── browseragent/       # Browser automation + pool
│   ├── mcp/                # MCP client (stdio + streamable HTTP)
│   ├── xhs/                # XHS (小红书) client & signer
│   └── pb/                 # Generated protobuf code
├── services/               # Microservices (cmd/ + internal/)
│   ├── gateway/            # HTTP gateway (Gin)
│   ├── chat-service/
│   ├── knowledge-service/
│   ├── memory-service/
│   ├── a2a-service/
│   ├── mcp-service/
│   ├── agent-service/
│   ├── harness-service/
│   └── mcp-demo-server/
├── frontend/               # Vue 3 + Element Plus + Tailwind
├── docker/                 # Docker Compose configs + otel
├── configs/                # Example configs
├── Makefile
└── go.mod                  # module agent-platform
```

## Protobuf Generation

Protobuf code is generated with `protoc` using the `module=agent-platform` flag so output is written under `pkg/pb/`.

```bash
make proto          # generate all: common, chat, knowledge, memory, a2a, mcp, harness
```

Each `proto-<name>` sub-target generates one proto package. Re-run after editing any `.proto` file.

## Build

```bash
make build              # build all services -> bin/
make build-gateway      # build a single service -> bin/gateway
make build-<service>    # e.g. make build-chat-service
```

`bin/` is gitignored - build artifacts are never committed.

## Run

```bash
make run-prod           # docker/docker-compose.yaml (full stack)
# or use the minimal stack:
docker compose -f docker/docker-compose.simple.yaml up -d
```

> `make run-dev` references `docker/docker-compose.dev.yaml`, which does not exist. Prefer `make run-prod`.

Gateway: `http://localhost:9000` · Frontend: `http://localhost:8888` (prod compose).

## Testing

```bash
make test               # all tests, with -race
make test-chat-service  # tests for one service
make test-coverage      # coverage report -> coverage.html
```

Tests follow the AAA pattern (Arrange-Act-Assert) with descriptive names.

## Code Quality

```bash
make fmt                # go fmt ./...
make lint               # golangci-lint run ./...
make tidy               # go mod tidy
```

## Frontend

```bash
cd frontend
npm install
npm run dev             # Vite dev server on :5173
npm run build           # production build
```

## Configuration for Development

Generate service configs (fills `llm.api_key` in each `services/*/config.yaml`) before running services:

```bash
bash scripts/init-config.sh sk-your-dashscope-key
```

See [Configuration](./configuration.md) for details.

## Adding a New Service

1. **Scaffold** under `services/<new-service>/`:
   - `cmd/main.go` - entrypoint
   - `internal/` - handlers, service logic, models
   - `config.yaml` - service config (with blank `llm.api_key`)
   - `Dockerfile`
2. **Proto:** add `proto/<new-service>/*.proto` and a `proto-<new-service>` target in `Makefile`; run `make proto`.
3. **Makefile:** add the service name to the `SERVICES` variable so `make build` includes it.
4. **Compose:** add a service block to both `docker/docker-compose.simple.yaml` and `docker/docker-compose.yaml` (mount `config.yaml` read-only).
5. **Gateway:** register HTTP routes in `services/gateway/internal/router/router.go` and add a handler.

## Conventions

- Go idioms: pointer receivers for struct mutation, explicit error handling.
- Config via YAML only - the real key lives in gitignored `config.yaml`, generated from `config.example.yaml` by `scripts/init-config.sh`.
- gRPC between services; HTTP only at the gateway.
- Keep functions focused; extract shared logic into `pkg/`.

## Further Reading

- [Architecture](./architecture.md)
- [Configuration](./configuration.md)
- [Deployment](./deployment.md)
