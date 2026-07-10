# 阶段 6:工程化卫生(并行)

> 优先级: 与功能改造并行
> 依据: `real-agent-analysis.md` §3.3

---

## 1. 现状

| 项 | 现状 |
|----|------|
| 测试 | 1 个测试文件(`demo/retry/retry_test.go`),≈ 0% 覆盖 |
| 日志 | `engine.go` 里 30+ 处 `fmt.Printf` 生产调试日志 |
| 代码执行沙箱 | `code_execute` 返回 mock,agent 不能写/跑代码 |
| 文件读写工具 | stub 返回固定内容 |
| 鉴权 | `Auth()` 中间件 pass-through,`// TODO: Validate JWT` |
| Secret | config.yaml 明文 API key |

## 2. 目标

核心包有单测;生产代码用结构化日志;code_execute 接真实沙箱;鉴权不是 stub。
**与阶段 1-5 并行推进,不阻塞功能改造。**

## 3. 分项计划

### 3.1 结构化日志(先做,低风险)
- `engine.go` 的 `fmt.Printf("[AgentEngine] ...")` 替换为结构化日志
- 用 `log/slog`(Go 标准库)或项目已有的 logger
- 改造点:`engine.go` 全部 `fmt.Printf`,约 30 处
- 保留语义:把 `[AgentEngine] Step %d: Agent=%s` 变成 `slog.Info("step", "agent", id, "step", n)`
- 先做 engine.go,其他服务后续跟进

### 3.2 核心包单测
优先给这几个包补测试(AAA 模式,目标 60% -> 80%):
- `pkg/agent/engine_test.go`:executeLoop 主路径、handoff、tool 执行、checkpoint 保存
- `pkg/agent/memory_client_test.go`:Recall/Write 接口 + mock
- `pkg/agent/reflection/loop_test.go`:Reflect、StrategyAdjuster.Evaluate
- `pkg/agent/checkpoint/checkpoint_test.go`:Save/Get/List
- `pkg/llm/client_test.go`:client 适配、metrics wrapper

用 mock 接口(LLMClient、ToolExecutor、MemoryClient、CheckpointStore),不依赖外部服务。

### 3.3 code_execute 真实沙箱
- 现状:`mcp-service/internal/tools/real_tools.go` 的 `code_execute` 返回 mock
- 方案:Docker 容器执行(gVisor 更安全)
  - 输入:代码 + 语言
  - 执行:起一个受限容器,跑代码,超时 30s,捕获 stdout/stderr
  - 返回:真实输出
- 参考 Modal Sandbox 模式(隔离 + 超时 + 资源限制)
- 先做 Docker 方案,后期可换 gVisor/Firecracker

### 3.4 文件读写工具
- `file_read` / `file_write` 接真实文件系统,限制工作目录(sandbox dir)
- 防路径穿越:`filepath.Clean` + 校验在 sandbox 内
- 配置工作目录根(如 `/data/agent-workspace/<session_id>/`)

### 3.5 鉴权
- `services/gateway/internal/middleware/middleware.go` 的 `Auth()`:实现真 JWT 校验
- 用 `golang-jwt/jwt` 库,校验签名 + 过期 + claims
- 从 token 提取 user_id 注入 context
- 密钥从环境变量读,不硬编码

### 3.6 Secret 管理
- `config.yaml` 的 `llm.api_key` 迁到环境变量 `LLM_API_KEY`
- `config.Load` 优先读 env,fallback 到 config.yaml(过渡期)
- 启动时校验必需 secret 存在,缺失 fatal

## 4. 实施顺序(并行)

| 顺序 | 项 | 依赖 | 风险 |
|------|----|------|------|
| 1 | 结构化日志(engine.go) | 无 | 低 |
| 2 | 核心包单测 | 阶段1-4 接口稳定 | 低 |
| 3 | Secret 迁 env | 无 | 低 |
| 4 | 鉴权 JWT | 无 | 中 |
| 5 | file_read/write | 无 | 中 |
| 6 | code_execute 沙箱 | Docker 环境 | 高 |

## 5. 验收标准

- [ ] `engine.go` 无 `fmt.Printf`,改用结构化日志
- [ ] `go test ./pkg/agent/... ./pkg/llm/...` 通过,覆盖率 ≥ 60%
- [ ] `code_execute` 能跑真实代码返回真实输出(在 Docker 里)
- [ ] `file_read`/`file_write` 能读写文件,且防路径穿越
- [ ] `Auth()` 中间件真校验 JWT,无效 token 返回 401
- [ ] API key 从环境变量读,config.yaml 不再有明文密钥

## 6. 与功能改造的关系

- 阶段 1-5 是"让 agent 变真"
- 阶段 6 是"让系统可上生产"
- 两者并行,但功能改造优先(阶段6.1 日志、6.3 沙箱 可稍后)
- 测试(6.2)在阶段 1-4 接口稳定后补,避免返工
