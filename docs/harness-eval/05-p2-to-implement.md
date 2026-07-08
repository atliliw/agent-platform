# 待实现功能 — P2（锦上添花但非阻塞）

> 本文档记录 P2 优先级的待实现功能——有了更好，没有也不影响核心可用性。

---

## P2-1：反思循环接入

### 问题

`pkg/agent/reflection/loop.go` 有完整的反思代码，但 engine.go 不调用。Agent 不能自我纠错和策略调整。

### 实现方案

#### 在 Engine 中添加反思字段

**文件**：`pkg/agent/engine.go`

```go
type Engine struct {
    // ... 现有字段
    reflectionLoop *reflection.ReflectionLoop
    errorAnalyzer  *reflection.ErrorAnalyzer
}
```

#### 在 executeLoop 中插入反思

三个插入点：

1. **工具执行后**（PhasePostAction）：评估结果质量

```go
// 工具执行完成后
reflectionCtx := &reflection.ReflectionContext{
    Task:         execCtx.Variables["task"],
    Goal:         currentAgent.Instructions,
    ActionsTaken: convertHistory(execCtx.AgentHistory),
    CurrentState: result,
    TokenUsage:   execCtx.TotalTokens,
    Success:      err == nil,
}
reflectResult, _ := e.reflectionLoop.Reflect(ctx, execCtx.SessionID, reflection.PhasePostAction, reflectionCtx)

// 如果反思分数低，添加自我修正提示
if reflectResult.Score < 0.5 && len(reflectResult.Suggestions) > 0 {
    execCtx.AddMessage(fmt.Sprintf("反思: %s. 建议: %s", 
        reflectResult.Analysis, 
        strings.Join(reflectResult.Suggestions, "; ")), "system")
}
```

2. **错误后**（PhaseError）：分析根因

```go
if err != nil {
    analysis, _ := e.errorAnalyzer.Analyze(ctx, execCtx.SessionID, step, err.Error(), "tool_execution")
    execCtx.AddMessage(fmt.Sprintf("错误分析: %s. 根因: %s. 建议: %s",
        analysis.Summary, analysis.RootCause, analysis.SuggestedFix), "system")
}
```

3. **完成时**（PhaseComplete）：总结学习

```go
// 任务完成后
reflectResult, _ := e.reflectionLoop.Reflect(ctx, execCtx.SessionID, reflection.PhaseComplete, reflectionCtx)
// 存储反思结果供后续参考
e.reflectionLoop.SaveResult(reflectResult)
```

#### 验证

1. Agent 执行工具失败 → 反思后自动尝试不同策略
2. Agent 完成任务 → 反思评分低 → 添加改进建议
3. 反思结果可以在 Session Replay 中查看

---

## P2-2：实时干预接入

### 问题

`pkg/agent/intervention/handler.go` 有完整的干预代码，但 engine.go 不调用。运行中的 Agent 无法暂停、修改参数、注入消息。

### 实现方案

#### 在 Engine 中添加干预字段

```go
type Engine struct {
    // ... 现有字段
    interventionManager *intervention.InterventionManager
}
```

#### 在 executeLoop 中插入干预检查

```go
func (e *Engine) executeLoop(...) {
    // 注册 Session
    e.interventionManager.RegisterSession(execCtx.SessionID, currentAgent.ID)
    defer e.interventionManager.UnregisterSession(execCtx.SessionID)
    
    for step := 0; step < e.config.MaxSteps; step++ {
        // 每步开始前检查是否有待处理干预
        state := e.interventionManager.GetSessionState(execCtx.SessionID)
        if state != nil {
            switch state.Status {
            case "paused":
                // 阻塞等待恢复
                event, _ := e.interventionManager.WaitForEvent(ctx, execCtx.SessionID)
                if event.Type == "resume" { continue }
                if event.Type == "stop" { break }
            case "stopped":
                break
            }
        }
        
        // ... 正常执行 ...
        
        // 记录执行步骤
        e.interventionManager.LogExecution(execCtx.SessionID, intervention.ExecutionEntry{
            Step:    step,
            AgentID: currentAgent.ID,
            Action:  "llm_call",
            Input:   prompt,
            Output:  llmResp.Content,
        })
    }
}
```

#### 暴露干预 API

