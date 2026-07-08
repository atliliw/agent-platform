# 前端实现方案

> 本文档是 P0-P1 待实现功能的**前端部分**详细方案。后端方案见 03/04/05 号文档。

---

## 一、前端架构现状

### 技术栈
- React 18 + TypeScript + Vite
- Ant Design（UI 组件库）
- Zustand（状态管理，仅 ChatStore 使用）
- Axios（HTTP 客户端，300s 超时）
- ReactMarkdown + remarkGfm（消息渲染）

### 关键问题

| 问题 | 说明 |
|------|------|
| **SSE 流式未启用** | `chatApi.chatStream()` 已实现但 `chatStore.sendMessage()` 用的是同步 `chatApi.chat()` |
| **双 API 调用风格** | Harness/Memory 页直接 `client.get/post`，其他页面用 API 模块 |
| **响应解包混乱** | Axios 拦截器自动解包 `{code, data}`，但很多调用方仍做 `res?.data?.x \|\| res?.x` |
| **Harness 页面巨型** | 1076 行单文件，6 个 Panel 全部内联 |
| **SSE 缺 Auth** | `chatStream()` 用原生 fetch，不发 Bearer token |
| **Observability 全 mock** | TraceViewer/CostDashboard/EvalReport/MemoryManager 数据源是 mock handler |

---

## 二、P0 前端方案

### P0-1：Chat 真流式

#### 2.1.1 修改 chatStore.sendMessage

**文件**：`frontend/src/stores/chatStore.ts`

当前：
```ts
sendMessage: async (content: string) => {
  set({ isLoading: true });
  const response = await chatApi.chat(request);  // 同步等待完整响应
  set(state => ({ 
    messages: [...state.messages, assistantMsg],
    isLoading: false 
  }));
}
```

改为流式：
```ts
sendMessage: async (content: string) => {
  set({ isLoading: true, isStreaming: true });
  
  // 先添加空的 assistant 消息占位
  const assistantId = `msg-${Date.now()}`;
  set(state => ({
    messages: [...state.messages, { 
      id: assistantId, role: 'assistant', content: '',
      agent_trace: [], tool_calls: [] 
    }],
  }));
  
  try {
    await chatApi.chatStream(
      request,
      // onToken: 逐 token 追加到消息
      (chunk) => {
        set(state => {
          const msgs = [...state.messages];
          const last = msgs[msgs.length - 1];
          if (last.id === assistantId) {
            msgs[msgs.length - 1] = { 
              ...last, 
              content: last.content + chunk.content 
            };
          }
          return { messages: msgs };
        });
      },
      // onAgentEvent: Agent 生命周期事件
      (event) => {
        set(state => {
          const msgs = [...state.messages];
          const last = msgs[msgs.length - 1];
          if (last.id === assistantId) {
            switch (event.chunk_type) {
              case 'think':
                last.agent_trace = [...(last.agent_trace || []), {
                  thought: event.content, step: event.step
                }];
                break;
              case 'tool_start':
                last.tool_calls = [...(last.tool_calls || []), {
                  id: event.tool_call?.id,
                  name: event.tool_call?.name,
                  arguments: event.tool_call?.arguments,
                  status: 'running'
                }];
                break;
              case 'tool_result':
                const tc = last.tool_calls?.find(t => t.id === event.tool_call?.id);
                if (tc) { tc.result = event.tool_call?.result; tc.status = 'completed'; }
                break;
              case 'handoff':
                // 显示 Agent 转交提示
                last.content += `\n> 🔄 转交至 ${event.agent_id}\n`;
                break;
            }
          }
          return { messages: msgs };
        });
      },
      // onDone: 流式结束
      () => {
        set({ isLoading: false, isStreaming: false });
      },
      // onError
      (error) => {
        set({ isLoading: false, isStreaming: false });
        message.error(`流式响应错误: ${error.message}`);
      }
    );
  } catch (error) {
    set({ isLoading: false, isStreaming: false });
  }
}
```

#### 2.1.2 修改 chatApi.chatStream

**文件**：`frontend/src/api/chat.ts`

当前 `chatStream` 不发 Auth token。修复：

