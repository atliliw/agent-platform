# Agent Platform 文档中心

> 欢迎查阅 Agent Platform 项目文档

---

## 📁 文档目录结构

```
docs/
├── architecture/     # 系统架构文档
├── api/              # API 设计文档
├── implementation/   # 实现细节文档
├── deployment/       # 部署运维文档
├── research/         # 技术研究文档
└── guides/           # 使用指南
```

---

## 📖 快速导航

### 1. 系统架构 (Architecture)

| 文档 | 说明 |
|------|------|
| [01-overview.md](architecture/01-overview.md) | 项目概述 |
| [02-architecture.md](architecture/02-architecture.md) | 系统架构设计 |
| [03-services.md](architecture/03-services.md) | 微服务设计 |
| [04-data-flow.md](architecture/04-data-flow.md) | 数据流设计 |
| [06-infrastructure.md](architecture/06-infrastructure.md) | 基础设施 |
| [07-timeline.md](architecture/07-timeline.md) | 开发时间线 |
| [AGENT_ARCHITECTURE.md](architecture/AGENT_ARCHITECTURE.md) | Agent 架构设计 |
| [multi-agent-architecture-plan.md](architecture/multi-agent-architecture-plan.md) | 多 Agent 架构规划 |
| [PROJECT_SUMMARY.md](architecture/PROJECT_SUMMARY.md) | 项目总结 |
| [CONFIG_SUMMARY.md](architecture/CONFIG_SUMMARY.md) | 配置总结 |

### 2. API 设计 (API)

| 文档 | 说明 |
|------|------|
| [05-api-design.md](api/05-api-design.md) | API 设计规范 |

### 3. 实现细节 (Implementation)

| 文档 | 说明 |
|------|------|
| [08-migration.md](implementation/08-migration.md) | 迁移指南 |
| [09-frontend.md](implementation/09-frontend.md) | 前端实现 |
| [MULTI_AGENT_IMPLEMENTATION.md](implementation/MULTI_AGENT_IMPLEMENTATION.md) | 多 Agent 实现 |
| [CHAT_MEMORY_INTEGRATION.md](implementation/CHAT_MEMORY_INTEGRATION.md) | 聊天记忆集成 |
| [IMPLEMENTATION_CHECKLIST.md](implementation/IMPLEMENTATION_CHECKLIST.md) | 实现检查清单 |

### 4. 部署运维 (Deployment)

| 文档 | 说明 |
|------|------|
| [DEPLOYMENT.md](deployment/DEPLOYMENT.md) | 部署指南 |

### 5. 技术研究 (Research)

| 文档 | 说明 |
|------|------|
| [agent-loop-guide.md](research/agent-loop-guide.md) | Agent Loop (ReAct) 入门指南 |
| [agent-loop-deep-research.md](research/agent-loop-deep-research.md) | Agent Loop (ReAct) 深度研究 |
| [agent-frameworks-research.md](research/agent-frameworks-research.md) | Agent 框架研究 |

### 6. 使用指南 (Guides)

| 文档 | 说明 |
|------|------|
| [multi-agent-usage.md](guides/multi-agent-usage.md) | 多 Agent 使用指南 |
| [TASKLIST.md](guides/TASKLIST.md) | 任务清单 |

---

## 🚀 快速开始

### 新手入门

1. 先阅读 [项目概述](architecture/01-overview.md)
2. 了解 [系统架构](architecture/02-architecture.md)
3. 学习 [Loop Agent 入门](research/loop-agent-guide.md)
4. 参考 [多 Agent 使用指南](guides/multi-agent-usage.md)

### 开发者

1. 查看 [API 设计](api/05-api-design.md)
2. 了解 [微服务设计](architecture/03-services.md)
3. 参考 [多 Agent 实现](implementation/MULTI_AGENT_IMPLEMENTATION.md)
4. 阅读 [实现检查清单](implementation/IMPLEMENTATION_CHECKLIST.md)

### 运维人员

1. 阅读 [部署指南](deployment/DEPLOYMENT.md)

---

## 📊 文档统计

| 分类 | 文档数量 |
|------|----------|
| architecture | 10 |
| api | 1 |
| implementation | 5 |
| deployment | 1 |
| research | 3 |
| guides | 2 |
| **总计** | **22** |

---

## 📝 文档规范

### 命名规范

- 架构文档: `01-overview.md`, `02-architecture.md` (数字前缀排序)
- 专题文档: `DEPLOYMENT.md`, `AGENT_ARCHITECTURE.md` (大写下划线)
- 研究文档: `loop-agent-guide.md` (小写连字符)

### 文档格式

- 使用 Markdown 格式
- 包含目录结构
- 代码块指定语言
- 表格对齐

---

*最后更新: 2024年6月*