# Architecture

> System architecture of the Agent Platform. [中文](../zh-CN/architecture.md)

Agent Platform is a microservice-based platform for running and governing AI agents. External traffic enters through an HTTP gateway; services communicate internally over gRPC; and a set of dedicated data stores hold metadata, documents, vectors, and cache.

## High-Level Topology

```
                    ┌──────────────────────────────────────────┐
   HTTP clients ───▶│  Gateway (Gin, :9000)                    │
   /api/v2/*        │  routing · tenant middleware · handlers  │
                    └───────────────┬──────────────────────────┘
                                    │ gRPC
        ┌───────────────┬───────────┼───────────┬───────────────┐
        ▼               ▼           ▼           ▼               ▼
  chat-service   knowledge-svc  memory-svc   mcp-service   agent-service
    :50001         :50002        :50003       :50005        :50006
        │               │           │           │               │
        └───────────────┴───────────┴───────────┴───────────────┘
                                    │ gRPC
                                    ▼
                          harness-service (:50007)
                          governance · eval · workflow · observability

  Cross-service: a2a-service (:50004)    Demo: mcp-demo-server (:50009)

  Data stores:
    SQLite   – per-service metadata
    MongoDB  – documents / agent persistence
    Qdrant   – vector index (RAG, memory)
    Redis    – cache / sessions
  Observability: OpenTelemetry Collector (:4317/:4318)
  Browser engine: Obscura stealth browser (:9222, CDP)
```

## Communication Model

- **External → Gateway:** Clients call REST endpoints under `/api/v2/*` on the Gateway (Gin, port 9000). A tenant middleware attaches tenant context to every request.
- **Gateway → Services:** The Gateway proxies to backend services over gRPC using the clients in `pkg/client/`.
- **Service ↔ Service:** Backend services call each other over gRPC (e.g. chat-service → mcp-service/agent-service/memory-service).
- **Health:** `GET /health` and `GET /ready` on the Gateway.

## Services

| Service | Port | Responsibility |
|---------|------|----------------|
| gateway | 9000 (HTTP) | API gateway, routing, tenant middleware, handler aggregation |
| chat-service | 50001 | Conversation orchestration + agent execution |
| knowledge-service | 50002 | RAG: upload, chunking, BM25 + vector search |
| memory-service | 50003 | Long-term memory (layered) |
| a2a-service | 50004 | Cross-service agent-to-agent communication |
| mcp-service | 50005 | MCP tool protocol, browser & XHS tools |
| agent-service | 50006 | Multi-agent orchestration, skills, approvals |
| harness-service | 50007 | Governance: eval, cost, prompt, workflow, observability |
| mcp-demo-server | 50009 | MCP protocol demo server for client testing |

## Data Stores

| Store | Role |
|-------|------|
| SQLite | Per-service local metadata (sessions, rules, configs) |
| MongoDB | Documents and agent persistence |
| Qdrant | Vector index for RAG retrieval and memory recall |
| Redis | Cache, session state, ephemeral data |

## Key Subsystems

### Multi-Agent Engine (`pkg/agent`, `agent-service`)
Concurrent agents with a ReAct-style loop, handoff between agents, streaming execution, and session replay. Runs are checkpointed and resumable; sessions support intervention, state inspection, and message injection.

### MCP Tool Layer (`pkg/mcp`, `mcp-service`)
Implements the Model Context Protocol for tool use. A built-in client supports stdio and streamable-HTTP transports. The service exposes fine-grained, session-aware browser primitives (navigate/click/type/extract/scroll/wait) backed by `pkg/browseragent` and a shared browser pool, plus stealth XHS (小红书) reading via the Obscura engine.

### Layered Memory (`memory-service`)
Episodic, semantic, and working memory with a configurable forgetting mechanism. An enhanced memory API adds timeline, graph, consolidation, and search views.

### Skills System (`agent-service`)
An independent skill library with CRUD, import/export, and progressive disclosure (`load_skill`). Agents mount skills by ID rather than bundling them.

### Context Compression
Lossless prompt compression that truncates long context while preserving content, cutting LLM token cost (roughly 44–53% reduction on long prompts).

### Harness Governance Plane (`harness-service`)
The operations layer: guardrails, eval suites & scheduling, A/B testing, SLOs, LLM call metrics, cost analytics & budgets, prompt management with versioning, an LLM gateway with routing/load-balancing, session replay with graph export, checkpoints, approvals, a visual workflow engine, interventions, RCA, golden-path templates, an optimizer, feature flags, RAG metrics, and a model playground.

## Request Flow Example (Chat)

```
Client
  │  POST /api/v2/chat  {message, session_id}
  ▼
Gateway (tenant middleware → ChatHandler)
  │  gRPC Chat()
  ▼
chat-service
  │  load/append working memory (memory-service)
  │  run agent loop (agent-service) → call tools (mcp-service) as needed
  │  persist trace (harness-service)
  ▼
Response (stream or JSON) → Client
```

## Observability

The production compose (`docker/docker-compose.yaml`) ships an OpenTelemetry Collector. Services emit traces/metrics over OTLP (`OTEL_EXPORTER_OTLP_ENDPOINT=otel-collector:4317`). The Harness service exposes observability endpoints (`/observability/traces`, `/metrics`, `/stats`).

## Further Reading

- [Configuration](./configuration.md)
- [Deployment](./deployment.md)
- [API Reference](./api-reference.md)
- [Development](./development.md)
