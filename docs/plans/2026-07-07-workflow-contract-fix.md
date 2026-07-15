# Workflow Contract Fix — 后端格式兼容 + condition 路由 UI

## 问题
经 UI 创建的多节点 workflow 完全不可用。前端↔后端 JSON 契约两处断裂 + condition 路由缺口:

1. **边字段不匹配(硬阻断)**:前端 `flowEdgeToWorkflowEdge` 发 `from`/`to`;后端 `reactFlowEdge` 只认 `source`/`target` → From/To 解析为空 → `Validate()` 报 "edge source node not found" → **带边的 workflow 存不进去**。已实测确认。
2. **节点字段不匹配**:前端发扁平 `name`/`agent_id`(顶层);后端 `reactFlowNode` 只读 `data.label`/`data.agent_id`(嵌套)→ Name/AgentID/Condition/ToolName 全空。已实测确认(扁平格式 node_name 返回空)。
3. **condition 路由缺口**:后端 `reactFlowEdge` 无 `Condition` 字段,`modelToWorkflow` 不映射 → edge.Condition 恒空 → condition 节点跳过所有条件边后走 default 兜底(取第一条边),与条件结果无关。executor 路由逻辑本身正确(单元测试覆盖)。
4. **前端无设边标签 UI**:`onConnect` 忽略 `sourceHandle`("true"/"false"),而 ConditionNode 已暴露 true/false 两个源 handle。连线时分支标签直接丢失。

## 修复方案

### 后端:`services/harness-service/internal/workflow/engine.go`
让 `modelToWorkflow` 同时容忍 ReactFlow 嵌套格式和扁平格式(更稳健,不改前端 API 类型)。

- `reactFlowNode`:加顶层 `Name`/`AgentID`/`ToolName`/`Condition` 字段。解析时先取顶层,空则从 `Data.label`/`Data.agent_id`/`Data.tool_name`/`Data.condition` 兜底。
- `reactFlowEdge`:加 `From`/`To`/`Condition` 字段。解析时 `From = Source 或 From`、`To = Target 或 To`,映射 `Condition`。
- 测试:扩展 `engine_test.go`,加 `TestModelToWorkflow_FlatFormat`(扁平节点 + from/to 边)和 `TestModelToWorkflow_EdgeCondition`(edge.condition 映射)。

### 前端:`frontend/src/pages/Workflow/index.tsx`
- `onConnect`:`params.sourceHandle` 为 "true"/"false" 时,给新边设 `data.condition = sourceHandle` 且 `label = sourceHandle`(连线即自动打分支标签,复用 ConditionNode 的 true/false handle)。
- 加 `selectedEdge` state + useEffect 追踪 `edges.filter(e => e.selected)`(仿节点选择)。
- 加 `updateEdgeData(key, value)`(更新 data,`key==='condition'` 时同步 label)。
- 右侧面板:`selectedEdge` 且无 `selectedNode` 时显示边属性:Source→Target(只读)+ 当源节点是 condition 时显示 "Branch" Select(none/true/false)。节点/边选择互斥(点边清节点)。
- 边 label 由 ReactFlow 默认渲染(设了就显示)。

### 部署
- 重建+重部署 `harness-service`(后端)。先 `docker system prune -af && builder prune -af`(磁盘 69%,防 build 再撑爆)。**校验容器 CREATED 时间 + build 输出无 "failed to solve",别信 exit code**。
- 前端源码 tar+ssh 同步到服务器,`docker compose build frontend && up -d frontend`(deploy-remote.sh 不同步前端)。
- 重跑集成测试:(a) 扁平格式+from/to 边的 workflow 能存能跑,node_name 有值;(b) condition workflow 按输入路由到正确分支;(c) UI 连 true/false handle 后存盘再执行路由正确。

## 范围外/可选
- 网关 `ExecuteWorkflow` 响应没映射 `retries`(P0 把它放进 pb 但网关丢弃)。次要,需要时加 `retries` 映射 + 重建 gateway。
- entry 节点恒为 `nodes[0]`(无 UI 设置);既有问题,不在本次范围。
- `services/harness-service/config.yaml` 明文 LLM api_key(既有);建议另行改环境变量注入,单独处理。

## 改动文件
- `services/harness-service/internal/workflow/engine.go`(结构体 + modelToWorkflow)
- `services/harness-service/internal/workflow/engine_test.go`(测试)
- `frontend/src/pages/Workflow/index.tsx`(onConnect + 边选择 + 边属性面板)

## 顺序
1. 后端 engine.go + 测试 → 本地 `go test -race` → 重建+重部署 harness-service。
2. API 验证:(a) 扁平格式存+跑;(b) condition 路由(curl 用 from/to + condition)。
3. 前端 index.tsx → 本地 tsc → 同步+重建 frontend。
4. UI 端到端(连线 true/false → 存盘 → 执行 → 路由正确)。
