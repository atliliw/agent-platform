// 可观测性 API 类型定义

// Span 表示执行中的一个操作单元
export interface Span {
  id: string;
  trace_id: string;
  parent_id?: string;
  operation: string;
  status: 'ok' | 'error' | 'blocked' | 'pending';
  started_at: string;
  ended_at?: string;
  duration_ms: number;
  attributes?: Record<string, unknown>;
  events?: SpanEvent[];
  error_msg?: string;
}

export interface SpanEvent {
  name: string;
  timestamp: string;
  attributes?: Record<string, unknown>;
}

// Trace 表示一次完整的执行追踪
export interface Trace {
  trace_id: string;
  session_id?: string;
  agent_id?: string;
  operation: string;
  status: 'ok' | 'error' | 'blocked' | 'pending';
  started_at: string;
  ended_at?: string;
  latency_ms: number;
  tokens: number;
  cost: number;
  spans: Span[];
  bottlenecks?: Bottleneck[];
  metadata?: Record<string, unknown>;
}

export interface Bottleneck {
  operation: string;
  duration: number;
  percent: number;
  severity: 'high' | 'medium' | 'low';
  suggestion?: string;
}

// 追踪列表查询参数
export interface TraceQueryParams {
  page?: number;
  size?: number;
  session_id?: string;
  agent_id?: string;
  status?: string;
  start_time?: string;
  end_time?: string;
  search?: string;
}

// 追踪列表响应
export interface TraceListResponse {
  traces: Trace[];
  total: number;
  page: number;
  size: number;
  success_rate: number;
  avg_latency: number;
  total_tokens: number;
  total_cost: number;
}

// 性能指标
export interface PerformanceMetrics {
  total_traces: number;
  success_rate: number;
  avg_latency_ms: number;
  p50_latency_ms: number;
  p95_latency_ms: number;
  p99_latency_ms: number;
  total_tokens: number;
  total_cost: number;
  traces_by_status: Record<string, number>;
  traces_by_agent: AgentMetric[];
  traces_by_operation: OperationMetric[];
}

export interface AgentMetric {
  agent_id: string;
  count: number;
  avg_latency_ms: number;
  success_rate: number;
  total_tokens: number;
  total_cost: number;
}

export interface OperationMetric {
  operation: string;
  count: number;
  avg_latency_ms: number;
  success_rate: number;
}

// 性能分析结果
export interface PerformanceProfile {
  session_id: string;
  trace_id: string;
  total_duration_ms: number;
  critical_path: Span[];
  bottlenecks: Bottleneck[];
  time_distribution: TimeDistribution[];
  recommendations: string[];
}

export interface TimeDistribution {
  category: string;
  duration_ms: number;
  percent: number;
}

// 统计数据
export interface TraceStats {
  total_traces: number;
  success_traces: number;
  error_traces: number;
  blocked_traces: number;
  success_rate: number;
  avg_latency_ms: number;
  total_tokens: number;
  total_cost: number;
  traces_today: number;
  cost_today: number;
}
