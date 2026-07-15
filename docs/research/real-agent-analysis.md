# Agent Platform 全方位总结:从"工具循环"到"真 Agent"

> 生成日期: 2026-07-01
> 基于对 `pkg/agent/`、`services/agent-service/`、`services/harness-service/` 实际代码的逐行核对
> 所有"缺口"均带文件:行号,可直接验证

---

## 一句话总结

> **你建了一个治理面领先、编排面及格的平台,但"agent"本身还停留在"会调工具的聊天机器人"阶段。最讽刺的是:记忆服务、checkpoint、反思策略调整器,这些让 agent 变"真"的关键部件,你全写了,却全没接进执行循环。**

agent ≠ LLM + 工具循环。agent 的本质是 LLM 之外那个循环架构 -- 让它在 LLM 出错时仍能达成目标。你现在的 `engine.go` 是 ReAct 骨架,缺的是让它"鲁棒、能规划、会记忆、会验证"的那几根线。

---

## 第一部分:你有什么(平台现状)

### 1.1 量化全景

| 维度 | 现状 |
|------|------|
| 微服务 | 8 个(Gateway / Chat / Knowledge / Memory / A2A / MCP / Agent / Harness) |
| gRPC 方法 | harness 一个 service 就有 81 个方法,~75 已接线 |
| 代码量 | services ~43k 行 + pkg ~75k 行 Go |
| 前端 | 15 个页面 / ~55 个 TSX,React 19 + AntD 6 |
| LLM | OpenAI + DashScope 已实现,网关 4 策略负载+降级 |
| 数据库 | SQLite(主)+ Qdrant(向量)+ Redis(限流)+ MongoDB(agent 注册表) |
| 测试 | 1 个文件,≈ 0% 覆盖 |
| CI/CD | 无 |

### 1.2 真正的强项(治理面,领先)

这些是 LangSmith / LangFuse 都没有的,是你的差异化护城河:

- **LLM 网关**:4 种负载策略(轮询/最低延迟/最低成本/加权)+ 降级路由 + 限流 + 每服务统计
- **Prompt 管理**:16 个方法,版本/diff/回滚/变量渲染/性能追踪
- **RAG 评测**:16 个 RAGAS 指标(faithfulness、context precision/recall、MRR、NDCG…)
- **SLO 管理**:burn rate 告警 + 自动调优触发
- **成本管理**:模型定价 + 用量记录 + 优化建议
- **A/B 实验**:统计显著性判定
- **会话回放**:步骤录制 + 执行图 + 导出
- **调度器**:cron/interval/once
- **特征开关**:百分比灰度 + stale 检测
- **RCA 根因分析**:变更事件关联分析(独有)

### 1.3 编排面(及格)

- Agent 引擎:run loop + handoff + 并行工具(信号量≤10)+ per-tool 超时 + 人机审批 + 干预(暂停/停止/恢复)
- DAG 工作流:agent/tool/condition/parallel/merge 五种节点
- 浏览器 agent:chromedp 全功能(差异化)
- MCP 协议:实现了 MCP service

---

## 第二部分:什么是"真 Agent"

### 2.1 本质

> **agent 的本质,是 LLM 之外的那个循环架构。LLM 是推理引擎,架构才是让它在 LLM 出错时仍能达成目标的那个东西。**

naive loop:LLM 选错一个工具,任务跑飞。
真 agent:LLM 选错了,它自己发现(反思)、换路(重规划)、最后还是能成。

**对 LLM 错误的鲁棒性,才是"真 agent"的检验标准 -- 不是功能多,不是拓扑花。**

### 2.2 真 agent 比工具循环多五样东西

| # | 能力 | 你有没有 | 说明 |
|---|------|---------|------|
| 1 | **目标 + 可检验成功条件** | ❌ | done 不是 LLM 说了算,是 verifier 说了算 |
| 2 | **规划 + 重规划** | ❌ | 动手前分解任务,执行中基于观察改 plan |
| 3 | **记忆在决策回路里** | ❌(建了没接) | 每步 decide 前 recall,act 后 write |
| 4 | **反思闭环** | ⚠️(半接) | 反思产物是策略修正,写进 memory 喂回下轮,不是打分丢弃 |
| 5 | **工具结果验证 + 失败恢复** | ❌ | 不信工具输出,不合理就换参数/换工具/换思路 |

