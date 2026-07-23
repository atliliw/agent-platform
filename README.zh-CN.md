# Agent Platform

> 基于微服务架构的 AI Agent 运维平台 —— 多 Agent 协作、RAG 知识库、长期记忆、MCP 工具协议，以及完整的 Harness 治理套件。

[English](./README.md) | [简体中文](./README.zh-CN.md)

---

## 项目概述

Agent Platform 是一个面向生产环境的 AI Agent 构建、运行与治理平台。它集成了对话式 Agent、检索增强生成（RAG）、分层长期记忆、跨服务 Agent 间通信（A2A）、工具调用的模型上下文协议（MCP），以及一套完整的 **Harness** 系统，用于可观测性、评测、成本、Prompt 和工作流治理。

平台采用 Go + gRPC 微服务架构，React 19 前端，通过 Docker Compose 部署，默认使用阿里云 DashScope（通义千问）作为 LLM 提供方。

## 功能特性

- **多 Agent 协作** —— 并发 Agent、任务交接、流式执行、会话回放。
- **RAG 知识库** —— 文档上传、分块、BM25 + 向量检索（Qdrant）。
- **分层长期记忆** —— 情景记忆 / 语义记忆 / 工作记忆，带遗忘机制。
- **A2A 协议** —— 跨服务发现、注册、派发任务。
- **MCP 工具协议** —— 调用外部工具；内置 MCP 客户端与演示服务器。
- **浏览器与小红书工具** —— 会话感知的细粒度浏览器原子操作；通过 Obscura 隐身引擎读取小红书。
- **技能系统** —— 独立技能库，渐进式加载；Agent 按 ID 挂载技能。
- **上下文压缩** —— 无损 Prompt 压缩，降低 LLM Token 成本。
- **Harness 治理** —— 护栏、评测、A/B 测试、SLO、成本分析、Prompt 管理、LLM 网关、会话回放、检查点、审批、可视化工作流引擎。

## 技术栈

| 层级 | 技术 |
|------|------|
| 语言 | Go 1.22 |
| RPC | gRPC + Protobuf |
| HTTP 网关 | Gin |
| 数据库 | SQLite（元数据）、MongoDB（文档）、Qdrant（向量）、Redis（缓存） |
| 可观测性 | OpenTelemetry Collector |
| 前端 | React 19 + Ant Design 6 + TanStack Query + Zustand + React Flow + Monaco + ECharts + Tailwind 4（Vite） |
| 部署 | Docker + Docker Compose |
| LLM | DashScope（通义千问），OpenAI 兼容接口 |

## 快速开始

**用 Docker 部署 - 无需本地工具链：**

```bash
# 1. 生成服务配置并填入你的 DashScope API Key
bash scripts/init-config.sh sk-your-dashscope-key
#   （Windows PowerShell：pwsh scripts/init-config.ps1 sk-your-dashscope-key）

# 2. 构建并启动完整栈（Docker 从源码构建所有服务）
docker compose -f docker/docker-compose.yaml up -d --build
```

就这两步。用 `curl http://localhost:9000/health` 验证，然后访问：
- 网关 API：`http://localhost:9000`
- 前端：`http://localhost:8888`

真实 Key 写在 `services/*/config.yaml`（已 gitignore，绝不提交）；`config.example.yaml` 是提交到仓库的模板。在 <https://dashscope.console.aliyun.com/> 获取 DashScope（通义千问）Key。

本地 Go 开发（二进制、测试、前端热更新）：见 [开发](./docs/zh-CN/development.md)。完整部署指南（运维、故障排查、Kubernetes）：见 [部署](./docs/zh-CN/deployment.md)。
- 健康检查：`GET http://localhost:9000/health`

## 服务列表

| 服务 | 端口 | 职责 |
|------|------|------|
| Gateway | 9000 | HTTP API 网关、请求路由、租户中间件 |
| Chat Service | 50001 | 对话 + Agent 执行 |
| Knowledge Service | 50002 | RAG 知识库（上传、分块、检索） |
| Memory Service | 50003 | 长期记忆 |
| A2A Service | 50004 | 跨服务 Agent 通信 |
| MCP Service | 50005 | MCP 工具协议 + 浏览器/小红书工具 |
| Agent Service | 50006 | 多 Agent 编排、技能、审批 |
| Harness Service | 50007 | 治理：评测、成本、Prompt、工作流、可观测性 |
| MCP Demo Server | 50009 | MCP 协议演示服务器（用于客户端测试） |

## 项目结构

