# 开发

> Agent Platform 本地开发指南。 [English](../en/development.md)

## 前置条件

- Go ≥ 1.22
- `protoc`、`protoc-gen-go`、`protoc-gen-go-grpc`
- Docker + Docker Compose（运行整套服务）
- Node.js ≥ 18（前端）
- `golangci-lint`（可选，用于 `make lint`）

## 项目结构

```
agent-platform/
├── docs/                   # 文档（en/ + zh-CN/）
├── proto/                  # Protobuf 定义（common、chat、knowledge、memory、a2a、mcp、harness）
├── pkg/                    # 公共库
│   ├── llm/                # LLM 客户端（OpenAI 兼容）
│   ├── qdrant/             # Qdrant 客户端
│   ├── mongodb/            # MongoDB 客户端
│   ├── redis/              # Redis 客户端
│   ├── config/             # 配置加载 + 环境变量覆盖
│   ├── agent/              # Agent 引擎原语（handoff、store 等）
│   ├── browseragent/       # 浏览器自动化 + 连接池
│   ├── mcp/                # MCP 客户端（stdio + streamable HTTP）
│   ├── xhs/                # 小红书客户端与签名
│   └── pb/                 # 生成的 protobuf 代码
├── services/               # 微服务（cmd/ + internal/）
│   ├── gateway/            # HTTP 网关（Gin）
│   ├── chat-service/
│   ├── knowledge-service/
│   ├── memory-service/
│   ├── a2a-service/
│   ├── mcp-service/
│   ├── agent-service/
│   ├── harness-service/
│   └── mcp-demo-server/
├── frontend/               # Vue 3 + Element Plus + Tailwind
├── docker/                 # Docker Compose 配置 + otel
├── configs/                # 示例配置
├── Makefile
└── go.mod                  # module agent-platform
```

## Protobuf 生成

使用 `protoc` 配合 `module=agent-platform` 标志生成代码，输出写到 `pkg/pb/`。

```bash
make proto          # 生成全部：common、chat、knowledge、memory、a2a、mcp、harness
```

每个 `proto-<name>` 子目标生成一个 proto 包。修改任何 `.proto` 后重新运行。

## 构建

```bash
make build              # 构建所有服务 -> bin/
make build-gateway      # 构建单个服务 -> bin/gateway
make build-<service>    # 如 make build-chat-service
```

`bin/` 已 gitignore--构建产物不会提交。

## 运行

```bash
make run-prod           # docker/docker-compose.yaml（完整栈）
# 或使用精简栈：
docker compose -f docker/docker-compose.simple.yaml up -d
```

> `make run-dev` 引用的 `docker/docker-compose.dev.yaml` 不存在，请用 `make run-prod`。

网关：`http://localhost:9000` · 前端：`http://localhost:8888`（生产 compose）。

## 测试

```bash
make test               # 全部测试，带 -race
make test-chat-service  # 单个服务测试
make test-coverage      # 覆盖率报告 -> coverage.html
```

测试遵循 AAA 模式（Arrange-Act-Assert），命名描述行为。

## 代码质量

```bash
make fmt                # go fmt ./...
make lint               # golangci-lint run ./...
make tidy               # go mod tidy
```

## 前端

```bash
cd frontend
npm install
npm run dev             # Vite 开发服务器 :5173
npm run build           # 生产构建
```

## 开发环境配置

运行服务前先复制环境模板并设置 Key：

```bash
cp .env.example .env
# 编辑 .env -> OPENAI_API_KEY=sk-...
```

详见 [配置](./configuration.md)。

## 新增一个服务

1. 在 `services/<new-service>/` 下搭骨架：
   - `cmd/main.go` - 入口
   - `internal/` - handler、服务逻辑、model
   - `config.yaml` - 服务配置（`llm.api_key` 留空）
   - `Dockerfile`
2. **Proto：** 添加 `proto/<new-service>/*.proto` 及 Makefile 的 `proto-<new-service>` 目标；运行 `make proto`。
3. **Makefile：** 把服务名加入 `SERVICES` 变量，`make build` 才会包含它。
4. **Compose：** 在 `docker/docker-compose.simple.yaml` 与 `docker/docker-compose.yaml` 都加服务块（挂载 `config.yaml`，设置 `env_file`）。
5. **网关：** 在 `services/gateway/internal/router/router.go` 注册 HTTP 路由并加 handler。

## 约定

- Go 风格：结构体修改用指针接收者，显式错误处理。
- 配置走 YAML + 环境变量覆盖（文件中不放密钥）。
- 服务间用 gRPC；HTTP 只在网关。
- 函数职责单一；共享逻辑抽到 `pkg/`。

## 延伸阅读

- [架构](./architecture.md)
- [配置](./configuration.md)
- [部署](./deployment.md)
