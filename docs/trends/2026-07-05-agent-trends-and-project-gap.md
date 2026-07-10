# 最近一个月 Agent 发展趋势 × 项目对比

> 文档日期: 2026-07-05
> 数据截止: 2026-06-24（来自项目内 `docs/research/2026-agent-projects.md`，GitHub API 实时查询）
> 时间窗口: 2026-06-05 ~ 2026-07-05

---

## 目录

- [一、数据来源说明](#一数据来源说明)
- [二、最近一个月 Agent 趋势 Top 10](#二最近一个月-agent-趋势-top-10)
- [三、关键开源项目清单](#三关键开源项目清单)
- [四、结合本项目对比](#四结合本项目对比)
- [五、Gap 分析](#五gap-分析)
- [六、建议优先方向](#六建议优先方向)
- [七、一句话总结](#七一句话总结)

---

## 一、数据来源说明

| 来源 | 状态 | 说明 |
|------|------|------|
| WebSearch 工具 | ❌ 返回空结果 | 沙盒内搜索接口未返回数据 |
| curl 外网访问 | ❌ DNS 解析超时 | 沙盒网络受限 |
| 项目内 `docs/research/2026-agent-projects.md` | ✅ 可用 | 数据截至 2026-06-24，GitHub API 实时查询，收录 60+ 个 2025-10 后新项目 |
| 项目内 `docs/research/2026-agent-technology.md` | ✅ 可用 | 覆盖 2026 年 6 月架构趋势（Deep Agents / Loop Engineering / MCP） |
| 项目内 `docs/research/superpowers-vs-agent-skills.md` | ✅ 可用 | 技能框架对比 |
| 项目内 `docs/harness-eval/08-implementation-roadmap.md` | ✅ 可用 | P0/P1/P2 实施路线图 |

**结论**：项目内已有截至 6 月 24 日的详细趋势研究，距今天 11 天，覆盖"最近一个月"窗口，数据质量高于搜索引擎。

---

## 二、最近一个月 Agent 趋势 Top 10

### 🔥 趋势 1：Agent Skills 技能市场（最热赛道，无之一）

**核心逻辑**：Agent 框架已经够多了，真正缺的是**高质量的技能/指令**。类似 npm 之于 Node.js。

| 项目 | Stars | 创建时间 | 说明 |
|------|-------|---------|------|
| obra/superpowers | 237K | 2025-10 | Agent 技能框架 + 软件开发方法论，强制 Agent 按工程师方式工作 |
| affaan-m/ECC | 221K | 2026-01 | Agent Harness 性能优化（技能/本能/记忆/安全）|
| addyosmani/agent-skills | 66K | 2026-02 | Google 工程师 Addy Osmani 出品，生产级技能库 |
| antigravity-awesome-skills | 42K | 2026-01 | 1600+ 可安装技能库 |
| coreyhaines31/marketingskills | 35K | 2026-01 | 营销垂直领域技能（CRO/SEO/文案）|

> superpowers + ECC 两个项目加起来 **458K stars**，比所有 Agent 框架加起来都多。**技能市场是 2026 年最大增量赛道。**

**两种理念流派**：
- **Superpowers**：严格强制，"不写测试就删代码"，堵住 Agent 偷懒
- **Agent-Skills**：引导推荐，"先规格后代码，逐步实现"，给 Agent 路走

---

### 🧠 趋势 2：上下文/记忆优化成为新瓶颈

Agent 的瓶颈从"能不能做"变成"**记不记得住、context 够不够用**"。

| 项目 | Stars | 创建时间 | 说明 |
|------|-------|---------|------|
| colbymchenry/codegraph | 54K | 2026-01 | 代码知识图谱，自动同步代码变更，给 Agent 提供代码索引 |
| headroomlabs-ai/headroom | 49K | 2026-01 | 压缩工具输出/日志/RAG，**节省 60%+ token** |
| volcengine/OpenViking | 26K | 2026-01 | 字节跳动出品，专为 Agent 设计的上下文数据库 |
| gastownhall/beads | 25K | 2025-10 | **Go 语言** Agent 记忆升级插件 |
| rohitg00/agentmemory | 24K | 2026-02 | Agent 持久化记忆（基准测试第一）|
| OthmanAdi/planning-with-files | 24K | 2026-01 | 基于文件的持久化规划，长任务不丢上下文 |
| mksglu/context-mode | 18K | 2026-02 | 上下文窗口优化，沙箱化工具输出 |

---

### 🌐 趋势 3：浏览器 Agent 2.0

从简单爬取升级为**完整浏览器操控**。

| 项目 | Stars | 创建时间 | 说明 |
|------|-------|---------|------|
| Panniantong/Agent-Reach | 39K | 2026-02 | 给 Agent 联网能力（Twitter/YouTube/HN）|
| vercel-labs/agent-browser | 37K | 2026-01 | Vercel 出品，Rust 高性能浏览器 Agent CLI |
| jackwener/OpenCLI | 25K | 2026-03 | 任何网站变 CLI，Agent 可用已登录浏览器 |
| h4ckf0r0day/obscura | 16K | 2026-04 | Agent 专用无头浏览器（**本项目 docker 内有 obscura 容器**）|
| microsoft/Webwright | 6K | 2026-04 | 微软 SOTA 浏览器 Agent 框架 |
| jo-inc/camofox-browser | 7K | 2026-01 | 隐身无头浏览器，绕过 Cloudflare |

> ⚠️ 本项目当前正在 `feature/optimize-browser-agent` 分支上工作，与该趋势直接对齐。

---

### 🤝 趋势 4：多 Agent 团队编排成为主流

| 项目 | Stars | 创建时间 | 说明 |
|------|-------|---------|------|
| code-yeongyu/oh-my-openagent | 63K | 2025-12 | 面向 token 优化的 Agent Harness |
| Yeachan-Heo/oh-my-claudecode | 37K | 2026-01 | "Teams-first" 多 Agent 团队编排 |
| openai/symphony | 26K | 2026-02 | OpenAI 出品，项目工作变自主实现运行（Elixir）|
| cft0808/edict | 16K | 2026-02 | OpenClaw 多 Agent 编排，9 个专业 Agent |
| open-multi-agent/open-multi-agent | 6K | 2026-03 | TS 多 Agent 编排框架，描述目标自动编排 |

---

### 🦀 趋势 5：Rust 重写基础设施

高性能 Agent 基础设施全面转向 Rust：

| 项目 | Stars | 说明 |
|------|-------|------|
| ultraworkers/claw-code | 194K | Rust Agent 演示 |
| zeroclaw-labs/zeroclaw | 32K | 全自主 AI 助理基础设施，离线优先 |
| vercel-labs/agent-browser | 37K | 浏览器 Agent CLI |
| h4ckf0r0day/obscura | 16K | Agent 专用无头浏览器 |
| RightNow-AI/openfang | 18K | 开源 Agent 操作系统 |

---

### 🏢 趋势 6：Agent 操作系统化

Agent 不再是工具，是 OS 级基础设施：

| 项目 | Stars | 说明 |
|------|-------|------|
| RightNow-AI/openfang | 18K | 开源 Agent 操作系统 |
| zeroclaw-labs/zeroclaw | 32K | 全自主 AI 个人助理基础设施 |
| EverMind-AI/EverOS | 9K | 自进化记忆层，跨 Agent/平台可移植 |

---

### 🔁 趋势 7：Deep Agents + 四层循环工程

架构层面核心趋势（详见 `docs/research/2026-agent-technology.md`）：

```
Loop 4: 爬山循环   ← 分析 traces 改进 harness（自动化自进化）
Loop 3: 事件驱动   ← cron / webhooks / 外部触发
Loop 2: 验证循环   ← grader 检查输出，失败重试
Loop 1: Agent 循环 ← 基础工具调用
```

**Deep Agents 四特征**：
1. 详细系统提示词
2. 规划工具（Todo list）
3. 子代理（任务委托）
4. 文件系统（持久化工作上下文）

**容错三大支柱**：Retries + Timeouts + Error Handlers
**沙盒隔离**：切断"致命三要素"（敏感数据 + 不可信内容 + 外部通信）

---

### 🔌 趋势 8：MCP 成为"AI 应用的 USB-C"

Model Context Protocol 标准化工具调用，一次开发到处集成（Claude / ChatGPT / VS Code / Cursor）。

```
AI Client (Claude/ChatGPT)
        │
        │ MCP 协议
        │
   数据源 / 工具 / 工作流 / 应用
```

---

### 🎬 趋势 9：Agent 进入垂直领域

| 领域 | 项目 | Stars |
|------|------|-------|
| 科研 | karpathy/autoresearch | 88K |
| 视频 | heygen-com/hyperframes | 31K |
| 手机 | zai-org/Open-AutoGLM | 26K |
| 交易 | HKUDS/AI-Trader | 20K |
| 营销 | coreyhaines31/marketingskills | 35K |
| 网络安全 | mukul975/Anthropic-Cybersecurity-Skills | 20K |

---

### 🎯 趋势 10：6 月新热点--"懒高级工程师"范式

| 项目 | Stars | 创建时间 | 说明 |
|------|-------|---------|------|
| DietrichGebert/ponytail | 53K | 2026-06-12 | 让 Agent 像最懒的高级工程师一样思考，最佳代码质量 |

6 月增长最快的新项目。理念是让 Agent **不写多余代码，只做必要的事**--这是对早期 Agent "过度生成"的反思。

---

## 三、关键开源项目清单

### 按赛道分类

#### Agent 技能/上下文工程（最热赛道 🔥🔥🔥）
| 项目 | Stars | 语言 | 一句话 |
|------|-------|------|--------|
| obra/superpowers | 237K | Shell | Agent 技能框架 + 软件开发方法论 |
| affaan-m/ECC | 221K | JS | Agent Harness 性能优化系统 |
| addyosmani/agent-skills | 66K | Shell | 生产级 Agent 编程技能库 |
| Leonxlnx/taste-skill | 50K | JS | 给 Agent "好品味"，不写垃圾代码 |
| sickn33/antigravity-awesome-skills | 42K | Python | 1600+ Agent 技能库 |

#### Agent 基础设施/平台
| 项目 | Stars | 语言 | 一句话 |
|------|-------|------|--------|
| HKUDS/nanobot | 45K | Python | 轻量开源 AI Agent |
| multica-ai/multica | 38K | **Go** | 开源 Agent 管理平台（最接近本项目）|
| zeroclaw-labs/zeroclaw | 32K | Rust | 全自主 AI 助理基础设施 |
| RightNow-AI/openfang | 18K | Rust | 开源 Agent 操作系统 |
| EverMind-AI/EverOS | 9K | Python | 自进化记忆层 |

#### Agent 记忆与上下文优化
| 项目 | Stars | 语言 | 一句话 |
|------|-------|------|--------|
| colbymchenry/codegraph | 54K | TS | 代码知识图谱 |
| headroomlabs-ai/headroom | 49K | Python | 压缩输出，节省 60%+ token |
| volcengine/OpenViking | 26K | Python | 字节出品，Agent 上下文数据库 |
| gastownhall/beads | 25K | **Go** | Agent 记忆升级插件 |
| rohitg00/agentmemory | 24K | TS | Agent 持久化记忆 |
| OthmanAdi/planning-with-files | 24K | Python | 基于文件的持久化规划 |

#### 多 Agent 编排
| 项目 | Stars | 语言 | 一句话 |
|------|-------|------|--------|
| code-yeongyu/oh-my-openagent | 63K | TS | token 优化 Agent Harness |
| Yeachan-Heo/oh-my-claudecode | 37K | TS | Teams-first 多 Agent 编排 |
| openai/symphony | 26K | Elixir | OpenAI 自主实现运行 |
| snarktank/ralph | 21K | TS | 自主 Agent 循环直到 PRD 完成 |
| cft0808/edict | 16K | Python | 9 个专业 Agent 编排 |

#### 浏览器/网页 Agent
| 项目 | Stars | 语言 | 一句话 |
|------|-------|------|--------|
| Panniantong/Agent-Reach | 39K | Python | Agent 联网能力 |
| vercel-labs/agent-browser | 37K | Rust | Vercel 浏览器 Agent CLI |
| jackwener/OpenCLI | 25K | JS | 网站->CLI |
| h4ckf0r0day/obscura | 16K | Rust | Agent 专用无头浏览器 |

#### 编程 Agent（新玩家）
| 项目 | Stars | 语言 | 一句话 |
|------|-------|------|--------|
| karpathy/autoresearch | 88K | Python | Karpathy 出品，AI 自动做研究 |
| DietrichGebert/ponytail | 53K | JS | Agent 像最懒高级工程师思考（6 月新）|
| esengine/DeepSeek-Reasonix | 24K | **Go** | DeepSeek 原生终端编程 Agent |

---

## 四、结合本项目对比

### 本项目定位

**Go + gRPC 微服务架构**的企业级 Agent 平台，已经是同类开源中最完整的。

| 维度 | 本项目 | multica (38K, 最接近开源) |
|------|--------|-------------------------|
| 微服务架构 | ✅ 8 个服务 | ✅ |
| 多 Agent 编排 | ✅ coordinate / planner | ⚠️ 基础 |
| 治理体系 | ✅ **12 个引擎** | ❌ |
| RAG 评测 | ✅ **15 个指标** | ❌ |
| MCP 支持 | ✅ mcp-service | ❌ |
| A2A 协议 | ✅ a2a-service | ❌ |
| 会话回放 | ✅ | ❌ |
| Prompt 管理 | ✅ 版本化 | ❌ |

**核心差异化**：治理 + 评测体系是开源里最全的。

### 本项目已有服务

| 服务 | 端口 | 职责 |
|------|------|------|
| gateway | 9000 | HTTP 网关 |
| chat-service | 50001 | 聊天服务 |
| knowledge-service | 50002 | 知识库 / RAG |
| memory-service | 50003 | 记忆服务 |
| mcp-service | 50004 | MCP 协议支持 |
| a2a-service | 50004/9001 | A2A 协议支持 |
| harness-service | 50007 | 治理 / 评测中心 |
| agent-service | 50006 | Agent 执行引擎 |

### 本项目已有治理引擎（12 个）

Rules、SLO、AB Test、Feature Flag、Rollback、RCA、Chaos、Evolve(Proposals)、Golden Path、Catalog、Coordinate、Planner

### 本项目已有 RAG 评测指标（15 个）

检索质量 7 个 + 生成质量 7 个 + 综合 1 个（详见 `docs/rag-metrics-design.md`）

---

## 五、Gap 分析

| 趋势 | 本项目现状 | 差距 | 影响 |
|------|-----------|------|------|
| 🔥 Agent Skills 技能市场 | ❌ 没有 | **最大缺口** | 错失最大赛道。有 Catalog（Agent 目录）但无 Skills 层 |
| 🧠 上下文压缩 (headroom) | ❌ 没有 | 高 | 长对话成本高，Cost 面板数字偏大 |
| 🧠 代码知识图谱 (codegraph) | ❌ 没有 | 中 | code agent 没有代码索引 |
| 🧠 持久化记忆 | ⚠️ roadmap P1 Step7 | 已规划 | 分层记忆未实现 |
| 🌐 浏览器 Agent | ⚠️ 当前分支在做 | 进行中 | obscura 容器在跑但未深度集成 |
| 🤝 多 Agent 编排 | ✅ 已有 | 低 | 但没有"团队优先"语义 |
| 🔁 四层循环 | ⚠️ 只有 Loop 1 | 高 | 验证循环/事件驱动/爬山循环未实现 |
| 🔁 Rubric 自我评估 | ⚠️ RAG 有，Agent 没有 | 中 | engine.go 没有 grader |
| 🔁 Checkpoint 持久化 | ⚠️ roadmap P1 Step8 | 已规划 | Deep Agents 长任务无法恢复 |
| 🔌 MCP | ✅ 有 mcp-service | 领先 | 领先多数开源 |
| 🏢 沙盒隔离 | ⚠️ 有 obscura 容器 | 中 | 未做内核级 microVM 隔离 |
| 🎬 Workflow DAG 编排 | ⚠️ roadmap P1 Step9 | 已规划 | 声明式编排未实现 |

### Roadmap 与趋势契合度

本项目 `docs/harness-eval/08-implementation-roadmap.md` 的 12 步规划与趋势高度吻合：

| Roadmap 步骤 | 对应趋势 | 优先级匹配 |
|-------------|---------|-----------|
| Step 2 真流式 | 基础能力 | ✅ 正确 |
| Step 3 人工审批 | 趋势 7 容错 | ✅ 正确 |
| Step 4 OTEL 分布式追踪 | 可观测性（LangSmith 方向）| ✅ 正确 |
| Step 7 分层记忆 | 趋势 2（最大缺口之一）| ✅ 正确 |
| Step 8 Checkpoint | Deep Agents（趋势 7）| ✅ 正确 |
| Step 9 Workflow DAG | 趋势 4 多 Agent 编排 | ✅ 正确 |

---

## 六、建议优先方向

### 🥇 第一优先：补 Agent Skills 层（趋势 1，最大赛道）

**现状**：有 Catalog（Agent 目录）+ Golden Path（模板），但缺一个 Skills 层把两者串起来。

**建议**：
- 参考 superpowers 的技能框架设计
- 在 Catalog 里增加 "Skill" 类型（除了 chat/rag/code/analytics 之外）
- 技能 = 可安装的指令模板 + 工具组合 + 验证规则
- 这能让平台从"Agent 运行时"升级为"Agent 应用市场"

**预期收益**：切入最大赛道，形成技能生态

### 🥈 第二优先：上下文压缩（趋势 2，ROI 最高）

**现状**：无 token 优化，长对话成本高。

**建议**：
- 参考 headroom（49K，节省 60%+ token）
- 在 `pkg/agent/engine.go` 工具执行后增加压缩中间件
- 长输出（工具结果、RAG chunks、日志）先压缩再进 LLM context
- 直接降本，Cost 面板会立刻反映效果

**预期收益**：token 成本下降 40-60%

### 🥉 第三优先：Loop 2 验证循环（趋势 7，质量提升）

**现状**：只有 Loop 1（基础工具循环），无自验证。

**建议**：
- engine.go 执行后加 grader 评估
- 复用已有的 RAG 评测能力（15 个指标）作为 grader
- 失败 -> 反馈 -> 重试，最多 N 次
- 这是"自进化 Agent"的基础，也是 Deep Agents 的核心特征

**预期收益**：Agent 输出质量提升，幻觉率下降

---

## 七、一句话总结

> **本项目在"治理 + 评测"上已经是开源最全，但在"技能市场 + 上下文优化 + 自验证循环"这三个 2026 年最热的方向上有缺口。** Roadmap 已覆盖记忆/Checkpoint/Workflow，建议补一个 **Skills 层 + 上下文压缩 + Loop 2 验证循环**，就能从"最完整的治理平台"升级为"最完整的 Agent 应用平台"。

---

## 参考资料

### 项目内文档
- [2026 Agent 技术全景](../research/2026-agent-technology.md) - Deep Agents / Loop Engineering / MCP
- [2025-2026 GitHub 最新热门 Agent 开源项目盘点](../research/2026-agent-projects.md) - 60+ 项目完整清单
- [Superpowers vs Agent-Skills 对比](../research/superpowers-vs-agent-skills.md) - 技能框架选型
- [实施路线图](../harness-eval/08-implementation-roadmap.md) - P0/P1/P2 12 步
- [RAG 评测指标体系设计文档](../rag-metrics-design.md) - 15 个指标详解

### 外部资源（无法直接访问，供参考）
- [obra/superpowers](https://github.com/obra/superpowers) - 237K stars
- [affaan-m/ECC](https://github.com/affaan-m/ECC) - 221K stars
- [headroomlabs-ai/headroom](https://github.com/headroomlabs-ai/headroom) - 49K stars
- [vercel-labs/agent-browser](https://github.com/vercel-labs/agent-browser) - 37K stars
- [multica-ai/multica](https://github.com/multica-ai/multica) - 38K stars（Go，最接近本项目）
- [LangGraph 文档](https://docs.langchain.com/langgraph/)
- [MCP 协议](https://modelcontextprotocol.io/)

---

> 文档版本: 1.0
> 创建日期: 2026-07-05
> 下次更新建议: 2026-08-05（月度更新）
