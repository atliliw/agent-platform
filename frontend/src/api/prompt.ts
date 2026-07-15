// Prompt Management API
import client from './client';

export interface Prompt {
  id: string;
  key: string;
  name: string;
  description: string;
  category: string;
  tags: string;
  tenant_id: string;
  created_at: number;
  updated_at: number;
  created_by: string;
}

export interface PromptVersion {
  id: string;
  prompt_id: string;
  version: string;
  content: string;
  variables: string;
  metadata: string;
  status: string;
  is_active: boolean;
  created_at: number;
  created_by: string;
  metrics?: any;
  [key: string]: any;  // allow additional properties
}

export interface PromptPerformance {
  id: string;
  version_id: string;
  total_calls: number;
  success_calls: number;
  success_rate: number;
  avg_latency: number;
  avg_input_tokens: number;
  avg_output_tokens: number;
  avg_total_tokens: number;
  avg_cost: number;
  user_rating: number;
  feedback_count: number;
  period_start: number;
  period_end: number;
}

export const promptApi = {
  createPrompt: (data: { key: string; name: string; description: string; category: string; tags?: string }) =>
    client.post('/api/v2/harness/prompt', data),

  listPrompts: (params?: { category?: string }) =>
    client.get('/api/v2/harness/prompt/list', { params }),

  getPrompt: (key: string) =>
    client.get(`/api/v2/harness/prompt/${key}`),

  deletePrompt: (key: string) =>
    client.delete(`/api/v2/harness/prompt/${key}`),

  createVersion: (key: string, data: { version: string; content: string; variables?: string; metadata?: string; activate?: boolean }) =>
    client.post(`/api/v2/harness/prompt/${key}/versions`, data),

  listVersions: (key: string) =>
    client.get(`/api/v2/harness/prompt/${key}/versions`),

  getActiveVersion: (key: string) =>
    client.get(`/api/v2/harness/prompt/${key}/active`),

  activateVersion: (data: { version_id: string }) =>
    client.post('/api/v2/harness/prompt/version/activate', data),

  compareVersions: (data: { version1_id: string; version2_id: string }) =>
    client.post('/api/v2/harness/prompt/compare', data),

  renderPrompt: (data: { prompt_key: string; variables?: string }) =>
    client.post('/api/v2/harness/prompt/render', data),

  getPerformance: (version_id: string) =>
    client.get(`/api/v2/harness/prompt/performance/${version_id}`),
};
