# Agent Platform 全方位对标分析

> 生成日期: 2026-07-01
> 分析对象: 本仓库 (`agent-platform`) — 8 微服务 + React 前端的 Go 微服务 agent 平台
> 对标范围: 主流 agent 编排框架 (LangGraph / CrewAI / AutoGen / OpenAI Agents SDK / Claude Agent SDK / Dify / Coze / Semantic Kernel) + 基础设施平台 (Ray Serve / BentoML / Modal / RunPod) + 可观测平台 (LangSmith / LangFuse / Phoenix) + 记忆系统 (MemGPT·Letta / Zep)

---

## 0. TL;DR — 一句话结论

| 维度 | 判断 |
|------|------|
| **整体定位** | 这是一个**「治理优先、编排够用」**的平台。治理面 (harness) 的广度和深度是最大亮点,agent 编排面是及格线,基础设施面是短板。 |
| **最该骄傲的** | 81 个 gRPC 方法覆盖的治理矩阵 — 灰度、SLO、成本、RCA、调度、prompt 版本、会话回放、RAG 评测、LLM 网关。这套东西 LangGraph/CrewAI 都没有,连 LangSmith 也只覆盖其中一部分。 |
| **最致命的缺口** | ① 测试覆盖率 ≈ 0% ② 无真实鉴权 ③ 无 CI/CD ④ OTel 写了库却没接到服务启动里 ⑤ 代码执行/文件读写工具是 stub。 |
| **最明显的过度建设** | Rollback / Chaos / Evolve(自动调优)在 demo 阶段用不上,运维心智负担 > 收益。MongoDB 定义了却没真正用。 |
| **与主流框架的关系** | 编排层对标 OpenAI Agents SDK + LangGraph;治理层对标 LangSmith + LangFuse + 自建控制台;记忆层对标 MemGPT。**你一个人把三家公司的活都干了**,广度赢,深度和工程化输。 |

---

## 1. 平台现状速览

### 1.1 架构总览

```
                         ┌──────────────────────────────────────────┐
   React 19 + AntD 6 ──► │  Gateway (Gin :9000)                     │
   Vite 8 · Zustand      │  路由 / CORS / 租户 / 限流 / 鉴权(stub)  │
                         └───────────────┬──────────────────────────┘
                                         │ gRPC (protobuf)
        ┌────────────────┬───────────────┼───────────────┬─────────────────┐
        ▼                ▼               ▼               ▼                 ▼
   Chat :50001     Knowledge :50002  Memory :50003   A2A :50004      MCP :50005
   会话/流式        RAG·BM25+向量    三层记忆+遗忘    Agent2Agent     工具注册/执行
                                                                     (6 真 + 3 stub)
        ▼                                                                          ▼
   Agent :50006                                                          Harness :50007 (★ 核心)
   pkg/agent 编排引擎                                                     81 方法 · 16 模块 · ~22.7k LOC
   handoff / DAG / 反思 / checkpoint                                       SLO/AB/成本/RCA/调度/prompt/回放/网关/工作流/评测
                                         │
        ┌────────────────┬───────────────┼───────────────┬────────────────┐
        ▼                ▼               ▼               ▼                ▼
   SQLite (Gorm)    Qdrant (向量)    Redis (缓存/限流)  MongoDB(定义未用)  OTel Collector
   元数据/治理数据   知识库+记忆召回   token bucket      文档存储预留       (未接到服务启动)
```

### 1.2 量化指标

| 指标 | 数值 |
|------|------|
| 微服务数 | 8 |
| Docker 容器数 | 14 |
| gRPC 方法总数 (harness) | 81 (其中 ~75 已接线实现) |
| Go 代码总量 | services ~43k 行 + pkg ~75k 行 |
| 最大单文件 | `harness_service.go` 3,799 行 (⚠️ 超过 800 行红线) |
| 前端页面 | 15 个目录 / ~55 个 TSX |
| 测试文件 | **1 个** (`demo/retry/retry_test.go`) |
| 测试覆盖率 | **≈ 0%** |
| CI/CD 流水线 | **无** |
| LLM Provider | OpenAI + DashScope (Qwen) 已实现;Azure/Anthropic 仅 stub |

