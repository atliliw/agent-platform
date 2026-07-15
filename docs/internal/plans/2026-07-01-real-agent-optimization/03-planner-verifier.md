# 阶段 3:规划器 + 验证器(P1)

> 优先级: P1(依赖阶段1记忆接线)
> 依据: `real-agent-analysis.md` §3.2③⑤

---

## 1. 现状

- `engine.go:244` `Run()` -> 直接 `executeLoop()`,中间无规划步
- `engine.go:221` `ExecutionRequest` 只有 `Message`,无 `Goal`,无成功条件
- `engine.go:607` 循环在 `len(ToolCalls) == 0` 时停 = agent 自己说"我做完了"就算完
- 无任何地方检验"任务到底完成没有"

## 2. 目标

- 动手前先分解任务为 todo list(结构化,存进 ExecutionContext)
- 完成时跑 verifier:成功条件满足才 done,否则回 planner 重规划
- done 不再是 LLM 说了算,是 verifier 说了算

## 3. 设计

### 3.1 数据结构

`pkg/agent/context.go` 的 `ExecutionContext` 加字段:
```go
type ExecutionContext struct {
    // ... 现有字段
    Goal            string         // 显式目标
    SuccessCriteria string         // 可检验的成功条件
    Plan            *TaskPlan      // 任务分解(todo list)
    // ...
}

type TaskPlan struct {
    Items    []TaskItem
    UpdatedAt time.Time
}

type TaskItem struct {
    ID       string
    Description string
    Status   string // pending / in_progress / done / skipped
    AddedAt  time.Time
}
```

`ExecutionRequest`(`engine.go:221`)加字段:
```go
type ExecutionRequest struct {
    // ... 现有字段
    Goal             string
    SuccessCriteria  string
}
```

### 3.2 Planner

两种实现,先做简单的:

**方案 A(推荐,先做):plan 作为工具**
- 给所有 agent 注入一个 `plan` 工具(create/update/list todo)
- agent 自己在循环里维护 todo
- 优点:零侵入循环,LLM 自主规划,像 Claude Code 的 TaskCreate
- 缺点:依赖 LLM 主动调

**方案 B(后做):显式 planner 步**
- `Run()` 开头先调一次 LLM 产 todo list,存进 `execCtx.Plan`
- 每步循环里把当前 todo 注入 system prompt

先做方案 A,够用且简单。

### 3.3 Verifier

在 `executeLoop`(`engine.go:607`)的"无 tool calls 即 done"分支前,加验证:
```go
if len(llmResp.ToolCalls) == 0 {
    // ★ 先验证,不直接 done
    if e.verifier != nil && execCtx.SuccessCriteria != "" {
        passed, evidence, err := e.verifier.Verify(ctx, execCtx)
        if err == nil && !passed {
            // 没过,回 planner 重规划
            execCtx.AddMessage("system", fmt.Sprintf(
                "验证未通过:%s。证据:%s。请重新规划并继续。", execCtx.SuccessCriteria, evidence))
            continue // 回到循环顶,让 LLM 重做
        }
    }
    // 过了才 done
    execCtx.AddMessage("assistant", llmResp.Content)
    execCtx.MarkCompleted()
    return e.buildResult(execCtx, llmResp.Content), nil
}
```

`Verifier` 接口(新建 `pkg/agent/verifier.go`):
```go
type Verifier interface {
    Verify(ctx context.Context, execCtx *ExecutionContext) (passed bool, evidence string, err error)
}
```
默认实现 `LLMVerifier`:用 LLM 判断 + 可选调工具验证(如健康检查)。

## 4. 改动清单

- `pkg/agent/context.go`:ExecutionContext 加 Goal/SuccessCriteria/Plan
- `pkg/agent/engine.go:221`:ExecutionRequest 加 Goal/SuccessCriteria
- `pkg/agent/engine.go:244` `Run()`:把 req.Goal/SuccessCriteria 写进 execCtx
- `pkg/agent/engine.go:607`:done 分支前加 verifier 检查
- `pkg/agent/verifier.go`(新):Verifier 接口 + LLMVerifier
- `pkg/agent/engine.go`:Engine 加 `verifier Verifier` + `SetVerifier`
- `pkg/agent/tools/plan_tool.go`(新):plan 工具(create/update/list todo),方案 A
- `agent_service.go`:装配 verifier(用 metricsLLM)

## 5. 实施步骤

1. 加数据结构(context.go, engine.go ExecutionRequest)
2. 写 verifier.go(接口 + LLMVerifier)
3. 改 executeLoop 的 done 分支加验证
4. 写 plan 工具(方案 A),注册到所有 agent
5. agent_service 装配 verifier
6. build + 测试:给一个带 SuccessCriteria 的任务,验证 agent 不会"自己说完了就算完"

## 6. 验收标准

- [ ] agent 收到带 SuccessCriteria 的任务时,完成前会跑 verify
- [ ] verify 不过时,agent 会重新规划继续(不直接结束)
- [ ] verify 过才 MarkCompleted
- [ ] plan 工具能创建/更新 todo,LLM 会主动调用
- [ ] 无 SuccessCriteria 时退化为原行为(向后兼容)

## 7. 风险

- verifier 调用增加 LLM 成本:可配置开关,默认开
- LLM verifier 判断不准:先用 LLM 判,后期可接"工具验证"(如调 API 检查状态)
