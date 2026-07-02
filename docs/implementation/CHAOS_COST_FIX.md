# Chaos 和 Cost 治理模块修复说明

## 问题诊断

### Chaos Engine 问题

**根本原因：故障注入逻辑没有实际执行任何故障**

1. `InjectFault` 方法（原 393-463 行）只是标记了 `fault.Success = true`，但没有真正：
   - 添加延迟
   - 强制超时
   - 注入错误
   - 降低响应质量

2. 故障配置解析被注释掉：
   ```go
   // json.Unmarshal([]byte(exp.FaultConfig), &config)  // 被注释！
   ```

3. 缺少与请求流程的集成 - `ShouldInjectFault` 从未在实际请求处理中被调用

### Cost Engine 问题

**根本原因：缺少数据收集流程**

1. `RecordUsage` 方法存在但从未被自动调用
2. `Recommendations` 基于空的 `e.usage` 数据生成建议
3. 缺少与 `llm.CallMetrics` 的自动连接

---

## 修复方案

### 1. Chaos Engine 增强 (`chaos/engine.go`)

**新增功能：**

- ✅ `FaultEffect` 结构体 - 定义实际要应用的故障效果
- ✅ `ShouldInjectFault` 增强 - 返回 `(bool, string, *FaultEffect)`
- ✅ `computeFaultEffect` - 根据故障类型计算实际效果
- ✅ `ApplyLatency` - 真正应用延迟（阻塞 sleep）
- ✅ `RecordFaultInjection` - 记录故障注入事件
- ✅ `GetStats` - 获取混沌实验统计

**故障类型映射：**

| 故障类型 | 实际效果 |
|---------|---------|
| `agent_timeout` | 返回超时错误，模拟响应超时 |
| `agent_error` | 返回模拟错误消息 |
| `network_latency` | 添加延迟后正常执行 |
| `model_degraded` | 降低响应质量（标记，供下游处理） |
| `budget_exhaust` | 返回预算耗尽错误 |
| `rate_limit` | 返回 429 错误 |

### 2. Cost Engine 增强 (`cost/engine.go`)

**新增方法：**

- ✅ `RecordLLMCall` - 从 LLM metrics 自动记录调用
- ✅ `GetAgentMetrics` - 获取 Agent 的实时指标
- ✅ `GetTopExpensiveAgents` - 获取成本最高的 Agents
- ✅ `GetCostBreakdown` - 按模型获取成本分解
- ✅ `GetStats` - 获取成本引擎统计

### 3. HarnessService 集成

**在 `harness_service.go` 中：**

#### 3.1 Cost 自动收集（`llmMetricsCallback`）

```go
// 在 llmMetricsCallback 中添加：
inputTokens := int64(m.TotalTokens * 6 / 10)
outputTokens := int64(m.TotalTokens * 4 / 10)
agentID := m.Caller
s.cost.RecordLLMCall(ctx, agentID, m.Model, inputTokens, outputTokens, m.Cost, m.LatencyMs, m.Success)
```

#### 3.2 Chaos 注入（在 `Chat` 方法开头添加）

```go
// Gate 0: Chaos injection
shouldInject, expID, faultEffect := s.chaos.ShouldInjectFault(ctx, req.AgentId)
if shouldInject && faultEffect != nil {
    // Record the fault injection attempt
    s.chaos.RecordFaultInjection(ctx, expID, requestID, req.SessionId, faultEffect, true)

    // Apply fault effects
    if faultEffect.ForceError {
        return error response
    }
    if faultEffect.ForceTimeout {
        time.Sleep(...)
        return timeout response
    }
    if faultEffect.AddLatencyMs > 0 {
        s.chaos.ApplyLatency(faultEffect)
    }
}
```

---

## 使用示例

### Chaos 实验

```bash
# 创建混沌实验
curl -X POST http://localhost:8080/api/v2/harness/chaos \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Latency Test",
    "agent_id": "chat-agent",
    "fault_type": "network_latency",
    "duration": 5,
    "blast_radius": 0.1,
    "fault_config": "{\"latency_ms\": 2000}"
  }'

# 启动实验
curl -X POST http://localhost:8080/api/v2/harness/chaos/{id}/start

# 现在 10% 的 chat-agent 请求会自动增加 2 秒延迟
```

### Cost 查看

```bash
# 获取成本报告
curl http://localhost:8080/api/v2/harness/cost/report?start=...&end=...

# 获取优化建议（现在有真实数据）
curl http://localhost:8080/api/v2/harness/cost/recommendations

# 获取成本最高的 Agents
# (通过 GetTopExpensiveAgents 内部方法)
```

---

## 数据流

```
LLM Call
    ↓
llm.MetricsClient (decorator)
    ↓
CallMetrics {model, tokens, cost, latency, success}
    ↓
┌─────────────────────────────────┐
│  llmMetricsCallback             │
│  ├─→ Store in buffer            │
│  ├─→ Cost.RecordLLMCall         │  ← NEW: 自动记录成本
│  └─→ SLO.RecordEvent            │
└─────────────────────────────────┘
    ↓
Cost Engine (now has real data)
    ↓
Recommendations (基于真实数据)
```

```
Chat Request
    ↓
┌─────────────────────────────────┐
│  Gate 0: Chaos Injection        │  ← NEW
│  ├─→ ShouldInjectFault?         │
│  │   ├─ Yes → Apply effect      │
│  │   │   ├─ ForceError? → ret   │
│  │   │   ├─ ForceTimeout? → ret │
│  │   │   └─ AddLatency → sleep  │
│  │   └─ No → continue           │
│  └─→ RecordFaultInjection       │
└─────────────────────────────────┘
    ↓
Gate 1: Guardrail
    ↓
Gate 2: Permission
    ↓
Gate 3: Rules
    ↓
LLM Call
    ↓
Gate 4: Output Guardrail
    ↓
Response
```

---

## 验证步骤

### 1. 验证 Chaos 注入

```bash
# 1. 创建并启动混沌实验
# 2. 发送聊天请求
# 3. 观察是否：
#    - 有延迟（network_latency）
#    - 返回错误（agent_error）
#    - 返回超时（agent_timeout）
```

### 2. 验证 Cost 收集

```bash
# 1. 发送几个聊天请求
# 2. 查看日志：[LLM Metrics] caller=... cost=...
# 3. 调用 /cost/report 应该显示真实数据
# 4. 调用 /cost/recommendations 应该基于真实数据
```

---

## 后续改进建议

1. **Chaos 增强**：
   - 持久化故障注入记录到数据库
   - 添加混沌实验 Web UI 可视化
   - 支持更复杂的故障模式（渐增延迟、间歇性故障）

2. **Cost 增强**：
   - 更精确的 input/output token 分割（从 LLM API 获取真实值）
   - 添加预算告警（Budget Alert）
   - 支持 Cost 预测和趋势分析

3. **集成改进**：
   - 在 Agent 执行层也注入 Chaos
   - 在 Memory、MCP 等模块也记录 Cost
   - 添加全局 Cost Dashboard

---

## 文件变更清单

| 文件 | 变更类型 | 说明 |
|-----|---------|------|
| `chaos/engine.go` | 重写 | 完整的故障注入实现 |
| `cost/engine.go` | 增强 | 添加数据收集和统计方法 |
| `harness_service.go` | 修改 | 集成 Chaos 和 Cost 自动收集 |
| `CHAOS_COST_FIX.md` | 新增 | 本说明文档 |
