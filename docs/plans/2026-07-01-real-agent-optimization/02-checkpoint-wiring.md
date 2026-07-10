# 阶段 2:接 checkpoint store(P0 · 一行接线)

> 优先级: P0(几乎零成本,`mongo_store.go` 已写好)
> 依据: `real-agent-analysis.md` §3.1②

---

## 1. 现状(为什么是禁用的)

| 证据 | 说明 |
|------|------|
| `pkg/agent/engine.go:84` | `NewEngine` 不初始化 `checkpointStore`(nil) |
| `services/agent-service/cmd/main.go` | 全文无 `SetCheckpointStore` 调用 |
| `pkg/agent/engine.go:725` `:1042` | `if e.checkpointStore != nil` 永远 false,整段跳过 |
| `pkg/agent/engine.go:881` | `ResumeFromCheckpoint` 直接返回 `"checkpoint store not configured"` |
| `pkg/agent/mongo_store.go` | 文件完整,没人调 |

## 2. 目标

checkpoint 每步落库,`ResumeFromCheckpoint` 真正可用,agent 崩溃后能从最近 step 恢复。

## 3. 改动清单

### 3.1 确认 `mongo_store.go` 是否实现 `CheckpointStore` 接口
- 读 `pkg/agent/mongo_store.go` 和 `pkg/agent/checkpoint/checkpoint.go`
- 确认 `MongoStore` 是否实现了 `checkpoint.CheckpointStore` 接口(Save/Get/List)
- 若已实现,直接用;若缺方法,补齐

### 3.2 `services/agent-service/cmd/main.go`
- `main.go:93` 已有 `mongoClient` 和 `mongoDB`
- 创建 engine 后(`main.go:155` 之后),加:
  ```go
  // 接 checkpoint store(用 MongoDB 持久化)
  cpStore := agent.NewMongoCheckpointStore(mongoClient.Client(), mongoDB) // 或复用现有 mongo_store.go 的构造函数
  agentService.GetEngine().SetCheckpointStore(cpStore)
  ```
- 若 `AgentService` 没暴露 `GetEngine()`,加一个,或改 `NewAgentService` 注入

### 3.3 验证 checkpoint store 接口匹配
- `engine.go:725` 调 `e.checkpointStore.Save(ctx, cp)`,`cp` 是 `*checkpoint.Checkpoint`
- 确认 mongo_store 的 Save 签名匹配

## 4. 实施步骤

1. 读 `pkg/agent/mongo_store.go` + `pkg/agent/checkpoint/checkpoint.go` 确认接口
2. 若 mongo_store 实现了 CheckpointStore,直接接;否则补方法
3. 改 `main.go`:创建 cpStore + `SetCheckpointStore`
4. `go build ./...` 验证
5. 跑 agent 任务,确认 MongoDB 出现 checkpoint 文档

## 5. 验收标准

- [ ] `go build ./...` 通过
- [ ] agent 跑几步后,MongoDB 里能看到 checkpoint 集合/文档
- [ ] `ResumeFromCheckpoint(checkpointID)` 不再返回 "not configured",能恢复执行
- [ ] checkpoint 存储失败不阻塞主循环(`engine.go:737` 已是 `fmt.Printf` 容错,确认不 panic)

## 6. 风险

- MongoDB 连接失败时:checkpoint 不应阻塞 agent。`engine.go:737` 已容错(只打日志),OK
- `mongo_store.go` 若实现不完整:需补 Save/Get/List,参考 `checkpoint.CheckpointStore` 接口
