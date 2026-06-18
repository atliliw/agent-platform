# Harness 功能移植 Task List

> 将 mcpgo 的 Harness 完整功能移植到 agent-platform

---

## 一、需要移植的模块

### 1. A/B 实验引擎 (evolve/abtest.go)
- [x] CreateExperiment - 创建实验（自动暂停同 Agent 其他实验）
- [x] Pause/Resume - 暂停/恢复实验
- [x] Promote/Reject - 晋升/拒绝变体
- [x] Delete - 删除实验
- [x] ShouldUseVariant - Hash-based 流量分配
- [x] RecordResult - 记录结果
- [x] Evaluate - 统计评估（t-test, p-value, 样本量计算）
- [x] GetStats - 获取实验统计
- [x] AutoPromote - 自动晋升配置
- [x] 持久化到数据库

### 2. SLO 管理器 (slo/manager.go)
- [x] 多窗口类型支持（rolling-1h, rolling-6h, calendar-day/week/month）
- [x] Burn Rate 计算
- [x] Alert Burn Rate 阈值
- [x] RecordEvent - 记录事件
- [x] GetErrorBudget - 获取错误预算
- [x] CheckBurnRate - 检查燃烧率
- [x] EvaluateAll - 评估所有 SLO
- [x] 持久化到数据库
- [x] 告警回调

### 3. Feature Flag 引擎 (featureflag/engine.go)
- [x] CreateFlag - 创建开关
- [x] ListFlags - 列出开关
- [x] GetFlag - 获取开关
- [x] UpdateFlag - 更新开关
- [x] Toggle - 开关切换
- [x] DeleteFlag - 删除开关
- [x] Evaluate - 评估开关值
- [x] Targeting 规则支持（eq, neq, contains, gt, lt）
- [x] Percentage Rollout - 百分比发布
- [x] DetectStaleFlags - 过期检测
- [x] 持久化到数据库

### 4. Rollback 引擎 (rollback/engine.go)
- [x] CreateConfig - 创建回滚配置
- [x] GetConfig - 获取配置
- [x] UpdateConfig - 更新配置
- [x] ListConfigs - 列出配置
- [x] TakeSnapshot - 创建配置快照
- [x] ListSnapshots - 列出快照
- [x] ExecuteRollback - 执行回滚
- [x] ListRollbackEvents - 列出回滚事件
- [x] 自动回滚逻辑
- [x] 冷却期控制

### 5. RCA 根因分析 (rca/engine.go)
- [x] RecordChange - 记录变更事件
- [x] Analyze - 分析根因
- [x] ListChanges - 列出变更
- [x] ListReports - 列出分析报告
- [x] 相关性评分算法
- [x] 推荐生成

### 6. Chaos 混沌工程 (chaos/engine.go)
- [x] CreateExperiment - 创建混沌实验
- [x] StartExperiment - 启动实验
- [x] StopExperiment - 停止实验
- [x] ListExperiments - 列出实验
- [x] GetExperiment - 获取实验
- [x] InjectFault - 故障注入
- [x] 故障类型支持：
  - [x] agent_timeout
  - [x] agent_error
  - [x] model_degraded
  - [x] guardrail_bypass
  - [x] budget_exhaust
  - [x] network_latency
- [x] Blast Radius 控制
- [x] SLO 联动自动停止

### 7. Cost Intelligence (cost/engine.go)
- [x] CostReport - 成本报告
- [x] IdleAgents - 闲置 Agent 检测
- [x] ModelComparison - 模型成本对比
- [x] Recommendations - 成本优化推荐
- [x] SetModelPricing - 设置模型定价
- [x] ListModelPricing - 列出定价

### 8. Golden Path Templates (goldenpath/)
- [x] CreateTemplate - 创建模板
- [x] ListTemplates - 列出模板
- [x] GetTemplate - 获取模板
- [x] DeleteTemplate - 删除模板
- [x] InstantiateTemplate - 实例化模板
- [x] ImportTemplates - 导入模板
- [x] ExportTemplates - 导出模板

### 9. Pipeline 执行流水线 (pipeline/)
- [ ] CreatePipeline - 创建流水线
- [ ] ListPipelines - 列出流水线
- [ ] GetPipeline - 获取流水线
- [ ] DeletePipeline - 删除流水线
- [ ] ExecutePipeline - 执行流水线
- [ ] AbortPipelineRun - 中止执行
- [ ] ListPipelineRuns - 列出执行记录
- [ ] GetPipelineRun - 获取执行详情

### 10. Catalog Agent 目录 (catalog/)
- [x] ListCatalogAgents - 列出 Agent 目录
- [x] GetCatalogAgent - 获取 Agent 信息
- [x] RefreshCatalog - 刷新目录

