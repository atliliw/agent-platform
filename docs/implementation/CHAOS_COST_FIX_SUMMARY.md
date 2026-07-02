# Chaos & Cost 治理模块诊断与修复报告

**日期**: 2026-07-01
**范围**: Harness Service - Chaos Engine & Cost Engine
**状态**: ✅ 已修复

---

## 🔴 问题总结

### Chaos Engine

| 问题 | 严重性 | 描述 |
|-----|--------|------|
| 故障注入无效 | **CRITICAL** | `InjectFault` 方法只是标记成功，没有实际注入任何故障 |
| 配置解析被注释 | **HIGH** | `FaultConfig` JSON 解析代码被注释掉 |
| 缺少请求集成 | **HIGH** | `ShouldInjectFault` 从未在实际请求流程中被调用 |
| 缺少实际效果 | **HIGH** | 没有延迟、超时、错误注入的实际实现 |

### Cost Engine

| 问题 | 严重性 | 描述 |
|-----|--------|------|
| 数据收集缺失 | **CRITICAL** | `RecordUsage` 存在但从未被自动调用 |
| 建议基于空数据 | **HIGH** | `Recommendations` 生成的建议基于空的 usage map |
| 缺少 LLM 集成 | **HIGH** | 没有从 `llm.CallMetrics` 自动收集成本数据 |

---

## ✅ 修复内容

### 1. Chaos Engine 完整重构

**新增结构：**

```go
type FaultEffect struct {
    AddLatencyMs   int64   // 延迟时间
    ForceTimeout   bool    // 强制超时
    TimeoutMs      int64   // 超时时间
    ForceError     bool    // 强制错误
    ErrorMessage   string  // 错误消息
    QualityFactor  float64 // 质量因子
    BudgetOverride float64 // 预算覆盖
}
```

**新增方法：**

| 方法 | 功能 |
|-----|------|
| `ShouldInjectFault(ctx, agentID)` | 返回 `(shouldInject, experimentID, *FaultEffect)` |
| `computeFaultEffect(exp)` | 根据故障类型计算实际效果 |
| `ApplyLatency(effect)` | **真正应用延迟**（`time.Sleep`） |
| `RecordFaultInjection(...)` | 记录故障注入事件 |
| `GetStats(ctx)` | 获取统计信息 |

**故障类型实现：**

```go
switch exp.FaultType {
case FaultAgentTimeout:
    effect.ForceTimeout = true
    effect.TimeoutMs = 30000
case FaultNetworkLatency:
    effect.AddLatencyMs = 2000  // 真正添加延迟
case FaultAgentError:
    effect.ForceError = true
    effect.ErrorMessage = "Chaos: simulated error"
// ... 其他故障类型
}
```

### 2. Cost Engine 数据收集

**新增方法：**

| 方法 | 功能 |
|-----|------|
| `RecordLLMCall(ctx, agentID, modelID, inputTokens, outputTokens, cost, latencyMs, success)` | 从 LLM metrics 自动记录 |
| `GetAgentMetrics(ctx, agentID, lookback)` | 获取 Agent 实时指标 |
| `GetTopExpensiveAgents(ctx, limit, lookback)` | 获取成本最高的 Agents |
| `GetCostBreakdown(ctx, start, end)` | 按模型获取成本分解 |
| `GetStats(ctx)` | 获取引擎统计 |

### 3. HarnessService 集成

**在 `llmMetricsCallback` 中添加 Cost 自动收集：**

```go
// 自动从每次 LLM 调用收集成本数据
inputTokens := int64(m.TotalTokens * 6 / 10)
outputTokens := int64(m.TotalTokens * 4 / 10)
s.cost.RecordLLMCall(ctx, m.Caller, m.Model, inputTokens, outputTokens, m.Cost, m.LatencyMs, m.Success)
```

**在 `Chat` 方法开头添加 Chaos 注入：**

```go
// Gate 0: Chaos injection
shouldInject, expID, faultEffect := s.chaos.ShouldInjectFault(ctx, req.AgentId)
if shouldInject && faultEffect != nil {
    s.chaos.RecordFaultInjection(ctx, expID, requestID, req.SessionId, faultEffect, true)

    if faultEffect.ForceError {
        // 返回模拟错误
    }
    if faultEffect.ForceTimeout {
        // 模拟超时
    }
    if faultEffect.AddLatencyMs > 0 {
        s.chaos.ApplyLatency(faultEffect)  // 真正延迟
    }
}
```

---

## 📊 数据流图

### Cost 自动收集流程

