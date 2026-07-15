# Agent Platform Documentation

> Agent Platform 文档中心 · [English](#english) | [中文](#中文)

---

## English

Standard reference documentation for the project.

| Topic | File |
|-------|------|
| Architecture | [en/architecture.md](./en/architecture.md) |
| Configuration | [en/configuration.md](./en/configuration.md) |
| Deployment | [en/deployment.md](./en/deployment.md) |
| API Reference | [en/api-reference.md](./en/api-reference.md) |
| Development | [en/development.md](./en/development.md) |

**Start here:** [Architecture](./en/architecture.md) -> [Configuration](./en/configuration.md) -> [Deployment](./en/deployment.md).

Back to [project README](../README.md).

---

## 中文

项目标准参考文档（中文版）。

| 主题 | 文件 |
|------|------|
| 架构 | [zh-CN/architecture.md](./zh-CN/architecture.md) |
| 配置 | [zh-CN/configuration.md](./zh-CN/configuration.md) |
| 部署 | [zh-CN/deployment.md](./zh-CN/deployment.md) |
| API 参考 | [zh-CN/api-reference.md](./zh-CN/api-reference.md) |
| 开发 | [zh-CN/development.md](./zh-CN/development.md) |

**阅读顺序：** [架构](./zh-CN/architecture.md) -> [配置](./zh-CN/configuration.md) -> [部署](./zh-CN/deployment.md)。

返回 [项目 README](../README.zh-CN.md)。

---

## 内部 / 归档文档（中文）

以下为项目积累的专题与研究文档，已归档至 `internal/`，可作为深入参考（非对外标准文档）。

### 架构与设计
- [项目概述](./internal/architecture/01-overview.md)
- [系统架构设计](./internal/architecture/02-architecture.md)
- [微服务设计](./internal/architecture/03-services.md)
- [数据流设计](./internal/architecture/04-data-flow.md)
- [基础设施](./internal/architecture/06-infrastructure.md)
- [Agent 架构设计](./internal/architecture/AGENT_ARCHITECTURE.md)
- [多 Agent 架构规划](./internal/architecture/multi-agent-architecture-plan.md)
- [项目总结](./internal/architecture/PROJECT_SUMMARY.md)
- [配置总结](./internal/architecture/CONFIG_SUMMARY.md)
- [API 设计规范](./internal/api/05-api-design.md)

### 实现细节
- [迁移指南](./internal/implementation/08-migration.md)
- [前端实现](./internal/implementation/09-frontend.md)
- [多 Agent 实现](./internal/implementation/MULTI_AGENT_IMPLEMENTATION.md)
- [聊天记忆集成](./internal/implementation/CHAT_MEMORY_INTEGRATION.md)
- [实现检查清单](./internal/implementation/IMPLEMENTATION_CHECKLIST.md)

### 技术研究
- [Agent Loop (ReAct) 入门指南](./internal/research/agent-loop-guide.md)
- [Agent Loop 深度研究](./internal/research/agent-loop-deep-research.md)
- [Agent 框架研究](./internal/research/agent-frameworks-research.md)
- [真实 Agent 分析](./internal/research/real-agent-analysis.md)
- [Agent 深度架构](./internal/research/agent-deep-architecture.md)
- [Agent 平台对比](./internal/research/agent-platform-comparison.md)
- [上下文压缩专题](./internal/research/2026-07-07-上下文压缩专题日志.md)
- [技能系统专题](./internal/skill/2026-07-12-技能系统专题.md)

### 部署与运维
- [部署指南](./internal/deployment/DEPLOYMENT.md)

### 使用指南
- [多 Agent 使用指南](./internal/guides/multi-agent-usage.md)
- [任务清单](./internal/guides/TASKLIST.md)

### 规划与趋势
- [Agent 趋势与项目缺口](./internal/trends/2026-07-05-agent-trends-and-project-gap.md)
- [RAG 指标设计](./internal/rag-metrics-design.md)
- [实施计划](./internal/plans/)（各阶段计划文档）

---

## 文档目录结构

```
docs/
├── README.md              # 本索引
├── en/                    # 英文标准文档
│   ├── architecture.md
│   ├── configuration.md
│   ├── deployment.md
│   ├── api-reference.md
│   └── development.md
├── zh-CN/                 # 中文标准文档
│   ├── architecture.md
│   ├── configuration.md
│   ├── deployment.md
│   ├── api-reference.md
│   └── development.md
└── internal/              # 内部 / 归档文档（中文专题、研究、计划等）
    ├── architecture/
    ├── api/
    ├── implementation/
    ├── deployment/
    ├── research/
    ├── skill/
    ├── guides/
    ├── plans/
    ├── trends/
    └── rag-metrics-design.md
```

## 文档规范

- 对外标准文档仅 `en/` 与 `zh-CN/`，保持中英文内容对应、结构一致。
- 历史积累的专题/研究/计划文档统一归入 `internal/`，不在对外标准文档中引用。
- 命名：标准文档用小写连字符（`api-reference.md`）；专题文档沿用原有命名。
- 格式：Markdown，代码块标注语言，表格对齐，含交叉链接。
- 新增功能时同步更新 `en/` 与 `zh-CN/` 对应文档。
