# 部署历程记录

> 记录本次部署过程中遇到的问题和解决方案，避免以后踩坑。

---

## 一、背景

本次部署主要修复和补全以下功能：
- **可观测性模块** - 执行追踪、成本监控、评测报告、记忆管理
- **运维治理模块** - 安全护栏、规则引擎、A/B 测试、SLO 监控、权限矩阵、审计日志

---

## 二、遇到的问题

### 问题 1：可观测性功能全部报错 404

**现象**：前端调用 `/api/v2/observability/traces` 等接口全部返回 404

**原因**：
- Handler 文件已创建（`observability_handler.go`、`cost_handler.go`、`memory_enhanced_handler.go`、`eval_handler.go`）
- 但路由没有在 `router.go` 中注册

**解决方案**：
```go
// router.go 添加路由
api.GET("/observability/traces", observabilityHandler.GetTraces)
api.GET("/observability/metrics", observabilityHandler.GetMetrics)
api.GET("/cost/summary", costHandler.GetSummary)
api.GET("/memory-enhanced/stats", memoryEnhancedHandler.GetStats)
api.GET("/eval/suites", evalHandler.GetSuites)
// ... 等更多路由
```

---

### 问题 2：前端 API 路径与后端不匹配

**现象**：前端调用 `/api/v2/harness/eval/suites`，后端注册的是 `/api/v2/eval/suites`

**原因**：前端 API 文件中路径写错了

**解决方案**：修改前端 API 文件
```typescript
// evaluationApi.ts
// 错误: '/api/v2/harness/eval/suites'
// 正确: '/api/v2/eval/suites'

// memoryApi.ts  
// 错误: '/api/v2/memory/stats'
// 正确: '/api/v2/memory-enhanced/stats'
```

---

### 问题 3：前端响应格式处理错误

**现象**：后端返回 `{ code: 0, data: {...} }`，前端直接用 `response.data` 拿到的是整个对象，而不是 data 字段的内容

**原因**：axios 响应拦截器处理不当

**解决方案**：修改 `client.ts`
```typescript
// 响应拦截器
client.interceptors.response.use(
  (response) => {
    const data = response.data;
    if (data && typeof data === 'object' && 'code' in data && 'data' in data) {
      if (data.code === 0) {
        return data.data;  // 提取 data 字段返回
      } else {
        return Promise.reject(new Error(data.message || '请求失败'));
      }
    }
    return data;
  },
  // ...
);
```

---

### 问题 4：Go 编译错误 - 浮点数时间乘法

**现象**：
```
services/gateway/internal/handler/observability_handler.go:38:120: -4.9 (untyped float constant) truncated to int64
```

**原因**：Go 不允许 `time.Minute * -4.9` 这种浮点数乘法

**解决方案**：
```go
// 错误写法
time.Now().Add(-4.9 * time.Minute)

// 正确写法  
time.Now().Add(-294 * time.Second)  // 4.9分钟 = 294秒
```

---

### 问题 5：治理页面太简陋

**现象**：原来的治理页面只有简单的规则管理，全是假数据

**原因**：前端没有对接后端丰富的功能（A/B 测试、SLO、护栏、审计等）

**解决方案**：重写整个治理页面，包含 7 个完整模块：
- 概览仪表盘（带治理对话测试）
- 规则引擎（CRUD）
- 安全护栏（输入/输出检测）
- A/B 测试（创建/查看结果）
- SLO 监控（创建/状态查看）
- 权限矩阵（展示）
- 审计日志（筛选/展开）

---

### 问题 6：SLO 创建 API 404

**现象**：`POST /api/v2/harness/slo` 返回 404

**原因**：
- Handler 没有 `CreateSLO` 方法
- 路由没有注册

**解决方案**：
1. 在 `harness_handler.go` 添加 `CreateSLO` 方法
2. 在 `router.go` 注册路由：
```go
api.POST("/harness/slo", harnessHandler.CreateSLO)
```

---

## 三、部署命令汇总

### 3.1 同步文件到服务器

```bash
# 同步单个文件
scp -i ~/.ssh/demo_deploy_key <本地文件> root@192.168.10.100:/opt/agent-platform/<目标路径>

# 示例
scp -i ~/.ssh/demo_deploy_key services/gateway/internal/router/router.go root@192.168.10.100:/opt/agent-platform/services/gateway/internal/router/
scp -i ~/.ssh/demo_deploy_key frontend/src/pages/Harness/index.tsx root@192.168.10.100:/opt/agent-platform/frontend/src/pages/Harness/
```

