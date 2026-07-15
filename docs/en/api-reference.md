# API Reference

> HTTP API reference for Agent Platform. [中文](../zh-CN/api-reference.md)

- **Base URL:** `http://localhost:9000`
- **API prefix:** all routes are under `/api/v2` and pass through tenant middleware.
- **Health:** `GET /health` · `GET /ready`

---

## Chat

| Method | Path | Description |
|--------|------|-------------|
| POST | `/chat` | Conversation |
| POST | `/chat/stream` | Streaming conversation |
| GET | `/sessions` | List sessions |
| GET | `/sessions/:id` | Get a session |
| DELETE | `/sessions/:id` | Delete a session |
| DELETE | `/sessions/empty` | Delete empty sessions |
| POST | `/multi-agent/chat` | Multi-agent conversation |

## Agents

| Method | Path | Description |
|--------|------|-------------|
| POST | `/agents` | Register an agent |
| GET | `/agents` | List agents |
| GET | `/agents/:id` | Get an agent |
| DELETE | `/agents/:id` | Unregister an agent |
| POST | `/agents/execute` | Execute an agent |
| POST | `/agents/execute/stream` | Stream agent execution |
| GET | `/agents/context/:id` | Get agent context |

## Skills

| Method | Path | Description |
|--------|------|-------------|
| POST | `/skills` | Create a skill |
| GET | `/skills` | List skills |
| POST | `/skills/import` | Import a skill |
| GET | `/skills/:id` | Get a skill |
| GET | `/skills/:id/export` | Export a skill |
| PUT | `/skills/:id` | Update a skill |
| DELETE | `/skills/:id` | Delete a skill |

## Knowledge

| Method | Path | Description |
|--------|------|-------------|
| POST | `/knowledge/upload` | Upload a document |
| GET | `/knowledge/documents` | List documents |
| GET | `/knowledge/documents/:id` | Get a document |
| DELETE | `/knowledge/documents/:id` | Delete a document |
| POST | `/knowledge/search` | Search the knowledge base |

## Memory - Basic

| Method | Path | Description |
|--------|------|-------------|
| POST | `/memory` | Save a memory |
| POST | `/memory/recall` | Recall memories |
| GET | `/memory/all` | Get all memories |
| GET | `/memory/session/:id` | Get session memory |
| DELETE | `/memory/session/:id` | Delete session memory |
| DELETE | `/memory/session/clear` | Delete all session memories |
| DELETE | `/memory/:id` | Delete a memory |

## Memory - Layered

| Method | Path | Description |
|--------|------|-------------|
| POST | `/memory/episodic` | Store an episode |
| GET | `/memory/episodic` | Get episodes |
| POST | `/memory/episodic/similar` | Get similar episodes |
| POST | `/memory/semantic/concept` | Store a concept |
| POST | `/memory/semantic/relation` | Store a relation |
| GET | `/memory/semantic` | Recall concepts |
| POST | `/memory/semantic/related` | Get related concepts |
| POST | `/memory/working` | Add working message |
| GET | `/memory/working/:sessionId` | Get working context |
| GET | `/memory/working/:sessionId/llm` | Get working messages for LLM |
| DELETE | `/memory/working/:sessionId` | Clear working context |
| GET | `/memory/forgetting/config` | Get forgetting config |
| PUT | `/memory/forgetting/config` | Update forgetting config |
| POST | `/memory/forgetting/cleanup` | Trigger cleanup |

## Memory - Enhanced

| Method | Path | Description |
|--------|------|-------------|
| GET | `/memory-enhanced/stats` | Memory stats |
| GET | `/memory-enhanced/timeline` | Memory timeline |
| GET | `/memory-enhanced/graph` | Memory graph |
| GET | `/memory-enhanced/episodic` | Episodic memories |
| GET | `/memory-enhanced/semantic` | Semantic memories |
| GET | `/memory-enhanced/procedural` | Procedural memories |
| POST | `/memory-enhanced/consolidate` | Consolidate memories |
| POST | `/memory-enhanced/search` | Search memories |