---

## 2. 与主流框架的全维度对比

### 2.1 Agent 编排能力对比

| 能力 | 本平台 | LangGraph | CrewAI | AutoGen | OpenAI Agents SDK | Claude Agent SDK | Dify | Coze |
|------|--------|-----------|--------|---------|-------------------|------------------|------|------|
| **单 agent 循环** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| **Handoff 切换** | ✅ | ✅(节点) | ✅(角色) | ✅(对话) | ✅(原生) | ✅ | ❌ | ❌ |
| **Group Chat / 轮询** | ❌ | ✅ | ✅ | ✅(强项) | ❌ | ❌ | ❌ | ✅ |
| **DAG 工作流** | ✅ | ✅(图原生) | ❌ | ❌ | ❌ | ❌ | ✅(可视化) | ✅(可视化) |
| **并行工具** | ✅(信号量≤10) | ✅ | ✅ | ✅ | ✅ | ✅ | 部分 | 部分 |
| **人机审批(HITL)** | ✅(approval flow) | ✅ | ✅ | ✅(强项) | ✅ | ✅ | ✅ | ✅ |
| **反思循环** | ✅(打分) | ✅(节点) | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |
| **Checkpoint/恢复** | ✅(内存) | ✅(持久化,强项) | ❌ | ❌ | ✅(session) | ✅ | ❌ | ❌ |
| **干预(暂停/停止)** | ✅ | ✅ | ❌ | ❌ | 部分 | 部分 | ❌ | ❌ |
| **流式输出** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| **代码执行沙箱** | ❌(stub) | ⚠️(需外接) | ⚠️ | ✅(Docker) | ⚠️ | ✅ | ✅ | ✅ |
| **可视化编排** | ❌(前端有 DAG 编辑器,但后端编排仍代码) | ⚠️(Studio) | ❌ | ❌ | ❌ | ❌ | ✅(强项) | ✅(强项) |

**结论**: 编排面**及格但不领先**。Handoff + DAG + 反思 + checkpoint 的组合在框架里算完整,但缺 Group Chat,且代码执行沙箱是 stub(致命短板)。LangGraph 在"可持久化的图"上仍是标杆;Dify/Coze 在"非开发者可用"上完胜你。

### 2.2 工具与代码执行对比

| 能力 | 本平台 | Modal | OpenAI Codex | AutoGen | Dify |
|------|--------|-------|--------------|---------|------|
| **内置工具数** | 9 (6 真 + 3 stub) | N/A(平台) | 编码为主 | 多种 | 插件市场 |
| **浏览器自动化** | ✅(chromedp,强项) | ❌ | ❌ | ❌ | ⚠️ |
| **代码沙箱** | ❌(mock 输出) | ✅(sub-10ms,强项) | ✅ | ✅(Docker) | ✅ |
| **文件读写** | ❌(stub) | ✅ | ✅ | ✅ | ✅ |
| **工具治理(审批/权限)** | ✅(强项) | ❌ | ❌ | ❌ | ❌ |
| **MCP 协议** | ✅(实现了 MCP service) | ❌ | ⚠️ | ❌ | ❌ |

**结论**: 浏览器自动化 + MCP + 工具审批是你的差异化优势。但 `code_execute`/`file_read`/`file_write` 是 stub,意味着 agent **实际上不能写代码、不能读写文件** —— 这对"agent"而言是功能性硬伤。Modal 的 Sandbox 原语是这个领域目前最好的解法。

### 2.3 记忆系统对比

