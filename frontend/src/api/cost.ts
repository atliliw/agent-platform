// 成本监控 API 类型定义

// 成本汇总
export interface CostSummary {
  total_cost: number;
  total_calls: number;
  total_tokens: number;
  input_tokens: number;
  output_tokens: number;
  avg_latency_ms: number;
  cost_trend: number;
  by_model: ModelCost[];
  by_agent: AgentCost[];
  by_tool: ToolCost[];
  by_date: DateCost[];
}

export interface ModelCost {
  model: string;
  calls: number;
  tokens: number;
  cost: number;
  avg_latency_ms: number;
}

export interface AgentCost {
  agent_id: string;
  calls: number;
  tokens: number;
  cost: number;
  success_rate: number;
}

export interface ToolCost {
  tool: string;
  calls: number;
  tokens: number;
  cost: number;
  success_rate: number;
}

export interface DateCost {
  date: string;
  calls: number;
  tokens: number;
  cost: number;
}

// 成本趋势数据
export interface CostTrend {
  timestamps: string[];
  costs: number[];
  tokens: number[];
  calls: number[];
}

// 预算定义
export interface Budget {
  id: string;
  name: string;
  limit: number;
  used: number;
  percent_used: number;
  period: 'daily' | 'weekly' | 'monthly';
  alert_threshold: number;
  exceeded: boolean;
  created_at: string;
  updated_at: string;
}

// 创建预算请求
export interface CreateBudgetRequest {
  name: string;
  limit: number;
  period: 'daily' | 'weekly' | 'monthly';
  alert_threshold?: number;
}

// 成本明细
export interface CostDetail {
  id: string;
  timestamp: string;
  session_id: string;
  agent_id: string;
  model: string;
  input_tokens: number;
  output_tokens: number;
  total_tokens: number;
  cost: number;
  latency_ms: number;
  success: boolean;
  operation?: string;
}

// 成本明细查询参数
export interface CostDetailQueryParams {
  page?: number;
  size?: number;
  range?: 'today' | 'week' | 'month' | 'custom';
  start_time?: string;
  end_time?: string;
  agent_id?: string;
  model?: string;
}

// 成本明细列表响应
export interface CostDetailListResponse {
  details: CostDetail[];
  total: number;
  page: number;
  size: number;
}

// 成本预警
export interface CostAlert {
  id: string;
  type: 'budget_exceeded' | 'budget_warning' | 'anomaly';
  severity: 'info' | 'warning' | 'critical';
  message: string;
  budget_id?: string;
  current_value: number;
  threshold: number;
  created_at: string;
  acknowledged: boolean;
}

// 模型价格信息
export interface ModelPricing {
  model: string;
  input_price_per_1k: number;
  output_price_per_1k: number;
  currency: string;
}
