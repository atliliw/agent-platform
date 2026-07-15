# 架构

> Agent Platform 系统架构。 [English](../en/architecture.md)

Agent Platform 是一个用于运行和治理 AI Agent 的微服务平台。外部流量经 HTTP 网关进入，服务间通过 gRPC 通信，一组专用数据存储分别承载元数据、文档、向量和缓存。

## 总体拓扑

```
                    ┌──────────────────────────────────────────┐
   HTTP 客户端 ────▶│  Gateway (Gin, :9000)                    │
   /api/v2/*        │  路由 · 租户中间件 · handler              │
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
                          治理 · 评测 · 工作流 · 可观测性

  跨服务：a2a-service (:50004)    演示：mcp-demo-server (:50009)

  数据存储：
    SQLite   – 各服务元数据
    MongoDB  – 文档 / Agent 持久化
    Qdrant   – 向量索引（RAG、记忆）
    Redis    – 缓存 / 会话
  可观测性：OpenTelemetry Collector (:4317/:4318)
  浏览器引擎：Obscura 隐身浏览器 (:9222, CDP)
```

## 通信模型

- **外部 -> 网关：** 客户端调用网关（Gin，端口 9000）上 `/api/v2/*` 的 REST 接口。租户中间件为每个请求附加租户上下文。
- **网关 -> 服务：** 网关通过 `pkg/client/` 中的 gRPC 客户端代理到后端服务。
- **服务间：** 后端服务通过 gRPC 互相调用（如 chat-service -> mcp-service/agent-service/memory-service）。
- **健康检查：** 网关上的 `GET /health` 与 `GET /ready`。

## 服务

| 服务 | 端口 | 职责 |
|------|------|------|
| gateway | 9000 (HTTP) | API 网关、路由、租户中间件、handler 聚合 |
| chat-service | 50001 | 对话编排 + Agent 执行 |
| knowledge-service | 50002 | RAG：上传、分块、BM25 + 向量检索 |
| memory-service | 50003 | 长期记忆（分层） |
| a2a-service | 50004 | 跨服务 Agent 间通信 |
| mcp-service | 50005 | MCP 工具协议、浏览器与小红书工具 |
| agent-service | 50006 | 多 Agent 编排、技能、审批 |
| harness-service | 50007 | 治理：评测、成本、Prompt、工作流、可观测性 |
| mcp-demo-server | 50009 | MCP 协议演示服务器（用于客户端测试） |

## 数据存储

| 存储 | 职责 |
|------|------|
| SQLite | 各服务本地元数据（会话、规则、配置） |
| MongoDB | 文档与 Agent 持久化 |
| Qdrant | RAG 检索与记忆召回的向量索引 |
| Redis | 缓存、会话状态、临时数据 |

## 关键子系统

### 多 Agent 引擎（`pkg/agent`、`agent-service`）
并发 Agent，ReAct 风格循环、Agent 间交接、流式执行、会话回放。运行可检查点保存并恢复；会话支持干预、状态查看、消息注入。

### MCP 工具层（`pkg/mcp`、`mcp-service`）
实现模型上下文协议用于工具调用。内置客户端支持 stdio 与 streamable-HTTP 两种传输。服务暴露会话感知的细粒度浏览器原子操作（navigate/click/type/extract/scroll/wait），由 `pkg/browseragent` 与共享浏览器连接池支撑，并通过 Obscura 引擎隐身读取小红书。

### 分层记忆（`memory-service`）
情景记忆、语义记忆、工作记忆，带可配置的遗忘机制。增强记忆 API 额外提供时间线、图谱、整合与搜索视图。

### 技能系统（`agent-service`）
独立技能库，支持 CRUD、导入导出与渐进式加载（`load_skill`）。Agent 按 ID 挂载技能，而非打包内置。

### 上下文压缩
无损 Prompt 压缩，截断长上下文同时保留内容，降低 LLM Token 成本（长 Prompt 约削减 44–53%）。

### Harness 治理平面（`harness-service`）
运维层：护栏、评测套件与调度、A/B 测试、SLO、LLM 调用指标、成本分析与预算、带版本管理的 Prompt 管理、支持路由/负载均衡的 LLM 网关、带图谱导出的会话回放、检查点、审批、可视化工作流引擎、干预、RCA、golden-path 模板、optimizer、feature flags、RAG 指标、模型 Playground。

## 请求流程示例（对话）

```
客户端
  │  POST /api/v2/chat  {message, session_id}
  ▼
网关（租户中间件 -> ChatHandler）
  │  gRPC Chat()
  ▼
chat-service
  │  加载/追加工作记忆（memory-service）
  │  运行 Agent 循环（agent-service）-> 按需调用工具（mcp-service）
  │  持久化 trace（harness-service）
  ▼
响应（流式或 JSON）-> 客户端
```

## 可观测性

生产 compose（`docker/docker-compose.yaml`）内置 OpenTelemetry Collector。服务通过 OTLP 上报 trace/metrics（`OTEL_EXPORTER_OTLP_ENDPOINT=otel-collector:4317`）。Harness 服务暴露可观测性接口（`/observability/traces`、`/metrics`、`/stats`）。

## 延伸阅读

- [配置](./configuration.md)
- [部署](./deployment.md)
- [API 参考](./api-reference.md)
- [开发](./development.md)