| 能力 | 本平台 | MemGPT/Letta | Zep | LangGraph | OpenAI |
|------|--------|--------------|-----|-----------|--------|
| **工作记忆** | ✅ | ✅(核心) | ✅ | ✅(thread) | ✅ |
| **情景记忆** | ✅(Qdrant) | ✅ | ✅ | ❌ | ❌ |
| **语义记忆** | ✅(Qdrant) | ✅ | ✅ | ❌ | ❌ |
| **向量召回** | ✅(Qdrant) | ✅ | ✅ | ⚠️ | ⚠️ |
| **遗忘机制** | ✅(衰减曲线) | ✅ | ✅(强项) | ❌ | ❌ |
| **记忆压缩/摘要** | ❌ | ✅(核心,强项) | ✅ | ❌ | ❌ |
| **知识图谱关联** | ❌ | ✅ | ✅(强项) | ❌ | ❌ |
| **持久化** | ⚠️(Qdrant+SQLite) | ✅ | ✅ | ✅ | ✅ |

**结论**: 三层记忆 + 向量召回 + 遗忘已经超过 LangGraph/OpenAI。但**缺记忆压缩摘要 + 缺知识图谱关联**,这两项恰恰是 MemGPT 和 Zep 的护城河 —— 长对话场景下你的上下文会越涨越大,而 MemGPT 会自动摘要。

### 2.4 治理 / 可观测对比 (★ 你的主场)

| 能力 | 本平台 | LangSmith | LangFuse | Phoenix |
|------|--------|-----------|----------|---------|
| **调用追踪(trace)** | ✅(库已写,未接线) | ✅(强项) | ✅(强项) | ✅ |
| **评测(eval suite)** | ✅(RAGAS 16 指标) | ✅ | ✅ | ✅ |
| **数据集管理** | ❌ | ✅ | ✅ | ✅ |
| **Prompt 版本管理** | ✅(16 方法,强项) | ✅ | ✅ | ❌ |
| **A/B 实验** | ✅(统计显著性) | ⚠️ | ⚠️ | ❌ |
| **SLO 管理** | ✅(burn rate) | ❌ | ❌ | ❌ |
| **成本追踪** | ✅(模型定价+建议) | ✅ | ✅ | ⚠️ |
| **RCA 根因分析** | ✅(独有) | ❌ | ❌ | ❌ |
| **特征开关** | ✅(独有) | ❌ | ❌ | ❌ |
| **调度器** | ✅(独有) | ❌ | ❌ | ❌ |
| **会话回放** | ✅(独有) | ⚠️ | ⚠️ | ❌ |
| **LLM 网关(负载/降级)** | ✅(4 策略,强项) | ❌ | ❌ | ❌ |
| **自动调优(Evolve)** | ✅(独有) | ❌ | ❌ | ❌ |
| **自托管/开源自部署** | ✅ | ❌(SaaS) | ✅ | ✅ |

**结论**: 治理面是**全表最强的差异化**。LangSmith/LangFuse 在 trace + eval + dataset 上做得更深、UI 更成熟,但**你有的 SLO/RCA/特征开关/调度器/LLM 网关/AutoTune,它们全都没有**。这套东西更像一个"agent 的 SRE 控制平面",定位独特。

### 2.5 基础设施 / 部署对比

| 能力 | 本平台 | Ray Serve | BentoML | Modal | RunPod |
|------|--------|-----------|---------|-------|--------|
| **GPU 调度** | ❌ | ✅ | ✅ | ✅ | ✅(强项) |
| **自动扩缩容** | ❌ | ✅(强项) | ✅ | ✅ | ✅ |
| **Serverless 冷启动** | ❌ | ⚠️ | ⚠️ | ✅(sub-10ms) | ✅ |
| **多模型组合** | ⚠️(网关层) | ✅(强项) | ✅ | ✅ | ⚠️ |
| **Docker 优先** | ✅ | ✅ | ✅ | ❌(自有) | ✅ |
| **K8s 原生** | ❌(Makefile 引用但无 manifest) | ✅ | ✅ | ❌ | ⚠️ |
| **CI/CD** | ❌ | ✅ | ✅ | ✅ | ✅ |
| **向量库内置** | ✅(Qdrant) | ❌ | ❌ | ❌ | ❌ |

