import client from './client';

// Agent 类型定义
export interface Agent {
  id: string;
  name: string;
  description: string;
  instructions: string;
  tools: string[];
  handoffs: string[];
  model?: string;
  max_tokens: number;
  temperature: number;
  created_at: number;
  updated_at: number;
}

// Agent 执行记录
export interface AgentExecutionRecord {
  agent_id: string;
  agent_name: string;
  thought: string;
  action: string;
  arguments: string;
  result: string;
  handoff_to?: string;
  tokens_used: number;
  started_at: number;
  completed_at: number;
  duration_ms: number;
}

// Agent 执行请求
export interface AgentExecuteRequest {
  session_id?: string;
  tenant_id?: string;
  user_id?: string;
  message: string;
  entry_agent?: string;
  context_vars?: Record<string, string>;
}

// Agent 执行响应
export interface AgentExecuteResponse {
  context_id: string;
  session_id: string;
  response: string;
  agent_history: AgentExecutionRecord[];
  total_tokens: number;
  total_cost: number;
  status: string;
  error?: string;
}

// API 响应类型
export interface ApiResponse<T> {
  code: number;
  data: T;
  error?: string;
}

// Agent API
// Note: client response interceptor already unwraps ApiResponse envelope,
// so return types reflect the inner data, not ApiResponse<T>.
export const agentApi = {
  // 获取所有 Agent
  listAgents: (): Promise<{ agents: Agent[]; pagination: { total: number } }> =>
    client.get('/api/v2/agents'),

  // 获取单个 Agent
  getAgent: (id: string): Promise<{ agent: Agent }> =>
    client.get(`/api/v2/agents/${id}`),

  // 注册 Agent
  registerAgent: (agent: Partial<Agent>): Promise<{ agent: Agent }> =>
    client.post('/api/v2/agents', agent),

  // 删除 Agent
  deleteAgent: (id: string): Promise<null> =>
    client.delete(`/api/v2/agents/${id}`),

  // 执行多 Agent
  execute: (params: AgentExecuteRequest): Promise<AgentExecuteResponse> =>
    client.post('/api/v2/agents/execute', params),

  // 获取执行上下文
  getContext: (contextId: string): Promise<{ context: unknown }> =>
    client.get(`/api/v2/agents/context/${contextId}`),
};

export default agentApi;