### 2.3 真 agent 的架构

```
        ┌─────────────────────────────────────┐
        │  目标 + 成功条件 (可检验)             │
        └──────────────────┬──────────────────┘
                           ▼
        ┌─────────────────────────────────────┐
        │  Planner: 分解 -> todo / 任务树        │
        │  (基于观察可重规划)                   │
        └──────────────────┬──────────────────┘
                           ▼
   ┌─────────────────────────────────────────────────┐
   │              执行循环 (ReAct)                     │
   │                                                   │
   │   recall      decide(LLM)       act(tool)         │
   │   记忆 ─────►   ──────────────►   ──────────►     │
   │                    ▲                    │         │
   │                    │                    ▼         │
   │                 reflect ◄────────── validate       │
   │               (产出策略修正)        (结果合理?)    │
   │                    │                              │
   │                    ▼                              │
   │                 write 记忆                         │
   └─────────────────────┬───────────────────────────┘
                           ▼
        ┌─────────────────────────────────────┐
        │  Verifier: 跑成功条件检验             │
        │  过 -> done │ 不过 -> 回 Planner 重做   │
        └──────────────────┬──────────────────┘
                           │
   ┌─────────────────────────────────────────────────┐
   │  TaskState (持久,扛 compaction / 重启)            │
   │  todo · 已尝试 · 已知事实 · 当前假设               │
   └─────────────────────────────────────────────────┘
```

### 2.4 2026 的诚实视角(别走弯路)

- **生产里真 work 的**:紧 ReAct 循环 + 显式 todo + 持久记忆 + 失败反思 + 验证恢复。Claude Code、Codex、Cursor、Devin 全是这套。
- **学术漂亮但大规模不 work**:Tree of Thoughts、MCTS 式搜索 -- 太贵太脆。
- **多 agent 角色扮演(CrewAI 那套)大多数时候输给"一个强 agent + 好工具"**,除非任务天然可并行。
- **赢点不在编排更花哨,在:记忆更好、规划更实、验证更狠、恢复更强。**

> 结论:你现在想加 group chat、加更复杂拓扑 -- 方向偏了。把现有单 loop 做深,比加多 agent 拓扑值钱得多。

---

## 第三部分:缺什么(基于代码的核对清单)

### 3.1 🔴 致命:你建了,但没接上

#### ① 记忆服务整个没接进 agent 引擎

| 证据 | 说明 |
|------|------|
| `main.go:155` | `NewAgentService(registry, llmClient, mcpClient, store, cfg)` -- 参数里没有 memory |
| `agent_service.go:40` | `NewAgentService` 签名无 memory client |
| `engine.go:67-81` | `Engine` struct 字段:registry/llmClient/tools/store/approval/ruleEngine/reflection/errorAnalyzer/intervention/checkpointStore -- **无 memory 字段** |
| `engine.go:749` | `buildAgentMessages` 拼 system prompt 只读 `Variables` 和 `AgentHistory`,**从不 recall** |
| `engine.go:531` | `executeLoop` 每步执行完,**从不 write** |
| `main.go:93` | 唯一连的 MongoDB 只给 `NewMongoStore` 做 agent 注册表,**不是记忆** |
| - | memory service 跑在 `:50003`,有 episodic/semantic/working + Qdrant,但 agent-service **从不 dial 它** |

> 你的 agent 跑任务时**完全不带记忆**。memory service 是个孤岛。"有记忆功能,agent 不会用记忆"的铁证。

#### ② Checkpoint 代码写了,但实际禁用