**结论**: 基础设施面是**明显短板**。你把 agent 当业务系统做(Docker compose + SQLite),而 Ray/BentoML/Modal 把它当推理基础设施做(扩缩容 + GPU + serverless)。规模一上来,单机 SQLite + 无扩缩容会先顶不住。

### 2.6 鉴权 / 安全对比

| 能力 | 本平台 | Dify | Coze | 企业级框架 |
|------|--------|------|------|-----------|
| **JWT 鉴权** | ❌(stub pass-through) | ✅ | ✅ | ✅ |
| **RBAC** | ⚠️(矩阵存在未强制) | ✅ | ✅ | ✅ |
| **租户隔离** | ✅(数据层) | ✅ | ✅ | ✅ |
| **限流** | ✅(Redis token bucket) | ✅ | ✅ | ✅ |
| **Guardrail** | ⚠️(关键词匹配) | ✅ | ✅ | ✅ |
| **Secret 管理** | ❌(config.yaml 明文) | ✅ | ✅ | ✅ |

**结论**: 安全面是**最大风险**。鉴权 stub + 明文密钥 + guardrail 仅关键词 = 无法上生产。这是 Dify/Coze 这种商业产品和你差距最大的地方,也是上生产前必须补的。

---

## 3. 缺少什么 (GAP 分析)

按"上生产前必须补"到"锦上添花"分三档。

### 🔴 P0 — 阻断生产 (必须补)

| # | 缺口 | 现状 | 影响 | 对标 |
|---|------|------|------|------|
| 1 | **测试** | 1 个测试文件,0% 覆盖 | 任何重构都是赌博,回滚无信心 | 所有框架都要求 ≥80% |
| 2 | **CI/CD** | 无流水线 | 变更全靠手动 deploy 脚本,易错 | Ray/BentoML 全有 |
| 3 | **真实鉴权** | `Auth()` 中间件 pass-through,`// TODO: Validate JWT` | 任何人可调任何 API | Dify/Coze/LangGraph 全有 |
| 4 | **Secret 管理** | `config.yaml` 明文 API key | 密钥泄露风险 | 所有生产框架 |
| 5 | **代码执行沙箱** | `code_execute` 返回 mock | agent 不能写/跑代码,名不副实 | Modal/AutoGen/Dify |
| 6 | **文件读写工具** | stub 返回固定内容 | agent 不能操作文件 | 全部框架 |
| 7 | **OTel 接线** | 库写好了但 `main.go` 没初始化 | 有库无数据,trace 看不见 | LangSmith/LangFuse |
| 8 | **checkpoint 持久化** | 用 `MemoryCheckpointStore` | 重启即丢所有进行中任务 | LangGraph(Persistence) |

### 🟡 P1 — 影响规模化 (建议补)

| # | 缺口 | 现状 | 对标 |
|---|------|------|------|
| 9 | **分布式 trace 传播** | 服务间无 trace 上下文透传 | LangSmith 跨服务链路 |
| 10 | **记忆压缩/摘要** | 长对话上下文无界增长 | MemGPT(核心能力) |
| 11 | **知识图谱** | 记忆间无关联 | Zep(强项) |
| 12 | **Group Chat 策略** | proto 定义了但只实现 handoff | AutoGen(强项) |
| 13 | **数据集管理** | eval 有但无 dataset 版本化 | LangSmith/LangFuse |
| 14 | **K8s manifest** | Makefile 引用但目录不存在 | Ray/BentoML |
| 15 | **自动扩缩容** | 无 | Ray/Modal/RunPod |
| 16 | **ML guardrail** | 仅关键词匹配 | Dify(模型检测) |
| 17 | **结构化日志** | 生产代码有 `fmt.Printf` | 所有生产系统 |
| 18 | **MongoDB 接入或移除** | 定义了 pkg/mongodb 但无服务用 | 死代码 |

### 🟢 P2 — 锦上添花

