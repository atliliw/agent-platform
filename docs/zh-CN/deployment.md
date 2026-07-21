# 部署

> 完整部署指南。克隆仓库即可运行，Docker 方式无需本地安装 Go/protoc/Node。 [English](../en/deployment.md)

Agent Platform 有两种运行方式：

- **方式 A - Docker Compose（推荐）：** 在 Docker 内从源码构建所有服务镜像。只需安装 Docker。这是"克隆即部署"的路径。
- **方式 B - 本地构建：** 用 `make build` 编译二进制直接运行（或用预构建镜像走 Compose）。需要 Go，前端开发还需 Node。

---

## 方式 A - Docker Compose（推荐）

### 1. 前置条件

只需安装 Docker 和 Docker Compose：

| 工具 | 版本 | 安装 |
|------|------|------|
| Docker | ≥ 20.10 | <https://docs.docker.com/get-docker/> |
| Docker Compose | v2（Docker Desktop 自带） | <https://docs.docker.com/compose/install/> |

验证：

```bash
docker --version
docker compose version
```

> 此方式无需 Go、protoc 或 Node.js - Docker 会从源码构建一切。

### 2. 克隆

```bash
git clone https://github.com/atliliw/agent-platform.git
cd agent-platform
```

### 3. 生成服务配置（必填）

每个服务读取自己的 `config.yaml`（已 gitignore，存放你的真实 Key）。从提交的模板生成，并一步填入 DashScope Key：

```bash
bash scripts/init-config.sh sk-your-dashscope-key
```

这会从 `services/*/config.example.yaml` 生成 `services/*/config.yaml`，并填入 `llm.api_key`。在 <https://dashscope.console.aliyun.com/> 获取 DashScope（通义千问）Key。没有有效 Key，依赖 LLM 的服务会失败。

> Windows PowerShell：`pwsh scripts/init-config.ps1 sk-your-dashscope-key`
> 不带参数则复制模板（占位 `<your-api-key>`），手动编辑。`config.yaml` 已 gitignore，真实 Key 不会提交。

### 4. 选择 compose 文件

| 文件 | 栈 | 适用场景 |
|------|----|---------|
| `docker/docker-compose.yaml` | 完整：全部服务 + Qdrant + MongoDB + Redis + Obscura 隐身浏览器 + OpenTelemetry Collector + 前端 | **默认 - 推荐** |
| `docker/docker-compose.simple.yaml` | 精简：仅服务 + Qdrant + MongoDB + Redis（无 otel/obscura/前端） | 无需可观测性/浏览器引擎的快速本地运行 |

### 5. 构建并启动

```bash
# 完整栈（推荐）
docker compose -f docker/docker-compose.yaml up -d --build

# 或精简栈
# docker compose -f docker/docker-compose.simple.yaml up -d --build
```

首次构建会编译每个 Go 服务（多阶段构建）和 Vue 前端。使用国内镜像（`goproxy.cn`、阿里云 APK、npmmirror），大陆网络友好。首次构建约几分钟，之后启动很快。

### 6. 验证

```bash
# 健康检查
curl http://localhost:9000/health
# {"status":"healthy","services":{...}}

# 就绪检查
curl http://localhost:9000/ready
# {"ready":true}
```

访问：

| 入口 | 地址 |
|------|------|
| 网关 API | <http://localhost:9000> |
| 前端（完整栈） | <http://localhost:8888> |
| Qdrant 面板 | <http://localhost:6333/dashboard> |
| MongoDB | `localhost:27017` |
| Redis | `localhost:6379` |

### 7. 运维操作

```bash
# 查看日志（跟踪）
docker compose -f docker/docker-compose.yaml logs -f
# 单个服务
docker compose -f docker/docker-compose.yaml logs -f chat-service

# 状态
docker compose -f docker/docker-compose.yaml ps

# 停止
docker compose -f docker/docker-compose.yaml down

# 代码改动后重建单个服务
docker compose -f docker/docker-compose.yaml up -d --build chat-service

# 停止并删除数据卷（完全重置）
docker compose -f docker/docker-compose.yaml down -v
```

---

## 方式 B - 本地构建

若想直接运行二进制或在不重建 Docker 镜像的情况下开发，用此方式。

### 1. 前置条件

| 工具 | 版本 | 说明 |
|------|------|------|
| Go | ≥ 1.22 | <https://go.dev/dl/> |
| protoc + 插件 | 最新 | 仅在编辑 `.proto` 时需要（生成代码已提交在 `pkg/pb/`） |
| Node.js | ≥ 18 | 仅前端开发需要 |
| Docker | ≥ 20.10 | 仍需用于 Qdrant/MongoDB/Redis（或自行运行它们） |

### 2. 生成服务配置

```bash
bash scripts/init-config.sh sk-...
```

如需按服务覆盖（如不同模型），编辑 `services/<service>/config.yaml`。

### 3. 生成与构建

```bash
make proto     # 可选：重新生成 protobuf（已有提交的生成代码）
make build     # 构建所有服务 -> bin/
```

### 4. 启动基础设施 + 服务

```bash
# 用 Compose 启动数据存储（精简栈去掉应用服务），或自行运行
docker compose -f docker/docker-compose.simple.yaml up -d qdrant mongodb redis

# 运行服务二进制（示例：chat-service）- config.yaml 已含 key
./bin/chat-service
```

---

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
| 前端（完整栈） | 8888 |

## 配置

- 每个服务读取 `services/<service>/config.yaml`（只读挂载到 `/app/config.yaml`）。
- `config.yaml` 已 gitignore，存放真实 `llm.api_key`；`config.example.yaml` 是提交到仓库的模板。详见 [配置](./configuration.md)。

## 故障排查

| 问题 | 解决 |
|------|------|
| LLM 调用失败 / 服务崩溃 | 运行 `bash scripts/init-config.sh sk-...`，让 `services/*/config.yaml` 含有效 DashScope key |
| `make run-dev` 失败 | 它引用的 `docker/docker-compose.dev.yaml` 不存在。改用 `docker compose -f docker/docker-compose.yaml up -d` |
| 端口被占用 | 停掉占用进程，或在 compose 文件中改主机端口映射 |
| Docker 构建 Go 模块失败 | Dockerfile 已设 `GOPROXY=https://goproxy.cn,direct`；若被墙，编辑 Dockerfile 换可达代理 |
| 前端构建失败 | 确认 `frontend/nginx.conf` 存在（仓库已含）；本地调试：`cd frontend && npm ci && npm run build` |
| `configs/agents` 挂载告警 | 目录会自动创建；agent-service 启动时不依赖其中的文件 |

## Kubernetes（可选）

Makefile 提供 k8s 目标（需 `k8s/` 下有清单）：

```bash
make k8s-deploy
make k8s-status
make k8s-delete
```

## 延伸阅读

- [配置](./configuration.md)
- [开发](./development.md)
- [架构](./architecture.md)
