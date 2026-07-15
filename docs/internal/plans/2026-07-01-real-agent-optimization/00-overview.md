# 真 Agent 优化计划 · 总览

> 创建日期: 2026-07-01
> 依据: [`real-agent-analysis.md`](../../research/real-agent-analysis.md)
> 目标: 把 `pkg/agent/engine.go` 从"会调工具的聊天机器人"升级为"真 agent"

---

## 1. 问题陈述

`real-agent-analysis.md` 核对出的最致命问题不是"缺功能",而是**建了没接**:

| 部件 | 代码状态 | 运行状态 |
|------|---------|---------|
| 记忆服务 | ✅ episodic/semantic/working + Qdrant 全实现 | ❌ agent 引擎从不调它(孤岛) |
| Checkpoint | ✅ `checkpoint.go` + `mongo_store.go` 完整 | ❌ `engine.go:84` 不初始化,全段跳过 |
| StrategyAdjuster | ✅ `reflection/loop.go:573` 类完整 | ❌ engine 从不调用(死代码) |

加上架构层缺失:**无规划器、无验证器、反思不闭环、MaxSteps=10、无上下文管理、无工具结果验证**。

## 2. 优化原则

1. **接线优先**:建了没接的先接(记忆、checkpoint),ROI 最高、风险最低
2. **最小侵入**:不改现有接口契约,只加字段、加调用点,保证可回退
3. **可验证**:每步有验收标准,能跑通即合并
4. **不破坏现有**:handoff、approval、intervention、streaming 等已 work 的功能不动
5. **工程化并行**:测试、日志、沙箱与功能改造并行推进

## 3. 路线图(6 个阶段)

| 阶段 | 计划文档 | 优先级 | 预计改动 | 依赖 |
|------|---------|--------|---------|------|
| 1 | [`01-memory-integration.md`](./01-memory-integration.md) | P0 | engine.go + 新接口 + service + main | 无 |
| 2 | [`02-checkpoint-wiring.md`](./02-checkpoint-wiring.md) | P0 | main.go 一行 + mongo_store 接线 | 无 |
| 3 | [`03-planner-verifier.md`](./03-planner-verifier.md) | P1 | ExecutionRequest + engine.go Run() | 阶段1 |
| 4 | [`04-reflection-closure.md`](./04-reflection-closure.md) | P1 | engine.go + reflection/loop.go | 阶段1 |
| 5 | [`05-context-steps.md`](./05-context-steps.md) | P1 | engine.go config + buildAgentMessages | 无 |
| 6 | [`06-engineering-hygiene.md`](./06-engineering-hygiene.md) | 并行 | 测试 + 日志 + 沙箱 + 鉴权 | 无 |

## 4. 里程碑

- **M1(阶段1+2 完成)**:agent 带记忆跑任务 + 崩溃可恢复 -- 从"无状态工具循环"变"有记忆有恢复的 agent"
- **M2(阶段3+4 完成)**:agent 会规划、会验证、会从失败学习 -- 接近"真 agent"
- **M3(阶段5+6 完成)**:能跑长任务(50+步)+ 工程化达标 -- 可上生产

## 5. 不做什么(防过度建设)

- ❌ 不加 group chat / 多 agent 角色扮演(红海,输给单强 agent + 好工具)
- ❌ 不加 Tree of Thoughts / MCTS(学术漂亮,大规模不 work)
- ❌ 不碰治理面(Rollback/Chaos/Evolve 暂留,后期单独评估删减)
- ❌ 不重写 engine.go,只增量接线

## 6. 验收总标准

- [ ] agent 执行时能 recall/write 记忆(可从 memory service 日志验证)
- [ ] checkpoint 每步落库,`ResumeFromCheckpoint` 能恢复
- [ ] agent 完成任务时跑 verifier,不再"自己说完了就算完"
- [ ] 反思产出的 lessons 写进 memory,跨 session 可用
- [ ] MaxSteps 可配置到 50+,长消息摘要不直接丢
- [ ] 核心包有单测,`go build` / `go vet` 通过