| # | 缺口 | 说明 |
|---|------|------|
| 19 | **可视化编排落地** | 前端有 DAG 编辑器,但后端编排仍是代码;Dify/Coze 是真·零代码 |
| 20 | **Anthropic/Azure 原生 client** | 现在只走 OpenAI 兼容协议,丢掉了 Anthropic 的 prompt caching 等特性 |
| 21 | **多模态** | 无图像/音频处理 |
| 22 | **GPU 推理** | 全部走外部 LLM API,无本地模型 |
| 23 | **评估数据集自动采集** | 从真实会话生成 eval set |

---

## 4. 多了什么 (过度建设 / 冗余)

按"删/砍/降级"建议排序。

### 4.1 demo 阶段用不上的治理模块

| 模块 | 状态 | 问题 | 建议 |
|------|------|------|------|
| **Rollback (5 方法)** | proto 定义,handler 未接线 | 半成品,有 proto 无实现 | **要么补实现,要么从 proto 删** — 半成品比没有更糟 |
| **Chaos (4 方法)** | proto 定义,handler 未接线 | 同上,且 chaos 工程在 demo 期无意义 | **从 proto 删除**,后期需要再加 |
| **Evolve / AutoTune** | 已实现,7 方法 | 自动改 agent 配置,风险高, demo 期无数据驱动调优 | **降级为只读建议**,不要自动 execute |
| **Golden Path** | 已实现,3 方法 | 模板实例化,与 agent registry 重叠 | 评估是否与 catalog 合并 |

**根因**: 治理面铺得太开(81 方法),每个都"实现了"但都停在"能跑通主路径"。广度优先策略在探索期合理,但现在进入了"要上生产"的阶段,需要收敛。

### 4.2 死代码 / 冗余

| 项 | 位置 | 建议 |
|----|------|------|
| **MongoDB 包** | `pkg/mongodb/` 定义但无服务使用 | 移除或接入一个真实场景 |
| **memory_service_enhanced.go** | 已删除(git status 显示 D) | 确认删除已清理干净 |
| **OTLP metrics exporter** | `otel.go:242` 标记 not implemented | 删掉或补实现,别留 stub 注释 |
| **重复 API 文件** | `api/memory.ts` + `api/memoryApi.ts`,`api/cost.ts`+`costApi.ts` 等 | 合并去重,防漂移 |
| **预编译二进制** | `gateway-linux`,`harness-service-linux` 提交进仓库 | 用 CI 产物替代,不入 git |

### 4.3 架构层面的"多"

| 项 | 问题 | 建议 |
|----|------|------|
| **8 个微服务** | chat/agent/knowledge/memory/mcp/a2a/harness 拆得过细,服务间 gRPC 往返增加延迟和运维成本 | demo 期可考虑合并为 2-3 个 (业务面 + 治理面),规模上来再拆 |
| **SQLite 作主库** | 多服务各写各的 SQLite,无统一数据视图,无法横向扩展 | 上量后必须迁 PostgreSQL |
| **harness_service.go 3,799 行** | 远超 800 行红线,单文件 81 方法的业务逻辑 | 按模块拆分文件 (prompt/engine.go 已经做了,service 层跟上) |
| **proto 81 方法单 service** | 单一 HarnessService 承载过多职责 | 考虑按域拆 service (SLOService, PromptService...) |

---

## 5. 各模块成熟度评分

> 满分 5 星。★ = 能跑通主路径,★★★ = 可上生产,★★★★★ = 业界领先。

### 5.1 编排与 agent

| 模块 | 评分 | 说明 |
|------|------|------|
| Agent 引擎 (run loop) | ★★★☆ | 完整,但缺 group chat |
| Handoff | ★★★★ | 实现干净 |
| DAG 工作流 | ★★★☆ | 5 种节点齐全 |
| 并行工具 | ★★★☆ | 信号量限流 OK |
| 反思循环 | ★★★☆ | 有打分,缺实际改进闭环 |
| Checkpoint | ★★☆☆ | 仅内存,重启即丢 |
| 浏览器 agent | ★★★★ | chromedp 全功能,差异化 |
| 代码执行 | ★☆☆☆ | stub,致命 |
| 人机审批 | ★★★☆ | 流程在,缺 UI 闭环 |

