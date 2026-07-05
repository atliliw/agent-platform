// 评测报告 API 服务
import client from './client';
import type {
  EvalSuite,
  EvalReport,
  RunEvalConfig,
  ComparisonReport,
  Baseline,
} from './evaluation';

// 获取评测套件列表
export async function getEvalSuites(): Promise<EvalSuite[]> {
  return client.get('/api/v2/eval/suites');
}

// 获取评测套件详情
export async function getEvalSuite(suiteId: string): Promise<EvalSuite> {
  return client.get(`/api/v2/eval/suites/${suiteId}`);
}

// 创建评测套件
export async function createEvalSuite(data: {
  name: string;
  description?: string;
  test_cases: unknown[];
}): Promise<EvalSuite> {
  return client.post('/api/v2/eval/suites', data);
}

// 运行评测
export async function runEvaluation(suiteId: string, config: RunEvalConfig): Promise<EvalReport> {
  return client.post(`/api/v2/eval/suites/${suiteId}/run`, config);
}

// 获取评测结果
export async function getEvalResults(suiteId: string): Promise<EvalReport> {
  return client.get(`/api/v2/eval/suites/${suiteId}/results`);
}

// 获取历史评测结果
export async function getEvalHistory(suiteId: string, params?: {
  limit?: number;
  start_time?: string;
  end_time?: string;
}): Promise<EvalReport[]> {
  return client.get(`/api/v2/harness/eval/history/${suiteId}`, { params });
}

// 对比评测结果
export async function compareEvalResults(params: {
  baseline_suite_id: string;
  current_suite_id: string;
}): Promise<ComparisonReport> {
  return client.post('/api/v2/harness/eval/compare', params);
}

// 获取基线列表
export async function getBaselines(suiteId?: string): Promise<Baseline[]> {
  return client.get('/api/v2/harness/eval/baselines', { params: { suite_id: suiteId } });
}

// 设置基线
export async function setBaseline(suiteId: string, data: {
  name: string;
  report_id: string;
}): Promise<Baseline> {
  return client.post(`/api/v2/harness/eval/baselines/${suiteId}`, data);
}

// 删除基线
export async function deleteBaseline(baselineId: string): Promise<void> {
  return client.delete(`/api/v2/harness/eval/baselines/${baselineId}`);
}

// 导出评测报告
export function exportEvalReport(suiteId: string, format: 'json' | 'html' | 'pdf' = 'json'): string {
  const baseUrl = import.meta.env.VITE_API_URL || '';
  return `${baseUrl}/api/v2/harness/eval/export/${suiteId}?format=${format}`;
}

// 获取回归检测结果
export async function getRegressions(suiteId: string): Promise<{
  has_regressions: boolean;
  regressions: unknown[];
}> {
  return client.get(`/api/v2/harness/eval/regressions/${suiteId}`);
}

// 获取 LLM-as-Judge 配置
export async function getJudgeConfig(): Promise<unknown> {
  return client.get('/api/v2/harness/eval/judge/config');
}

// 更新 LLM-as-Judge 配置
export async function updateJudgeConfig(config: unknown): Promise<void> {
  return client.put('/api/v2/harness/eval/judge/config', config);
}

export const evaluationApi = {
  getEvalSuites,
  getEvalSuite,
  createEvalSuite,
  runEvaluation,
  getEvalResults,
  getEvalHistory,
  compareEvalResults,
  getBaselines,
  setBaseline,
  deleteBaseline,
  exportEvalReport,
  getRegressions,
  getJudgeConfig,
  updateJudgeConfig,
};
