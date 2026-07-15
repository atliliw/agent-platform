# 阶段 4:闭环反思(P1)

> 优先级: P1(依赖阶段1记忆接线)
> 依据: `real-agent-analysis.md` §3.2④

---

## 1. 现状(反思为什么不闭环)

| 证据 | 问题 |
|------|------|
| `engine.go:614-629` | 任务完成反思打分后 `fmt.Printf` 打印就丢弃,`LessonsLearned` 没存 memory、没喂回下轮 |
| `reflection/loop.go:175` | 反思结果只存内存 `map`,不持久化、不跨 session |
| `reflection/loop.go:573` | `StrategyAdjuster` 类完整(Evaluate/Apply),但 engine 从不调用 = 死代码 |
| `engine.go:703` | 唯一闭环:score<0.5 时把 suggestions 塞 system message,太弱 |

## 2. 目标

- 反思的产物(lessons / strategy adjustment)写进 memory,跨 session 复用
- `StrategyAdjuster` 接进 engine:反思后产出策略修正,影响下一步决策
- session A 学到的经验,session B 能用

## 3. 改动清单

### 3.1 任务完成反思写进 memory(`engine.go:614`)
现状:
```go
reflectResult, _ := e.reflectionLoop.Reflect(ctx, ...)
if reflectResult != nil {
    fmt.Printf("[AgentEngine] Final reflection: score=%.2f, lessons=%v\n", ...)
}
```
改为:
```go
reflectResult, _ := e.reflectionLoop.Reflect(ctx, ...)
if reflectResult != nil {
    // ★ 把 lessons 写进 memory(跨 session 复用)
    if e.memoryClient != nil && len(reflectResult.LessonsLearned) > 0 {
        lessons := strings.Join(reflectResult.LessonsLearned, "; ")
        e.memoryClient.Write(ctx, execCtx.SessionID, execCtx.TenantID,
            "semantic", // 经验类 -> 语义记忆
            fmt.Sprintf("任务[%s]的经验教训: %s", taskDesc, lessons),
            map[string]any{"score": reflectResult.Score, "phase": "complete"})
    }
    // ★ 把策略修正写进 working memory(本 session 内喂回)
    if len(reflectResult.Suggestions) > 0 {
        execCtx.AddMessage("system", fmt.Sprintf(
            "本次任务反思 - 优点:%s;不足:%s;下次改进:%s",
            strings.Join(reflectResult.Strengths, ", "),
            strings.Join(reflectResult.Weaknesses, ", "),
            strings.Join(reflectResult.Suggestions, "; ")))
    }
}
```

### 3.2 步后反思接 StrategyAdjuster(`engine.go:691`)
现状:score<0.5 才塞 suggestions。
改为:每次步后反思都喂给 `StrategyAdjuster`:
```go
if e.reflectionLoop != nil && len(llmResp.ToolCalls) > 0 {
    // ... 现有 Reflect 调用
    if reflectResult != nil {
        // ★ 接 StrategyAdjuster(之前是死代码)
        if e.strategyAdjuster != nil {
            adjustment, _ := e.strategyAdjuster.Evaluate(ctx, execCtx.SessionID, reflectResult)
            if adjustment != nil {
                // 策略修正写进 working memory,下一步能看到
                execCtx.AddMessage("system", fmt.Sprintf(
                    "策略调整建议:%s。原因:%s。新策略:%s",
                    adjustment.Trigger, adjustment.Reason, adjustment.NewStrategy))
                e.strategyAdjuster.Apply(adjustment.ID, "applied")
            }
        }
        // 保留原有的 score<0.5 suggestions 逻辑作为兜底
    }
}
```

### 3.3 Engine 加 strategyAdjuster 字段
- `engine.go:67` Engine struct 加:`strategyAdjuster *reflection.StrategyAdjuster`
- `NewEngine`(`:84`)里初始化:`e.strategyAdjuster = reflection.NewStrategyAdjuster()`
- 加默认调整规则(在 NewEngine 里 AddRule):
  - `low_score`:score<0.5 -> 触发"放慢节奏,重新评估"
  - `high_errors`:weaknesses>2 -> 触发"换工具或换思路"

### 3.4 反思结果持久化(可选,后期)
- `reflection/loop.go:175` 现在存内存 map
- 后期可加 `ReflectionStore` 接口落 SQLite,跨进程可见
- 本阶段先靠 memory client 把关键 lessons 落库,够用

## 4. 实施步骤

1. Engine 加 `strategyAdjuster` 字段 + NewEngine 初始化 + 默认规则
2. 改 `engine.go:614` 完成反思:lessons 写 memory + suggestions 喂回
3. 改 `engine.go:691` 步后反思:接 StrategyAdjuster.Evaluate + Apply
4. `resumeLoop`(`engine.go:934` 对应位置)同步改
5. build + 测试:跑一个会失败的任务,确认 lessons 进了 memory,下次相似任务能看到

## 5. 验收标准

- [ ] 任务完成后,memory service 出现 semantic 类的 lessons 记录
- [ ] 新 session 跑相似任务时,recall 能召回到上次的 lessons
- [ ] 步后反思 score 低时,下一步 system message 出现策略调整建议
- [ ] `StrategyAdjuster` 不再是死代码(Evaluate/Apply 被调用,日志可见)
- [ ] `engine.go` 不再只 `fmt.Printf` 丢弃反思结果

## 6. 风险

- 反思 + strategy adjust 增加每步 LLM 调用(成本):可配置 `reflectionEnabled` 开关
- 策略调整建议可能误导:默认只作为 system message 提示,不强制改 prompt