### 5.2 治理 (harness)

| 模块 | 评分 | 说明 |
|------|------|------|
| LLM 网关 | ★★★★ | 4 策略+降级+限流,领先 |
| Prompt 管理 | ★★★★ | 16 方法,版本/diff/性能,领先 |
| RAG 评测 | ★★★★ | 16 RAGAS 指标,领先 |
| 成本管理 | ★★★☆ | 定价+建议,实用 |
| A/B 实验 | ★★★☆ | 有统计显著性 |
| SLO | ★★★☆ | burn rate,独有 |
| 会话回放 | ★★★☆ | 独有 |
| 调度器 | ★★★☆ | 独有 |
| 特征开关 | ★★★☆ | 有 stale 检测 |
| RCA | ★★☆☆ | 独有但实现浅 |
| Evolve | ★★☆☆ | 风险大于收益 |
| Rollback/Chaos | ☆☆☆☆ | proto 有,实现无 |

### 5.3 基础设施

| 模块 | 评分 | 说明 |
|------|------|------|
| 可观测库 | ★★★☆ | 写得好,没接线 |
| 鉴权 | ★☆☆☆ | stub |
| 测试 | ☆☆☆☆ | 0% |
| CI/CD | ☆☆☆☆ | 无 |
| 部署 | ★★☆☆ | Docker compose 够 demo |
| 安全 | ★☆☆☆ | 明文密钥 |

---

## 6. 战略定位建议

### 6.1 你到底该是什么?

当前你的平台同时想做三件事:
1. **agent 编排引擎** (对标 LangGraph/CrewAI) — 及格
2. **治理 / 可观测控制台** (对标 LangSmith/LangFuse + 自研 SRE) — 领先
3. **应用平台** (对标 Dify/Coze) — 落后(无可视化、无鉴权)

**建议聚焦**: 把 (2) 当核心卖点,(1) 保持及格,(3) 暂缓。理由:
- (2) 是你唯一有差异化护城河的地方 —— SLO/RCA/特征开关/调度器/AutoTune,LangSmith 都没有
- (1) 是红海,Lan­gGraph 背靠 LangChain 生态,你拼不过
- (3) 需要大量产品+设计投入,Dify/Coze 已占位

### 6.2 三种走法

**走法 A:开源治理控制平面 (推荐)**
- 把 harness 独立成"任何 agent 框架都能接的治理层"
- 提供 SDK,让 LangGraph/CrewAI 用户把 trace/eval/SLO 接到你的平台
- 对标 LangFuse 开源路线,但多了 SRE 能力
- 优势:差异化清晰,不和编排框架正面竞争

**走法 B:垂直一体化 agent 平台**
- 继续全栈自研,补齐鉴权/沙箱/CI/CD
- 对标 Dify/Coze,靠治理深度取胜
- 风险:战线太长,工程化欠债多

**走法 C:内部基础设施**
- 不对外,服务自己公司的 agent 业务
- 优先补 P0 (测试/鉴权/沙箱),砍掉 Evolve/Chaos/Rollback
- 最务实,适合当前阶段

---

## 7. 行动清单 (按优先级)

### 本周 (P0 阻断项)
- [ ] 给 `code_execute` 接入真实沙箱 (Docker 或 gVisor,参考 Modal Sandbox)
- [ ] 实现 `file_read`/`file_write` (限制工作目录)
- [ ] 把 `Auth()` 中间件改成真 JWT 校验
- [ ] 密钥从 config.yaml 迁到环境变量
- [ ] 把 OTel manager 接到每个服务的 `main.go` 启动流程
- [ ] checkpoint store 从内存切到 MongoDB(已有 mongo_store.go)

### 本月 (P1 规模化)
- [ ] 给核心包 (pkg/agent, pkg/llm, harness 关键模块) 补单测,目标 60%→80%
- [ ] 加 GitHub Actions: lint + test + docker build
- [ ] 服务间 gRPC 透传 trace context (otelgrpc 已在 go.mod)
- [ ] 移除 MongoDB 死代码 或 接入一个真实场景
- [ ] 从 proto 删除 Rollback/Chaos (半成品)
- [ ] Evolve 降级为只读建议,关闭 auto-execute
- [ ] 合并重复的 frontend api 文件