### 11. Optimizer 优化器 (evolve/optimizer.go)
- [x] RunOptimizer - 运行优化器
- [x] 自动调参逻辑

### 12. Proposal 自演化提案 (evolve/proposal.go)
- [x] CreateProposal - 创建提案
- [x] ListProposals - 列出提案
- [x] GetProposal - 获取提案
- [x] ApproveProposal - 批准提案
- [x] RejectProposal - 拒绝提案

### 13. Scheduler 定时评估
- [x] SetEvalSchedule - 设置评估计划
- [x] GetSchedule - 获取计划
- [x] ListSchedules - 列出计划
- [x] PauseSchedule - 暂停计划
- [x] ResumeSchedule - 恢复计划
- [x] DeleteSchedule - 删除计划
- [x] RunNow - 立即执行
- [x] GetResults - 获取执行结果
- [x] SchedulerStatus - 调度状态
- [x] SchedulerControl - 启动/停止调度器
- [x] GetStats - 获取统计信息
- [x] 定时执行评估逻辑

### 14. Orchestrator Quality 编排质量 (coordinate/)
- [x] RecordOrchestration - 记录编排
- [x] OrchestratorReport - 编排报告
- [x] OrchestratorRuns - 编排执行记录
- [x] ScoreOrchestration - 编排评分

### 15. Planner Eval 规划评估
- [x] ScorePlan - 规划评分

---

## 二、不需要移植的模块

| 模块 | 原因 |
|------|------|
| Alert | 告警功能已在 SLO 中集成回调，不需要单独模块 |
| Tenant | 租户管理由外部系统处理 |
| Pipeline | 暂不需要流水线功能 |

---

## 三、移植顺序

### Phase 1: 核心引擎（必须） ✅ 已完成
1. A/B 实验引擎 - 最常用，功能差异最大
2. SLO 管理器 - 监控核心
3. Feature Flag 引擎 - 发布控制核心

### Phase 2: 安全与恢复（重要） ✅ 已完成
4. Rollback 引擎 - 配置回滚
5. RCA 根因分析 - 故障分析

### Phase 3: 高级功能（可选） ✅ 已完成
6. Chaos 混沌工程
7. Cost Intelligence
8. Optimizer + Proposal（自演化）

### Phase 4: 辅助功能 ✅ 已完成
9. Golden Path Templates
10. Catalog Agent 目录
11. Orchestrator Quality
12. Planner Eval

---

## 四、文件结构规划

```
services/harness-service/internal/
├── abtest/
│   └── engine.go          # A/B 实验引擎（已完成）
├── slo/
│   └── manager.go         # SLO 管理器（已完成）
├── featureflag/
│   └── engine.go          # Feature Flag 引擎（已完成）
├── rollback/
│   └── engine.go          # Rollback 引擎（已完成）
├── rca/
│   └── engine.go          # RCA 根因分析（已完成）
├── chaos/
│   └── engine.go          # Chaos 混沌工程（已完成）
├── cost/
│   └── engine.go          # Cost Intelligence（已完成）
├── goldenpath/
│   └── engine.go          # Golden Path（已完成）
├── catalog/
│   └── engine.go          # Catalog（已完成）
├── evolve/
│   └── evolve.go          # Optimizer + Proposal（已完成）
├── coordinate/
│   └── quality.go         # 编排质量（已完成）
├── planner/
│   └── eval.go            # 规划评估（已完成）
└── scheduler/
    └── engine.go          # 定时调度（已完成）
```

---

## 五、完成状态

| 阶段 | 模块 | 状态 |
|------|------|------|
| Phase 1 | A/B 实验引擎 | ✅ 已完成 |
| Phase 1 | SLO 管理器 | ✅ 已完成 |
| Phase 1 | Feature Flag 引擎 | ✅ 已完成 |
| Phase 2 | Rollback 引擎 | ✅ 已完成 |
| Phase 2 | RCA 根因分析 | ✅ 已完成 |
| Phase 3 | Chaos 混沌工程 | ✅ 已完成 |
| Phase 3 | Cost Intelligence | ✅ 已完成 |
| Phase 3 | Optimizer + Proposal | ✅ 已完成 |
| Phase 4 | Golden Path Templates | ✅ 已完成 |
| Phase 4 | Catalog Agent 目录 | ✅ 已完成 |
| Phase 4 | Orchestrator Quality | ✅ 已完成 |
| Phase 4 | Planner Eval | ✅ 已完成 |
| Phase 4 | Scheduler 定时评估 | ✅ 已完成 |

---

## 六、待办事项

1. **数据库迁移** - 需要添加新表
2. **Proto 定义** - 需要更新 pb/harness.proto
3. **Handler 路由** - ✅ 已添加
4. **前端页面** - 需要更新 Harness 页面

---

*文档更新时间: 2026-06-16*
*移植完成时间: 2026-06-16*