### 3.2 重新构建服务

```bash
ssh -i ~/.ssh/demo_deploy_key root@192.168.10.100 "cd /opt/agent-platform/docker && docker compose build gateway frontend && docker compose up -d gateway frontend"
```

### 3.3 查看服务状态

```bash
ssh -i ~/.ssh/demo_deploy_key root@192.168.10.100 "docker compose ps"
ssh -i ~/.ssh/demo_deploy_key root@192.168.10.100 "docker logs docker-gateway-1 --tail 50"
```

---

## 四、最终验证

所有 API 测试结果：

| API | 结果 |
|-----|------|
| `GET /api/v2/harness/rules` | ✅ 返回规则列表 |
| `POST /api/v2/harness/rules` | ✅ 创建规则 |
| `DELETE /api/v2/harness/rules/:id` | ✅ 删除规则 |
| `POST /api/v2/harness/guardrail/check` | ✅ 检测通过/拦截 |
| `POST /api/v2/harness/abtest` | ✅ 创建 A/B 测试 |
| `GET /api/v2/harness/abtest/:id/result` | ✅ 返回测试结果 |
| `POST /api/v2/harness/slo` | ✅ 创建 SLO |
| `GET /api/v2/harness/slo/status` | ✅ 返回 SLO 状态 |
| `POST /api/v2/harness/chat` | ✅ 治理对话（带护栏） |
| `GET /api/v2/observability/traces` | ✅ 返回追踪列表 |
| `GET /api/v2/cost/summary` | ✅ 返回成本汇总 |
| `GET /api/v2/eval/suites` | ✅ 返回评测套件 |

---

## 五、踩坑总结

### 5.1 路由注册最容易漏

**教训**：写了 Handler 不等于 API 能用，必须在 router.go 注册路由

**检查方法**：
```bash
# 测试 API 是否存在
curl http://192.168.10.100:9000/api/v2/<path>

# 如果返回 404，说明路由没注册
```

### 5.2 前后端路径要一致

**教训**：前端 API 文件和后端 router.go 的路径必须完全匹配

**检查方法**：
```bash
# 后端路由
grep "api.GET" services/gateway/internal/router/router.go

# 前端 API  
grep "client.get" frontend/src/api/*.ts
```

### 5.3 Go 时间运算只能用整数

**教训**：`time.Duration` 乘法只能用整数，浮点数会报错

**正确写法**：
```go
time.Now().Add(5 * time.Minute)      // 正确
time.Now().Add(300 * time.Second)    // 正确
time.Now().Add(-4.9 * time.Minute)   // 错误！
```

### 5.4 TypeScript 未使用变量报错

**教训**：TypeScript 编译会检查未使用的导入和变量

**解决**：删除未使用的 import，或者用 `_` 忽略

### 5.5 响应格式处理要统一

**教训**：后端统一返回 `{ code: 0, data: {...} }`，前端拦截器要提取 data

---

## 六、服务端口对照表

| 服务 | 端口 | 说明 |
|------|------|------|
| Frontend | 8888 | 前端界面 |
| Gateway | 9000 | API 网关 |
| Chat Service | 50001 | 对话服务 |
| Knowledge Service | 50002 | 知识库服务 |
| Memory Service | 50003 | 记忆服务 |
| A2A Service | 50004 | 跨服务通信 |
| MCP Service | 50005 | 工具协议服务 |
| Agent Service | 50006 | Agent 编排服务 |
| Harness Service | 50007 | 运维治理服务 |

---

## 七、常用操作

```bash
# 连接服务器
ssh -i ~/.ssh/demo_deploy_key root@192.168.10.100

# 查看所有容器状态
docker compose ps

# 查看特定服务日志
docker logs docker-gateway-1 -f

# 重启单个服务
docker compose restart gateway

# 重新构建并启动
docker compose build gateway
docker compose up -d gateway

# 查看容器资源使用
docker stats
```

---

## 八、访问地址

- **前端**: http://192.168.10.100:8888
- **API**: http://192.168.10.100:9000
- **健康检查**: http://192.168.10.100:9000/health

---

*文档生成时间: 2026-06-15*