```ts
export async function chatStream(
  request: ChatRequest,
  onToken: (chunk: StreamChunk) => void,
  onAgentEvent: (event: AgentStreamEvent) => void,
  onDone: () => void,
  onError: (error: Error) => void,
) {
  const token = localStorage.getItem('token');
  const tenantId = localStorage.getItem('tenantId') || 'default';
  
  const response = await fetch(`${baseURL}/api/v2/chat/stream`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${token}`,       // 新增
      'X-Tenant-ID': tenantId,                   // 新增
    },
    body: JSON.stringify(request),
  });
  
  const reader = response.body!.getReader();
  const decoder = new TextDecoder();
  let buffer = '';
  
  while (true) {
    const { done, value } = await reader.read();
    if (done) break;
    
    buffer += decoder.decode(value, { stream: true });
    const lines = buffer.split('\n');
    buffer = lines.pop() || '';
    
    for (const line of lines) {
      if (!line.startsWith('data: ')) continue;
      const data = line.slice(6);
      if (data === '[DONE]') { onDone(); return; }
      
      try {
        const chunk = JSON.parse(data);
        
        if (chunk.chunk_type === 'token') {
          onToken(chunk);
        } else if (['think', 'tool_start', 'tool_result', 'handoff', 'final'].includes(chunk.chunk_type)) {
          onAgentEvent(chunk);
        }
      } catch { /* ignore parse errors */ }
    }
  }
  onDone();
}
```

#### 2.1.3 ChatMessage 增量渲染

**文件**：`frontend/src/components/Chat/ChatMessage.tsx`

当前 `ReactMarkdown` 对完整内容渲染。流式时需要处理不完整的 Markdown：

```tsx
// 流式进行中时，对不完整的代码块做容错
function StreamingMarkdown({ content, isStreaming }: { content: string; isStreaming: boolean }) {
  // 如果流式进行中且内容以 ``` 开头但没有闭合，补上闭合
  let safeContent = content;
  if (isStreaming) {
    const codeBlockCount = (content.match(/```/g) || []).length;
    if (codeBlockCount % 2 !== 0) {
      safeContent = content + '\n```';  // 临时闭合
    }
  }
  
  return <ReactMarkdown remarkPlugins={[remarkGfm]}>{safeContent}</ReactMarkdown>;
}
```

#### 2.1.4 流式状态指示器

在 ChatInput 上方添加实时状态栏：

```tsx
{isStreaming && (
  <div style={{ padding: '8px 16px', background: '#f6f6f6', borderBottom: '1px solid #eee' }}>
    <Space>
      <Spin size="small" />
      <span>{thinkingAgent ? `${thinkingAgent} 正在思考...` : '正在生成...'}</span>
      {activeToolCall && (
        <Tag color="blue">🔧 {activeToolCall}</Tag>
      )}
    </Space>
  </div>
)}
```

#### 关键文件

| 文件 | 改动 |
|------|------|
| `frontend/src/stores/chatStore.ts` | 重写 `sendMessage` 为流式 |
| `frontend/src/api/chat.ts` | 重写 `chatStream`：添加 Auth + chunk_type 分发 |
| `frontend/src/components/Chat/ChatMessage.tsx` | 添加 `StreamingMarkdown` + 流式状态 |
| `frontend/src/pages/Chat/index.tsx` | 添加流式状态指示器 |

---

### P0-2：Knowledge 页面修复

Knowledge 页面 UI 已存在，只需后端接线（见 03 号文档）。前端无需改动。

---

### P0-3：审批 UI

#### 2.3.1 审批通知组件

**新文件**：`frontend/src/components/Chat/ApprovalNotification.tsx`

```tsx
interface ApprovalNotificationProps {
  request: ApprovalRequest;
  onApprove: (id: string, modifiedParams?: Record<string, any>) => void;
  onReject: (id: string, reason: string) => void;
}

export function ApprovalNotification({ request, onApprove, onReject }: ApprovalNotificationProps) {
  const [reason, setReason] = useState('');
  const [showReject, setShowReject] = useState(false);
  
  return (
    <Card 
      style={{ borderColor: '#faad14', marginBottom: 8 }}
      size="small"
    >
      <Space direction="vertical" style={{ width: '100%' }}>
        <Space>
          <Tag color="warning">⚠️ 等待审批</Tag>
          <span>Agent <b>{request.agent_id}</b> 请求执行 <b>{request.tool_name}</b></span>
        </Space>
        
        <div style={{ background: '#f5f5f5', padding: 8, borderRadius: 4, fontSize: 12 }}>
          <pre>{JSON.stringify(request.details, null, 2)}</pre>
        </div>
        
        {showReject ? (
          <Space>
            <Input 
              placeholder="拒绝原因" 
              value={reason} 
              onChange={e => setReason(e.target.value)} 
              style={{ width: 300 }}
            />
            <Button danger onClick={() => onReject(request.id, reason)}>确认拒绝</Button>
            <Button onClick={() => setShowReject(false)}>取消</Button>
          </Space>
        ) : (
          <Space>
            <Button type="primary" onClick={() => onApprove(request.id)}>批准</Button>
            <Button danger onClick={() => setShowReject(true)}>拒绝</Button>
          </Space>
        )}
      </Space>
    </Card>
  );
}
```

#### 2.3.2 Chat 页面集成审批

**文件**：`frontend/src/pages/Chat/index.tsx`

在消息列表中插入审批卡片：

```tsx
// 在 assistant 消息的 tool_calls 中，如果有 status === 'pending_approval' 的
{message.tool_calls?.filter(tc => tc.status === 'pending_approval').map(tc => (
  <ApprovalNotification
    key={tc.id}
    request={{
      id: tc.id,
      agent_id: message.agent_id,
      tool_name: tc.name,
      details: tc.arguments,
    }}
    onApprove={(id) => client.post(`/api/v2/harness/approval/${id}/approve`)}
    onReject={(id, reason) => client.post(`/api/v2/harness/approval/${id}/reject`, { reason })}
  />
))}
```

#### 2.3.3 Harness 页面审批管理 Tab

**文件**：`frontend/src/pages/Harness/index.tsx`

新增第 7 个 Tab "审批管理"：

```tsx
{
  key: 'approval',
  label: '审批管理',
  children: <ApprovalPanel />,
}
```

**新文件**：`frontend/src/pages/Harness/ApprovalPanel.tsx`

```tsx
export function ApprovalPanel() {
  const [rules, setRules] = useState([]);
  const [pending, setPending] = useState([]);
  
  // 加载审批规则
  useEffect(() => {
    client.get('/api/v2/harness/approval/rules').then(res => setRules(res || []));
  }, []);
  
  // 加载待审批列表
  useEffect(() => {
    const timer = setInterval(() => {
      client.get('/api/v2/harness/approval/pending').then(res => setPending(res || []));
    }, 3000);  // 3 秒轮询
    return () => clearInterval(timer);
  }, []);
  
  return (
    <Row gutter={16}>
      <Col span={12}>
        <Card title="审批规则">
          <Table dataSource={rules} columns={[
            { title: '工具', dataIndex: 'tool_name' },
            { title: '风险阈值', dataIndex: 'risk_threshold' },
            { title: '自动批准', dataIndex: 'auto_approve', render: v => v ? '✅' : '❌' },
            { title: '启用', dataIndex: 'enabled', render: v => <Switch checked={v} /> },
          ]} />
        </Card>
      </Col>
      <Col span={12}>
        <Card title={`待审批 (${pending.length})`}>
          {pending.map(req => (
            <ApprovalNotification key={req.id} request={req} ... />
          ))}
        </Card>
      </Col>
    </Row>
  );
}
```

#### 2.3.4 审批 API 模块

**新文件**：`frontend/src/api/approval.ts`

```ts
import client from './client';

export const approvalApi = {
  listRules: () => client.get('/api/v2/harness/approval/rules'),
  createRule: (rule: any) => client.post('/api/v2/harness/approval/rules', rule),
  listPending: () => client.get('/api/v2/harness/approval/pending'),
  approve: (id: string, modifiedParams?: any) => 
    client.post(`/api/v2/harness/approval/${id}/approve`, { modified_params: modifiedParams }),
  reject: (id: string, reason: string) => 
    client.post(`/api/v2/harness/approval/${id}/reject`, { reason }),
};
```

---

### P0-4：Observability 页面真实数据

#### 2.4.1 TraceViewer 替换 mock

**文件**：`frontend/src/pages/Observability/TraceViewer.tsx`

当前从 `observabilityApi.getTraces()` 获取 mock 数据。改为：

```ts
// 如果 OTEL 已部署，从真实 API 获取
const loadTraces = async () => {
  setLoading(true);
  try {
    // 尝试从真实 trace API 获取
    const data = await client.get('/api/v2/observability/traces', {
      params: { limit: 50, service: selectedService }
    });
    setTraces(data?.traces || []);
  } catch {
    // 降级：从 harness LLM metrics 构造 trace-like 数据
    try {
      const metrics = await client.get('/api/v2/harness/llm/metrics', {
        params: { limit: 50 }
      });
      setTraces(convertMetricsToTraces(metrics || []));
    } catch {
      setTraces([]);
    }
  } finally {
    setLoading(false);
  }
};

// 将 LLM metrics 转为 trace 格式
function convertMetricsToTraces(metrics: any[]): Trace[] {
  return metrics.map(m => ({
    id: m.trace_id || m.id,
    name: m.model || 'llm_call',
    duration: m.latency_ms,
    status: m.success ? 'ok' : 'error',
    startTime: m.timestamp * 1000,
    attributes: {
      model: m.model,
      tokens_in: m.input_tokens,
      tokens_out: m.output_tokens,
      cost: m.cost,
    },
    spans: [{
      id: m.id,
      name: `${m.caller || 'unknown'} → ${m.model}`,
      duration: m.latency_ms,
      status: m.success ? 'ok' : 'error',
    }],
  }));
}
```

#### 2.4.2 Trace 详情页

```tsx
// 点击 trace 展开详情
const TraceDetail = ({ trace }: { trace: Trace }) => (
  <Card size="small" style={{ marginTop: 8 }}>
    <Timeline items={trace.spans?.map(span => ({
      color: span.status === 'ok' ? 'green' : 'red',
      children: (
        <Space direction="vertical">
          <Space>
            <Tag>{span.name}</Tag>
            <span>{span.duration?.toFixed(0)}ms</span>
          </Space>
          {span.attributes && Object.entries(span.attributes).map(([k, v]) => (
            <div key={k} style={{ fontSize: 12, color: '#666' }}>{k}: {String(v)}</div>
          ))}
        </Space>
      ),
    }))} />
  </Card>
);
```

#### 2.4.3 CostDashboard 替换 mock

**文件**：`frontend/src/pages/Observability/CostDashboard.tsx`

改为调用 harness-service 的真实 cost API：

```ts
const loadCostData = async () => {
  try {
    const [report, pricing] = await Promise.all([
      client.get('/api/v2/harness/cost/report'),
      client.get('/api/v2/harness/cost/pricing'),
    ]);
    setCostReport(report);
    setPricing(pricing);
  } catch {
    // 降级到独立 cost API
    const summary = await costApi.getSummary();
    setCostReport(summary);
  }
};
```

---

## 三、P1 前端方案

### P1-1：Workflow 可视化编辑器

#### 3.1.1 新建 Workflow 页面

**新文件**：`frontend/src/pages/Workflow/index.tsx`

使用 React Flow 库（`reactflow`）实现拖拽式 DAG 编辑器：

```tsx
import ReactFlow, { 
  Node, Edge, addEdge, removeElements, 
  Controls, MiniMap, Background 
} from 'reactflow';
import 'reactflow/dist/style.css';

export function WorkflowEditor() {
  const [nodes, setNodes, onNodesChange] = useNodesState(initialNodes);
  const [edges, setEdges, onEdgesChange] = useEdgesState(initialEdges);
  
  const onConnect = useCallback((params) => 
    setEdges(eds => addEdge({ ...params, animated: true }, eds)), 
    [setEdges]
  );
  
  // 自定义节点类型
  const nodeTypes = useMemo(() => ({
    agent: AgentNode,
    tool: ToolNode,
    condition: ConditionNode,
    parallel: ParallelNode,
    merge: MergeNode,
  }), []);
  
  return (
    <div style={{ height: '100vh' }}>
      <ReactFlow
        nodes={nodes}
        edges={edges}
        onNodesChange={onNodesChange}
        onEdgesChange={onEdgesChange}
        onConnect={onConnect}
        nodeTypes={nodeTypes}
        fitView
      >
        <Controls />
        <MiniMap />
        <Background variant="dots" gap={12} size={1} />
      </ReactFlow>
    </div>
  );
}
```

#### 3.1.2 自定义节点组件

**新文件**：`frontend/src/pages/Workflow/nodes/`

```tsx
// AgentNode.tsx
export function AgentNode({ data }: NodeProps) {
  return (
    <div style={{ 
      padding: '8px 16px', borderRadius: 8, 
      background: '#e6f7ff', border: '2px solid #1890ff' 
    }}>
      <Space>
        <span>🤖</span>
        <b>{data.label}</b>
      </Space>
      <div style={{ fontSize: 12, color: '#666' }}>
        {data.agentId}
      </div>
      <Handle type="target" position={Position.Top} />
      <Handle type="source" position={Position.Bottom} />
    </div>
  );
}

// ConditionNode.tsx
export function ConditionNode({ data }: NodeProps) {
  return (
    <div style={{ 
      padding: '8px 16px', borderRadius: 8, 
      background: '#fff7e6', border: '2px solid #faad14' 
    }}>
      <Space>
        <span>🔀</span>
        <b>{data.label}</b>
      </Space>
      <div style={{ fontSize: 12, color: '#666' }}>
        {data.condition}
      </div>
      <Handle type="target" position={Position.Top} />
      <Handle type="source" position={Position.Bottom} id="yes" style={{ left: '25%' }} />
      <Handle type="source" position={Position.Bottom} id="no" style={{ left: '75%' }} />
    </div>
  );
}

// ParallelNode.tsx
export function ParallelNode({ data }: NodeProps) {
  return (
    <div style={{ 
      padding: '8px 16px', borderRadius: 8, 
      background: '#f6ffed', border: '2px solid #52c41a' 
    }}>
      <Space>
        <span>⚡</span>
        <b>并行执行</b>
      </Space>
      <div style={{ fontSize: 12, color: '#666' }}>
        {data.branches?.length} 个分支
      </div>
      <Handle type="target" position={Position.Top} />
      <Handle type="source" position={Position.Bottom} />
    </div>
  );
}
```

#### 3.1.3 节点面板 + 属性面板

```tsx
// 左侧：可拖拽的节点类型列表
const nodePanelItems = [
  { type: 'agent', label: 'Agent', icon: '🤖' },
  { type: 'tool', label: '工具', icon: '🔧' },
  { type: 'condition', label: '条件', icon: '🔀' },
  { type: 'parallel', label: '并行', icon: '⚡' },
  { type: 'merge', label: '合并', icon: '🔗' },
];

// 右侧：选中节点的属性编辑
function PropertyPanel({ selectedNode, agents, tools, onUpdate }) {
  if (!selectedNode) return <Empty description="选择节点编辑属性" />;
  
  switch (selectedNode.type) {
    case 'agent':
      return (
        <Form layout="vertical">
          <Form.Item label="Agent">
            <Select options={agents.map(a => ({ label: a.name, value: a.id }))} />
          </Form.Item>
        </Form>
      );
    case 'condition':
      return (
        <Form layout="vertical">
          <Form.Item label="条件表达式">
            <Input.TextArea placeholder='last_result.contains("技术")' />
          </Form.Item>
        </Form>
      );
  }
}
```

#### 3.1.4 Workflow API 模块

**新文件**：`frontend/src/api/workflow.ts`

```ts
import client from './client';

export interface Workflow {
  id: string;
  name: string;
  nodes: WorkflowNode[];
  edges: WorkflowEdge[];
  entry_node_id: string;
}

export const workflowApi = {
  list: () => client.get('/api/v2/harness/workflows'),
  get: (id: string) => client.get(`/api/v2/harness/workflows/${id}`),
  save: (wf: Workflow) => client.post('/api/v2/harness/workflows', wf),
  delete: (id: string) => client.delete(`/api/v2/harness/workflows/${id}`),
  execute: (id: string, input: string) => client.post(`/api/v2/harness/workflows/${id}/execute`, { input }),
};
```

#### 3.1.5 添加路由

**文件**：`frontend/src/App.tsx`

```tsx
import WorkflowEditor from './pages/Workflow';

// 在路由中添加
<Route path="/workflow" element={<WorkflowEditor />} />
<Route path="/workflow/:id" element={<WorkflowEditor />} />
```

#### 3.1.6 安装依赖

```bash
npm install reactflow
```

---

### P1-2：Checkpoint 时间线 UI

#### 3.2.1 Session Replay 页面添加 Checkpoint

**文件**：`frontend/src/pages/Session/Replay.tsx`

在播放控制条下方添加 Checkpoint 标记：

```tsx
const [checkpoints, setCheckpoints] = useState<Checkpoint[]>([]);

// 加载 Checkpoint 列表
useEffect(() => {
  if (sessionId) {
    client.get(`/api/v2/harness/session/${sessionId}/checkpoints`)
      .then(res => setCheckpoints(res?.checkpoints || []));
  }
}, [sessionId]);

// 在 Progress bar 上叠加 Checkpoint 标记
<div style={{ position: 'relative', width: 150 }}>
  <Progress percent={progress} showInfo={false} />
  {checkpoints.map(cp => {
    const pos = (cp.step / steps.length) * 100;
    return (
      <Tooltip key={cp.id} title={`Step ${cp.step} - ${dayjs(cp.created_at).format('HH:mm:ss')}`}>
        <div
          style={{
            position: 'absolute', left: `${pos}%`, top: -4,
            width: 8, height: 8, borderRadius: '50%',
            background: '#722ed1', cursor: 'pointer',
          }}
          onClick={() => {
            // 从此 Checkpoint 恢复
            Modal.confirm({
              title: `从 Step ${cp.step} 恢复执行？`,
              onOk: () => client.post(`/api/v2/harness/session/${sessionId}/checkpoint/${cp.id}/resume`),
            });
          }}
        />
      </Tooltip>
    );
  })}
</div>
```

---

### P1-3：分层记忆 UI

#### 3.3.1 Memory 页面扩展

**文件**：`frontend/src/pages/Memory/index.tsx`

当前只有基础 CRUD（保存/召回/列表/删除）。扩展为 4 个 Tab：

```tsx
const memoryTabs = [
  {
    key: 'basic',
    label: '基础记忆',
    children: <BasicMemoryPanel />,  // 现有功能
  },
  {
    key: 'episodic',
    label: '情节记忆',
    children: <EpisodicMemoryPanel />,
  },
  {
    key: 'semantic',
    label: '语义图谱',
    children: <SemanticGraphPanel />,
  },
  {
    key: 'working',
    label: '工作记忆',
    children: <WorkingMemoryPanel />,
  },
];
```

#### 3.3.2 情节记忆面板

**新文件**：`frontend/src/pages/Memory/EpisodicMemoryPanel.tsx`

```tsx
export function EpisodicMemoryPanel({ sessionId }: { sessionId?: string }) {
  const [episodes, setEpisodes] = useState([]);
  const [timeline, setTimeline] = useState(null);
  
  // 加载时间线
  useEffect(() => {
    if (sessionId) {
      client.get(`/api/v2/memory/episodic`, { params: { session_id: sessionId } })
        .then(res => setEpisodes(res?.episodes || []));
      client.get(`/api/v2/memory/timeline`, { params: { session_id: sessionId } })
        .then(res => setTimeline(res));
    }
  }, [sessionId]);
  
  return (
    <Row gutter={16}>
      <Col span={16}>
        <Card title="事件列表">
          <Timeline items={episodes.map(ep => ({
            color: ep.importance > 0.7 ? 'red' : ep.importance > 0.4 ? 'blue' : 'gray',
            children: (
              <Space direction="vertical">
                <Space>
                  <Tag>{ep.type}</Tag>
                  <b>{ep.title}</b>
                </Space>
                <div style={{ fontSize: 12 }}>{ep.description}</div>
                <div style={{ fontSize: 11, color: '#999' }}>
                  {dayjs(ep.start_time).format('HH:mm')} - {dayjs(ep.end_time).format('HH:mm')}
                  {ep.outcome && ` | 结果: ${ep.outcome}`}
                </div>
              </Space>
            ),
          }))} />
        </Card>
      </Col>
      <Col span={8}>
        <Card title="统计">
          {timeline?.stats && (
            <Descriptions column={1} size="small">
              <Descriptions.Item label="总事件数">{timeline.stats.total_episodes}</Descriptions.Item>
              <Descriptions.Item label="成功率">{(timeline.stats.success_rate * 100).toFixed(0)}%</Descriptions.Item>
              <Descriptions.Item label="平均重要性">{timeline.stats.avg_importance?.toFixed(2)}</Descriptions.Item>
            </Descriptions>
          )}
        </Card>
      </Col>
    </Row>
  );
}
```

#### 3.3.3 语义图谱面板

**新文件**：`frontend/src/pages/Memory/SemanticGraphPanel.tsx`

用 React Flow 渲染知识图谱：

```tsx
export function SemanticGraphPanel() {
  const [concepts, setConcepts] = useState([]);
  const [relations, setRelations] = useState([]);
  
  useEffect(() => {
    client.get('/api/v2/memory/semantic').then(res => {
      setConcepts(res?.concepts || []);
      setRelations(res?.relations || []);
    });
  }, []);
  
  // 将 Concept + Relation 转为 React Flow nodes + edges
  const nodes = concepts.map((c, i) => ({
    id: c.id,
    type: 'default',
    data: { label: `${conceptIcon(c.type)} ${c.name}` },
    position: { x: i * 120, y: Math.random() * 200 },  // 简单布局
  }));
  
  const edges = relations.map(r => ({
    id: r.id,
    source: r.from_concept,
    target: r.to_concept,
    label: r.type,
    animated: r.type === 'causes',
  }));
  
  return (
    <div style={{ height: 500 }}>
      <ReactFlow nodes={nodes} edges={edges} fitView>
        <Controls />
        <MiniMap />
      </ReactFlow>
    </div>
  );
}

function conceptIcon(type: string) {
  const icons: Record<string, string> = {
    entity: '🏷️', fact: '📌', rule: '📜', procedure: '⚙️', concept: '💡'
  };
  return icons[type] || '📋';
}
```

#### 3.3.4 工作记忆面板

**新文件**：`frontend/src/pages/Memory/WorkingMemoryPanel.tsx`

```tsx
export function WorkingMemoryPanel({ sessionId }: { sessionId?: string }) {
  const [context, setContext] = useState(null);
  const [messages, setMessages] = useState([]);
  
  useEffect(() => {
    if (sessionId) {
      client.get(`/api/v2/memory/working/${sessionId}`).then(res => {
        setContext(res);
        setMessages(res?.messages || []);
      });
    }
  }, [sessionId]);
  
  return (
    <Space direction="vertical" style={{ width: '100%' }} size={16}>
      {/* Token 使用进度 */}
      {context && (
        <Card size="small">
          <Space>
            <span>Token 使用:</span>
            <Progress 
              percent={Math.round((context.total_tokens / context.max_tokens) * 100)} 
              style={{ width: 200 }}
            />
            <span style={{ fontSize: 12 }}>{context.total_tokens} / {context.max_tokens}</span>
          </Space>
        </Card>
      )}
      
      {/* 消息列表 */}
      <Card title="上下文消息">
        <List dataSource={messages} renderItem={(msg: any) => (
          <List.Item>
            <Space>
              <Tag color={msg.is_key ? 'red' : 'default'}>{msg.is_key ? '🔑' : ''} {msg.role}</Tag>
              <span style={{ fontSize: 12 }}>{truncate(msg.content, 100)}</span>
              <Tag>{msg.tokens} tokens</Tag>
              <Tag color={msg.importance > 0.7 ? 'red' : 'default'}>
                重要性: {msg.importance?.toFixed(2)}
              </Tag>
            </Space>
          </List.Item>
        )} />
      </Card>
    </Space>
  );
}
```

---

### P1-4：Harness 页面拆分

#### 3.4.1 拆分巨型文件

当前 `Harness/index.tsx` 1076 行，6 个 Panel 全部内联。拆分为独立组件：

**新文件**：`frontend/src/pages/Harness/`

```
Harness/
  index.tsx          — 主页面 + Tab 容器（~100 行）
  DashboardPanel.tsx — 概览面板
  RulesPanel.tsx     — 规则引擎
  ABTestPanel.tsx    — A/B 测试
  SLOPanel.tsx       — SLO 监控
  CostPanel.tsx      — 成本分析
  ProposalsPanel.tsx — 自演化
  ApprovalPanel.tsx  — 审批管理（新增）
```

**主页面**：

```tsx
import DashboardPanel from './DashboardPanel';
import RulesPanel from './RulesPanel';
// ...

export default function HarnessPage() {
  const tabItems = [
    { key: 'dashboard', label: '概览', children: <DashboardPanel /> },
    { key: 'rules', label: '规则引擎', children: <RulesPanel /> },
    { key: 'abtest', label: 'A/B 测试', children: <ABTestPanel /> },
    { key: 'slo', label: 'SLO 监控', children: <SLOPanel /> },
    { key: 'cost', label: '成本', children: <CostPanel /> },
    { key: 'proposals', label: '自演化', children: <ProposalsPanel /> },
    { key: 'approval', label: '审批管理', children: <ApprovalPanel /> },
  ];
  
  return (
    <div>
      <h2>Harness 治理中心</h2>
      <Tabs items={tabItems} />
    </div>
  );
}
```

#### 3.4.2 统一 API 调用

**新文件**：`frontend/src/api/harness.ts`

将所有 Harness 面板的直接 `client` 调用统一到 API 模块：

```ts
import client from './client';

export const harnessApi = {
  // Rules
  listRules: () => client.get('/api/v2/harness/rules'),
  createRule: (rule: any) => client.post('/api/v2/harness/rules', rule),
  deleteRule: (id: string) => client.delete(`/api/v2/harness/rules/${id}`),
  checkGuardrail: (message: string) => client.post('/api/v2/harness/guardrail/check', { message }),
  
  // AB Tests
  listABTests: () => client.get('/api/v2/harness/abtest/list'),
  createABTest: (test: any) => client.post('/api/v2/harness/abtest', test),
  deleteABTest: (id: string) => client.delete(`/api/v2/harness/abtest/${id}`),
  getABTestResult: (id: string) => client.get(`/api/v2/harness/abtest/${id}/result`),
  
  // SLO
  getSLOStatus: () => client.get('/api/v2/harness/slo/status'),
  createSLO: (slo: any) => client.post('/api/v2/harness/slo', slo),
  
  // Cost
  getCostReport: () => client.get('/api/v2/harness/cost/report'),
  getPricing: () => client.get('/api/v2/harness/cost/pricing'),
  updatePricing: (pricing: any) => client.post('/api/v2/harness/cost/pricing', pricing),
  getRecommendations: () => client.get('/api/v2/harness/cost/recommendations'),
  recordUsage: (usage: any) => client.post('/api/v2/harness/cost/usage', usage),
  
  // Proposals
  listProposals: () => client.get('/api/v2/harness/proposals'),
  createProposal: (proposal: any) => client.post('/api/v2/harness/proposals', proposal),
  approveProposal: (id: string) => client.post(`/api/v2/harness/proposals/${id}/approve`),
  rejectProposal: (id: string) => client.post(`/api/v2/harness/proposals/${id}/reject`),
  executeProposal: (id: string) => client.post(`/api/v2/harness/proposals/${id}/execute`),
  analyzeProposals: () => client.post('/api/v2/harness/proposals/analyze'),
  
  // LLM Metrics
  getLLMMetrics: (params?: any) => client.get('/api/v2/harness/llm/metrics', { params }),
};
```

---

## 四、前端改动文件总览

| 优先级 | 文件 | 改动类型 |
|--------|------|---------|
| **P0** | `frontend/src/stores/chatStore.ts` | 重写 sendMessage 为流式 |
| **P0** | `frontend/src/api/chat.ts` | 重写 chatStream（Auth + chunk_type） |
| **P0** | `frontend/src/components/Chat/ChatMessage.tsx` | StreamingMarkdown + 流式状态 |
| **P0** | `frontend/src/pages/Chat/index.tsx` | 流式状态指示器 |
| **P0** | `frontend/src/components/Chat/ApprovalNotification.tsx` | 新建 |
| **P0** | `frontend/src/api/approval.ts` | 新建 |
| **P0** | `frontend/src/pages/Observability/TraceViewer.tsx` | 真实数据替换 mock |
| **P0** | `frontend/src/pages/Observability/CostDashboard.tsx` | harness cost API 替换 mock |
| **P1** | `frontend/src/pages/Workflow/index.tsx` | 新建（React Flow DAG 编辑器） |
| **P1** | `frontend/src/pages/Workflow/nodes/*.tsx` | 新建（自定义节点） |
| **P1** | `frontend/src/api/workflow.ts` | 新建 |
| **P1** | `frontend/src/pages/Session/Replay.tsx` | Checkpoint 标记 + 恢复 |
| **P1** | `frontend/src/pages/Memory/EpisodicMemoryPanel.tsx` | 新建 |
| **P1** | `frontend/src/pages/Memory/SemanticGraphPanel.tsx` | 新建 |
| **P1** | `frontend/src/pages/Memory/WorkingMemoryPanel.tsx` | 新建 |
| **P1** | `frontend/src/pages/Memory/index.tsx` | 添加 4 Tab 布局 |
| **P1** | `frontend/src/pages/Harness/index.tsx` | 拆分为独立 Panel 组件 |
| **P1** | `frontend/src/pages/Harness/*.Panel.tsx` | 新建（7 个独立面板） |
| **P1** | `frontend/src/api/harness.ts` | 新建（统一 API 模块） |
| **P1** | `frontend/src/App.tsx` | 添加 /workflow 路由 |