| 证据 | 说明 |
|------|------|
| `engine.go:84` | `NewEngine` 不初始化 `checkpointStore`(保持 nil) |
| `main.go` | 全文无 `SetCheckpointStore` 调用 |
| `engine.go:725` `:1042` | `if e.checkpointStore != nil` 永远 false,**整段跳过** |
| `engine.go:881` | `ResumeFromCheckpoint` 直接返回 `"checkpoint store not configured"` |
| `mongo_store.go` | 文件在那,没人调 |

> 你以为有 checkpoint/恢复,实际上每步都没存,挂了就没了。

### 3.2 🔴 架构层面缺的(让工具循环变真 agent)

#### ③ 没有规划器
- `engine.go:244` `Run()` -> 直接 `executeLoop()`,中间无规划步
- `engine.go:221` `ExecutionRequest` 只有 `Message`,无 `Goal`,无任务分解
- `ExecutionContext` 无 `plan`/`todo` 字段,状态就是 `Variables map[string]any` + 消息历史
- 每步从全 history 重新决策,无任务骨架

#### ④ 反思没闭环,`StrategyAdjuster` 是死代码
- `engine.go:614-629` 任务完成反思:打完分 `fmt.Printf` 打印就**丢弃**,`LessonsLearned` 没存 memory、没喂回下轮
- `reflection/loop.go:175` 反思结果只存内存 `map`,**不持久化、不跨 session** -- session A 的经验 session B 用不上
- `reflection/loop.go:573` `StrategyAdjuster` 类完整(Evaluate/Apply 都有),但 `engine.go` **从不调用** -- 写了没用
- 唯一闭环:`engine.go:703` 当 `score < 0.5` 时把 suggestions 塞 system message。太弱,只看分数不看内容

#### ⑤ 没有目标 / 成功条件 / 验证器
- `engine.go:607` 循环在 `len(llmResp.ToolCalls) == 0` 时停 -- 等于 **agent 自己说"我做完了"就算完**
- 没有任何地方检验"任务到底完成没有"
- 你有 SLO、有 eval,但都没接到"任务完成判定"上

#### ⑥ `MaxSteps=10` + 没有上下文管理
- `engine.go:28` `MaxSteps: 10` -- 真 agent 要 50~200 步,10 步做不了有深度的任务
- `engine.go:29` `MaxHistoryLength: 50`
- `engine.go:778-785` `buildAgentMessages` 直接**截断丢老消息**,无摘要、无压缩
- 长任务跑到一半,前面关键上下文就没了

#### ⑦ 错误恢复只到"建议",不到"执行"
- `engine.go:457` `ErrorAnalyzer` 提取了 `RecoveryOptions`
- 但 `engine.go:471` 只把 `description` 文本塞进 system message
- **不自动重试换参数,不执行 `AlternativeActions`,不升级** -- 有恢复知识,无恢复动作

#### ⑧ 没有工具结果验证
- `executeParallelTools` 执行完直接 `processToolResults` 记录
- 没有"这个结果合理吗 / 符合预期吗"的检查
- 工具返回错结果,agent 沿错方向继续走

### 3.3 工程化缺口(上生产前必须补,但非"agent 能力"核心)

| # | 缺口 | 现状 |
|---|------|------|
| ⑨ | 测试 | 1 个测试文件,≈ 0% 覆盖 |
| ⑩ | CI/CD | 无流水线 |
| ⑪ | 鉴权 | `Auth()` 中间件 pass-through,`// TODO: Validate JWT` |
| ⑫ | Secret 管理 | config.yaml 明文 API key |
| ⑬ | 代码执行沙箱 | `code_execute` 返回 mock,agent 不能写/跑代码 |
| ⑭ | 文件读写工具 | stub 返回固定内容 |
| ⑮ | 结构化日志 | 生产代码大量 `fmt.Printf`(`engine.go` 里 30+ 处) |

> ⚠️ 纠正前文一处错:我曾说"OTel 没接到服务启动"。对 agent-service 这是**错的** -- `main.go:46` `observability.InitServiceTracing("agent-service")` 调了,`main.go:161` gRPC server 也挂了 otelgrpc interceptor。OTel 在 agent-service 是接了的(其他服务待逐一核查)。

---

## 第四部分:多了什么(过度建设 / 死代码)

