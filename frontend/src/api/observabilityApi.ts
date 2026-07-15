// 可观测性 API 服务
import client from './client';
import type {
  Trace,
  TraceQueryParams,
  TraceListResponse,
  PerformanceMetrics,
  PerformanceProfile,
  TraceStats,
} from './observability';

// 获取追踪列表
export async function getTraces(params: TraceQueryParams): Promise<TraceListResponse> {
  return client.get('/api/v2/observability/traces', { params });
}

// 获取单个追踪详情
export async function getTrace(traceId: string): Promise<Trace> {
  return client.get(`/api/v2/observability/traces/${traceId}`);
}

// 获取会话的追踪
export async function getSessionTraces(sessionId: string): Promise<Trace[]> {
  return client.get(`/api/v2/observability/traces/session/${sessionId}`);
}

// 获取性能指标
export async function getPerformanceMetrics(params?: {
  start_time?: string;
  end_time?: string;
  agent_id?: string;
}): Promise<PerformanceMetrics> {
  return client.get('/api/v2/observability/metrics', { params });
}

// 获取性能分析
export async function getPerformanceProfile(sessionId: string): Promise<PerformanceProfile> {
  return client.get(`/api/v2/observability/profile/${sessionId}`);
}

// 获取追踪统计
export async function getTraceStats(): Promise<TraceStats> {
  return client.get('/api/v2/observability/stats');
}

// 导出追踪数据
export function exportTraces(format: 'json' | 'csv' = 'json'): string {
  const baseUrl = import.meta.env.VITE_API_URL || '';
  return `${baseUrl}/api/v2/observability/traces/export?format=${format}`;
}

// 删除追踪
export async function deleteTrace(traceId: string): Promise<void> {
  return client.delete(`/api/v2/observability/traces/${traceId}`);
}

// 清理过期追踪
export async function cleanupTraces(olderThanDays: number): Promise<{ deleted: number }> {
  return client.post('/api/v2/observability/traces/cleanup', { older_than_days: olderThanDays });
}

export const observabilityApi = {
  getTraces,
  getTrace,
  getSessionTraces,
  getPerformanceMetrics,
  getPerformanceProfile,
  getTraceStats,
  exportTraces,
  deleteTrace,
  cleanupTraces,
};
