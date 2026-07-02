# 前端 Cost 数据显示问题说明

## 🔴 问题

前端显示的成本数据是**硬编码的 Mock 数据**：

```typescript
// 原代码 (index.tsx:420)
<Statistic title="本月成本" value={1256.78} prefix="$" precision={2} />
<Statistic title="预测成本" value={1450.00} prefix="$" precision={2} />
```

这些数字**不是真实数据**！

---

## ✅ 修复内容

### 1. 添加真实数据获取

```typescript
const [costStats, setCostStats] = useState({
  totalCost: 0,
  forecastCost: 0,
  totalRequests: 0,
});

const loadCostData = async () => {
  // 从 API 获取本月成本报告
  const now = new Date();
  const startOfMonth = new Date(now.getFullYear(), now.getMonth(), 1);
  const reportRes = await client.get(`/api/v2/harness/cost/report?start=${startOfMonth.toISOString()}&end=${now.toISOString()}`);

  if (reportRes) {
    const totalCost = reportRes.total_cost || 0;
    const daysInMonth = new Date(now.getFullYear(), now.getMonth() + 1, 0).getDate();
    const daysPassed = now.getDate();
    const forecastCost = daysPassed > 0 ? (totalCost / daysPassed) * daysInMonth : 0;

    setCostStats({ totalCost, forecastCost, totalRequests: reportRes.request_count || 0 });
  }
};
```

### 2. 使用真实数据显示

```typescript
<Statistic title="本月成本" value={costStats.totalCost} prefix="¥" precision={2} />
<Statistic title="预测成本" value={costStats.forecastCost} prefix="¥" precision={2} />
<Statistic title="请求总数" value={costStats.totalRequests} />
```

---

## ⚠️ 重要说明

### 为什么现在显示 0？

**因为目前没有实际的 LLM 调用数据！**

数据来源链：

```
用户发送聊天请求
    ↓
Chat Service (chat_service.go)
    ↓
llmClient.Chat()  ← ⚠️ 这里没有使用 MetricsClient
    ↓
❌ 没有调用 metrics callback
    ↓
❌ Harness Service 没有收到成本数据
    ↓
❌ Cost Engine 数据为空
    ↓
❌ 前端显示 0
```

### 如何修复数据源？

**需要修改 Chat Service**，使用 `MetricsClient` 装饰器：

**文件：** `services/chat-service/internal/service/chat_service.go`

```go
func NewChatService(llmClient llm.Client, harnessClient pb.HarnessServiceClient, ...) *ChatService {
    // 包装 LLM client with metrics
    metricsClient := llm.NewMetricsClient(llmClient, func(ctx context.Context, m *llm.CallMetrics) {
        log.Printf("[LLM Metrics] caller=chat model=%s latency=%dms tokens=%d cost=%.6f",
            m.Model, m.LatencyMs, m.TotalTokens, m.Cost)

        // 发送到 Harness Service 记录成本
        harnessClient.RecordCostUsageGRPC(ctx, &pb.RecordCostUsageRequest{
            AgentId:      "chat-service",
            ModelId:      m.Model,
            InputTokens:  int64(m.TotalTokens * 6 / 10),
            OutputTokens: int64(m.TotalTokens * 4 / 10),
        })
    }, "chat")

    return &ChatService{
        llmClient: metricsClient, // 使用包装后的 client
        // ...
    }
}
```

---

## 📊 数据修复后的效果

修复后，当用户发送聊天：

1. **Chat Service 调用 LLM**
2. **MetricsClient 自动记录** tokens, cost, latency
3. **发送到 Harness Service**
4. **Cost Engine 存储数据**
5. **前端显示真实成本**

```bash
# 修复后，查看 Harness Service 日志会看到：
[LLM Metrics] caller=chat model=qwen-plus latency=234ms tokens=150 cost=0.000015

# 前端会显示：
本月成本: ¥0.000015
预测成本: ¥0.000450 (基于使用量外推)
请求总数: 1
```

---

## 🎯 总结

| 问题 | 状态 | 说明 |
|-----|------|------|
| 前端硬编码数据 | ✅ 已修复 | 现在从 API 获取真实数据 |
| 数据源缺失 | ⚠️ 待修复 | Chat Service 未使用 MetricsClient |
| 前端显示 0 | ✅ 正常 | 因为没有实际数据 |

**下一步：修改 Chat Service 以启用自动成本收集。**

---

## 📝 已修改的文件

| 文件 | 变更 |
|-----|------|
| `frontend/src/pages/Harness/index.tsx` | 添加真实数据获取逻辑 |
