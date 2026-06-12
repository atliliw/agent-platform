# Agent Platform 部署指南

## 服务器信息

| 项目 | 值 |
|------|------|
| **服务器 IP** | `192.168.10.100` |
| **SSH 用户** | `root` |
| **SSH 私钥** | `~/.ssh/demo_deploy_key` |
| **系统** | CentOS 7 |
| **项目路径** | `/opt/agent-platform` |

## 服务端口

| 服务 | 端口 | 说明 |
|------|------|------|
| **Frontend** | `8888` | 前端界面 |
| **Gateway** | `9000` | API 网关 |
| Chat Service | `50001` | 对话服务 |
| Knowledge Service | `50002` | 知识库服务 |
| Memory Service | `50003` | 记忆服务 |
| A2A Service | `50004` | 跨服务通信 |
| A2A HTTP | `9001` | A2A HTTP 端点 |
| MCP Service | `50005` | 工具协议服务 |
| Agent Service | `50006` | Agent 编排服务 |
| Harness Service | `50007` | 运维治理服务 |
| Redis | `6379` | 缓存服务 |

## 快速部署命令

```bash
# 连接服务器
ssh -i ~/.ssh/demo_deploy_key root@192.168.10.100

# 进入项目目录
cd /opt/agent-platform/docker

# 创建网络（首次需要）
docker network create agent-network

# 构建所有服务
docker compose build

# 启动所有服务
docker compose up -d

# 查看服务状态
docker compose ps

# 查看日志
docker compose logs -f [service-name]

# 停止所有服务
docker compose down
```

## 访问地址

- **前端**: http://192.168.10.100:8888
- **API**: http://192.168.10.100:9000
- **A2A Agent Card**: http://192.168.10.100:9001/.well-known/agent.json

## API 测试命令

```bash
# 测试对话
curl -X POST http://192.168.10.100:9000/api/v2/chat \
  -H "Content-Type: application/json" \
  -d '{"message":"你好","tenant_id":"test"}'

# 测试 MCP 工具列表
curl http://192.168.10.100:9000/api/v2/mcp/tools

# 测试计算器工具
curl -X POST http://192.168.10.100:9000/api/v2/mcp/call \
  -H "Content-Type: application/json" \
  -d '{"name":"calculator","arguments":{"expression":"2+3*4"}}'

# 测试数据分析工具
curl -X POST http://192.168.10.100:9000/api/v2/mcp/call \
  -H "Content-Type: application/json" \
  -d '{"name":"data_analysis","arguments":{"data":[1,2,3,4,5,6,7,8,9,10]}}'
```

## LLM 配置

配置文件位置: `services/*/config.yaml`

```yaml
llm:
  provider: dashscope
  api_key: "sk-6eb65fcf5d17491ca10b984efe1f43e7"
  base_url: "https://dashscope.aliyuncs.com/compatible-mode/v1"
  model: "qwen3.7-max-2026-05-17"
  embedding_model: "text-embedding-v3"
  max_tokens: 4096
```

## 常见问题

### 1. 服务启动失败

```bash
# 查看日志
docker compose logs [service-name]

# 重启单个服务
docker compose restart [service-name]
```

### 2. 端口被占用

```bash
# 查看端口占用
netstat -tlnp | grep [port]

# 停止占用的容器
docker stop [container-id]
```

### 3. 镜像构建失败

```bash
# 清理缓存重新构建
docker compose build --no-cache [service-name]
```

### 4. SQLite CGO 错误

Memory Service 使用 `glebarez/sqlite` (纯 Go 实现)，不需要 CGO。

如果遇到 CGO 错误，检查 `memory_repository.go` 导入：
```go
import "github.com/glebarez/sqlite"  // 正确
// import "gorm.io/driver/sqlite"   // 错误 - 需要 CGO
```

### 5. 网络问题

```bash
# 确保网络存在
docker network create agent-network

# 检查网络
docker network ls
```

## 更新部署

```bash
# 1. 同步代码（在本地执行）
rsync -avz --exclude 'node_modules' --exclude '.git' \
  -e "ssh -i ~/.ssh/demo_deploy_key" \
  ./ root@192.168.10.100:/opt/agent-platform/

# 2. 重新构建并启动（在服务器执行）
cd /opt/agent-platform/docker
docker compose build
docker compose up -d
```

## 服务依赖关系

```
Frontend (8888)
    └── Gateway (9000)
            ├── Chat Service (50001)
            │     └── MCP Service (50005)
            ├── Knowledge Service (50002)
            │     └── Qdrant (6333) [外部服务]
            ├── Memory Service (50003)
            ├── A2A Service (50004)
            ├── MCP Service (50005)
            ├── Agent Service (50006)
            │     └── MCP Service (50005)
            └── Harness Service (50007)
```

## Docker Compose 文件位置

`/opt/agent-platform/docker/docker-compose.yaml`

## 部署历史

- **2026-06-08**: 完成部署
  - 所有后端服务正常运行
  - 前端正常访问
  - DashScope LLM 配置完成
  - MCP 工具正常工作

- **2026-06-09**: Browser Agent 配置更新
  - 添加 `tool_config` 支持传递 Agent 级别的工具配置
  - MCP Service 配置增加 `tools.browser` 配置项
  - Agent YAML 支持 `tool_config` 字段
  - Browser Agent 可从 YAML 配置读取 LLM 参数
  - 所有服务重新构建部署成功
