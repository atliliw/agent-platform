# MCP 治理系统集成指南

## 架构概览

```
┌─────────────────────────────────────────────────────────────────┐
│                        Gateway / Chat Service                    │
└─────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│                     MCP Service (带治理)                         │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │                  Governance Pipeline                       │  │
│  │  ┌─────────┐  ┌──────────┐  ┌───────────┐  ┌──────────┐  │  │
│  │  │Guardrail│→ │Permission│→ │Rule Engine│→ │A/B Test  │  │  │
│  │  │(护栏)   │  │(权限)    │  │(规则引擎) │  │(分流测试)│  │  │
│  │  └─────────┘  └──────────┘  └───────────┘  └──────────┘  │  │
│  └───────────────────────────────────────────────────────────┘  │
│                                │                                  │
│                                ▼                                  │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │                      Tool Executor                         │  │
│  │  browser_execute | quick_fetch | web_search | ...          │  │
│  └───────────────────────────────────────────────────────────┘  │
│                                │                                  │
│                                ▼                                  │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │                     SLO Manager                            │  │
│  │  成功率追踪 | 延迟监控 | Error Budget | 告警               │  │
│  └───────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

## 快速开始

### 1. 在 MCP Service 中启用治理

```go
// services/mcp-service/cmd/main.go
package main

import (
    "agent-platform/services/mcp-service/internal/service"
    "agent-platform/services/mcp-service/internal/handler"
)

func main() {
    // 创建带治理的 MCP Service
    mcpService := service.NewMCPService(llmClient, cfg)
    
    // 创建治理 API Handler
    govHandler := handler.NewGovernanceHandler(mcpService)
    
    // 注册路由
    mux := http.NewServeMux()
    govHandler.RegisterRoutes(mux)
    
    // 启动服务
    http.ListenAndServe(":50005", mux)
}
```

### 2. 在 Gateway 中集成治理调用

```go
// services/gateway/internal/handler/chat_handler.go

func (h *ChatHandler) handleToolCall(ctx context.Context, toolName string, args map[string]interface{}) (string, error) {
    // 构建治理请求
    govReq := &service.CallToolRequest{
        Name:         toolName,
        Arguments:    toJSON(args),
        AgentType:    h.getAgentType(ctx),     // 从上下文获取 Agent 类型
        InputContent: extractInputContent(args),
        TestID:       h.getABTestID(ctx),      // A/B 测试 ID（可选）
    }
    
    // 调用带治理的 MCP 服务
    resp, err := h.mcpClient.CallToolWithGovernance(ctx, govReq)
    if err != nil {
        return "", err
    }
    
    // 检查治理结果
    if resp.GovernanceResult.Blocked {
        return "", fmt.Errorf("工具调用被拦截: %s", resp.GovernanceResult.BlockReason)
    }
    
    // 返回清理后的内容
    return resp.SanitizedContent, nil
}
```

### 3. 配置 Agent 类型和权限

```go
// 在服务启动时配置
func setupGovernance(mcpService *service.MCPService) {
    // 配置浏览器 Agent 权限
    mcpService.SetPermission(&governance.Permission{
        AgentType:    "browser",
        AllowedTools: []string{"browser_execute", "quick_fetch", "web_search"},
        DeniedTools:  []string{"exec_*", "delete_*"},
        MaxTokens:    100000,
        MaxCostUSD:   5.0,
        MaxDuration:  600, // 10分钟
    })
    
    // 配置搜索 Agent 权限
    mcpService.SetPermission(&governance.Permission{
        AgentType:    "search",
        AllowedTools: []string{"web_search", "knowledge_search", "quick_fetch"},
        DeniedTools:  []string{"browser_*"},
        MaxTokens:    50000,
        MaxCostUSD:   1.0,
        MaxDuration:  120,
    })
    
    // 添加自定义规则
    mcpService.AddRule("browser", governance.ToolRule{
        Name:         "limit_csdn_publish",
        ToolPattern:  "csdn_publish",
        Effect:       "allow",
        MaxCalls:     10, // 每天最多发布 10 篇
        RequiresApproval: false,
    })
}
```

## API 使用示例

### 1. 工具调用（带治理检查）

```bash
# 调用浏览器工具
curl -X POST http://localhost:50005/api/v1/tools/call \
  -H "Content-Type: application/json" \
  -d '{
    "name": "browser_execute",
    "arguments": {
      "task": "打开百度搜索 Go 语言"
    },
    "agent_type": "browser",
    "input_content": "打开百度搜索 Go 语言"
  }'

# 响应
{
  "is_error": false,
  "content": "搜索结果: ...",
  "sanitized_content": "搜索结果: ...",
  "governance_result": {
    "input_checked": true,
    "input_violations": [],
    "tool_checked": true,
    "tool_allowed": true,
    "tool_violations": [],
    "output_checked": true,
    "output_violations": [],
    "blocked": false
  },
  "slo_status": "healthy: 95.23% (budget: 0.85)",
  "latency_ms": 3500
}
```

### 2. 查询 SLO 状态

```bash
curl http://localhost:50005/api/v1/governance/slo/list

# 响应
{
  "slos": [
    {
      "name": "工具调用成功率",
      "target": 0.95,
      "current": 0.9723,
      "budget_remaining": 0.8567,
      "status": "healthy",
      "burn_rate": 0.0023
    },
    {
      "name": "浏览器工具成功率",
      "target": 0.85,
      "current": 0.8821,
      "budget_remaining": 0.9213,
      "status": "healthy"
    }
  ]
}
```

### 3. 创建 A/B 测试

```bash
curl -X POST http://localhost:50005/api/v1/governance/abtest \
  -H "Content-Type: application/json" \
  -d '{
    "id": "test-browser-model-2024",
    "name": "浏览器模型对比",
    "control_model": "qwen-plus",
    "variant_model": "qwen-max",
    "traffic_split": 0.3
  }'

