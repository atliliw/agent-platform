# 阶段 5:放开 MaxSteps + 上下文管理(P1)

> 优先级: P1
> 依据: `real-agent-analysis.md` §3.2⑥

---

## 1. 现状

- `engine.go:28` `MaxSteps: 10` -- 真 agent 要 50~200 步,10 步做不了有深度的任务
- `engine.go:29` `MaxHistoryLength: 50`
- `engine.go:778-785` `buildAgentMessages` 直接截断丢老消息,无摘要、无压缩
- 长任务跑到一半,前面关键上下文就没了

## 2. 目标

- MaxSteps 可配置,默认提到 50
- 老消息不直接丢,摘要后保留关键信息
- agent 能跑长任务不"失忆"

## 3. 改动清单

### 3.1 MaxSteps 可配置 + 默认 50
`engine.go:26` `DefaultEngineConfig`:
```go
func DefaultEngineConfig() EngineConfig {
    return EngineConfig{
        MaxSteps:         50,  // 10 -> 50
        MaxHistoryLength: 50,
    }
}
```
- `EngineConfig` 加可选字段 `MaxSteps` 已有,确认 `agent_service.go:74` 用的是 `DefaultEngineConfig()`
- 后期可从 config.yaml 读

### 3.2 上下文摘要(替换截断)
现状 `buildAgentMessages`(`engine.go:778`):
```go
start := 0
if len(execCtx.Messages) > e.config.MaxHistoryLength {
    start = len(execCtx.Messages) - e.config.MaxHistoryLength
}
```
改为:超过阈值时,把被丢弃的老消息**摘要**成一条 system message 保留:
```go
if len(execCtx.Messages) > e.config.MaxHistoryLength {
    dropped := execCtx.Messages[:len(execCtx.Messages)-e.config.MaxHistoryLength]
    // 用已摘要的缓存,避免每步重新摘要
    summary := e.getOrBuildSummary(execCtx, dropped)
    if summary != "" {
        messages = append(messages, Message{
            Role: "system",
            Content: fmt.Sprintf("[之前的对话摘要]\n%s", summary),
        })
    }
}
```

### 3.3 摘要缓存
- `ExecutionContext` 加字段:`SummaryCache string`
- 第一次摘要后缓存,后续步直接用,只有新消息再被截断时才增量更新
- 摘要用 LLM:`e.llmClient.Chat` 带"请把以下对话摘要成关键信息"prompt
- 摘要失败时降级为直接截断(不阻塞)

### 3.4 上下文管理器(可选,后期)
- 抽出 `ContextManager` 接口,策略可插拔(截断/摘要/滑窗+摘要)
- 本阶段先做"摘要替换截断"

## 4. 实施步骤

1. `DefaultEngineConfig` MaxSteps 10->50
2. `ExecutionContext` 加 `SummaryCache` 字段
3. `buildAgentMessages` 加摘要逻辑(带缓存 + 降级)
4. 摘要 LLM 调用复用 `e.llmClient`
5. build + 测试:跑一个 30+ 步的任务,确认不丢关键上下文

## 5. 验收标准

- [ ] `DefaultEngineConfig().MaxSteps == 50`
- [ ] 消息超过 50 条时,老消息被摘要成一条 system message,不是直接丢
- [ ] 摘要有缓存,不每步重新 LLM 调用
- [ ] 摘要 LLM 失败时降级为截断,不 panic
- [ ] 30+ 步任务能跑完,不会因为上下文丢失而跑偏

## 6. 风险

- 摘要 LLM 调用增加成本:靠缓存控制,只在截断点触发
- 摘要丢信息:prompt 强调保留"关键决策、工具结果、用户约束"
