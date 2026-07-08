# 已实现功能清单

> 本文档记录 agent-platform 中**已完整实现且端到端可用**的功能。

---

## 一、核心对话与 Agent 执行

### 1.1 多 Agent 对话（chat-service → agent-service）

| 子功能 | 说明 |
|--------|------|
| 多 Agent Handoff | 5 个默认 Agent（main/researcher/coder/analyst/browser），通过 `transfer_to_<agent_id>` 工具实现 Agent 转交 |
| 单 Agent ReAct 循环 | Thought→Action→Observation 循环，最多 5 步，自动累积 AgentState |
| MCP 工具调用 | 14+ 工具（web_search, knowledge_search, code_execute, browser_*, weather, calculator 等） |
| Agent 注册/持久化 | MongoDB 存储 Agent 定义，支持 RegisterOrUpdateWithPersistence |
| LLM 调用 | 支持 OpenAI 和 DashScope（Qwen）两种 Provider，带 Metrics 包装（成本/延迟/Token 统计） |

**数据流**：用户 → Chat API → chat-service → agent-service（gRPC） → engine.executeLoop → LLM + MCP 工具 → 结果持久化 → 返回

### 1.2 Session 管理

| 子功能 | 说明 |
|--------|------|
| 创建/列表/删除 Session | chat-service gRPC + gateway HTTP API |
| 消息计数 | `message_count` 字段，从 DB COUNT 查询，不在消息数组中计算 |
| 空 Session 清理 | `DELETE /api/v2/sessions/empty` 端点，前端有"隐藏空会话"开关 |
| Agent Trace 持久化 | `chatWithMultiAgent` 和 `chatWithSingleAgent` 都保存 AgentState + ToolCall 到 DB |
| 独立 Context | 所有 Agent 执行后的 gRPC 调用使用 `context.WithTimeout(context.Background(), ...)` 防止父 ctx 超时 |

### 1.3 Session Replay

| 子功能 | 说明 |
|--------|------|
| 消息分解 | `decomposeMessagesToSteps()` — agent_trace → think/tool_call/observation/decision 子步骤 |
| 执行图 | `buildExecutionGraph()` — 有分支支持（并行工具调用扇出） |
| 播放控制 | Play/Pause/Step Forward/Step Backward/Speed（0.5x-4x） |
| 步骤详情 | StepDetail drawer 按类型渲染（think=紫色, tool_call=蓝色, observation=绿色, decision=橙色） |
| SVG 执行图 | 拓扑布局，当前步骤高亮 |
| 导出 | JSON 导出端点 |

---

## 二、Harness 治理层

### 2.1 规则引擎与安全检查

| 子功能 | 说明 |
|--------|------|
| 规则 CRUD | POST/GET/DELETE `/api/v2/harness/rules` |
| Guardrail 检查 | `POST /api/v2/harness/guardrail/check` — chat-service 每次对话前调用 |
| 规则类型 | keyword_block / regex_block / topic_filter / length_limit / pii_filter |
| 拦截流程 | 用户消息 → checkRules → 如果被拦截则返回拒绝 |

### 2.2 A/B 测试

| 子功能 | 说明 |
|--------|------|
| 实验创建 | 定义 Variant（prompt/model/parameter 变化）和分配策略 |
| 分配逻辑 | 随机 / 按用户 / 按 Session |
| 结果追踪 | 记录每个 Variant 的响应质量/成本/满意度 |
| 前端展示 | Harness 页面 ABTestPanel |

### 2.3 SLO 监控

| 子功能 | 说明 |
|--------|------|
| SLO 定义 | 目标指标（延迟 < 2s / 成本 < ¥0.01 / 质量 > 80%） |
| 状态追踪 | 实时计算 SLO 达成率（burn rate） |
| 前端展示 | Harness 页面 SLOPanel |

### 2.4 成本分析

| 子功能 | 说明 |
|--------|------|
| 成本报表 | `GET /api/v2/harness/cost/report` — 按时间/模型/Agent 聚合 |
| 定价管理 | `POST /api/v2/harness/cost/pricing` — 自定义 Token 价格 |
| 成本推荐 | 建议切换更便宜的模型 |
| 使用记录 | `POST /api/v2/harness/cost/usage` — 记录每次使用 |

### 2.5 Prompt 管理

| 子功能 | 说明 |
|--------|------|
| 模板 CRUD | 创建/列表/获取/删除 Prompt 模板 |
| 版本管理 | 每次编辑创建新版本，支持版本列表 |
| 版本激活 | 选择某个版本作为活跃版本 |
| 模板对比 | 两个版本内容 diff |
| 模板渲染 | `POST /api/v2/harness/prompt/render` — 用变量替换模板 |
| 性能追踪 | 每个版本的响应质量/成本统计 |

