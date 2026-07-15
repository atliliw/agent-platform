// 成本监控 API 服务
import client from './client';
import type {
  CostSummary,
  CostTrend,
  Budget,
  CreateBudgetRequest,
  CostDetailQueryParams,
  CostDetailListResponse,
  CostAlert,
  ModelPricing,
} from './cost';

// 获取成本汇总
export async function getCostSummary(params?: {
  range?: 'today' | 'week' | 'month' | 'custom';
  start_time?: string;
  end_time?: string;
}): Promise<CostSummary> {
  return client.get('/api/v2/cost/summary', { params });
}

// 获取成本趋势
export async function getCostTrend(params?: {
  range?: 'today' | 'week' | 'month' | 'custom';
  start_time?: string;
  end_time?: string;
  granularity?: 'hour' | 'day' | 'week';
}): Promise<CostTrend> {
  return client.get('/api/v2/cost/trend', { params });
}

// 获取预算列表
export async function getBudgets(): Promise<Budget[]> {
  return client.get('/api/v2/cost/budgets');
}

// 创建预算
export async function createBudget(data: CreateBudgetRequest): Promise<Budget> {
  return client.post('/api/v2/cost/budgets', data);
}

// 更新预算
export async function updateBudget(id: string, data: Partial<CreateBudgetRequest>): Promise<Budget> {
  return client.put(`/api/v2/cost/budgets/${id}`, data);
}

// 删除预算
export async function deleteBudget(id: string): Promise<void> {
  return client.delete(`/api/v2/cost/budgets/${id}`);
}

// 获取成本明细
export async function getCostDetails(params: CostDetailQueryParams): Promise<CostDetailListResponse> {
  return client.get('/api/v2/cost/details', { params });
}

// 获取成本预警
export async function getCostAlerts(params?: {
  acknowledged?: boolean;
  severity?: string;
}): Promise<CostAlert[]> {
  return client.get('/api/v2/cost/alerts', { params });
}

// 确认成本预警
export async function acknowledgeAlert(alertId: string): Promise<void> {
  return client.post(`/api/v2/cost/alerts/${alertId}/acknowledge`);
}

// 获取模型价格
export async function getModelPricing(): Promise<ModelPricing[]> {
  return client.get('/api/v2/cost/pricing');
}

// 导出成本数据
export function exportCostData(format: 'json' | 'csv' = 'csv', range?: string): string {
  const baseUrl = import.meta.env.VITE_API_URL || '';
  let url = `${baseUrl}/api/v2/cost/export?format=${format}`;
  if (range) {
    url += `&range=${range}`;
  }
  return url;
}

// 获取实时成本
export async function getRealtimeCost(): Promise<{
  today_cost: number;
  today_calls: number;
  last_hour_cost: number;
  last_hour_calls: number;
}> {
  return client.get('/api/v2/cost/realtime');
}

// 预测成本
export async function predictCost(params: {
  days: number;
  model?: string;
}): Promise<{
  predicted_cost: number;
  confidence: number;
  based_on_days: number;
}> {
  return client.post('/api/v2/cost/predict', params);
}

export const costApi = {
  getCostSummary,
  getCostTrend,
  getBudgets,
  createBudget,
  updateBudget,
  deleteBudget,
  getCostDetails,
  getCostAlerts,
  acknowledgeAlert,
  getModelPricing,
  exportCostData,
  getRealtimeCost,
  predictCost,
};