```
agent-platform/
├── docs/                   # 文档（英文 + 中文）
├── proto/                  # Protobuf 定义
├── pkg/                    # 公共库
│   ├── llm/                # LLM 客户端（OpenAI 兼容）
│   ├── qdrant/             # Qdrant 向量库客户端
│   ├── mongodb/            # MongoDB 客户端
│   ├── redis/              # Redis 客户端
│   ├── config/             # 配置加载（YAML）
│   ├── agent/              # Agent 引擎原语
│   ├── browseragent/       # 浏览器自动化 + 连接池
│   ├── mcp/                # MCP 客户端（stdio + streamable HTTP）
│   ├── xhs/                # 小红书客户端与签名
│   └── pb/                 # 生成的 protobuf 代码
├── services/               # 微服务（每个含 cmd/ + internal/）
│   ├── gateway/            # HTTP 网关
│   ├── chat-service/
│   ├── knowledge-service/
│   ├── memory-service/
│   ├── a2a-service/
│   ├── mcp-service/
│   ├── agent-service/
│   ├── harness-service/
│   └── mcp-demo-server/
├── frontend/               # React 19 前端
├── docker/                 # Docker Compose 配置 + otel
├── configs/                # 示例配置
├── Makefile
└── go.mod
```

## 配置

每个服务读取自己的 `config.yaml`（以只读方式挂载进容器）。真实 `llm.api_key` 写在 `config.yaml`（已 gitignore）；`config.example.yaml` 是提交到仓库的模板。用一条命令生成配置：

```bash
bash scripts/init-config.sh sk-your-dashscope-key   # 一次填好所有服务的 key
```

完整说明见 [`docs/zh-CN/configuration.md`](./docs/zh-CN/configuration.md)。

## API 概览

所有接口位于 `/api/v2` 下，经过租户中间件。主要域：

| 域 | 示例接口 |
|----|---------|
| Chat | `POST /chat`、`POST /chat/stream`、`GET /sessions`、`POST /multi-agent/chat` |
| Agents | `POST /agents`、`POST /agents/execute/stream`、`GET /agents/context/:id` |
| Skills | `POST /skills`、`POST /skills/import`、`GET /skills/:id/export` |
| Knowledge | `POST /knowledge/upload`、`POST /knowledge/search` |
| Memory | `POST /memory`、`POST /memory/recall`、分层记忆 + 增强记忆 API |
| A2A | `POST /a2a/discover`、`POST /a2a/tasks/send` |
| MCP | `GET /mcp/tools`、`POST /mcp/call`、`POST /mcp/connect` |
| Harness | 规则、护栏、评测、A/B 测试、SLO、成本、Prompt、工作流、会话回放、审批、LLM 网关、Playground |

完整参考见 [`docs/zh-CN/api-reference.md`](./docs/zh-CN/api-reference.md)。

## 开发

```bash
make proto           # 重新生成 protobuf
make build           # 构建所有服务 -> bin/
make build-gateway   # 构建单个服务
make test            # 运行所有测试（带 -race）
make test-coverage   # 覆盖率报告 -> coverage.html
make lint            # golangci-lint
make fmt             # go fmt
make docker-logs     # 查看 compose 日志
make help            # 列出所有目标
```

前端：

```bash
cd frontend
npm install
npm run dev          # Vite 开发服务器 :5173
npm run build        # 生产构建
```

## 部署

- **Docker Compose（生产）**：`docker/docker-compose.yaml` —— 包含全部服务、Qdrant、MongoDB、Redis、Obscura 隐身浏览器、OpenTelemetry Collector。
- **Docker Compose（精简）**：`docker/docker-compose.simple.yaml` —— 不含 otel/obscura 的最小栈。

```bash
make run-prod        # 启动
make stop            # 停止
make docker-build    # 构建镜像
```

详见 [`docs/zh-CN/deployment.md`](./docs/zh-CN/deployment.md)。

## 文档

| 主题 | English | 中文 |
|------|---------|------|
| 架构 | [EN](./docs/en/architecture.md) | [中文](./docs/zh-CN/architecture.md) |
| 配置 | [EN](./docs/en/configuration.md) | [中文](./docs/zh-CN/configuration.md) |
| 部署 | [EN](./docs/en/deployment.md) | [中文](./docs/zh-CN/deployment.md) |
| API 参考 | [EN](./docs/en/api-reference.md) | [中文](./docs/zh-CN/api-reference.md) |
| 开发 | [EN](./docs/en/development.md) | [中文](./docs/zh-CN/development.md) |

## License

MIT
