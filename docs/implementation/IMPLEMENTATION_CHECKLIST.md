# Agent Platform 功能实现清单

> 状态说明：🔴 待实现 | 🟡 部分实现 | ✅ 已完成

---

## 一、MCP 服务 - 工具实现 (P0 最高优先级)

### 1.1 knowledge_search - 知识库搜索工具 ✅ 已完成
- [x] 实现调用 Knowledge Service 的 HTTP 接口
- [x] 支持向量搜索、BM25、混合搜索
- [x] 返回真实搜索结果
- **文件**: `services/mcp-service/internal/tools/knowledge_search_tool.go`

### 1.2 web_search - 网络搜索工具 ✅ 已完成
- [x] 集成搜索 API (SerpAPI / Bing)
- [x] 解析搜索结果
- [x] 支持配置多个搜索提供商
- **文件**: `services/mcp-service/internal/tools/real_tools.go`
- **配置**: `WEB_SEARCH_API_KEY`, `WEB_SEARCH_PROVIDER`

### 1.3 weather - 天气查询工具 ✅ 已完成
- [x] 集成天气 API (OpenWeatherMap / 和风天气)
- [x] 支持中文城市名
- [x] 返回实时天气和预报
- **文件**: `services/mcp-service/internal/tools/real_tools.go`
- **配置**: `WEATHER_API_KEY`, `WEATHER_PROVIDER`

### 1.4 code_execute - 代码执行工具 🟡 部分实现
- [x] 基础架构已实现
- [ ] 实现安全的沙箱执行环境 (需要 Docker)
- [ ] 支持 Python 执行
- [ ] 超时控制
- **文件**: `services/mcp-service/internal/tools/tools.go`

### 1.5 file_read - 文件读取工具 🟡 部分实现
- [x] 基础架构已实现
- [ ] 实现安全路径校验
- [ ] 支持配置允许访问的目录
- **文件**: `services/mcp-service/internal/tools/tools.go`

### 1.6 file_write - 文件写入工具 🟡 部分实现
- [x] 基础架构已实现
- [ ] 实现安全路径校验
- **文件**: `services/mcp-service/internal/tools/tools.go`

### 1.7 browser_execute - 浏览器自动化工具 ✅ 已完成
- [x] HTTP 调用 browser-service
- [x] 超时处理
- [x] 错误处理
- [x] 返回执行结果
- **文件**: `services/mcp-service/internal/tools/real_tools.go`

---

## 二、服务间集成 (P0)

### 2.1 MCP <-> Knowledge Service 集成 ✅ 已完成
- [x] MCP 服务添加 Knowledge Service HTTP Client
- [x] SearchTool 调用 KnowledgeService.Search
- [x] 支持租户隔离
- **文件**: `services/mcp-service/internal/tools/knowledge_search_tool.go`

### 2.2 MCP <-> Memory Service 集成 🟡 部分
- [ ] 添加记忆保存工具 (memory_save)
- [ ] 添加记忆召回工具 (memory_recall)
- **文件**: 待创建

### 2.3 A2A 任务真实执行 ✅ 已完成
- [x] HandleSendTask 支持 AgentClient 接口
- [x] 传递任务上下文
- [x] 返回真实执行结果
- [x] 本地 fallback 处理
- **文件**: `services/a2a-service/internal/service/a2a_service.go`

---

## 三、知识库服务改进 (P1)

### 3.1 真正的 BM25 实现 ✅ 已完成
- [x] 实现真正的 BM25 评分算法
- [x] 文档词频统计
- [x] 逆文档频率计算
- [x] BM25 分数计算
- [x] 支持中英文停用词
- **文件**: `services/knowledge-service/internal/search/bm25.go`

### 3.2 Rerank 重排序 🟡 待实现
- [ ] 集成 Rerank 模型 (BGE-Rerank / Cohere Rerank)
- [ ] Hybrid Search 后添加 Rerank 步骤
- **文件**: 待创建

### 3.3 文档解析增强 🟡 待实现
- [ ] PDF 解析 (支持图片、表格)
- [ ] Word 文档解析
- [ ] HTML 解析
- **文件**: `services/knowledge-service/internal/parser/`

### 3.4 分块策略优化 🟡 待实现
- [ ] 语义分块
- [ ] 递归分块
- **文件**: `services/knowledge-service/internal/chunker/`

---

## 四、LLM 客户端完善 (P1)

### 4.1 OpenAI Streaming 实现 ✅ 已完成
- [x] 实现 SSE (Server-Sent Events) 解析
- [x] 真正的流式返回
- [x] 错误处理
- **文件**: `pkg/llm/client.go`

### 4.2 DashScope (阿里云) 完整实现 🟡 待实现
- [ ] Chat 接口
- [ ] Streaming 接口
- [ ] Embedding 接口
- **文件**: `pkg/llm/client.go`

### 4.3 更多 LLM 提供商 🟡 待实现
- [ ] Anthropic Claude
- [ ] Google Gemini
- [ ] 本地模型 (Ollama)
- **文件**: `pkg/llm/`

---

## 五、Memory 服务增强 (P1)

### 5.1 记忆遗忘机制 ✅ 已完成
- [x] 基于时间衰减的分数计算
- [x] 重要性排序
- [x] 自动清理低分记忆
- [x] 记忆过期时间
- [x] Cleanup API
- **文件**: `services/memory-service/internal/service/memory_service_with_forgetting.go`