| 项 | 位置 | 问题 | 建议 |
|----|------|------|------|
| **`StrategyAdjuster`** | `reflection/loop.go:573` | 类完整但 engine 从不调用 | 要么接进 engine 闭环反思,要么删 |
| **Rollback (5 方法)** | proto 定义 | handler 未接线,半成品 | 从 proto 删,或补实现 |
| **Chaos (4 方法)** | proto 定义 | handler 未接线,demo 期无意义 | 从 proto 删 |
| **Evolve / AutoTune** | 已实现 | 自动改 agent 配置,风险高,无数据支撑 | 降级为只读建议,关闭 auto-execute |
| **MongoDB for memory** | `pkg/mongodb/` | 定义了,memory 不用它(用 Qdrant+SQLite) | 接入 checkpoint 或移除死码 |
| **重复 frontend api** | `api/memory.ts`+`memoryApi.ts` 等 | 易漂移 | 合并 |
| **预编译二进制入 git** | `gateway-linux` 等 | 应是 CI 产物 | 移出仓库 |

**架构层面的"多":**
- 8 微服务拆过细,demo 期可合并为 2-3(业务面 + 治理面)
- SQLite 作主库,多服务各写各的,无统一视图,无法横向扩展
- `harness_service.go` 3,799 行超 800 行红线
- proto 81 方法单 service,职责过重,建议按域拆

---

## 第五部分:行动路线(按优先级)

### 第一优先:接记忆这根线(投入产出比最高,东西都建好了)

1. `pkg/agent` 加 `MemoryClient` 接口(`Recall` / `Write` 两个方法)
2. `engine.go` `Engine` struct 加 `memoryClient` 字段 + `SetMemoryClient`
3. `buildAgentMessages`(`engine.go:749`)里 decide 前调 `Recall`,召回结果作 system message 注入
4. `executeLoop`(`engine.go:531`)每步 tool 结果后调 `Write`,存"这一步学到什么"
5. `agent_service.go` 加 memory gRPC client,`main.go` dial `:50003` 注入

> 接上这根线,agent 从"无状态工具循环"变"带记忆的 agent"。

### 第二优先:接 checkpoint(一行接线)

- `main.go` 创建 engine 后调 `s.engine.SetCheckpointStore(checkpoint.NewMongoStore(...))`
- 或实现 `MongoCheckpointStore`(mongo_store.go 已有基础)
- 立刻获得崩溃恢复 + `ResumeFromCheckpoint` 真正可用

### 第三优先:补规划 + 验证

- `ExecutionRequest` 加 `Goal` 字段 + `SuccessCriteria`
- `Run()` 开头加 planner 步:LLM 产出结构化 todo list 存进 `ExecutionContext.Plan`
- 最简方案:把 `plan` 做成工具,agent 自己维护(像 Claude Code 的 TaskCreate)
- `Run()` 结束不直接返回,先跑 verifier:成功条件满足才 done,否则回 planner 重规划

### 第四优先:闭环反思

- 把 `StrategyAdjuster`(`reflection/loop.go:573`)接进 engine:反思后调 `Evaluate`,产出 `StrategyAdjustment` 写进 working memory
- `PhaseComplete` 反思的 `LessonsLearned`(`engine.go:614`)写进 memory service,跨 session 复用
- 反思结果持久化(现在只存内存 map)

### 第五优先:放开口子 + 上下文管理

- `MaxSteps` 10 -> 50(或可配置)
- `MaxHistoryLength` 截断换成"摘要压缩":老消息 LLM 摘要后替换,不直接丢
- 加工具结果验证步:工具返回后 LLM 判"是否符合预期",不符合触发恢复

### 工程化(与上面并行)

- 核心包(`pkg/agent`、`pkg/llm`)补单测,目标 60% -> 80%
- GitHub Actions:lint + test + docker build
- `Auth()` 改真 JWT 校验,密钥迁环境变量
- `code_execute` 接真实沙箱(Docker/gVisor,参考 Modal Sandbox)
- 生产代码 `fmt.Printf` 换结构化日志

