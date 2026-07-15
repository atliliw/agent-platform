# 上下文压缩（Context Compression）实现计划 v1

> 日期: 2026-07-07
> 目标: 在 agent LLM 调用前压缩过大上下文（工具输出 / AgentHistory / 累积历史），降低 token 成本 40-60%
> 范围: contained v1 —— 只截断 content、不删 message（不破坏 tool 配对），lossless 对 checkpoint

## 一、设计决策

**放置点：`llm.Client` decorator（照抄 `metricsClient` 范式）**

- 一个 `CompressingClient` 包装 `llm.Client`，在 `Chat`/`ChatStream` 委托前压缩 `req.Messages`
- 在 agent-service 和 chat-service 两处 wiring，**同时覆盖多 agent 和单 agent 两条链路**
- 透传 `Embed`/`EmbedBatch`
- lossless：不动 `execCtx.Messages`，只压缩发给 LLM 的瞬态请求 → checkpoint 完整、可恢复

**为什么不放 `agent.LLMClient` 层**：只覆盖多 agent 链路，漏掉单 agent 链路（chat-service 直连 `llm.Client`）。
**为什么不放 `AddToolMessage` 永久截断**：v1 保持 lossless；永久 cap 留 v1.1（减 checkpoint 体积）。
**为什么不删 message**：llm.Message 无 tool_call_id 字段，平台疑似自定义 tool-calling 协议，删 message 有破坏配对风险；只截 content 绝对安全。

**压缩策略（分层截断，零额外 LLM 调用）**：
- system 消息：cap `MaxSystemChars`（12K）——只防异常膨胀，保留指令
- 最近 `RecentCount`（8）条：cap `MaxRecentChars`（6K）——保留近期细节
- 更早的消息：cap `MaxOldChars`（1K）——激进压缩旧工具输出（browser HTML/日志）
- 超长 content 截断后加 `...(truncated, was N chars)` 标记
- 不删任何 message，tool 配对零风险

## 二、改动清单

### 1. 新文件 `pkg/llm/compress.go`（~150 行）
- `CompressionConfig` struct: `Enable bool`, `MaxSystemChars/MaxRecentChars/MaxOldChars int`, `RecentCount int`
- `EstimateTokens(messages []Message) int` —— CJK 感知（从 `memory-service/internal/working/working.go:494` 移植：英文 ~4 chars/token，CJK ~1.5）
- `compressingClient` struct + `NewCompressionClient(inner Client, cfg CompressionConfig) Client`
  - `!cfg.Enable` 时 nil-shortcut 直接返回 `inner`（同 `NewMetricsClient` 范式）
- `Chat`: 复制 `req.Messages` → 分层截断 → 记录 original/compressed token 估算（debug log）→ 委托 `inner.Chat`
- `ChatStream`: 同上压缩后委托
- `Embed`/`EmbedBatch`: 透传
- 截断辅助 `truncateWithMarker(s string, max int) string`（参考 `verifier.go:48-56`）

### 2. `pkg/config/config.go` —— 加压缩配置
- `CompressionConfig` 字段进 LLM/agent config，默认值如上
- 从环境变量/ yaml 读取

### 3. `services/agent-service/internal/service/agent_service.go` ~line 57 —— wiring（多 agent 链路）
- `metricsLLM := llm.NewMetricsClient(llm.NewCompressionClient(llmClient, compCfg), cb, "engine")`
- 顺序：compress → measure（这样 metrics 反映压缩后 token，Cost 面板直接体现降幅）

### 4. `services/chat-service/internal/service/chat_service.go` ~line 102 —— wiring（单 agent 链路）
- 同样包一层 `NewCompressionClient`，caller `"chat"`

### 5. `pkg/agent/engine.go:900-905` —— AgentHistory cap（2 行）
- `buildAgentMessages` 里把每条 `r.Result` 截到 ~1K chars 再拼进 system prompt
- 防止 system prompt 每步无界增长（decorator 的 system cap 是兜底，这里从源头限）

## 三、不在 v1 做（留 v1.1）
- `AddToolMessage` 永久 cap（减 checkpoint 体积）
- LLM 摘要式压缩（`working.go` 的 Summary/Hybrid 策略，需额外 LLM 调用，加延迟）
- observability 显式 "tokens saved" 指标（v1 靠 PromptTokens 自然下降体现）
- workflow engine（harness-service）链路 wrapping（单次调用少累积，优先级低）

## 四、验证
1. `go build ./...` 通过
2. 本地单测：`EstimateTokens` 准确性 + 分层截断正确性（构造 system/recent/old 消息，断言截断后长度与标记）
3. 部署 agent-service + chat-service 到 192.168.10.100
4. 端到端：发一条会触发 browser_execute / knowledge_search 的对话，对比压缩前后 `PromptTokens`（从 `/api/v2/harness/llm/metrics` 看），确认下降
5. 回归：普通对话回复正常、tool 调用不报配对错误

## 五、风险
- 截断旧工具结果可能让 agent 丢失早期细节 → `MaxOldChars` 设 1K（保守），可配置调大
- system cap 误伤指令 → 只在 >12K 时触发，正常 system prompt 远低于此
- CJK token 估算是启发式 → 仅用于决策是否截断，不用于计费；计费仍用 provider 返回的 PromptTokens
