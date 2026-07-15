# 2025-2026 GitHub 最新热门 Agent 开源项目盘点

> 数据截至 2026年6月24日，全部来自 GitHub API 实时查询
> **仅收录 2025年10月后创建的项目**

---

## 目录

- [Top 30 新项目排行](#top-30-新项目排行)
- [分类详解](#分类详解)
  - [一、Agent 技能/上下文工程](#一-agent-技能上下文工程最热赛道)
  - [二、Agent 基础设施/平台](#二-agent-基础设施平台)
  - [三、Agent 记忆与上下文优化](#三-agent-记忆与上下文优化)
  - [四、Agent 编排/多 Agent 协作](#四-agent-编排多-agent-协作)
  - [五、浏览器/网页 Agent](#五-浏览器网页-agent)
  - [六、编程 Agent（新玩家）](#六-编程-agent新玩家)
  - [七、Agent + 垂直领域](#七-agent--垂直领域)
  - [八、Agent 连接/桥接](#八-agent-连接桥接)
- [2026 年 Agent 新趋势总结](#2026-年-agent-新趋势总结)
- [与 agent-platform 项目的关联建议](#与-agent-platform-项目的关联建议)
- [附录：完整项目列表](#附录完整项目列表)

---

## Top 30 新项目排行

| # | 项目 | Stars | 创建时间 | 语言 | 一句话定位 |
|---|------|---------|----------|------|-----------|
| 1 | **obra/superpowers** | 237K | 2025-10 | Shell | Agent 技能框架 + 软件开发方法论 |
| 2 | **affaan-m/ECC** | 221K | 2026-01 | JS | Agent Harness 性能优化系统（技能/记忆/安全） |
| 3 | **ultraworkers/claw-code** | 194K | 2026-03 | Rust | Agent 管理的博物馆展览（Rust Agent 演示） |
| 4 | **karpathy/autoresearch** | 88K | 2026-03 | Python | Karpathy 出品：AI Agent 自动做研究 |
| 5 | **VoltAgent/awesome-design-md** | 93K | 2026-03 | - | DESIGN.md 设计系统文件集合 |
| 6 | **addyosmani/agent-skills** | 66K | 2026-02 | Shell | 生产级 Agent 编程技能库 |
| 7 | **code-yeongyu/oh-my-openagent** | 63K | 2025-12 | TS | 面向 token 优化的编程 Agent Harness |
| 8 | **shanraisshan/claude-code-best-practice** | 59K | 2025-10 | HTML | 从 vibe coding 到 agentic engineering |
| 9 | **colbymchenry/codegraph** | 54K | 2026-01 | TS | 代码知识图谱，为 Agent 提供代码索引 |
| 10 | **DietrichGebert/ponytail** | 53K | 2026-06 | JS | 让 Agent 像最懒的高级工程师一样思考 |
| 11 | **headroomlabs-ai/headroom** | 49K | 2026-01 | Python | 压缩 Agent 工具输出，节省 60%+ token |
| 12 | **Leonxlnx/taste-skill** | 50K | 2026-02 | JS | 给 Agent "好品味"，避免生成垃圾代码 |
| 13 | **HKUDS/nanobot** | 45K | 2026-02 | Python | 轻量开源 AI Agent（工具/聊天/工作流） |
| 14 | **sickn33/antigravity-awesome-skills** | 42K | 2026-01 | Python | 1600+ Agent 技能库 |
| 15 | **Yeachan-Heo/oh-my-claudecode** | 37K | 2026-01 | TS | Claude Code 多 Agent 团队编排 |
| 16 | **vercel-labs/agent-browser** | 37K | 2026-01 | Rust | AI Agent 浏览器自动化 CLI |
| 17 | **multica-ai/multica** | 38K | 2026-01 | Go | 开源 Agent 管理平台，Agent 变队友 |
| 18 | **Panniantong/Agent-Reach** | 39K | 2026-02 | Python | 给 Agent 联网能力（Twitter/YouTube/HN） |
| 19 | **heygen-com/hyperframes** | 31K | 2026-03 | TS | 写 HTML 渲染视频，为 Agent 设计 |
| 20 | **zeroclaw-labs/zeroclaw** | 32K | 2026-02 | Rust | 全自主 AI 个人助理基础设施 |
| 21 | **zai-org/Open-AutoGLM** | 26K | 2025-12 | Python | 开源手机 Agent 模型与框架 |
| 22 | **openai/symphony** | 26K | 2026-02 | Elixir | OpenAI 出品：项目工作变自主实现运行 |
| 23 | **volcengine/OpenViking** | 26K | 2026-01 | Python | 字节出品：专为 Agent 设计的上下文数据库 |
| 24 | **jackwener/OpenCLI** | 25K | 2026-03 | JS | 把任何网站变 CLI，Agent 可用浏览器 |
| 25 | **gastownhall/beads** | 25K | 2025-10 | Go | Agent 记忆升级插件 |
| 26 | **esengine/DeepSeek-Reasonix** | 24K | 2026-04 | Go | DeepSeek 原生终端编程 Agent |
| 27 | **OthmanAdi/planning-with-files** | 24K | 2026-01 | Python | Agent 基于文件的持久化规划 |
| 28 | **rohitg00/agentmemory** | 24K | 2026-02 | TS | Agent 持久化记忆（实测基准） |
| 29 | **coreyhaines31/marketingskills** | 35K | 2026-01 | JS | Agent 营销技能（CRO/SEO/文案） |
| 30 | **googleworkspace/cli** | 27K | 2026-03 | Rust | Google Workspace CLI（Agent 可调用） |

---

## 分类详解

### 一、Agent 技能/上下文工程（最热赛道 🔥🔥🔥）

这是 2026 年最火的方向——**不造新 Agent，而是给现有 Agent 加技能**。

#### 1. obra/superpowers ⭐ 237K
- **GitHub**: https://github.com/obra/superpowers
- **创建时间**: 2025-10-09
- **语言**: Shell
- **描述**: An agentic skills framework & software development methodology that works.
- **核心特点**:
  - 定义了一套 Agent 技能框架 + 软件开发方法论
  - Shell 脚本形式，可安装到 Claude Code/Codex/Cursor 等
  - 包含代码审查、测试、部署等多种技能
- **适合**: 想给现有编程 Agent 添加专业技能的开发者

#### 2. affaan-m/ECC ⭐ 221K
- **GitHub**: https://github.com/affaan-m/ECC
- **创建时间**: 2026-01-18
- **语言**: JavaScript
- **描述**: The agent harness performance optimization system. Skills, instincts, memory, security.
- **核心特点**:
  - Agent Harness 性能优化系统
  - 四大模块：技能(Skills)、本能(Instincts)、记忆(Memory)、安全(Security)
  - 综合性的 Agent 能力增强框架
- **适合**: Agent 系统性能优化和能力增强

#### 3. addyosmani/agent-skills ⭐ 66K
- **GitHub**: https://github.com/addyosmani/agent-skills
- **创建时间**: 2026-02-15
- **语言**: Shell
- **作者**: Addy Osmani (Google Chrome 团队)
- **描述**: Production-grade engineering skills for AI coding agents.
- **核心特点**:
  - 生产级编程技能库
  - Google 工程师出品，质量可靠
  - 包含代码质量、性能优化、架构设计等技能
- **适合**: 企业级 Agent 技能需求

#### 4. Leonxlnx/taste-skill ⭐ 50K
- **GitHub**: https://github.com/Leonxlnx/taste-skill
- **创建时间**: 2026-02-19
- **语言**: JavaScript
- **描述**: Taste-Skill - gives your AI good taste. stops the AI from generating bad code.
- **核心特点**:
  - "品味技能"——让 Agent 不写垃圾代码
  - 代码质量评估和改进
  - 防止生成低质量代码
- **适合**: 提升 Agent 代码生成质量

#### 5. sickn33/antigravity-awesome-skills ⭐ 42K
- **GitHub**: https://github.com/sickn33/antigravity-awesome-skills
- **创建时间**: 2026-01-14
- **语言**: Python
- **描述**: Installable GitHub library of 1,600+ agentic skills for Claude Code, Codex, Cursor, Copilot.
- **核心特点**:
  - 1600+ 可安装技能库
  - 支持多种编程 Agent
  - GitHub 库形式，易于安装和使用
- **适合**: 需要大量技能的开发者

#### 6. coreyhaines31/marketingskills ⭐ 35K
- **GitHub**: https://github.com/coreyhaines31/marketingskills
- **创建时间**: 2026-01-15
- **语言**: JavaScript
- **描述**: Marketing skills for Claude Code and AI agents. CRO, copywriting, SEO, analytics.
- **核心特点**:
  - 营销领域 Agent 技能
  - 包含 CRO(转化率优化)、SEO、文案、分析等
  - 垂直领域技能典范
- **适合**: 营销自动化 Agent

#### 7. K-Dense-AI/scientific-agent-skills ⭐ 29K
- **GitHub**: https://github.com/K-Dense-AI/scientific-agent-skills
- **创建时间**: 2025-10-19
- **语言**: Python
- **描述**: Turn any AI agent into an AI Scientist. The #1 Agent Skills library for research.
- **核心特点**:
  - 把 Agent 变成 AI 科学家
  - 研究领域的技能库
  - 文献检索、实验设计、数据分析等
- **适合**: 科研自动化 Agent

#### 8. VoltAgent/awesome-agent-skills ⭐ 26K
- **GitHub**: https://github.com/VoltAgent/awesome-agent-skills
- **创建时间**: 2025-10-28
- **语言**: None (Awesome List)
- **描述**: A curated collection of 1000+ agent skills from official dev teams and top contributors.
- **核心特点**:
  - 1000+ Agent 技能精选列表
  - 官方开发团队和顶级贡献者贡献
  - Awesome List 形式，易于浏览
- **适合**: 寻找高质量技能资源

> **💡 趋势解读**: Agent Skills 是 2026 年最大的新趋势。核心逻辑是——Agent 框架已经够多了，真正缺的是**高质量的技能/指令**。类似 npm 之于 Node.js，技能市场正在形成。

---

### 二、Agent 基础设施/平台

#### 1. multica-ai/multica ⭐ 38K
- **GitHub**: https://github.com/multica-ai/multica
- **创建时间**: 2026-01-13
- **语言**: Go
- **描述**: The open-source managed agents platform. Turn coding agents into real teammates.
- **核心特点**:
  - 开源 Agent 管理平台
  - 把编程 Agent 变成真正的队友
  - 支持多 Agent 协作
  - **Go 语言编写，值得深入研究**
- **适合**: 构建企业级 Agent 管理平台

#### 2. zeroclaw-labs/zeroclaw ⭐ 32K
- **GitHub**: https://github.com/zeroclaw-labs/zeroclaw
- **创建时间**: 2026-02-13
- **语言**: Rust
- **描述**: Fast, small, and fully autonomous AI personal assistant infrastructure, any OS, offline-first.
- **核心特点**:
  - 全自主 AI 个人助理基础设施
  - 跨 OS 支持
  - 离线优先设计
  - Rust 编写，高性能
- **适合**: 高性能、离线 Agent 系统

#### 3. RightNow-AI/openfang ⭐ 18K
- **GitHub**: https://github.com/RightNow-AI/openfang
- **创建时间**: 2026-02-24
- **语言**: Rust
- **描述**: Open-source Agent Operating System.
- **核心特点**:
  - 开源 Agent 操作系统
  - OS 级别的 Agent 支持
  - Rust 编写
- **适合**: Agent 作为操作系统核心组件

#### 4. HKUDS/nanobot ⭐ 45K
- **GitHub**: https://github.com/HKUDS/nanobot
- **创建时间**: 2026-02-01
- **语言**: Python
- **描述**: Lightweight, open-source AI agent for your tools, chats, and workflows.
- **核心特点**:
  - 轻量开源 AI Agent
  - 工具+聊天+工作流一体化
  - 香港大学数据科学实验室出品
- **适合**: 轻量级 Agent 需求

#### 5. EverMind-AI/EverOS ⭐ 9K
- **GitHub**: https://github.com/EverMind-AI/EverOS
- **创建时间**: 2025-10-28
- **语言**: Python
- **描述**: Self-evolving memory across Agent and platform. The one portable memory layer for AI.
- **核心特点**:
  - 自进化记忆层
  - 跨 Agent/平台可移植
  - 记忆是核心差异化能力
- **适合**: 需要持久化、可迁移记忆的 Agent

#### 6. builderz-labs/mission-control ⭐ 5K
- **GitHub**: https://github.com/builderz-labs/mission-control
- **创建时间**: 2026-02-13
- **语言**: TypeScript
- **描述**: Self-hosted AI agent orchestration platform: dispatch tasks, run agents, monitor progress.
- **核心特点**:
  - 自托管 Agent 编排平台
  - 任务分发、Agent 运行、进度监控
  - TypeScript 编写
- **适合**: 自托管 Agent 编排需求

---

### 三、Agent 记忆与上下文优化

#### 1. gastownhall/beads ⭐ 25K
- **GitHub**: https://github.com/gastownhall/beads
- **创建时间**: 2025-10-12
- **语言**: Go
- **描述**: Beads - A memory upgrade for your coding agent.
- **核心特点**:
  - Agent 记忆升级插件
  - **Go 语言编写**
  - 插件式设计，易于集成
- **适合**: Agent 记忆系统设计参考

#### 2. rohitg00/agentmemory ⭐ 24K
- **GitHub**: https://github.com/rohitg00/agentmemory
- **创建时间**: 2026-02-25
- **语言**: TypeScript
- **描述**: #1 Persistent memory for AI coding agents based on real-world benchmarks.
- **核心特点**:
  - Agent 持久化记忆
  - 基于真实基准测试
  - #1 排名说明质量较高
- **适合**: Agent 持久化记忆实现

#### 3. headroomlabs-ai/headroom ⭐ 49K
- **GitHub**: https://github.com/headroomlabs-ai/headroom
- **创建时间**: 2026-01-07
- **语言**: Python
- **描述**: Compress tool outputs, logs, files, and RAG chunks before they reach the LLM. 60%+ token savings.
- **核心特点**:
  - 压缩工具输出/日志/RAG
  - 节省 60%+ token
  - 解决 Agent 上下文瓶颈
- **适合**: 大规模 Agent 的 token 优化

#### 4. volcengine/OpenViking ⭐ 26K
- **GitHub**: https://github.com/volcengine/OpenViking
- **创建时间**: 2026-01-05
- **语言**: Python
- **作者**: 字节跳动
- **描述**: OpenViking is an open-source context database designed specifically for AI Agents.
- **核心特点**:
  - 专为 Agent 设计的上下文数据库
  - 字节跳动出品
  - 高效的上下文存储和检索
- **适合**: Agent 上下文存储系统

#### 5. colbymchenry/codegraph ⭐ 54K
- **GitHub**: https://github.com/colbymchenry/codegraph
- **创建时间**: 2026-01-18
- **语言**: TypeScript
- **描述**: Pre-indexed code knowledge graph, auto syncs on code changes, for Claude Code, Codex, Cursor.
- **核心特点**:
  - 代码知识图谱
  - 自动同步代码变更
  - 给 Agent 提供代码索引
  - 支持主流编程 Agent
- **适合**: 大型代码库的 Agent 辅助

#### 6. mksglu/context-mode ⭐ 18K
- **GitHub**: https://github.com/mksglu/context-mode
- **创建时间**: 2026-02-23
- **语言**: TypeScript
- **描述**: Context window optimization for AI coding agents. Sandboxes tool outputs.
- **核心特点**:
  - Agent 上下文窗口优化
  - 沙箱化工具输出
  - 防止上下文污染
- **适合**: Agent 上下文管理

#### 7. OthmanAdi/planning-with-files ⭐ 24K
- **GitHub**: https://github.com/OthmanAdi/planning-with-files
- **创建时间**: 2026-01-03
- **语言**: Python
- **描述**: Persistent file-based planning for AI coding agents and long-running agents.
- **核心特点**:
  - 基于文件的持久化规划
  - 长任务不丢上下文
  - 文件作为规划载体
- **适合**: 长时间运行的 Agent

---

### 四、Agent 编排/多 Agent 协作

#### 1. Yeachan-Heo/oh-my-claudecode ⭐ 37K
- **GitHub**: https://github.com/Yeachan-Heo/oh-my-claudecode
- **创建时间**: 2026-01-09
- **语言**: TypeScript
- **描述**: Teams-first Multi-agent orchestration for Claude Code.
- **核心特点**:
  - Claude Code 多 Agent 团队编排
  - 团队优先的设计理念
  - TypeScript 编写
- **适合**: Claude Code 多 Agent 协作

#### 2. openai/symphony ⭐ 26K
- **GitHub**: https://github.com/openai/symphony
- **创建时间**: 2026-02-26
- **语言**: Elixir
- **作者**: OpenAI
- **描述**: Symphony turns project work into isolated, autonomous implementation runs.
- **核心特点**:
  - OpenAI 出品
  - 项目工作→自主实现运行
  - Elixir 编写（高并发语言）
  - 隔离的自主运行
- **适合**: 大型项目的自主实现

#### 3. code-yeongyu/oh-my-openagent ⭐ 63K
- **GitHub**: https://github.com/code-yeongyu/oh-my-openagent
- **创建时间**: 2025-12-03
- **语言**: TypeScript
- **描述**: The coding agent for tokenmaxxers; the one and only agent harness for complex codebases.
- **核心特点**:
  - Agent Harness
  - token 优化导向
  - 复杂代码库支持
- **适合**: 大型复杂项目的 Agent

#### 4. snarktank/ralph ⭐ 21K
- **GitHub**: https://github.com/snarktank/ralph
- **创建时间**: 2026-01-07
- **语言**: TypeScript
- **描述**: Ralph is an autonomous AI agent loop that runs repeatedly until all PRD items are complete.
- **核心特点**:
  - 自主 Agent 循环
  - 反复运行直到 PRD 完成
  - 目标驱动的设计
- **适合**: 目标明确的任务自动化

#### 5. cft0808/edict ⭐ 16K
- **GitHub**: https://github.com/cft0808/edict
- **创建时间**: 2026-02-23
- **语言**: Python
- **描述**: OpenClaw Multi-Agent Orchestration System - 9 specialized agents.
- **核心特点**:
  - OpenClaw 多 Agent 编排系统
  - 9 个专业 Agent
  - Python 编写
- **适合**: 多专业 Agent 协作

#### 6. gastownhall/gastown ⭐ 16K
- **GitHub**: https://github.com/gastownhall/gastown
- **创建时间**: 2025-12-16
- **语言**: Go
- **描述**: Gas Town - multi-agent workspace manager.
- **核心特点**:
  - 多 Agent 工作区管理器
  - **Go 语言编写**
  - 工作区概念
- **适合**: Go 项目的多 Agent 管理

#### 7. AgentWrapper/agent-orchestrator ⭐ 8K
- **GitHub**: https://github.com/AgentWrapper/agent-orchestrator
- **创建时间**: 2026-02-13
- **语言**: TypeScript
- **描述**: Agentic orchestrator for parallel coding agents - plans tasks, splits work, merges results.
- **核心特点**:
  - 并行编程 Agent 编排器
  - 任务规划、工作拆分、结果合并
- **适合**: 并行 Agent 任务编排

#### 8. open-multi-agent/open-multi-agent ⭐ 6K
- **GitHub**: https://github.com/open-multi-agent/open-multi-agent
- **创建时间**: 2026-03-31
- **语言**: TypeScript
- **描述**: TypeScript multi-agent orchestration framework. Describe a goal, auto orchestrates.
- **核心特点**:
  - TS 多 Agent 编排框架
  - 描述目标，自动编排
  - 声明式设计
- **适合**: TypeScript 多 Agent 项目

---

### 五、浏览器/网页 Agent

#### 1. vercel-labs/agent-browser ⭐ 37K
- **GitHub**: https://github.com/vercel-labs/agent-browser
- **创建时间**: 2026-01-11
- **语言**: Rust
- **作者**: Vercel
- **描述**: Browser automation CLI for AI agents.
- **核心特点**:
  - Vercel 出品
  - Agent 浏览器自动化 CLI
  - Rust 编写，高性能
- **适合**: Agent 浏览器自动化

#### 2. jackwener/OpenCLI ⭐ 25K
- **GitHub**: https://github.com/jackwener/OpenCLI
- **创建时间**: 2026-03-14
- **语言**: JavaScript
- **描述**: Make Any Website into CLI & Use your logged-in browser by AI agent.
- **核心特点**:
  - 任何网站→CLI
  - Agent 可用已登录浏览器
  - JavaScript 编写
- **适合**: 网站 CLI 化和浏览器 Agent

#### 3. h4ckf0r0day/obscura ⭐ 16K
- **GitHub**: https://github.com/h4ckf0r0day/obscura
- **创建时间**: 2026-04-13
- **语言**: Rust
- **描述**: The headless browser for AI agents and web scraping.
- **核心特点**:
  - Agent 专用无头浏览器
  - Rust 编写
  - 网页抓取优化
- **适合**: 高性能网页 Agent

#### 4. Panniantong/Agent-Reach ⭐ 39K
- **GitHub**: https://github.com/Panniantong/Agent-Reach
- **创建时间**: 2026-02-24
- **语言**: Python
- **描述**: Give your AI agent eyes to see the entire internet. Read & search Twitter, YouTube, HN.
- **核心特点**:
  - Agent 联网能力
  - Twitter/YouTube/HN 搜索
  - Python 编写
- **适合**: Agent 信息获取能力

#### 5. microsoft/Webwright ⭐ 6K
- **GitHub**: https://github.com/microsoft/Webwright
- **创建时间**: 2026-04-08
- **语言**: Python
- **作者**: Microsoft
- **描述**: A simple SWE style browser agent framework that achieves SOTA results.
- **核心特点**:
  - 微软出品
  - SOTA 浏览器 Agent 框架
  - SWE 风格设计
- **适合**: 高性能浏览器 Agent

#### 6. jo-inc/camofox-browser ⭐ 7K
- **GitHub**: https://github.com/jo-inc/camofox-browser
- **创建时间**: 2026-01-26
- **语言**: JavaScript
- **描述**: Stealth headless browser for AI agents - bypass Cloudflare, bot detection.
- **核心特点**:
  - 隐身无头浏览器
  - 绕过 Cloudflare 和机器人检测
- **适合**: 需要绕过反爬的 Agent

---

### 六、编程 Agent（新玩家）

#### 1. esengine/DeepSeek-Reasonix ⭐ 24K
- **GitHub**: https://github.com/esengine/DeepSeek-Reasonix
- **创建时间**: 2026-04-21
- **语言**: Go
- **描述**: DeepSeek-native AI coding agent for your terminal. Engineered around prefix-cache optimization.
- **核心特点**:
  - DeepSeek 原生终端编程 Agent
  - 前缀缓存优化
  - **Go 语言编写**
- **适合**: DeepSeek 模型用户

#### 2. DietrichGebert/ponytail ⭐ 53K
- **GitHub**: https://github.com/DietrichGebert/ponytail
- **创建时间**: 2026-06-12
- **语言**: JavaScript
- **描述**: Makes your AI agent think like the laziest senior dev in the room. The best code quality.
- **核心特点**:
  - 让 Agent 像最懒高级工程师一样思考
  - 最佳代码质量导向
  - 2026年6月新项目，增长极快
- **适合**: 高质量代码生成

#### 3. can1357/oh-my-pi ⭐ 14K
- **GitHub**: https://github.com/can1357/oh-my-pi
- **创建时间**: 2025-12-31
- **语言**: TypeScript
- **描述**: AI Coding agent for the terminal - hash-anchored edits, optimized performance.
- **核心特点**:
  - 终端 AI 编程 Agent
  - hash-anchored 编辑
  - 性能优化
- **适合**: 高效终端编程 Agent

#### 4. karpathy/autoresearch ⭐ 88K
- **GitHub**: https://github.com/karpathy/autoresearch
- **创建时间**: 2026-03-06
- **语言**: Python
- **作者**: Andrej Karpathy (前 Tesla AI总监、OpenAI创始成员)
- **描述**: AI agents running research on single-GPU nanochat training automatically.
- **核心特点**:
  - Karpathy 出品，影响力巨大
  - AI Agent 自动做研究
  - 单 GPU 运行
  - 自动训练 nanochat
- **适合**: 自动化研究流程

---

### 七、Agent + 垂直领域

#### 1. zai-org/Open-AutoGLM ⭐ 26K
- **GitHub**: https://github.com/zai-org/Open-AutoGLM
- **创建时间**: 2025-12-08
- **语言**: Python
- **描述**: An Open Phone Agent Model & Framework. Unlocking the AI Phone for Everyone.
- **核心特点**:
  - 开源手机 Agent 模型与框架
  - AI 操作手机
  - 全新交互范式
- **适合**: 手机自动化 Agent

#### 2. heygen-com/hyperframes ⭐ 31K
- **GitHub**: https://github.com/heygen-com/hyperframes
- **创建时间**: 2026-03-10
- **语言**: TypeScript
- **作者**: HeyGen (AI视频公司)
- **描述**: Write HTML. Render video. Built for agents.
- **核心特点**:
  - 写 HTML 渲染视频
  - HeyGen 出品
  - 为 Agent 设计
- **适合**: Agent 生成视频内容

#### 3. HKUDS/AI-Trader ⭐ 20K
- **GitHub**: https://github.com/HKUDS/AI-Trader
- **创建时间**: 2025-10-23
- **语言**: Python
- **描述**: AI-Trader: 100% Fully-Automated Agent-Native Trading.
- **核心特点**:
  - 100% 全自动 Agent 交易
  - Agent-Native 设计
  - 香港大学出品
- **适合**: 量化交易 Agent

#### 4. HKUDS/Vibe-Trading ⭐ 13K
- **GitHub**: https://github.com/HKUDS/Vibe-Trading
- **创建时间**: 2026-04-01
- **语言**: Python
- **描述**: Vibe-Trading: Your Personal Trading Agent.
- **核心特点**:
  - 个人交易 Agent
  - Vibe 风格设计
- **适合**: 个人投资者

#### 5. agentscope-ai/QwenPaw ⭐ 20K
- **GitHub**: https://github.com/agentscope-ai/QwenPaw
- **创建时间**: 2026-02-24
- **语言**: Python
- **作者**: 阿里巴巴（通义千问）
- **描述**: Your Personal AI Assistant; easy to install, deploy on your own machine or on the cloud.
- **核心特点**:
  - 通义千问个人 AI 助手
  - 易安装、可本地/云端部署
  - 阿里出品
- **适合**: 个人 AI 助手

#### 6. googleworkspace/cli ⭐ 27K
- **GitHub**: https://github.com/googleworkspace/cli
- **创建时间**: 2026-03-02
- **语言**: Rust
- **作者**: Google
- **描述**: Google Workspace CLI - one command-line tool for Drive, Gmail, Calendar, Sheets, etc.
- **核心特点**:
  - Google Workspace CLI
  - Agent 可调用
  - Rust 编写
- **适合**: Agent 操作 Google Workspace

#### 7. github/copilot-sdk ⭐ 9K
- **GitHub**: https://github.com/github/copilot-sdk
- **创建时间**: 2026-01-14
- **语言**: Java
- **作者**: GitHub
- **描述**: Multi-platform SDK for integrating GitHub Copilot Agent into apps and services.
- **核心特点**:
  - GitHub Copilot Agent SDK
  - 多平台支持
  - 集成到应用和服务
- **适合**: 集成 Copilot Agent

---

### 八、Agent 连接/桥接

#### 1. chenhg5/cc-connect ⭐ 13K
- **GitHub**: https://github.com/chenhg5/cc-connect
- **创建时间**: 2026-02-28
- **语言**: Go
- **描述**: Bridge local AI coding agents (Claude Code, Cursor, Gemini CLI, Codex) to messaging platforms.
- **核心特点**:
  - 桥接本地 Agent 到消息平台
  - 支持 Claude Code/Cursor/Gemini CLI/Codex
  - **Go 语言编写**
- **适合**: Agent 消息平台集成

#### 2. EKKOLearnAI/hermes-studio ⭐ 8K
- **GitHub**: https://github.com/EKKOLearnAI/hermes-studio
- **创建时间**: 2026-04-11
- **语言**: TypeScript
- **描述**: Web dashboard for Hermes Agent - multi-platform AI chat, session management, logs.
- **核心特点**:
  - Hermes Agent Web 仪表盘
  - 多平台 AI 聊天
  - 会话管理和日志
- **适合**: Agent Web 管理界面

#### 3. superset-sh/superset ⭐ 12K
- **GitHub**: https://github.com/superset-sh/superset
- **创建时间**: 2025-10-21
- **语言**: TypeScript
- **描述**: Code Editor for the AI Agents Era - Run an army of Claude Code, Codex, Cursor agents.
- **核心特点**:
  - AI Agent 时代的代码编辑器
  - 同时运行多个 Agent
  - Agent Army 概念
- **适合**: 多 Agent 并行开发

---

## 2026 年 Agent 新趋势总结

| 趋势 | 代表项目 | 解读 |
|------|---------|------|
| **🔥 Agent Skills 技能市场** | superpowers(237K), agent-skills(66K) | **最大的新赛道**！不造框架，给现有 Agent 加技能。类似 npm 之于 Node.js |
| **🧠 上下文/记忆优化** | headroom(49K), codegraph(54K), beads(25K) | Agent 的瓶颈从"能不能做"变成"记不记得住" |
| **🌐 浏览器 Agent 2.0** | agent-browser(37K), OpenCLI(25K) | 从简单爬取→完整浏览器操控 |
| **📱 手机 Agent** | Open-AutoGLM(26K) | AI 操作手机，全新交互范式 |
| **🎬 Agent 生成视频** | hyperframes(31K) | HTML→视频，Agent 原生内容创作 |
| **🏢 Agent 操作系统** | openfang(18K), zeroclaw(32K) | Agent 不只是工具，是 OS 级基础设施 |
| **🔗 Agent 桥接** | cc-connect(13K) | 打通不同 Agent 和消息平台 |
| **💰 Agent 交易** | AI-Trader(20K) | 全自动量化交易 Agent |
| **🎯 Rust 重写一切** | agent-browser, zeroclaw, obscura | 高性能 Agent 基础设施转向 Rust |
| **🤝 多 Agent 编排** | oh-my-claudecode, symphony | 团队式 Agent 协作成为主流 |

---

## 与 agent-platform 项目的关联建议

你的项目是 **Go + gRPC 微服务架构**的 Agent 平台，以下新项目最值得参考：

### 架构参考（Go 项目）

| 项目 | Stars | 值得研究的原因 |
|------|-------|---------------|
| **multica-ai/multica** | 38K | 同为 Go 语言的 Agent 管理平台，架构设计可参考 |
| **gastownhall/beads** | 25K | Go 语言的 Agent 记忆系统，插件式设计 |
| **gastownhall/gastown** | 16K | Go 多 Agent 工作区管理器 |
| **chenhg5/cc-connect** | 13K | Go Agent 桥接消息平台 |
| **esengine/DeepSeek-Reasonix** | 24K | Go 终端编程 Agent |

### 记忆/上下文系统

| 项目 | Stars | 值得研究的原因 |
|------|-------|---------------|
| **volcengine/OpenViking** | 26K | 字节出品，专为 Agent 设计的上下文数据库 |
| **gastownhall/beads** | 25K | Agent 记忆升级插件 |
| **headroomlabs-ai/headroom** | 49K | Token 优化，解决上下文瓶颈 |
| **colbymchenry/codegraph** | 54K | 代码知识图谱，为 Agent 提供索引 |

### 多 Agent 编排

| 项目 | Stars | 值得研究的原因 |
|------|-------|---------------|
| **Yeachan-Heo/oh-my-claudecode** | 37K | Claude Code 多 Agent 团队编排 |
| **openai/symphony** | 26K | OpenAI 出品，隔离自主运行 |
| **cft0808/edict** | 16K | 9 个专业 Agent 的编排系统 |

### 技能系统

| 项目 | Stars | 值得研究的原因 |
|------|-------|---------------|
| **obra/superpowers** | 237K | Agent 技能框架设计思路 |
| **addyosmani/agent-skills** | 66K | 生产级技能库结构 |

### 浏览器 Agent

| 项目 | Stars | 值得研究的原因 |
|------|-------|---------------|
| **vercel-labs/agent-browser** | 37K | Rust 高性能浏览器 Agent CLI |
| **Panniantong/Agent-Reach** | 39K | Agent 联网能力实现 |

---

## 附录：完整项目列表

### 按创建时间排序

| 项目 | Stars | 创建时间 | 语言 | 描述 |
|------|-------|----------|------|------|
| obra/superpowers | 237K | 2025-10-09 | Shell | Agent 技能框架 + 软件开发方法论 |
| gastownhall/beads | 25K | 2025-10-12 | Go | Agent 记忆升级插件 |
| HKUDS/AI-Trader | 20K | 2025-10-23 | Python | 100% 全自动 Agent 交易 |
| VoltAgent/awesome-agent-skills | 26K | 2025-10-28 | - | 1000+ Agent 技能精选列表 |
| EverMind-AI/EverOS | 9K | 2025-10-28 | Python | 自进化记忆层 |
| shanraisshan/claude-code-best-practice | 59K | 2025-10-31 | HTML | 从 vibe coding 到 agentic engineering |
| K-Dense-AI/scientific-agent-skills | 29K | 2025-10-19 | Python | 把 Agent 变成 AI 科学家 |
| superset-sh/superset | 12K | 2025-10-21 | TypeScript | AI Agent 时代代码编辑器 |
| code-yeongyu/oh-my-openagent | 63K | 2025-12-03 | TypeScript | Agent Harness for tokenmaxxers |
| zai-org/Open-AutoGLM | 26K | 2025-12-08 | Python | 开源手机 Agent |
| gastownhall/gastown | 16K | 2025-12-16 | Go | 多 Agent 工作区管理器 |
| can1357/oh-my-pi | 14K | 2025-12-31 | TypeScript | 终端 AI 编程 Agent |
| volcengine/OpenViking | 26K | 2026-01-05 | Python | Agent 上下文数据库 |
| OthmanAdi/planning-with-files | 24K | 2026-01-03 | Python | 基于文件的持久化规划 |
| headroomlabs-ai/headroom | 49K | 2026-01-07 | Python | 压缩工具输出，节省 60%+ token |
| snarktank/ralph | 21K | 2026-01-07 | TypeScript | 自主 Agent 循环 |
| Yeachan-Heo/oh-my-claudecode | 37K | 2026-01-09 | TypeScript | Claude Code 多 Agent 团队编排 |
| vercel-labs/agent-browser | 37K | 2026-01-11 | Rust | Agent 浏览器自动化 CLI |
| multica-ai/multica | 38K | 2026-01-13 | Go | 开源 Agent 管理平台 |
| sickn33/antigravity-awesome-skills | 42K | 2026-01-14 | Python | 1600+ Agent 技能库 |
| github/copilot-sdk | 9K | 2026-01-14 | Java | GitHub Copilot Agent SDK |
| coreyhaines31/marketingskills | 35K | 2026-01-15 | JavaScript | Agent 营销技能 |
| colbymchenry/codegraph | 54K | 2026-01-18 | TypeScript | 代码知识图谱 |
| affaan-m/ECC | 221K | 2026-01-18 | JavaScript | Agent Harness 性能优化系统 |
| Leonxlnx/taste-skill | 50K | 2026-02-19 | JavaScript | 给 Agent "好品味" |
| HKUDS/nanobot | 45K | 2026-02-01 | Python | 轻量开源 AI Agent |
| addyosmani/agent-skills | 66K | 2026-02-15 | Shell | 生产级 Agent 编程技能库 |
| zeroclaw-labs/zeroclaw | 32K | 2026-02-13 | Rust | 全自主 AI 助理基础设施 |
| builderz-labs/mission-control | 5K | 2026-02-13 | TypeScript | 自托管 Agent 编排平台 |
| AgentWrapper/agent-orchestrator | 8K | 2026-02-13 | TypeScript | 并行 Agent 编排器 |
| golutra/golutra | 4K | 2026-02-15 | Rust | 多 Agent 编排平台 |
| SawyerHood/dev-browser | 6K | 2025-12-02 | TypeScript | Agent 浏览器技能 |
| jo-inc/camofox-browser | 7K | 2026-01-26 | JavaScript | 隐身无头浏览器 |
| mksglu/context-mode | 18K | 2026-02-23 | TypeScript | 上下文窗口优化 |
| cft0808/edict | 16K | 2026-02-23 | Python | OpenClaw 多 Agent 编排系统 |
| RightNow-AI/openfang | 18K | 2026-02-24 | Rust | 开源 Agent 操作系统 |
| agentscope-ai/QwenPaw | 20K | 2026-02-24 | Python | 通义千问个人 AI 助手 |
| Panniantong/Agent-Reach | 39K | 2026-02-24 | Python | Agent 联网能力 |
| rohitg00/agentmemory | 24K | 2026-02-25 | TypeScript | Agent 持久化记忆 |
| mukul975/Anthropic-Cybersecurity-Skills | 20K | 2026-02-25 | Python | 817 网络安全技能 |
| openai/symphony | 26K | 2026-02-26 | Elixir | OpenAI 自主实现运行 |
| chenhg5/cc-connect | 13K | 2026-02-28 | Go | Agent 桥接消息平台 |
| waooAI/waoowaoo | 13K | 2026-01-22 | TypeScript | 专业 AI Agent 平台 |
| brokermr810/QuantDinger | 9K | 2025-12-28 | Python | AI 量化交易平台 |
| datawhalechina/easy-vibe | 17K | 2025-12-28 | JavaScript | Vibe coding 课程 |
| googleworkspace/cli | 27K | 2026-03-02 | Rust | Google Workspace CLI |
| greensock/gsap-skills | 10K | 2026-03-04 | - | GSAP 官方 Agent 技能 |
| karpathy/autoresearch | 88K | 2026-03-06 | Python | Karpathy AI Agent 自动研究 |
| jnMetaCode/agency-agents-zh | 15K | 2026-03-06 | Shell | 266 个 AI Agent 中文版 |
| heygen-com/hyperframes | 31K | 2026-03-10 | TypeScript | HTML 渲染视频，为 Agent 设计 |
| THU-MAIC/OpenMAIC | 19K | 2026-03-11 | TypeScript | 多 Agent 交互课堂 |
| open-multi-agent/open-multi-agent | 6K | 2026-03-31 | TypeScript | TS 多 Agent 编排框架 |
| ultraworkers/claw-code | 194K | 2026-03-31 | Rust | Agent 管理博物馆展览 |
| VoltAgent/awesome-design-md | 93K | 2026-03-31 | - | DESIGN.md 设计系统文件集合 |
| jackwener/OpenCLI | 25K | 2026-03-14 | JavaScript | 网站→CLI，Agent 浏览器 |
| HKUDS/Vibe-Trading | 13K | 2026-04-01 | Python | 个人交易 Agent |
| h4ckf0r0day/obscura | 16K | 2026-04-13 | Rust | Agent 专用无头浏览器 |
| esengine/DeepSeek-Reasonix | 24K | 2026-04-21 | Go | DeepSeek 原生编程 Agent |
| microsoft/Webwright | 6K | 2026-04-08 | Python | 微软 SOTA 浏览器 Agent |
| EKKOLearnAI/hermes-studio | 8K | 2026-04-11 | TypeScript | Hermes Agent Web 仪表盘 |
| DietrichGebert/ponytail | 53K | 2026-06-12 | JavaScript | Agent 像最懒高级工程师思考 |

---

## 快速筛选建议

### 如果你关注...

| 你的兴趣 | 推荐首先看这些 |
|----------|---------------|
| **Agent 技能框架** | obra/superpowers, addyosmani/agent-skills |
| **Go 语言实现** | multica, beads, gastown, cc-connect, DeepSeek-Reasonix |
| **记忆/上下文** | beads, OpenViking, headroom, codegraph |
| **多 Agent 编排** | oh-my-claudecode, symphony, edict |
| **浏览器 Agent** | agent-browser, OpenCLI, Agent-Reach |
| **Rust 高性能** | agent-browser, zeroclaw, obscura, openfang |
| **垂直领域** | Open-AutoGLM(手机), hyperframes(视频), AI-Trader(交易) |
| **大厂出品** | symphony(OpenAI), OpenViking(字节), QwenPaw(阿里), Webwright(微软) |
| **明星开发者** | autoresearch(Karpathy), agent-skills(Addy Osmani) |

---

> 最后更新: 2026-06-24
> 数据来源: GitHub API 实时查询