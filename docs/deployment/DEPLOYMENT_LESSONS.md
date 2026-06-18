# 部署经验总结

## 部署耗时原因分析

### 1. 编译错误多
每次构建都会发现新的编译错误，需要反复修复：
- `totalTokens undefined` - 变量未声明
- `EmbeddingModel undefined` - 结构体字段缺失
- `GetAllChunks undefined` - 方法未实现
- `Delete undefined` - 方法未实现
- 工具重复定义 - `tools.go` 和 `real_tools.go` 有同名函数

### 2. 文件同步慢
使用 `scp` 一个个上传文件，没有批量同步。

### 3. 架构问题
- A2A Service 的 `NewA2AService` 签名改变了，但 `main.go` 没同步更新
- docker-compose.yaml 的 context 路径配置错误

## 改进建议

### 1. 本地先编译验证
在本地先运行 `go build ./...` 确保编译通过，再同步到服务器。

### 2. 使用 rsync 批量同步
```bash
rsync -avz --exclude 'node_modules' --exclude '.git' --exclude 'bin' \
  -e "ssh -i ~/.ssh/demo_deploy_key" \
  ./ root@192.168.10.100:/opt/agent-platform/
```

### 3. 预检查清单
部署前检查：
- [ ] `go build ./pkg/...` 通过
- [ ] `go build ./services/...` 通过
- [ ] docker-compose.yaml 格式正确
- [ ] 所有 Dockerfile 的 COPY 路径正确
- [ ] config.yaml 文件存在

### 4. 代码改进
- 统一使用 `glebarez/sqlite` 避免 CGO 问题
- 所有 interface 方法都要实现
- 新增方法要同步更新调用方

## 快速部署流程

```bash
# 1. 本地验证编译
go build ./...

# 2. 批量同步
rsync -avz --exclude 'node_modules' --exclude '.git' --exclude 'bin' --exclude '*.db' \
  -e "ssh -i ~/.ssh/demo_deploy_key" \
  ./ root@192.168.10.100:/opt/agent-platform/

# 3. 服务器构建
ssh -i ~/.ssh/demo_deploy_key root@192.168.10.100 "
cd /opt/agent-platform/docker
docker network create agent-network 2>/dev/null || true
docker compose build
docker compose up -d
"
```

## 关键配置文件

| 文件 | 作用 |
|------|------|
| `docker/docker-compose.yaml` | 服务编排 |
| `services/*/config.yaml` | 服务配置（含 LLM API Key） |
| `pkg/config/config.go` | 配置结构定义 |
| `pkg/llm/client.go` | LLM 客户端 |
| `pkg/llm/dashscope.go` | DashScope 实现 |

## 避坑指南

### 问题 1: 端口冲突
检查端口是否被占用：`netstat -tlnp | grep [port]`

### 问题 2: SQLite CGO
使用 `github.com/glebarez/sqlite` 替代 `gorm.io/driver/sqlite`

### 问题 3: Docker context 路径
docker-compose.yaml 中：
```yaml
build:
  context: ..          # 从 docker 目录往上找
  dockerfile: services/gateway/Dockerfile  # 路径从项目根目录开始
```

### 问题 4: 配置文件挂载
```yaml
volumes:
  - ../services/gateway/config.yaml:/app/config.yaml:ro
```
路径要相对于 docker-compose.yaml 所在目录。