### 本季度 (P2 收敛)
- [ ] 评估服务合并 (8 → 4)
- [ ] SQLite → PostgreSQL 迁移评估
- [ ] 拆分 `harness_service.go` 按模块分文件
- [ ] 接入 Anthropic 原生 client (利用 prompt caching 降本)
- [ ] 补记忆压缩/摘要 (对标 MemGPT)
- [ ] 评估 K8s manifest (对接 Ray/Kruise)

---

## 8. 一页纸总结

```
┌─────────────────────────────────────────────────────────────────┐
│  你的 agent 平台 = LangGraph(及格) + LangFuse(领先) + Dify(落后) │
│                                                                   │
│  ★ 强项: 治理控制平面 (SLO/RCA/网关/调度/prompt/评测) — 独一档    │
│  ★ 强项: 浏览器自动化 + MCP + 工具审批                            │
│  ★ 强项: 三层记忆 + 向量召回 + 遗忘                               │
│                                                                   │
│  ✗ 致命缺口: 0% 测试 · 无鉴权 · 无 CI/CD · 代码执行是 stub       │
│  ✗ 过度建设: Rollback/Chaos 半成品 · Evolve 风险 · MongoDB 死码  │
│  ✗ 架构债: 8 服务过细 · SQLite 单点 · 3799 行单文件              │
│                                                                   │
│  → 短期: 补 P0(沙箱/鉴权/测试/OTel接线), 砍半成品治理模块        │
│  → 中期: 把治理面独立成可被任何框架接入的控制平面 (走法 A)        │
│  → 长期: 治理深度是你的护城河, 别在编排红海里和 LangGraph 拼      │
└─────────────────────────────────────────────────────────────────┘
```

---

## 附录 A: 框架速查卡

| 框架 | 类型 | Stars(约) | 一句话 | 你的对应能力 |
|------|------|----------|--------|-------------|
| LangGraph | 编排 | 11k+ | 可持久化的状态图 | DAG 工作流 (不及持久化) |
| CrewAI | 编排 | 25k+ | 角色化多 agent | Handoff |
| AutoGen | 编排 | 35k+ | 对话式多 agent + 代码执行 | Group Chat 缺 |
| OpenAI Agents SDK | 编排 | 20k+ | 原生 handoff + guardrail | Handoff + Guardrail |
| Claude Agent SDK | 编排 | — | 工具 + 子 agent | 类似 |
| Dify | 平台 | 55k+ | 可视化 workflow + RAG | 编排 UI 落后 |
| Coze | 平台 | — | 字节系 bot 平台 | 编排 UI 落后 |
| Ray Serve | 基础设施 | 33k+ | 分布式推理 serving | 无扩缩容 |
| BentoML | 基础设施 | 7k+ | 模型 serving + 网关 | LLM 网关有 |
| Modal | 基础设施 | 10k+ | Serverless 沙箱 | 代码沙箱缺 |
| RunPod | 基础设施 | 8k+ | 低价 GPU serverless | 无 GPU |
| LangSmith | 可观测 | — | trace + eval + dataset | eval 有, trace 未接线 |
| LangFuse | 可观测 | 6k+ | 开源可观测 | 治理更广 |
| MemGPT/Letta | 记忆 | 12k+ | 分层记忆 + 压缩 | 三层有, 压缩缺 |
| Zep | 记忆 | 3k+ | 对话记忆 + 知识图谱 | 知识图谱缺 |

## 附录 B: 数据来源

- 代码库全量扫描 (Explore agent): 8 服务、81 gRPC 方法、~118k Go LOC、15 前端页面
- 框架特性: 各框架官方文档 + GitHub (数据截至 2026-07)
- 注: 部分 star 数为近似值,框架版本迭代快,以官方为准