---

## 第六部分:战略定位

### 你同时想做三件事,建议聚焦一件

| 角色 | 对标 | 你的位置 |
|------|------|---------|
| agent 编排引擎 | LangGraph / CrewAI | 及格,红海,拼不过生态 |
| 治理 / 可观测控制台 | LangSmith / LangFuse + 自研 SRE | **领先,有护城河** |
| 应用平台 | Dify / Coze | 落后(无可视化、无鉴权) |

**推荐走法:把治理面独立成"任何 agent 框架都能接的控制平面"**

- 提供 SDK,让 LangGraph/CrewAI 用户把 trace/eval/SLO 接到你的平台
- 对标 LangFuse 开源路线,但多了 SRE 能力(SLO/RCA/特征开关/调度器/AutoTune)
- 不和编排框架正面竞争,差异化清晰

**关键认知:你的治理 scaffolding 在"真 agent"上反而更值钱** -- 真 agent 跑得久、花得多、容易跑飞,正需要 SLO/成本/回放/eval 兜底。所以不是推倒重来,是把 `engine.go` 那 5 根线接上,平台就从"有 agent 功能的系统"变成"有真 agent + 业界最深治理的平台"。

---

## 第七部分:一页纸

```
┌──────────────────────────────────────────────────────────────────┐
│  现状: 治理面领先 · 编排面及格 · agent 本身停在"工具循环"阶段        │
│                                                                    │
│  ★ 强项(护城河): LLM网关/Prompt管理/RAG评测/SLO/成本/回放/调度     │
│  ★ 强项: 浏览器agent + MCP + 工具审批                              │
│                                                                    │
│  ✗ 最致命(建了没接):                                                │
│     ① 记忆服务没接进 engine (main.go:155, engine.go:67)            │
│     ② checkpoint 禁用 (engine.go:84, main.go 无 SetCheckpointStore) │
│                                                                    │
│  ✗ 架构缺: 无规划器 · 反思不闭环(StrategyAdjuster死码) ·            │
│            无verifier · MaxSteps=10 · 无上下文管理 · 无工具验证      │
│                                                                    │
│  ✗ 过度建设: Rollback/Chaos半成品 · Evolve风险 · MongoDB死码        │
│                                                                    │
│  -> 第一刀: 接记忆这根线 (东西都建好了,只差接线)                    │
│  -> 第二刀: 接 checkpoint (一行 SetCheckpointStore)                 │
│  -> 第三刀: planner + verifier + 闭环反思                            │
│  -> 战略: 治理面独立成可被任何框架接入的控制平面                      │
└──────────────────────────────────────────────────────────────────┘
```

---

## 附录:核心代码位置索引

| 关注点 | 文件:行 |
|--------|---------|
| Engine 主循环 | `pkg/agent/engine.go:531` (`executeLoop`) |
| Run 入口 | `pkg/agent/engine.go:244` |
| 消息构建(该接 recall) | `pkg/agent/engine.go:749` (`buildAgentMessages`) |
| Engine struct(该加 memory) | `pkg/agent/engine.go:67` |
| NewEngine(该初始化 checkpoint) | `pkg/agent/engine.go:84` |
| 任务完成反思(打分丢弃) | `pkg/agent/engine.go:614` |
| 步后反思(唯一弱闭环) | `pkg/agent/engine.go:691` |
| 错误分析(只到建议) | `pkg/agent/engine.go:457` |
| 停止条件(无 verifier) | `pkg/agent/engine.go:607` |
| MaxSteps/History 配置 | `pkg/agent/engine.go:28` |
| 反思结果存内存不持久化 | `pkg/agent/reflection/loop.go:175` |
| StrategyAdjuster 死代码 | `pkg/agent/reflection/loop.go:573` |
| engine 装配(无 memory) | `services/agent-service/internal/service/agent_service.go:69` |
| service 装配(无 memory) | `services/agent-service/cmd/main.go:155` |
| MongoDB 仅用于注册表 | `services/agent-service/cmd/main.go:93` |