## A2A

| Method | Path | Description |
|--------|------|-------------|
| POST | `/a2a/discover` | Discover agents |
| POST | `/a2a/agents` | Register an agent |
| GET | `/a2a/agents` | List agents |
| DELETE | `/a2a/agents/:id` | Unregister an agent |
| POST | `/a2a/tasks/send` | Send a task |
| GET | `/a2a/tasks/:id` | Get a task |
| POST | `/a2a/tasks/:id/cancel` | Cancel a task |
| GET | `/a2a/tasks` | List tasks |

## MCP

| Method | Path | Description |
|--------|------|-------------|
| GET | `/mcp/tools` | List available tools |
| POST | `/mcp/call` | Call a tool |
| POST | `/mcp/connect` | Connect to an MCP server |
| GET | `/mcp/connections` | List connections |
| DELETE | `/mcp/connections/:id` | Disconnect |

## Harness - Rules & Guardrail

| Method | Path | Description |
|--------|------|-------------|
| POST | `/harness/rules` | Create a rule |
| GET | `/harness/rules` | List rules |
| DELETE | `/harness/rules/:id` | Delete a rule |
| POST | `/harness/guardrail/check` | Check guardrail |
| POST | `/harness/eval/run` | Run an eval |
| GET | `/eval/suites` | List eval suites |
| GET | `/eval/suites/:id/results` | Eval suite results |
| POST | `/eval/suites/:id/run` | Run an eval suite |
| POST | `/harness/abtest` | Create A/B test |
| POST | `/harness/abtest/list` | List A/B tests |
| DELETE | `/harness/abtest/:id` | Delete A/B test |
| GET | `/harness/abtest/:id/result` | A/B test result |
| GET | `/harness/slo/status` | SLO status |
| POST | `/harness/slo` | Create an SLO |
| GET | `/harness/llm/metrics` | LLM call metrics |
| POST | `/harness/chat` | Governance chat |

## Harness - Feature Flags & Cost

| Method | Path | Description |
|--------|------|-------------|
| POST | `/harness/flags` | Create a feature flag |
| GET | `/harness/flags` | List feature flags |
| PUT | `/harness/flags/toggle` | Toggle a flag |
| POST | `/harness/flags/evaluate` | Evaluate a flag |
| GET | `/harness/cost/report` | Cost report |
| POST | `/harness/cost/pricing` | Set model pricing |
| GET | `/harness/cost/pricing` | List model pricing |
| GET | `/harness/cost/recommendations` | Cost recommendations |
| POST | `/harness/cost/usage` | Record cost usage |
| GET | `/cost/summary` | Cost summary |
| GET | `/cost/trend` | Cost trend |
| GET | `/cost/budgets` | List budgets |
| POST | `/cost/budgets` | Create a budget |
| GET | `/cost/details` | Cost details |

## Harness - Proposals, Golden Path, Optimizer, RCA

| Method | Path | Description |
|--------|------|-------------|
| POST | `/harness/proposals` | Create a proposal |
| GET | `/harness/proposals` | List proposals |
| POST | `/harness/proposals/:id/approve` | Approve a proposal |
| POST | `/harness/proposals/:id/reject` | Reject a proposal |
| POST | `/harness/proposals/:id/execute` | Execute a proposal |
| POST | `/harness/proposals/analyze` | Analyze and propose |
| POST | `/harness/goldenpath/templates` | Create golden-path template |
| GET | `/harness/goldenpath/templates` | List golden-path templates |
| POST | `/harness/goldenpath/instantiate` | Instantiate a template |
| POST | `/harness/evolve/optimize` | Run the optimizer |
| POST | `/harness/rca/changes` | Record a change |
| GET | `/harness/rca/incidents/:id/analyze` | Analyze an incident |

## Harness - Scheduler

