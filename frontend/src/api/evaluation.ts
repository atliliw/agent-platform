// 评测报告 API 类型定义

// 评测套件
export interface EvalSuite {
  id: string;
  name: string;
  description: string;
  test_count: number;
  created_at: string;
  updated_at: string;
}

// 评测报告
export interface EvalReport {
  suite_id: string;
  suite_name: string;
  avg_score: number;
  total_tests: number;
  passed_tests: number;
  failed_tests: number;
  duration_ms: number;
  tokens_used: number;
  cost: number;
  score_by_category: Record<string, number>;
  results: EvalResult[];
  regressions?: Regression[];
  metrics_summary?: MetricsSummary;
  created_at: string;
}

// 评测结果
export interface EvalResult {
  id: string;
  name: string;
  category: string;
  score: number;
  passed: boolean;
  score_details: ScoreDetails;
  duration_ms: number;
  steps: number;
  tokens: number;
  cost: number;
  output?: string;
  expected?: string;
  tool_calls?: ToolCallRecord[];
  trajectory?: TrajectoryScore;
  react_score?: ReActScore;
  error?: string;
}

// 得分明细
export interface ScoreDetails {
  faithfulness: number;
  relevancy: number;
  precision: number;
  reasoning: number;
}

// 工具调用记录
export interface ToolCallRecord {
  name: string;
  arguments: Record<string, unknown>;
  result?: string;
  success: boolean;
  duration_ms: number;
  retry_count: number;
}

// 执行路径评分
export interface TrajectoryScore {
  efficiency: number;
  redundant_steps: number;
  missing_steps: string[];
  optimal_steps: number;
  actual_steps: number;
}

// ReAct 评分
export interface ReActScore {
  reasoning_quality: number;
  action_relevance: number;
  thought_action_coherence: number;
  error_handling: number;
}

// 回归检测
export interface Regression {
  case_name: string;
  before_score: number;
  after_score: number;
  delta: number;
  severity: 'high' | 'medium' | 'low';
}

// 指标汇总
export interface MetricsSummary {
  avg_steps: number;
  avg_latency_ms: number;
  total_tool_calls: number;
  tool_success_rate: number;
  avg_tokens_per_test: number;
  avg_cost_per_test: number;
}

// 运行评测配置
export interface RunEvalConfig {
  model?: string;
  parallel?: boolean;
  evaluate_trajectory?: boolean;
  evaluate_react?: boolean;
  compare_to?: string;
  save_baseline?: boolean;
}

// 对比报告
export interface ComparisonReport {
  baseline_suite_id: string;
  current_suite_id: string;
  baseline_avg_score: number;
  current_avg_score: number;
  score_delta: number;
  improvements: EvalResult[];
  regressions: EvalResult[];
  category_deltas: Record<string, number>;
}

// LLM-as-Judge 配置
export interface JudgeConfig {
  model: string;
  criteria: string[];
  temperature: number;
  max_tokens: number;
}

// 基线
export interface Baseline {
  id: string;
  suite_id: string;
  name: string;
  avg_score: number;
  created_at: string;
  created_by: string;
}
