# Agent Platform - AI Agent 微服务平台

## 项目概述

Agent Platform 是一个基于微服务架构的 AI Agent 运维平台，支持：
- 多 Agent 协作对话
- 知识库管理（RAG）
- 长期记忆
- 跨服务 Agent 通信（A2A 协议）
- MCP 工具协议
- 完整的运维治理（Harness 系统）

## 技术栈

| 层级 | 技术 |
|------|------|
| 语言 | Go 1.22+ |
| RPC | gRPC + Protobuf |
| HTTP | Gin (Gateway) |
| 数据库 | SQLite (元数据), MongoDB (文档), Qdrant (向量), Redis (缓存) |
| 部署 | Docker + Docker Compose |
| 前端 | Vue 3 + Element Plus |

## 快速开始

```bash
# 生成 protobuf
make proto

# 构建所有服务
make build

# 运行开发环境
make run-dev
```

## 服务列表

| 服务 | 端口 | 说明 |
|------|------|------|
| Gateway | 8080 | API 网关 |
| Chat Service | 50001 | 对话 + Agent |
| Knowledge Service | 50002 | 知识库 |
| Memory Service | 50003 | 长期记忆 |
| A2A Service | 50004 | 跨服务通信 |
| MCP Service | 50005 | 工具协议 |
| Harness Service | 50006 | 运维治理 |

## 项目结构

```
agent-platform/
├── docs/                   # 规划文档
├── proto/                  # Protobuf 定义
├── pkg/                    # 公共库
│   ├── llm/               # LLM 客户端
│   ├── qdrant/            # Qdrant 客户端
│   ├── mongodb/           # MongoDB 客户端
│   ├── redis/             # Redis 客户端
│   ├── config/            # 配置加载
│   └── client/            # gRPC 客户端
├── services/               # 微服务
│   ├── gateway/           # API 网关
│   ├── chat-service/      # 对话服务
│   ├── knowledge-service/ # 知识库服务
│   ├── memory-service/    # 记忆服务
│   ├── a2a-service/       # A2A 服务
│   ├── mcp-service/       # MCP 服务
│   └── harness-service/   # Harness 服务
├── frontend/               # 前端
├── docker/                 # Docker 配置
├── Makefile
└── go.mod
```

## API 端点

### Chat
- `POST /api/v2/chat` - 对话
- `POST /api/v2/chat/stream` - 流式对话
- `GET /api/v2/sessions` - 会话列表

### Knowledge
- `POST /api/v2/knowledge/upload` - 文件上传
- `POST /api/v2/knowledge/search` - 检索

### Memory
- `POST /api/v2/memory` - 保存记忆
- `POST /api/v2/memory/recall` - 召回记忆

### A2A
- `POST /api/v2/a2a/discover` - 发现 Agent
- `POST /api/v2/a2a/tasks/send` - 发送任务

### MCP
- `GET /api/v2/mcp/tools` - 工具列表
- `POST /api/v2/mcp/call` - 调用工具

### Harness
- `POST /api/v2/harness/rules` - 创建规则
- `POST /api/v2/harness/chat` - 治理对话

## 文档

- [01-项目概述](docs/01-overview.md)
- [02-架构设计](docs/02-architecture.md)
- [03-服务详细设计](docs/03-services.md)
- [04-数据流设计](docs/04-data-flow.md)
- [05-API设计](docs/05-api-design.md)
- [06-基础设施](docs/06-infrastructure.md)
- [07-时间规划](docs/07-timeline.md)
- [08-迁移计划](docs/08-migration.md)

## License

MIT