| Method | Path | Description |
|--------|------|-------------|
| POST | `/harness/scheduler/schedules` | Create a schedule |
| GET | `/harness/scheduler/schedules` | List schedules |
| GET | `/harness/scheduler/schedules/:id` | Get a schedule |
| PUT | `/harness/scheduler/schedules/:id/pause` | Pause a schedule |
| PUT | `/harness/scheduler/schedules/:id/resume` | Resume a schedule |
| DELETE | `/harness/scheduler/schedules/:id` | Delete a schedule |
| POST | `/harness/scheduler/schedules/:id/run` | Run a schedule now |
| GET | `/harness/scheduler/schedules/:id/results` | Schedule results |
| GET | `/harness/scheduler/status` | Scheduler status |
| POST | `/harness/scheduler/control` | Scheduler control |
| GET | `/harness/scheduler/stats` | Scheduler stats |

## Harness - Session Replay & Checkpoints

| Method | Path | Description |
|--------|------|-------------|
| POST | `/harness/session` | Create a session |
| GET | `/harness/session/list` | List session replays |
| GET | `/harness/session/stats` | Session stats |
| GET | `/harness/session/:id` | Get a session replay |
| GET | `/harness/session/:id/graph` | Session graph |
| POST | `/harness/session/:id/replay` | Replay a session |
| GET | `/harness/session/:id/export` | Export a session |
| DELETE | `/harness/session/:id` | Delete a session replay |
| GET | `/harness/session/:id/checkpoints` | List checkpoints |
| GET | `/harness/session/:id/checkpoint/:checkpointId` | Get a checkpoint |
| POST | `/harness/session/:id/checkpoint/:checkpointId/resume` | Resume from checkpoint |

## Harness - Prompt Management

| Method | Path | Description |
|--------|------|-------------|
| POST | `/harness/prompt` | Create a prompt |
| GET | `/harness/prompt/list` | List prompts |
| GET | `/harness/prompt/:key` | Get a prompt |
| DELETE | `/harness/prompt/:key` | Delete a prompt |
| POST | `/harness/prompt/:key/versions` | Create a prompt version |
| GET | `/harness/prompt/:key/versions` | List versions |
| GET | `/harness/prompt/:key/active` | Get active version |
| POST | `/harness/prompt/version/activate` | Activate a version |
| POST | `/harness/prompt/compare` | Compare versions |
| POST | `/harness/prompt/render` | Render a prompt |
| GET | `/harness/prompt/performance/:versionId` | Prompt performance |

## Harness - RAG Metrics

| Method | Path | Description |
|--------|------|-------------|
| POST | `/harness/rag/evaluate` | Evaluate RAG |
| POST | `/harness/rag/batch-evaluate` | Batch evaluate RAG |
| GET | `/harness/rag/metrics` | List RAG metrics |
| GET | `/harness/rag/metrics/:id` | Get RAG metrics |
| POST | `/harness/rag/evaluation` | Create a RAG evaluation |
| GET | `/harness/rag/evaluations` | List RAG evaluations |
| POST | `/harness/rag/evaluation/:id/run` | Run a RAG evaluation |

## Harness - Approval

| Method | Path | Description |
|--------|------|-------------|
| GET | `/harness/approval/pending` | Pending approvals |
| GET | `/harness/approval/rules` | Approval rules |
| POST | `/harness/approval/approve` | Approve a request |
| POST | `/harness/approval/reject` | Reject a request |
| POST | `/harness/approval/rules` | Add an approval rule |

## Harness - LLM Gateway