```
POST /api/v2/harness/session/:id/intervene   — 发起干预（pause/stop/modify/inject）
GET  /api/v2/harness/session/:id/state        — 获取 Session 运行状态
POST /api/v2/harness/session/:id/resume       — 恢复执行
POST /api/v2/harness/session/:id/inject       — 注入消息
```

#### 前端干预 UI

在 Chat 页面添加运行状态栏：
- Agent 执行中显示 "运行中" 标识 + 暂停/停止按钮
- 暂停后显示干预面板（修改参数 / 注入消息 / 恢复）

---

## P2-3：Feature Flags / Scheduler / Catalog 前端

### 问题

Harness 后端已实现 Feature Flags、Scheduler、Catalog、Rollback、Golden Path、RCA 等功能，gRPC 路由也存在，但前端没有对应 UI。

### 实现方案

在 Harness 页面新增 Tab：

| Tab | 功能 | API 端点 |
|-----|------|---------|
| Feature Flags | 创建/切换/评估特性开关 | `/api/v2/harness/flags/*` |
| Scheduler | 定时任务管理（创建/暂停/恢复/删除） | `/api/v2/harness/scheduler/*` |
| Catalog | Agent 市场（浏览/评分/使用统计） | `/api/v2/harness/catalog/*` |
| Rollback | 配置快照和回滚 | `/api/v2/harness/rollback/*` |
| Chaos | 混沌工程实验 | `/api/v2/harness/chaos/*` |

每个 Tab 都是标准的 CRUD 面板，可以按需逐个实现。

---

## P2-4：A2A 服务接线

### 问题

前端 Agents 页面的 A2A Tab 调用 `/api/v2/a2a/*` 端点，但 Gateway 返回 nil。a2a-service 在 Docker 中运行但未接入。

### 实现方案

与 Knowledge 接线类似：
1. Gateway A2AHandler 注入 `a2apb.A2AServiceClient`
2. 重写每个方法（Discover/Register/SendTask/GetTask）代理到 a2a-service
3. 验证前端 A2A Tab 能发现和调用远程 Agent

---

## P2-5：Observability Mock 替换

### 问题

前端 Observability 页面的 TraceViewer、CostDashboard、EvalReport、MemoryManager 都是 mock 数据。有两条路：

### 方案 A：P0-4 分布式追踪实现后直接替换

如果 P0-4 实现了 OpenTelemetry，TraceViewer 自动变成真实数据。CostDashboard 可以复用 harness-service 的 cost API。

### 方案 B：低成本桥接

如果暂时不引入 OTEL，可以在 gateway handler 中聚合 harness-service 的 LLM metrics 作为 trace 数据源：

```go
func (h *ObservabilityHandler) GetTraces(c *gin.Context) {
    // 从 harness-service 的 LLM metrics 构造 trace-like 数据
    metrics, _ := h.harnessClient.GetLLMMetrics(ctx, &harnesspb.GetLLMMetricsRequest{
        Limit: 50,
    })
    // 转换为 trace 格式
    traces := convertMetricsToTraces(metrics)
    c.JSON(200, gin.H{"data": traces})
}
```

---

## 优先级总览

| 优先级 | 功能 | 工作量 | 前置依赖 |
|--------|------|--------|---------|
| **P0** | 真流式输出 | 1-2 天 | 无 |
| **P0** | Knowledge 接线 | 1 天 | 无 |
| **P0** | 人工审批接入 | 1 天 | 无 |
| **P0** | 分布式追踪 | 2-3 天 | 无 |
| **P1** | 图编排/条件路由 | 2-3 天 | 无 |
| **P1** | 并行工具执行 | 0.5 天 | 无 |
| **P1** | Checkpoint 持久化 | 2 天 | 无 |
| **P1** | 分层记忆接入 | 2 天 | 无 |
| **P2** | 反思循环接入 | 1 天 | P0 审批 |
| **P2** | 实时干预接入 | 1 天 | P0 审批 |
| **P2** | Feature Flags/Scheduler UI | 2-3 天 | 无 |
| **P2** | A2A 接线 | 1 天 | 无 |
| **P2** | Observability 去 mock | 1 天 | P0 追踪 |

**总计**：P0 约 5-7 天，P1 约 7-8 天，P2 约 6-8 天。全部约 18-23 天。

**建议顺序**：P0 全部 → P1-2（并行工具，最简单）→ P1-1（图编排，最核心）→ P1-3（Checkpoint）→ P1-4（分层记忆）→ P2 按需
