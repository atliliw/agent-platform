# 开发

> Agent Platform 本地开发指南。 [English](../en/development.md)

## 前置条件

按平台安装工具链：

| 工具 | 版本 | 安装 |
|------|------|------|
| Go | ≥ 1.22 | <https://go.dev/dl/>（验证：`go version`） |
| protoc + 插件 | 最新 | 仅在编辑 `.proto` 时需要 - 生成代码已提交在 `pkg/pb/`。从 <https://grpc.io/docs/protoc-installation/> 安装 protoc，再 `go install google.golang.org/protobuf/cmd/protoc-gen-go@latest` 和 `go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest` |
| Docker + Compose | ≥ 20.10 | <https://docs.docker.com/get-docker/>（用于 Qdrant/MongoDB/Redis 及完整栈） |
| Node.js | ≥ 18 | <https://nodejs.org/>（仅前端开发） |
| golangci-lint | 最新 | 可选，用于 `make lint`：<https://golangci-lint.run/usage/install/> |

国内加速提示：设置 `GOPROXY=https://goproxy.cn,direct` 和 `npm config set registry https://registry.npmmirror.com`。

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
│   ├── config/             # 配置加载（YAML）
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

运行服务前先生成服务配置（填入每个 `services/*/config.yaml` 的 `llm.api_key`）：

```bash
bash scripts/init-config.sh sk-your-dashscope-key
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
4. **Compose：** 在 `docker/docker-compose.simple.yaml` 与 `docker/docker-compose.yaml` 都加服务块（只读挂载 `config.yaml`）。
5. **网关：** 在 `services/gateway/internal/router/router.go` 注册 HTTP 路由并加 handler。

## 约定

- Go 风格：结构体修改用指针接收者，显式错误处理。
- 配置纯走 YAML - 真实 key 写在已 gitignore 的 `config.yaml`，由 `scripts/init-config.sh` 从 `config.example.yaml` 生成。
- 服务间用 gRPC；HTTP 只在网关。
- 函数职责单一；共享逻辑抽到 `pkg/`。

## 延伸阅读

- [架构](./architecture.md)
- [配置](./configuration.md)
- [部署](./deployment.md)