| Method | Path | Description |
|--------|------|-------------|
| POST | `/harness/gateway/chat` | Gateway chat |
| POST | `/harness/gateway/config` | Create gateway config |
| GET | `/harness/gateway/config` | List gateway configs |
| GET | `/harness/gateway/config/:id` | Get a gateway config |
| PUT | `/harness/gateway/config/:id` | Update a gateway config |
| DELETE | `/harness/gateway/config/:id` | Delete a gateway config |
| GET | `/harness/gateway/stats` | Gateway stats |
| POST | `/harness/gateway/route` | Create a route |
| GET | `/harness/gateway/routes` | List routes |
| DELETE | `/harness/gateway/route/:id` | Delete a route |
| POST | `/harness/gateway/strategy` | Set load-balance strategy |

## Harness - Playground

| Method | Path | Description |
|--------|------|-------------|
| POST | `/harness/playground/execute` | Execute in playground |
| POST | `/harness/playground/compare` | Compare models |
| POST | `/harness/playground/stream` | Stream playground |
| GET | `/harness/playground/history` | Playground history |
| GET | `/harness/playground/stats` | Playground stats |

## Harness - Workflow

| Method | Path | Description |
|--------|------|-------------|
| POST | `/harness/workflows` | Create a workflow |
| GET | `/harness/workflows` | List workflows |
| GET | `/harness/workflows/:id` | Get a workflow |
| PUT | `/harness/workflows/:id` | Update a workflow |
| DELETE | `/harness/workflows/:id` | Delete a workflow |
| POST | `/harness/workflows/:id/execute` | Execute a workflow |
| POST | `/harness/workflows/:id/execute-stream` | Stream workflow execution |
| POST | `/harness/workflows/validate` | Validate a workflow |
| GET | `/harness/workflows/:id/executions` | List executions |
| GET | `/harness/workflows/executions/:executionId` | Get an execution |
| POST | `/harness/workflows/executions/:executionId/cancel` | Cancel an execution |

## Harness - Intervention

| Method | Path | Description |
|--------|------|-------------|
| POST | `/harness/session/:id/intervene` | Intervene a session |
| GET | `/harness/session/:id/state` | Get session state |
| POST | `/harness/session/:id/resume` | Resume a session |
| POST | `/harness/session/:id/inject` | Inject a message |

## Observability

| Method | Path | Description |
|--------|------|-------------|
| GET | `/observability/traces` | List traces |
| GET | `/observability/traces/:id` | Get a trace |
| GET | `/observability/metrics` | Metrics |
| GET | `/observability/profile/:id` | Get a profile |
| GET | `/observability/stats` | Stats |

## Cookies

| Method | Path | Description |
|--------|------|-------------|
| POST | `/cookies` | Save cookies |
| GET | `/cookies` | Get cookies |
| GET | `/cookies/all` | Get all cookies |
| DELETE | `/cookies` | Delete cookies |

## User

| Method | Path | Description |
|--------|------|-------------|
| GET | `/user/info` | User info |
| GET | `/user/settings` | User settings |
| PUT | `/user/settings` | Update user settings |

---

## Examples

### Chat

```bash
curl -X POST http://localhost:9000/api/v2/chat \
  -H "Content-Type: application/json" \
  -d '{"session_id":"s1","message":"Hello, what can you do?"}'
```

### Knowledge Search

```bash
curl -X POST http://localhost:9000/api/v2/knowledge/search \
  -H "Content-Type: application/json" \
  -d '{"query":"agent memory","top_k":5}'
```

### MCP Tool Call

```bash
curl -X POST http://localhost:9000/api/v2/mcp/call \
  -H "Content-Type: application/json" \
  -d '{"tool":"web_search","arguments":{"query":"golang gRPC"}}'
```

### Create & Execute a Workflow

```bash
# Create
curl -X POST http://localhost:9000/api/v2/harness/workflows \
  -H "Content-Type: application/json" \
  -d '{"name":"my-flow","definition":{...}}'
# -> { "id": "wf_xxx" }

# Execute
curl -X POST http://localhost:9000/api/v2/harness/workflows/wf_xxx/execute \
  -H "Content-Type: application/json" \
  -d '{"input":{}}'
```

## Further Reading

- [Architecture](./architecture.md)
- [Configuration](./configuration.md)
