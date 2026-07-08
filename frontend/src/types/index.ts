// API 类型定义
export interface ApiResponse<T = unknown> {
  code: number;
  message?: string;
  data: T;
}

export interface PaginationParams {
  page?: number;
  page_size?: number;
}

export interface PaginationResponse {
  total: number;
  page: number;
  page_size: number;
}

// Chat 类型
export interface Message {
  id: string;
  role: 'user' | 'assistant' | 'system';
  content: string;
  tool_calls?: ToolCall[];
  agent_trace?: AgentState[];
  timestamp: number;
}

export interface AgentState {
  thought: string;
  action: string;
  arguments: Record<string, unknown>;
  result: string;
  step: number;
}

export interface ToolCall {
  id: string;
  name: string;
  arguments: string | Record<string, unknown>;
  result?: string;
  status: 'pending' | 'running' | 'completed' | 'error';
}

export interface Session {
  id: string;
  title: string;
  messages?: Message[];
  created_at: number;
  updated_at: number;
}

export interface ChatRequest {
  session_id?: string;
  message: string;
  model?: string;
  system_prompt?: string;
  prompt_template_key?: string;
  tool_ids?: string[];
  stream?: boolean;
  tenant_id?: string;
  user_id?: string;
}

export interface ChatResponse {
  session_id: string;
  content: string;
  tool_calls?: ToolCall[];
  agent_states?: AgentState[];  // Agent 调用轨迹
  total_tokens: number;
  cost: number;
}

export interface MultiAgentRequest {
  session_id?: string;
  message: string;
  master_agent?: string;
  tenant_id?: string;
  user_id?: string;
}

export interface AgentStep {
  agent_id: string;
  action: string;
  result: string;
}

export interface MultiAgentResponse {
  session_id: string;
  final_answer: string;
  steps: AgentStep[];
  total_tokens: number;
  cost: number;
}

// Knowledge 类型
export interface Document {
  id: string;
  filename: string;
  title?: string;
  chunk_count: number;
  status: 'ready' | 'processing' | 'error';
  created_at: string;
}

export interface Chunk {
  id: string;
  content: string;
  chunk_index: number;
  score?: number;
}

export interface SearchResult {
  chunk_id: string;
  document_id: string;
  content: string;
  score: number;
  metadata?: Record<string, unknown>;
}

export interface UploadConfig {
  chunk_strategy: 'token' | 'semantic' | 'markdown';
  chunk_size: number;
  chunk_overlap: number;
}

export interface SearchRequest {
  query: string;
  top_k?: number;
  search_type?: 'vector' | 'bm25' | 'hybrid';
  score_threshold?: number;
  filters?: Record<string, unknown>;
}

// Memory 类型
export interface Memory {
  id: string;
  session_id?: string;
  agent_id?: string;
  type: 'important' | 'summary' | 'fact';
  content: string;
  importance: number;
  created_at: string;
}

// A2A 类型
export interface AgentCard {
  id: string;
  name: string;
  description: string;
  capabilities: string[];
  input_modes: string[];
  output_modes: string[];
  url: string;
}

export interface Task {
  id: string;
  agent_id: string;
  status: 'submitted' | 'working' | 'completed' | 'failed' | 'cancelled';
  messages: Message[];
  result?: string;
  created_at: string;
  updated_at?: string;
}

// MCP 类型
export interface Tool {
  name: string;
  description: string;
  input_schema: Record<string, unknown>;
}

export interface MCPConnection {
  id: string;
  name: string;
  type: 'stdio' | 'sse' | 'websocket';
  status: 'connected' | 'disconnected';
}

// Harness 类型
export interface Rule {
  id: string;
  agent_id: string;
  name: string;
  type: 'constraint' | 'permission' | 'budget';
  config: Record<string, unknown>;
  enabled: boolean;
  created_at: string;
}

export interface GuardrailResult {
  passed: boolean;
  violations: string[];
}

export interface ABTest {
  id: string;
  name: string;
  control_model: string;
  variant_model: string;
  traffic_split: number;
  status: 'running' | 'completed' | 'paused';
}

export interface SLOStatus {
  name: string;
  current: number;
  target: number;
  budget_remaining: number;
  status: 'healthy' | 'at_risk' | 'exhausted';
}