### 5.2 记忆压缩 🟡 待实现
- [ ] 定期总结旧记忆
- [ ] 合并相似记忆
- **文件**: 待创建

### 5.3 记忆类型扩展 ✅ 已支持
- [x] 对话记忆 (Conversation)
- [x] 事实记忆 (Fact)
- [x] 程序记忆 (Procedure)

---

## 六、Agent 服务完善 (P1)

### 6.1 Agent 配置持久化
- [ ] 数据库存储 Agent 定义
- [ ] 启动时从数据库加载
- [ ] 支持动态注册/注销
- [ ] 配置版本管理
- **文件**: `services/agent-service/internal/repository/agent_repository.go`

### 6.2 并行 Agent 执行
- [ ] 同时调用多个 Agent
- [ ] 结果合并策略
- [ ] 超时控制
- **文件**: `pkg/agent/parallel.go`

### 6.3 Agent 协作模式
- [ ] 主从模式 (Orchestrator)
- [ ] 投票模式 (Voting)
- [ ] 辩论模式 (Debate)
- [ ] 层级模式 (Hierarchical)
- **文件**: `pkg/agent/patterns/`

### 6.4 Agent 配置热更新
- [ ] 监听配置变化
- [ ] 无缝切换 Agent 定义
- [ ] 运行中任务不受影响
- **文件**: `services/agent-service/internal/service/hot_reload.go`

---

## 七、Chat 服务改进 (P1)

### 7.1 对话历史压缩
- [ ] 滑动窗口 + 总结
- [ ] 提取关键信息保留
- [ ] Token 预算控制
- **文件**: `services/chat-service/internal/service/history_compression.go`

### 7.2 多轮对话上下文
- [ ] 自动保存对话摘要
- [ ] 长期记忆关联
- [ ] 上下文继承
- **文件**: `services/chat-service/internal/service/context.go`

### 7.3 流式响应改进
- [ ] 真正的流式 Agent 执行
- [ ] 实时返回思考过程
- [ ] 工具调用进度
- **文件**: `services/chat-service/internal/service/stream.go`

---

## 八、Gateway 服务 (P2)

### 8.1 API 文档
- [ ] Swagger/OpenAPI 集成
- [ ] 自动生成文档
- [ ] 在线测试界面
- **文件**: `services/gateway/docs/`

### 8.2 请求限流
- [ ] 基于租户限流
- [ ] 基于用户限流
- [ ] 基于 IP 限流
- **文件**: `services/gateway/internal/middleware/ratelimit.go`

### 8.3 请求日志
- [ ] 结构化日志
- [ ] 请求追踪 ID
- [ ] 性能监控
- **文件**: `services/gateway/internal/middleware/logging.go`

---

## 九、基础设施 (P2)

### 9.1 健康检查
- [ ] 所有服务添加 /health 端点
- [ ] /ready 就绪检查
- [ ] 依赖服务状态检查
- **文件**: `services/*/cmd/main.go`

### 9.2 Prometheus 监控
- [ ] 请求计数
- [ ] 响应时间
- [ ] 错误率
- [ ] Token 使用量
- **文件**: `pkg/metrics/`

### 9.3 配置管理
- [ ] 配置验证
- [ ] 默认值管理
- [ ] 环境变量覆盖
- [ ] 配置热更新
- **文件**: `pkg/config/validator.go`

---

## 十、测试 (P2)

### 10.1 单元测试
- [ ] pkg/agent/* 测试
- [ ] pkg/llm/* 测试
- [ ] 各服务核心逻辑测试
- **覆盖率目标**: 70%

### 10.2 集成测试
- [ ] 服务间调用测试
- [ ] 端到端流程测试
- **文件**: `tests/integration/`

### 10.3 压力测试
- [ ] 并发请求测试
- [ ] 内存泄漏检测
- [ ] 性能基准
- **文件**: `tests/benchmark/`

---

## 十一、前端 (P3)

### 11.1 对话界面
- [ ] 流式消息显示
- [ ] Markdown 渲染
- [ ] 代码高亮
- [ ] Agent 执行过程可视化

### 11.2 管理界面
- [ ] Agent 配置管理
- [ ] 知识库管理
- [ ] 日志查看
- [ ] 监控面板

---

## 实施计划

### 第一周 (P0 核心功能)
1. MCP 工具实现 (knowledge_search, browser_execute)
2. 服务间集成 (MCP <-> Knowledge, MCP <-> Memory)
3. A2A 真实任务执行

### 第二周 (P0 + P1)
4. Web Search / Weather API 集成
5. OpenAI Streaming 实现
6. BM25 真实实现

### 第三周 (P1)
7. Rerank 集成
8. Memory 遗忘机制
9. Agent 配置持久化

### 第四周 (P2)
10. 健康检查 + 监控
11. 单元测试
12. 文档完善

---

## 快速开始实现

```bash
# 1. 创建新的工具实现目录
mkdir -p services/mcp-service/internal/tools/impl

# 2. 开始实现第一个工具: knowledge_search
# 编辑 services/mcp-service/internal/tools/search_tool.go

# 3. 添加 Knowledge Service 客户端
# 编辑 services/mcp-service/internal/service/mcp_service.go
```

---

**更新日志**:
- 2026-06-07: 创建清单，明确所有待实现功能