### 2.6 自演化 Proposals

| 子功能 | 说明 |
|--------|------|
| Proposal 创建 | Agent 或系统自动提出优化建议（改 Prompt / 调参数 / 换模型） |
| 审批流程 | approve / reject |
| 执行 | 执行已批准的 Proposal |
| 自动分析 | `POST /api/v2/harness/proposals/analyze` — 分析历史数据生成建议 |

### 2.7 LLM 网关

| 子功能 | 说明 |
|--------|------|
| Provider 配置 | 多个 LLM Provider（OpenAI/DashScope）配置 CRUD |
| 路由规则 | 按条件路由到不同 Provider/模型 |
| 负载均衡 | round_robin / least_latency / cost_optimized |
| 统计 | 请求量/延迟/成功率/Token 用量 |
| 代理聊天 | `POST /api/v2/harness/gateway/chat` — 通过网关调用 LLM |

### 2.8 Feature Flags（后端完整，前端缺失）

| 子功能 | 说明 |
|--------|------|
| Flag CRUD | 创建/列表/切换 |
| 条件评估 | 按用户/环境/时间评估 Flag 状态 |

### 2.9 Chaos Engineering（后端完整，前端缺失）

| 子功能 | 说明 |
|--------|------|
| 实验创建 | 定义故障注入类型 |
| 启动/停止 | `start` / `stop` 实验 |

### 2.10 Rollback（后端完整，前端缺失）

| 子功能 | 说明 |
|--------|------|
| 配置快照 | 保存 Agent/Prompt/模型配置的历史版本 |
| 回滚执行 | 恢复到某个快照版本 |

### 2.11 Scheduler（后端完整，前端缺失）

| 子功能 | 说明 |
|--------|------|
| 定时任务 | 定期执行 Agent / 评估 / 数据收集 |
| 控制 | pause / resume / 手动触发 |

---

## 三、Playground 与评估

### 3.1 Playground

| 子功能 | 说明 |
|--------|------|
| 多模型执行 | 选择不同 Provider/模型执行同一 Prompt |
| 模型对比 | 同时执行两个模型，对比输出 |
| SSE 流式 | `POST /api/v2/harness/playground/stream` — SSE 逐 token 推送 |
| 历史/统计 | 执行历史和质量统计 |

### 3.2 RAG 评估

| 子功能 | 说明 |
|--------|------|
| 单文档评估 | Faithfulness / Relevancy / Hallucination 指标 |
| 批量评估 | 多文档批量跑评估 |
| 评估历史 | 列表和指标查看 |

---

## 四、记忆系统（基础版）

| 子功能 | 说明 |
|--------|------|
| 保存记忆 | `POST /api/v2/memory` — 自动生成 Embedding，存 Qdrant + SQLite |
| 向量召回 | `POST /api/v2/memory/recall` — 查询 Embedding + Qdrant top-K 搜索 |
| Session 记忆 | 按 session_id 查询 |
| 全部记忆 | 按 tenant_id（用户级别）查询 |
| 删除 | 按 session / 按 id / 清除全部 |

---

## 五、浏览器自动化

| 子功能 | 说明 |
|--------|------|
| chromedp 池 | `BrowserPool` 单例，5 个浏览器实例，30s 健康检查 |
| Cookie 持久化 | MongoDB 存储，按 user+tenant+domain 按键 |
| 反检测 | navigator.webdriver 覆盖 / 自定义 UA / AutomationControlled 禁用 |
| Obscura 支持 | 远程 CDP 连接（`OBSCURA_CDP_URL`） |
| 页面解析 | goquery 提取交互元素（a/button/input/select/textarea/contenteditable） |
| CSDN 专用 | CKEditor/CodeMirror/textarea 编辑器 / 发布按钮 |

---

## 六、前端页面（12 个）

| 页面 | 功能覆盖 |
|------|---------|
| Home | 概览 |
| Chat | 多 Agent 对话 + Session 管理 |
| Agents | Agent 列表 + MCP 工具 + A2A（mock） + 执行追踪 |
| Knowledge | 文档管理（前端完整，后端 stub） |
| Memory | 记忆 CRUD + 向量搜索 + 统计 |
| Harness | 6 Tab 治理中心（Dashboard/Rules/AB/SLO/Cost/Proposals） |
| Gateway | LLM Provider 配置 + 路由 + 统计 |
| Observability | 5 Tab（Traces/Cost/Eval/Memory/Agent Editor） — 多为 mock |
| Playground | 多模型执行 + SSE 流式 + 对比 |
| Prompt | Prompt 模板 + 版本 + 对比 |
| RAGMetrics | RAG 评估 |
| Session | Session 列表 + Replay |
| Settings | 系统配置 + Cookie 管理 |
