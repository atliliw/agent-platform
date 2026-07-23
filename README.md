# Agent Platform

> A microservice-based AI Agent operations platform — multi-agent collaboration, RAG knowledge base, long-term memory, MCP tool protocol, and a full Harness governance suite.

[English](./README.md) | [简体中文](./README.zh-CN.md)

---

## Overview

Agent Platform is a production-oriented platform for building, running, and governing AI agents. It brings together conversational agents, retrieval-augmented generation (RAG), layered long-term memory, cross-service agent-to-agent (A2A) communication, the Model Context Protocol (MCP) for tool use, and a comprehensive **Harness** system for observability, evaluation, cost, prompt, and workflow governance.

Built in Go with gRPC microservices and a React 19 frontend, it deploys via Docker Compose and uses Alibaba Cloud DashScope (Qwen) as the default LLM provider.

## Features

- **Multi-Agent Collaboration** — concurrent agents with handoff, streaming execution, and session replay.
- **RAG Knowledge Base** — document upload, chunking, BM25 + vector search (Qdrant).
- **Layered Long-Term Memory** — episodic / semantic / working memory with a forgetting mechanism.
- **A2A Protocol** — discover, register, and dispatch tasks across services.
- **MCP Tool Protocol** — call external tools; includes a built-in MCP client and demo server.
- **Browser & XHS Tools** — fine-grained, session-aware browser primitives; stealth XHS (小红书) reading via the Obscura engine.
- **Skills System** — independent skill library with progressive disclosure; agents mount skills by ID.
- **Context Compression** — lossless prompt compression to cut LLM token cost.
- **Harness Governance** — guardrails, evals, A/B testing, SLOs, cost analytics, prompt management, LLM gateway, session replay, checkpoints, approvals, and a visual workflow engine.

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Language | Go 1.22 |
| RPC | gRPC + Protobuf |
| HTTP Gateway | Gin |
| Databases | SQLite (metadata), MongoDB (documents), Qdrant (vectors), Redis (cache) |
| Observability | OpenTelemetry Collector |
| Frontend | React 19 + Ant Design 6 + TanStack Query + Zustand + React Flow + Monaco + ECharts + Tailwind 4 (Vite) |
| Deployment | Docker + Docker Compose |
| LLM | DashScope (Qwen) via OpenAI-compatible API |

## Quick Start

**Deploy with Docker - no local toolchain needed:**

```bash
# 1. Generate service configs and inject your DashScope API Key
bash scripts/init-config.sh sk-your-dashscope-key
#   (Windows PowerShell: pwsh scripts/init-config.ps1 sk-your-dashscope-key)

# 2. Build & start the full stack (Docker builds every service from source)
docker compose -f docker/docker-compose.yaml up -d --build
```

That's it. Verify with `curl http://localhost:9000/health`, then open:
- Gateway API: `http://localhost:9000`
- Frontend: `http://localhost:8888`

The real key lives in `services/*/config.yaml` (gitignored, never committed); `config.example.yaml` is the committed template. Get a DashScope (Qwen) key at <https://dashscope.console.aliyun.com/>.

Local Go development (binaries, tests, frontend hot-reload): see [Development](./docs/en/development.md). Full deployment guide (operations, troubleshooting, Kubernetes): see [Deployment](./docs/en/deployment.md).
- Health check: `GET http://localhost:9000/health`

## Services

| Service | Port | Responsibility |
|---------|------|----------------|
| Gateway | 9000 | HTTP API gateway, request routing, tenant middleware |
| Chat Service | 50001 | Conversation + agent execution |
| Knowledge Service | 50002 | RAG knowledge base (upload, chunk, search) |
| Memory Service | 50003 | Long-term memory |
| A2A Service | 50004 | Cross-service agent communication |
| MCP Service | 50005 | MCP tool protocol + browser/XHS tools |
| Agent Service | 50006 | Multi-agent orchestration, skills, approvals |
| Harness Service | 50007 | Governance: eval, cost, prompt, workflow, observability |
| MCP Demo Server | 50009 | MCP protocol demo server for client testing |

## Project Structure

