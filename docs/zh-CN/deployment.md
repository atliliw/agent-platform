# 部署

> 使用 Docker Compose 部署 Agent Platform。 [English](../en/deployment.md)

## 前置条件

- Go ≥ 1.22（从源码构建）
- Docker + Docker Compose
- `protoc`、`protoc-gen-go`、`protoc-gen-go-grpc`（用于 `make proto`）
- Node.js ≥ 18（仅前端开发）
- DashScope（通义千问）API Key

## Compose 文件

有两个可用的 compose 文件：

| 文件 | 栈 | env_file |
|------|----|----------|
| `docker/docker-compose.simple.yaml` | 精简：全部服务 + Qdrant、MongoDB、Redis | `../.env` |
| `docker/docker-compose.yaml` | 完整：上述 + Obscura 隐身浏览器 + OpenTelemetry Collector + 前端 | `../.env` |

> ⚠️ Makefile 的 `make run-dev` 引用 `docker/docker-compose.dev.yaml`，该文件在仓库中**不存在**。请改用 `make run-prod`（`docker/docker-compose.yaml` 栈）或 `docker/docker-compose.simple.yaml`。

## 服务与端口

| 服务 | 容器端口 | 主机端口 |
|------|----------|----------|
| gateway | 9000 | 9000 |
| chat-service | 50001 | 50001 |
| knowledge-service | 50002 | 50002 |
| memory-service | 50003 | 50003 |
| a2a-service | 50004 | 50004 |
| mcp-service | 50005 | 50005 |
| agent-service | 50006 | 50006 |
| harness-service | 50007 | 50007 |
| mcp-demo-server | 50009 | 50009 |

## 基础设施端口

| 组件 | 主机端口 |
|------|----------|
| Qdrant（HTTP / gRPC） | 6333 / 6334 |
| MongoDB | 27017 |
| Redis | 6379 |
| Obscura 隐身浏览器（CDP） | 9222 |
| OpenTelemetry Collector（gRPC / HTTP） | 4317 / 4318 |
| 前端（生产 compose） | 8888 |

## 步骤

```bash
# 1. 配置环境变量
cp .env.example .env
#   编辑 .env -> OPENAI_API_KEY=sk-...

# 2.（可选）重新生成 protobuf 并本地构建
make proto
make build

# 3. 启动整套服务
make run-prod            # 使用 docker/docker-compose.yaml
# 或使用精简栈：
# docker compose -f docker/docker-compose.simple.yaml up -d
```

启动后：

- 网关 API：`http://localhost:9000`
- 前端（生产 compose）：`http://localhost:8888`
- 健康检查：`GET http://localhost:9000/health`
- 就绪检查：`GET http://localhost:9000/ready`

## 健康检查

```bash
curl http://localhost:9000/health
# {"status":"healthy","services":{...}}
```

## 停止 / 重建

```bash
make stop            # 停止所有 compose 环境
make docker-build    # 重建镜像（生产 compose）
make clean           # 清除 bin/、生成的 pb/、dev 卷
```

## 镜像构建与推送

```bash
make docker-build    # 构建所有服务镜像
make docker-push     # 推送到 registry（在 compose 中配置 registry）
```

## Kubernetes（可选）

Makefile 提供 k8s 目标（需 `k8s/` 下有清单）：

```bash
make k8s-deploy
make k8s-status
make k8s-delete
```

## 注意事项

- 使用 LLM 的服务通过 `env_file` 从 `.env` 读取 `OPENAI_API_KEY`。缺少有效 Key 时，依赖 LLM 的调用会失败。
- 生产 compose 包含 Obscura 隐身浏览器（`h4ckf0r0day/obscura`），供 MCP 浏览器/小红书工具使用，地址 `ws://obscura:9222/devtools/browser`。
- OpenTelemetry trace/metrics 上报到 `otel-collector:4317`。

## 延伸阅读

- [配置](./configuration.md)
- [开发](./development.md)
- [架构](./architecture.md)
