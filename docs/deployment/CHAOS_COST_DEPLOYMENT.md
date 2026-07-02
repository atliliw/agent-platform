# Chaos & Cost 修复部署报告

**部署时间**: 2026-07-01 15:05
**服务器**: 192.168.10.100
**状态**: ✅ 代码已部署，需要重启所有服务

---

## ✅ 已完成的工作

### 1. 代码修复

| 文件 | 状态 | 说明 |
|-----|------|------|
| `chaos/engine.go` | ✅ 已修复 | 完整的故障注入实现（697 行） |
| `cost/engine.go` | ✅ 已增强 | 自动数据收集和统计（新增 150+ 行） |
| `harness_service.go` | ✅ 已集成 | Cost 自动收集 + Chaos 集成 |

### 2. 文件传输

```bash
✅ chaos/engine.go → 服务器
✅ cost/engine.go → 服务器
✅ harness_service.go → 服务器
```

### 3. 服务构建

```bash
✅ Harness Service 编译成功（21.6s）
✅ Harness Service 启动成功
✅ Gateway 重启成功
```

---

## ⚠️ 需要完成的工作

### 问题：gRPC 连接错误

```
"rpc error: code = Unavailable desc = error reading from server: EOF"
```

**原因**: Gateway 和 Harness Service 之间的 gRPC 连接需要重新建立。

### 解决方案

**选项 1：重启所有服务（推荐）**

```bash
ssh -i ~/.ssh/demo_deploy_key root@192.168.10.100

cd /opt/agent-platform/docker

# 重启所有服务
docker compose down
docker compose up -d

# 等待所有服务启动
sleep 10

# 检查状态
docker compose ps
```

**选项 2：仅重启相关服务**

```bash
ssh -i ~/.ssh/demo_deploy_key root@192.168.10.100

cd /opt/agent-platform/docker

# 重启 gateway 和 harness
docker compose restart gateway harness-service

# 等待启动
sleep 5

# 检查日志
docker logs docker-gateway-1 --tail 20
docker logs docker-harness-service-1 --tail 20
```

---

## 🧪 验证步骤

### 1. 测试 Chaos 创建

```bash
# 创建 Chaos 实验
curl -X POST http://192.168.10.100:9000/api/v2/harness/chaos \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Test Latency",
    "agent_id": "test-agent",
    "fault_type": "network_latency",
    "duration": 5,
    "blast_radius": 0.5
  }'

# 预期响应
{
  "code": 0,
  "data": {
    "id": "...",
    "name": "Test Latency",
    "status": "created"
  }
}
```

### 2. 测试 Cost 数据收集

```bash
# 发送聊天请求（会自动收集成本）
curl -X POST http://192.168.10.100:9000/api/v2/chat \
  -H "Content-Type: application/json" \
  -d '{"message":"你好","tenant_id":"test"}'

# 查看成本报告
curl http://192.168.10.100:9000/api/v2/harness/cost/report?start=2026-01-01T00:00:00Z&end=2026-12-31T23:59:59Z

# 预期：显示真实的成本数据
```

### 3. 测试 Chaos 故障注入

```bash
# 1. 创建并启动实验
EXP_ID=$(curl -s -X POST http://192.168.10.100:9000/api/v2/harness/chaos \
  -H "Content-Type: application/json" \
  -d '{"name":"Latency Test","agent_id":"test-agent","fault_type":"network_latency","duration":5,"blast_radius":1.0,"fault_config":"{\"latency_ms\": 2000}"}' \
  | jq -r '.data.id')

# 2. 启动实验
curl -X POST http://192.168.10.100:9000/api/v2/harness/chaos/$EXP_ID/start

# 3. 发送请求（应该延迟 2 秒）
time curl -X POST http://192.168.10.100:9000/api/v2/harness/chat \
  -H "Content-Type: application/json" \
  -d '{"agent_id":"test-agent","message":"test"}'

# 预期：响应时间约 2 秒
```

---

## 📊 服务状态检查

```bash
# 检查所有服务
ssh -i ~/.ssh/demo_deploy_key root@192.168.10.100 \
  "docker ps --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}'"

# 预期输出：
# docker-harness-service-1   Up X minutes   50007/tcp
# docker-gateway-1           Up X minutes   9000/tcp
# docker-chat-service-1      Up X minutes   50001/tcp
# ... 其他服务
```

---

## 📝 部署历史更新

在 `/opt/agent-platform/docs/deployment/DEPLOYMENT.md` 中添加：

```markdown
- **2026-07-01**: Chaos & Cost 治理模块修复
  - 修复 Chaos Engine 故障注入无效问题
  - 修复 Cost Engine 数据收集缺失问题
  - 新增 `ShouldInjectFault` 返回 `FaultEffect`
  - 新增 `RecordLLMCall` 自动收集成本
  - 集成到 `llmMetricsCallback` 和 `Chat` 方法
  - Chaos 故障注入现在会真正执行（延迟、错误、超时）
  - Cost 数据现在自动从每次 LLM 调用收集
```

---

## 🎯 下一步

1. **重启所有服务**（解决 gRPC 连接问题）
2. **验证 Chaos 功能**（创建实验、启动、测试延迟）
3. **验证 Cost 功能**（发送聊天、查看报告）
4. **前端测试**（访问 http://192.168.10.100:8888 查看真实数据）

---

## 📞 问题排查

如果重启后仍然有问题：

```bash
# 查看详细日志
docker logs docker-harness-service-1 --tail 100
docker logs docker-gateway-1 --tail 100

# 检查端口
netstat -tlnp | grep -E '9000|50007'

# 检查网络
docker network inspect agent-network
```