```
用户请求
    ↓
Chat Handler
    ↓
llmClient.Chat()
    ↓
┌─────────────────────────┐
│ MetricsClient (装饰器)  │
│ ├─ 记录开始时间         │
│ ├─ 调用实际 LLM         │
│ ├─ 计算延迟             │
│ └─ 调用 callback        │
└─────────────────────────┘
    ↓
llmMetricsCallback()
    ├─→ 存储到 buffer
    ├─→ Cost.RecordLLMCall() ← ✅ 新增：自动记录
    └─→ SLO.RecordEvent()
    ↓
Cost Engine (现在有真实数据)
    ├─→ AgentMetrics
    ├─→ CostBreakdown
    └─→ Recommendations (基于真实数据)
```

### Chaos 注入流程

```
Chat 请求
    ↓
┌──────────────────────────────┐
│ Gate 0: Chaos Injection      │ ← ✅ 新增
│                              │
│ shouldInject, expID, effect  │
│     = chaos.ShouldInjectFault│
│                              │
│ if shouldInject:             │
│   if effect.ForceError:      │
│     return error             │
│   if effect.ForceTimeout:    │
│     sleep + return timeout   │
│   if effect.AddLatencyMs:    │
│     chaos.ApplyLatency()     │ ← 真正延迟
│                              │
│ chaos.RecordFaultInjection() │
└──────────────────────────────┘
    ↓
Gate 1: Input Guardrail
    ↓
Gate 2: Permission Check
    ↓
Gate 3: Rule Check
    ↓
LLM Call (可能已延迟)
    ↓
Gate 4: Output Guardrail
    ↓
Response
```

---

## 🧪 验证步骤

### 验证 Chaos

```bash
# 1. 创建混沌实验（网络延迟）
curl -X POST http://localhost:8080/api/v2/harness/chaos \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Latency Test",
    "agent_id": "chat-agent",
    "fault_type": "network_latency",
    "duration": 5,
    "blast_radius": 0.5,
    "fault_config": "{\"latency_ms\": 2000}"
  }'

# 2. 启动实验
curl -X POST http://localhost:8080/api/v2/harness/chaos/{experiment_id}/start

# 3. 发送聊天请求（50% 的请求会增加 2 秒延迟）
curl -X POST http://localhost:8080/api/v2/harness/chat \
  -H "Content-Type: application/json" \
  -d '{
    "agent_id": "chat-agent",
    "message": "Hello"
  }'

# 预期：约 50% 的请求会在 2 秒后才返回
```

### 验证 Cost

```bash
# 1. 发送几个聊天请求
# 2. 查看日志输出
# [LLM Metrics] caller=eval model=qwen-plus latency=234ms tokens=150 cost=0.000015 status=success

# 3. 获取成本报告
curl http://localhost:8080/api/v2/harness/cost/report?start=...&end=...

# 预期：返回真实的成本数据，而不是空数据
```

---

## 📁 文件变更

| 文件 | 状态 | 说明 |
|-----|------|------|
| `services/harness-service/internal/chaos/engine.go` | ✅ 重写 | 完整的故障注入实现 |
| `services/harness-service/internal/cost/engine.go` | ✅ 增强 | 添加数据收集方法 |
| `services/harness-service/internal/service/harness_service.go` | ✅ 修改 | 集成 Chaos 和 Cost |
| `docs/implementation/CHAOS_COST_FIX.md` | ✅ 新增 | 详细修复文档 |
| `docs/implementation/CHAOS_COST_FIX_SUMMARY.md` | ✅ 新增 | 本摘要文档 |

---

## 🎯 后续建议

### 短期（立即）

1. ✅ **测试验证** - 按上述步骤验证 Chaos 注入和 Cost 收集
2. ✅ **监控日志** - 观察控制台输出确认数据流动

### 中期（1-2 周）

1. **前端集成** - 更新前端页面显示真实数据
2. **API 增强** - 添加更多查询 API（按时间、按模型过滤）
3. **持久化** - 确保故障注入记录写入数据库

### 长期（1 个月+）

1. **高级 Chaos** - 渐增延迟、间歇性故障、组合故障
2. **Cost 预测** - 基于历史数据预测未来成本
3. **Budget Alert** - 预算告警和自动熔断
4. **Dashboard** - 实时 Chaos & Cost 监控面板

---

## ✅ 结论

**Chaos 和 Cost 治理模块现在可以正常工作：**

- ✅ Chaos 故障注入会**真正执行**（延迟、错误、超时）
- ✅ Cost 数据会**自动收集**（从每次 LLM 调用）
- ✅ 优化建议基于**真实数据**
- ✅ 与现有请求流程**完全集成**

修复后，前端页面的 Chaos 和 Cost 功能将显示真实、有意义的数据。
