# Chaos & Cost 快速测试指南

## 🚀 快速验证步骤

### 1. 启动服务

```bash
cd D:\BaiduNetdiskDownload\LLM\agent-platform

# 使用 Docker Compose 启动
docker-compose up -d

# 或者手动启动
go run services/harness-service/cmd/main.go
```

### 2. 测试 Chaos 故障注入

#### 2.1 创建混沌实验（网络延迟）

```bash
curl -X POST http://localhost:8080/api/v2/harness/chaos ^
  -H "Content-Type: application/json" ^
  -d "{\"name\":\"Latency Test\",\"agent_id\":\"chat-agent\",\"fault_type\":\"network_latency\",\"duration\":5,\"blast_radius\":0.5,\"fault_config\":\"{\\\"latency_ms\\\": 2000}\"}"
```

**预期响应：**
```json
{
  "id": "experiment-uuid",
  "name": "Latency Test",
  "status": "created"
}
```

#### 2.2 启动实验

```bash
curl -X POST http://localhost:8080/api/v2/harness/chaos/{experiment_id}/start
```

#### 2.3 发送测试请求

```bash
curl -X POST http://localhost:8080/api/v2/harness/chat ^
  -H "Content-Type: application/json" ^
  -d "{\"agent_id\":\"chat-agent\",\"message\":\"Hello\"}"
```

**预期现象：**
- 约 50% 的请求会延迟 2 秒后返回
- 控制台日志：`[LLM Metrics] ... latency=2000+ms ...`

#### 2.4 测试错误注入

```bash
curl -X POST http://localhost:8080/api/v2/harness/chaos ^
  -H "Content-Type: application/json" ^
  -d "{\"name\":\"Error Test\",\"agent_id\":\"chat-agent\",\"fault_type\":\"agent_error\",\"duration\":5,\"blast_radius\":1.0}"

curl -X POST http://localhost:8080/api/v2/harness/chaos/{experiment_id}/start

curl -X POST http://localhost:8080/api/v2/harness/chat ^
  -H "Content-Type: application/json" ^
  -d "{\"agent_id\":\"chat-agent\",\"message\":\"Test\"}"
```

**预期响应：**
```json
{
  "error": "chaos_injected",
  "content": "Chaos experiment error: Chaos injection: simulated agent error"
}
```

### 3. 测试 Cost 数据收集

#### 3.1 发送几个聊天请求

```bash
# 发送 3-5 个请求
curl -X POST http://localhost:8080/api/v2/harness/chat ^
  -H "Content-Type: application/json" ^
  -d "{\"agent_id\":\"chat-agent\",\"message\":\"你好\"}"

curl -X POST http://localhost:8080/api/v2/harness/chat ^
  -H "Content-Type: application/json" ^
  -d "{\"agent_id\":\"chat-agent\",\"message\":\"Hello\"}"
```

**查看控制台日志：**
```
[LLM Metrics] caller=chat model=qwen-plus latency=234ms tokens=150 cost=0.000015 status=success
[LLM Metrics] caller=chat model=qwen-plus latency=198ms tokens=120 cost=0.000012 status=success
```

#### 3.2 查看成本报告

```bash
curl "http://localhost:8080/api/v2/harness/cost/report?start=2026-01-01T00:00:00Z&end=2026-12-31T23:59:59Z"
```

**预期响应（现在有真实数据）：**
```json
{
  "total_cost": 0.000027,
  "total_input_tokens": 162,
  "total_output_tokens": 108,
  "request_count": 2,
  "by_agent": {
    "chat": {
      "total_cost": 0.000027,
      "request_count": 2
    }
  }
}
```

#### 3.3 查看优化建议

```bash
curl http://localhost:8080/api/v2/harness/cost/recommendations
```

**预期响应（现在基于真实数据）：**
```json
{
  "recommendations": [
    {
      "type": "model_switch",
      "priority": "medium",
      "title": "Switch from qwen-plus to qwen-turbo",
      "potential_savings": 0.000015
    }
  ]
}
```

### 4. 验证 SLO 自动停止 Chaos

#### 4.1 创建 SLO

```bash
curl -X POST http://localhost:8080/api/v2/harness/slo ^
  -H "Content-Type: application/json" ^
  -d "{\"name\":\"Latency SLO\",\"agent_id\":\"chat-agent\",\"type\":\"latency\",\"target\":0.95}"
```

#### 4.2 创建带 SLO 自动停止的 Chaos 实验

```bash
curl -X POST http://localhost:8080/api/v2/harness/chaos ^
  -H "Content-Type: application/json" ^
  -d "{\"name\":\"Auto-stop Test\",\"agent_id\":\"chat-agent\",\"fault_type\":\"network_latency\",\"duration\":60,\"blast_radius\":1.0,\"auto_stop_on_slo\":true,\"slo_threshold\":0.8,\"fault_config\":\"{\\\"latency_ms\\\": 5000}\"}"
```

#### 4.3 启动实验并发送请求

```bash
curl -X POST http://localhost:8080/api/v2/harness/chaos/{experiment_id}/start

# 发送多个请求，当 SLO 低于阈值时，实验会自动停止
```

**预期现象：**
- 实验启动后，所有请求延迟 5 秒
- SLO 监控检测到性能下降
- 当 SLO 低于 0.8 时，实验自动停止
- 控制台日志：`SLO Alert: ... burn rate ...`

---

## 📊 验证清单

### Chaos 功能验证

- [ ] 创建实验成功
- [ ] 启动实验成功
- [ ] **实际体验到延迟**（network_latency）
- [ ] **收到模拟错误**（agent_error）
- [ ] **请求超时**（agent_timeout）
- [ ] Blast Radius 生效（只有部分请求受影响）
- [ ] SLO 自动停止生效

### Cost 功能验证

- [ ] LLM 调用后控制台显示 `[LLM Metrics]`
- [ ] 成本报告显示真实数据
- [ ] 优化建议基于真实数据
- [ ] 按模型分解显示
- [ ] 按时间范围过滤生效

---

## 🐛 常见问题

### Q1: Chaos 注入没有生效？

**检查：**
1. 实验状态是否为 `running`？
   ```bash
   curl http://localhost:8080/api/v2/harness/chaos?status=running
   ```
2. `blast_radius` 是否太小？（设为 1.0 测试）
3. `agent_id` 是否匹配？

### Q2: Cost 数据为空？

**检查：**
1. 是否发送了聊天请求？
2. 控制台是否有 `[LLM Metrics]` 日志？
3. 时间范围是否正确？

### Q3: 编译错误？

```bash
# 确保在项目根目录
cd D:\BaiduNetdiskDownload\LLM\agent-platform

# 安装依赖
go mod tidy

# 编译
go build ./services/harness-service/...
```

---

## 🎯 下一步

1. **前端集成** - 前端页面现在可以显示真实数据
2. **监控面板** - 创建 Chaos & Cost Dashboard
3. **告警配置** - 配置 Budget Alert 和 SLO Alert
4. **自动化测试** - 编写集成测试验证功能

---

## 📝 相关文档

- [详细修复说明](./CHAOS_COST_FIX.md)
- [API 文档](../api/harness-api.md)
- [架构设计](../architecture/04-data-flow.md)