```
agent-platform/
├── docs/                   # Documentation (EN + zh-CN)
├── proto/                  # Protobuf definitions
├── pkg/                    # Shared libraries
│   ├── llm/                # LLM client (OpenAI-compatible)
│   ├── qdrant/             # Qdrant vector DB client
│   ├── mongodb/            # MongoDB client
│   ├── redis/              # Redis client
│   ├── config/             # Config loading + env overrides
│   ├── agent/              # Agent engine primitives
│   ├── browseragent/       # Browser automation + pool
│   ├── mcp/                # MCP client (stdio + streamable HTTP)
│   ├── xhs/                # XHS (小红书) client & signer
│   └── pb/                 # Generated protobuf code
├── services/               # Microservices (each with cmd/ + internal/)
│   ├── gateway/            # HTTP gateway
│   ├── chat-service/
│   ├── knowledge-service/
│   ├── memory-service/
│   ├── a2a-service/
│   ├── mcp-service/
│   ├── agent-service/
│   ├── harness-service/
│   └── mcp-demo-server/
├── frontend/               # React 19 frontend
├── docker/                 # Docker Compose configs + otel
├── configs/                # Example configs
├── Makefile
└── go.mod
```

## Configuration

Each service reads its own `config.yaml` (mounted read-only into the container). The real `llm.api_key` lives in `config.yaml` (gitignored); `config.example.yaml` is the committed template. Generate configs with:

```bash
bash scripts/init-config.sh sk-your-dashscope-key   # one command fills every service's key
```

See [`docs/en/configuration.md`](./docs/en/configuration.md) for full details.

## API Overview

All endpoints are under `/api/v2` and pass through tenant middleware. Main domains:

| Domain | Sample Endpoints |
|--------|-----------------|
| Chat | `POST /chat`, `POST /chat/stream`, `GET /sessions`, `POST /multi-agent/chat` |
| Agents | `POST /agents`, `POST /agents/execute/stream`, `GET /agents/context/:id` |
| Skills | `POST /skills`, `POST /skills/import`, `GET /skills/:id/export` |
| Knowledge | `POST /knowledge/upload`, `POST /knowledge/search` |
| Memory | `POST /memory`, `POST /memory/recall`, layered + enhanced memory APIs |
| A2A | `POST /a2a/discover`, `POST /a2a/tasks/send` |
| MCP | `GET /mcp/tools`, `POST /mcp/call`, `POST /mcp/connect` |
| Harness | rules, guardrail, eval, A/B test, SLO, cost, prompt, workflow, session replay, approvals, LLM gateway, playground |

Full reference: [`docs/en/api-reference.md`](./docs/en/api-reference.md).

## Development

```bash
make proto           # regenerate protobuf
make build           # build all services -> bin/
make build-gateway   # build a single service
make test            # run all tests with -race
make test-coverage   # coverage report -> coverage.html
make lint            # golangci-lint
make fmt             # go fmt
make docker-logs     # tail compose logs
make help            # list all targets
```

Frontend:

```bash
cd frontend
npm install
npm run dev          # Vite dev server on :5173
npm run build        # production build
```

## Deployment

- **Docker Compose (production):** `docker/docker-compose.yaml` — includes all services, Qdrant, MongoDB, Redis, Obscura stealth browser, and OpenTelemetry Collector.
- **Docker Compose (simple):** `docker/docker-compose.simple.yaml` — minimal stack without otel/obscura.

```bash
make run-prod        # up
make stop            # down
make docker-build    # build images
```

See [`docs/en/deployment.md`](./docs/en/deployment.md) for details.

## Documentation

| Topic | English | 中文 |
|-------|---------|------|
| Architecture | [EN](./docs/en/architecture.md) | [中文](./docs/zh-CN/architecture.md) |
| Configuration | [EN](./docs/en/configuration.md) | [中文](./docs/zh-CN/configuration.md) |
| Deployment | [EN](./docs/en/deployment.md) | [中文](./docs/zh-CN/deployment.md) |
| API Reference | [EN](./docs/en/api-reference.md) | [中文](./docs/zh-CN/api-reference.md) |
| Development | [EN](./docs/en/development.md) | [中文](./docs/zh-CN/development.md) |

## License

MIT
