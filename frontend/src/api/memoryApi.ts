// 记忆管理 API 服务
import client from './client';
import type {
  WorkingMemory,
  EpisodicMemory,
  SemanticMemory,
  ProceduralMemory,
  TimelineEvent,
  TimelineQueryParams,
  KnowledgeGraph,
  ConsolidateRequest,
  ConsolidateResult,
  MemorySearchRequest,
  MemorySearchResult,
  ForgettingConfig,
  MemoryStats,
} from './memory';

// ===== 工作记忆 =====

// 获取工作记忆
export async function getWorkingMemory(sessionId: string): Promise<WorkingMemory> {
  return client.get(`/api/v2/memory/working/${sessionId}`);
}

// 更新工作记忆
export async function updateWorkingMemory(sessionId: string, data: {
  messages?: unknown[];
  max_tokens?: number;
}): Promise<WorkingMemory> {
  return client.put(`/api/v2/memory/working/${sessionId}`, data);
}

// 压缩工作记忆
export async function compressWorkingMemory(sessionId: string): Promise<{
  original_tokens: number;
  compressed_tokens: number;
  compression_ratio: number;
}> {
  return client.post(`/api/v2/memory/working/${sessionId}/compress`);
}

// ===== 情节记忆 =====

// 获取情节记忆
export async function getEpisodicMemory(memoryId: string): Promise<EpisodicMemory> {
  return client.get(`/api/v2/memory/episodic/${memoryId}`);
}

// 创建情节记忆
export async function createEpisodicMemory(data: Omit<EpisodicMemory, 'id' | 'created_at'>): Promise<EpisodicMemory> {
  return client.post('/api/v2/memory/episodic', data);
}

// 更新情节记忆
export async function updateEpisodicMemory(memoryId: string, data: Partial<EpisodicMemory>): Promise<EpisodicMemory> {
  return client.put(`/api/v2/memory/episodic/${memoryId}`, data);
}

// 删除情节记忆
export async function deleteEpisodicMemory(memoryId: string): Promise<void> {
  return client.delete(`/api/v2/memory/episodic/${memoryId}`);
}

// ===== 语义记忆 =====

// 获取语义记忆
export async function getSemanticMemory(memoryId: string): Promise<SemanticMemory> {
  return client.get(`/api/v2/memory/semantic/${memoryId}`);
}

// 创建语义记忆
export async function createSemanticMemory(data: Omit<SemanticMemory, 'id' | 'created_at' | 'updated_at' | 'last_accessed' | 'access_count'>): Promise<SemanticMemory> {
  return client.post('/api/v2/memory/semantic', data);
}

// 更新语义记忆
export async function updateSemanticMemory(memoryId: string, data: Partial<SemanticMemory>): Promise<SemanticMemory> {
  return client.put(`/api/v2/memory/semantic/${memoryId}`, data);
}

// 删除语义记忆
export async function deleteSemanticMemory(memoryId: string): Promise<void> {
  return client.delete(`/api/v2/memory/semantic/${memoryId}`);
}

// 获取知识图谱
export async function getKnowledgeGraph(params?: {
  concept?: string;
  depth?: number;
}): Promise<KnowledgeGraph> {
  return client.get('/api/v2/memory-enhanced/graph', { params });
}

// ===== 程序记忆 =====

// 获取程序记忆
export async function getProceduralMemory(memoryId: string): Promise<ProceduralMemory> {
  return client.get(`/api/v2/memory/procedural/${memoryId}`);
}

// 创建程序记忆
export async function createProceduralMemory(data: Omit<ProceduralMemory, 'id' | 'created_at' | 'updated_at' | 'success_rate' | 'usage_count' | 'last_used'>): Promise<ProceduralMemory> {
  return client.post('/api/v2/memory/procedural', data);
}

// 执行程序记忆（技能）
export async function executeProceduralMemory(memoryId: string, params?: Record<string, unknown>): Promise<{
  success: boolean;
  result: string;
  duration_ms: number;
}> {
  return client.post(`/api/v2/memory/procedural/${memoryId}/execute`, { params });
}

// ===== 时间线 =====

// 获取时间线
export async function getTimeline(params?: TimelineQueryParams): Promise<TimelineEvent[]> {
  return client.get('/api/v2/memory-enhanced/timeline', { params });
}

// ===== 记忆整合 =====

// 触发记忆整合
export async function consolidateMemory(data: ConsolidateRequest): Promise<ConsolidateResult> {
  return client.post('/api/v2/memory-enhanced/consolidate', data);
}

// ===== 记忆搜索 =====

// 搜索记忆
export async function searchMemory(data: MemorySearchRequest): Promise<MemorySearchResult[]> {
  return client.post('/api/v2/memory-enhanced/search', data);
}

// ===== 遗忘曲线 =====

// 获取遗忘配置
export async function getForgettingConfig(): Promise<ForgettingConfig> {
  return client.get('/api/v2/memory/forgetting/config');
}

// 更新遗忘配置
export async function updateForgettingConfig(config: Partial<ForgettingConfig>): Promise<ForgettingConfig> {
  return client.put('/api/v2/memory/forgetting/config', config);
}

// ===== 统计 =====

// 获取记忆统计
export async function getMemoryStats(): Promise<MemoryStats> {
  return client.get('/api/v2/memory-enhanced/stats');
}

// 导出记忆
export function exportMemory(format: 'json' | 'csv' = 'json'): string {
  const baseUrl = import.meta.env.VITE_API_URL || 'http://192.168.10.100:9000';
  return `${baseUrl}/api/v2/memory-enhanced/export?format=${format}`;
}

export const memoryApi = {
  // 工作记忆
  getWorkingMemory,
  updateWorkingMemory,
  compressWorkingMemory,
  // 情节记忆
  getEpisodicMemory,
  createEpisodicMemory,
  updateEpisodicMemory,
  deleteEpisodicMemory,
  // 语义记忆
  getSemanticMemory,
  createSemanticMemory,
  updateSemanticMemory,
  deleteSemanticMemory,
  getKnowledgeGraph,
  // 程序记忆
  getProceduralMemory,
  createProceduralMemory,
  executeProceduralMemory,
  // 时间线
  getTimeline,
  // 整合
  consolidateMemory,
  // 搜索
  searchMemory,
  // 遗忘
  getForgettingConfig,
  updateForgettingConfig,
  // 统计
  getMemoryStats,
  exportMemory,
};