# 查询结果
curl http://localhost:50005/api/v1/governance/abtest/test-browser-model-2024

# 响应
{
  "test_id": "test-browser-model-2024",
  "control_score": 0.82,
  "variant_score": 0.87,
  "delta": 0.05,
  "p_value": 0.03,
  "significant": true,
  "recommended": "variant",
  "control_count": 150,
  "variant_count": 50
}
```

### 4. 添加规则

```bash
curl -X POST http://localhost:50005/api/v1/governance/rules \
  -H "Content-Type: application/json" \
  -d '{
    "agent_type": "browser",
    "rule_name": "limit_browser_calls",
    "tool_pattern": "browser_execute",
    "effect": "allow",
    "max_calls": 50
  }'
```

### 5. 获取告警

```bash
curl http://localhost:50005/api/v1/governance/alerts

# 响应（有告警时）
{
  "alerts": {
    "工具调用成功率": "warning",
    "浏览器工具延迟": "critical"
  }
}
```

## 治理流程详解

### Gate 1: 输入护栏检查

```go
// 检测内容：
// - Prompt Injection 攻击
// - 敏感信息泄露（API Key、密码）
// - 危险指令

violations := guardrail.CheckInput(content)
if len(violations) > 0 {
    for _, v := range violations {
        if v.Severity == "high" {
            return fmt.Errorf("输入被拦截: %s", v.Description)
        }
    }
}
```

### Gate 2: 权限检查

```go
// 检查：
// - Agent 是否有权限使用该工具
// - 是否在黑名单中
// - Token/Cost 限制

err := permission.CheckPermission(agentType, toolName)
if err != nil {
    return err
}
```

### Gate 3: 规则检查

```go
// 检查：
// - 调用次数限制
// - 自定义规则
// - 审批要求

result := ruleEngine.Check(agentType, toolName, callCount)
if !result.Passed {
    return fmt.Errorf("规则检查失败: %v", result.Violations)
}
```

### Gate 4: A/B 测试分流

```go
// 根据流量比例分配模型
model, isVariant := abEngine.AssignVariant(testID)
if isVariant {
    // 使用变体模型
    useModel = variantModel
}
```

### Gate 5: 输出护栏检查

```go
// 检查输出是否包含敏感信息
violations := guardrail.CheckOutput(output)
if len(violations) > 0 {
    // 清理敏感信息
    output = guardrail.SanitizeOutput(output)
}
```

## 监控和可观测性

### Prometheus 指标

```go
// 在 SLO Manager 中自动记录
var (
    toolCallsTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "mcp_tool_calls_total",
            Help: "Total tool calls",
        },
        []string{"tool", "agent_type", "status"},
    )
    
    toolLatency = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "mcp_tool_latency_seconds",
            Help: "Tool call latency",
        },
        []string{"tool"},
    )
)
```

### Grafana Dashboard

导入 `grafana-dashboard.json` 查看实时监控：

- 工具调用成功率趋势
- P99 延迟分布
- Error Budget 消耗速度
- A/B 测试对比
- 告警历史

## 最佳实践

### 1. Agent 类型划分

| Agent 类型 | 允许的工具 | 典型场景 |
|-----------|-----------|---------|
| browser | browser_execute, quick_fetch | 浏览器自动化 |
| search | web_search, knowledge_search | 信息检索 |
| writer | csdn_publish, quick_fetch | 内容创作 |
| general | * (受限) | 通用对话 |

### 2. 规则配置建议

```yaml
# configs/governance.yaml
rules:
  browser:
    - name: limit_browser_calls
      tool_pattern: browser_execute
      max_calls: 100  # 每会话最多 100 次
      
  writer:
    - name: limit_publish
      tool_pattern: "*_publish"
      max_calls: 20   # 每天最多发布 20 篇
      requires_approval: true  # 需要审批
```

### 3. SLO 目标设定

| 指标 | 目标 | 告警阈值 |
|-----|-----|---------|
| 工具成功率 | 95% | < 90% |
| P99 延迟 | 5s | > 10s |
| 浏览器成功率 | 85% | < 80% |

### 4. A/B 测试流程

1. 创建测试（设置对照组和变体组）
2. 运行至少 7 天或收集 1000+ 样本
3. 检查统计显著性（p < 0.05）
4. 根据推荐选择模型
5. 推广胜出模型或继续测试

## 故障排查

### 问题：工具调用被拦截

```bash
# 检查原因
curl -X POST http://localhost:50005/api/v1/tools/call \
  -d '{"name": "browser_execute", "agent_type": "search", ...}'

# 响应
{
  "governance_result": {
    "blocked": true,
    "block_reason": "agent search denied tool browser_execute (matches pattern browser_*)"
  }
}

# 解决方案：使用正确的 agent_type 或修改权限配置
```

### 问题：SLO 进入 critical 状态

```bash
# 检查告警
curl http://localhost:50005/api/v1/governance/alerts

# 检查具体 SLO
curl http://localhost:50005/api/v1/governance/slo \
  -d '{"slo_id": "browser_tool_success"}'

# 解决方案：
# 1. 检查日志找出失败原因
# 2. 调整 SLO 目标或优化工具
# 3. 增加 Error Budget
```

## 下一步

1. **集成到现有系统**: 在 Gateway 的 chat handler 中调用 `CallToolWithGovernance`
2. **配置监控**: 导入 Grafana dashboard，设置告警规则
3. **设置 A/B 测试**: 对比不同模型的效果
4. **调优规则**: 根据实际使用情况调整权限和规则

完整代码示例见 `services/mcp-service/examples/governance_demo.go`