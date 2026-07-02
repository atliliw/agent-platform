# Superpowers vs Agent-Skills 对比分析

> 两个最火的 Agent 技能框架深度对比，帮你选适合的

---

## 基本信息

| | Superpowers | Agent-Skills |
|--|-------------|-------------|
| **GitHub** | [obra/superpowers](https://github.com/obra/superpowers) | [addyosmani/agent-skills](https://github.com/addyosmani/agent-skills) |
| **Stars** | 237K | 67K |
| **作者** | Jesse Vincent（Prime Radiant） | Addy Osmani（Google Chrome 团队） |
| **语言** | Shell | Shell |
| **创建时间** | 2025-10-09 | 2026-02-15 |
| **技能数** | 14 个 | 24 个 |
| **命令数** | 无（自动触发） | 8 个斜杠命令 |
| **兼容 Agent** | Claude Code / Codex / Cursor / Gemini CLI / Copilot / Kimi / Pi / OpenCode | Claude Code / Cursor / Antigravity / Gemini CLI |
| **License** | MIT | MIT |

---

## 核心理念对比

| | Superpowers | Agent-Skills |
|--|-------------|-------------|
| **一句话理念** | 强制 Agent 按工程师方式工作 | 给 Agent 一套完整开发流程 |
| **哲学** | "不写测试就删代码" | "先规格后代码，逐步实现" |
| **执行风格** | **严格强制**，不可跳过 | **引导推荐**，灵活执行 |
| **对 Agent 的态度** | Agent 会偷懒和找借口，必须堵住 | Agent 需要方向和方法，给它路走 |

---

## 技能覆盖对比

### Superpowers 的 14 个技能

```
流程技能（核心）：
├── brainstorming              # 头脑风暴 → 设计文档
├── writing-plans              # 写详细实施计划
├── subagent-driven-development # 子Agent驱动开发
├── executing-plans            # 批量执行计划
├── finishing-a-development-branch # 完成分支
└── dispatching-parallel-agents # 并行派遣Agent

质量技能（强制）：
├── test-driven-development    # TDD 强制循环
├── systematic-debugging       # 4阶段系统化调试
├── verification-before-completion # 完成前必须验证
├── requesting-code-review     # 请求代码审查
└── receiving-code-review      # 接收审查反馈

辅助技能：
├── using-git-worktrees        # Git工作树隔离
├── writing-skills             # 创建新技能（元技能）
└── using-superpowers          # 入门指南
```

### Agent-Skills 的 24 个技能

```
设计阶段：
├── spec-driven-development    # 规格驱动开发
├── idea-refine                # 创意细化
└── api-and-interface-design   # API与接口设计

规划阶段：
├── planning-and-task-breakdown # 规划与任务拆分
└── interview-me               # 需求访谈

实现阶段：
├── incremental-implementation # 增量实现
├── source-driven-development  # 源码驱动开发
└── frontend-ui-engineering    # 前端UI工程

测试阶段：
├── test-driven-development    # TDD
├── browser-testing-with-devtools # 浏览器测试
└── doubt-driven-development   # 怀疑驱动开发

质量阶段：
├── code-review-and-quality    # 代码审查与质量
├── code-simplification        # 代码简化
└── debugging-and-error-recovery # 调试与错误恢复

运维阶段：
├── ci-cd-and-automation       # CI/CD与自动化
├── observability-and-instrumentation # 可观测性
└── deprecation-and-migration  # 废弃与迁移

专项技能：
├── performance-optimization   # 性能优化
├── security-and-hardening     # 安全加固
├── documentation-and-adrs     # 文档与ADR
├── git-workflow-and-versioning # Git工作流
├── shipping-and-launch        # 发布上线
├── context-engineering        # 上下文工程
└── using-agent-skills         # 入门指南
```

---

## 覆盖面对比

| 领域 | Superpowers | Agent-Skills |
|------|:-----------:|:------------:|
| 需求分析/头脑风暴 | ✅ brainstorming | ✅ idea-refine、interview-me |
| 规格设计 | ✅ brainstorming 输出设计文档 | ✅ spec-driven-development |
| API/接口设计 | ❌ | ✅ api-and-interface-design |
| 任务规划 | ✅ writing-plans | ✅ planning-and-task-breakdown |
| TDD | ✅ 强制执行 | ✅ 推荐执行 |
| 代码实现 | ✅ 子Agent执行 | ✅ incremental-implementation |
| 前端专项 | ❌ | ✅ frontend-ui-engineering |
| 调试 | ✅ 4阶段系统化 | ✅ debugging-and-error-recovery |
| 代码审查 | ✅ 强制审查循环 | ✅ code-review-and-quality |
| 代码简化 | ❌ | ✅ code-simplification |
| 完成验证 | ✅ 强制验证（铁律） | ❌ |
| Git工作流 | ✅ worktree 隔离 | ✅ git-workflow-and-versioning |
| 多Agent协作 | ✅ 子Agent + 并行派遣 | ❌ |
| 性能优化 | ❌ | ✅ performance-optimization |
| 安全加固 | ❌ | ✅ security-and-hardening |
| CI/CD | ❌ | ✅ ci-cd-and-automation |
| 可观测性 | ❌ | ✅ observability |
| 文档/ADR | ❌ | ✅ documentation-and-adrs |
| 发布上线 | ❌ | ✅ shipping-and-launch |
| 浏览器测试 | ❌ | ✅ browser-testing-with-devtools |
| 上下文工程 | ❌ | ✅ context-engineering |
| 怀疑驱动开发 | ❌ | ✅ doubt-driven-development |
| 废弃与迁移 | ❌ | ✅ deprecation-and-migration |
| 创建自定义技能 | ✅ writing-skills（TDD方式） | ❌ |
| 进度持久化 | ✅ ledger 文件 | ❌ |

---

## 工作流对比

### Superpowers 工作流

```
你说：做个登录功能

Agent：[自动触发 brainstorming]
       先问你需求（一次一个问题）
       → 提出方案让你选
       → 写设计文档给你审查
       → 确认后触发 writing-plans
       → 写详细计划（每步2-5分钟）

       [自动触发 using-git-worktrees]
       → 创建隔离工作区

       [自动触发 subagent-driven-development]
       → 每个任务派子Agent
       → 子Agent TDD循环
       → 审查子Agent的代码
       → 审查不过就修，修完再审查
       → 所有任务完成

       [自动触发 verification-before-completion]
       → 运行测试验证
       → 确认真的完成了

       [自动触发 finishing-a-development-branch]
       → 合并/PR
```

**特点**：全程自动触发，强制执行，不给你跳过的机会。

---

### Agent-Skills 工作流

```
你：/spec 做个登录功能

Agent：[触发 spec-driven-development]
       写规格文档
       → 你确认

你：/plan

Agent：[触发 planning-and-task-breakdown]
       拆任务
       → 你确认

你：/build auto

Agent：[触发 incremental-implementation]
       自动按计划实现每个任务
       → 每个任务 TDD
       → 遇到问题暂停
       → 全部完成

你：/test

Agent：[触发 test-driven-development]
       补充测试

你：/review

Agent：[触发 code-review-and-quality]
       审查代码

你：/ship

Agent：[触发 shipping-and-launch]
       发布上线
```

**特点**：手动触发每个阶段，灵活可控，支持 `/build auto` 一键跑完。

---

## 执行力度对比

| 方面 | Superpowers | Agent-Skills |
|------|-------------|-------------|
| **TDD** | 写代码前没测试 → 删掉代码重来 | 推荐先写测试，但不强制 |
| **调试** | 4阶段强制流程，不猜 | 引导系统化调试，可跳步 |
| **完成验证** | 声称完成前必须运行验证命令 | 无强制验证步骤 |
| **代码审查** | 每个任务必须审查，审查不过必须修 | 推荐审查，可跳过 |
| **需求澄清** | 一次只问一个问题，必须走完 | 可以跳过，直接开始 |
| **工作区隔离** | 强制 git worktree | 无强制隔离 |
| **跳过流程** | 基本不可能，HARD-GATE 阻止 | 可以跳过任何步骤 |

---

## 适合场景对比

| 你的场景 | 推荐 | 原因 |
|----------|------|------|
| 团队开发，需要纪律 | Superpowers | 强制流程，不会偷懒 |
| 个人项目，想快速出活 | Agent-Skills | 灵活，`/build auto` 一键跑 |
| 代码质量优先 | Superpowers | TDD、审查、验证铁律 |
| 开发效率优先 | Agent-Skills | 覆盖面广，从设计到上线 |
| 多Agent协作 | Superpowers | 有子Agent + 并行派遣 |
| 需要性能/安全专项 | Agent-Skills | 有对应技能 |
| 学习 Agent 最佳实践 | 两个都装 | 互补 |
| 新手入门 | Agent-Skills | 命令清晰，`/spec → /plan → /build` |
| 严格项目 | Superpowers | 不可跳过流程 |

---

## 可以同时安装吗？

**可以。** 两者不冲突：

```bash
# 安装 Superpowers
/plugin install superpowers@claude-plugins-official

# 安装 Agent-Skills
/plugin marketplace add addyosmani/agent-skills
/plugin install agent-skills@addy-agent-skills
```

同时安装后：
- Superpowers 负责**流程强制**（TDD、验证、审查）
- Agent-Skills 负责**开发流程**（设计、规划、上线）
- 互补，不冲突

---

## 总结

| | Superpowers | Agent-Skills |
|--|-------------|-------------|
| **类比** | 严厉的项目经理 | 经验丰富的高级工程师 |
| **风格** | "你必须这样做" | "我建议这样做" |
| **强项** | 工程纪律、质量保证 | 开发全流程、覆盖面广 |
| **弱项** | 覆盖面窄（没有安全/性能/CI/CD） | 执行力度弱（可以跳步） |
| **最佳搭配** | 两个一起装 | 两个一起装 |

---

> 最后更新: 2026-06-25
> 数据来源: GitHub API 实时查询
