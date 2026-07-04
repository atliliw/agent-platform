// Playground API
import client from './client';

export interface PlaygroundResult {
  content: string;
  total_tokens: number | string;
  input_tokens: number | string;
  output_tokens: number | string;
  cost: number;
  latency: number | string;
  model: string;
  finish_reason: string;
  log_id: string;
  created_at: number | string;
}

export interface ModelComparison {
  best_model: string;
  fastest_model: string;
  cheapest_model: string;
  avg_latency: number;
  avg_cost: number;
  avg_tokens: number;
}

export interface PlaygroundStats {
  total_executions: number | string;
  streamed_executions: number | string;
  comparison_executions: number | string;
  total_tokens: number | string;
  total_cost: number;
  avg_latency: number;
  avg_cost: number;
  avg_tokens: number;
  model_counts: Record<string, number | string>;
}

export interface PlaygroundHistory {
  id: string;
  tenant_id: string;
  user_id: string;
  session_id: string;
  model: string;
  messages: Array<{ role: string; content: string }>;
  result?: PlaygroundResult;
  comparison?: {
    results: PlaygroundResult[];
    comparison: ModelComparison;
    created_at: number | string;
  };
  temperature: number;
  max_tokens: number | string;
  topP: number;
  parameters: Record<string, string>;
  streamed: boolean;
  created_at: number | string;
}

/** Convert a protobuf int64 value (serialized as string) to a number */
function toNumber(value: number | string | undefined | null): number {
  if (value == null) return 0;
  return Number(value);
}

export const playgroundHelpers = {
  toNumber,
};

export const playgroundApi = {
  execute: (data: { model: string; messages: Array<{ role: string; content: string }>; temperature?: number; max_tokens?: number; topP?: number }) =>
    client.post('/api/v2/harness/playground/execute', data),

  compareModels: (data: { models: string[]; messages: Array<{ role: string; content: string }>; temperature?: number; max_tokens?: number }) =>
    client.post('/api/v2/harness/playground/compare', data),

  getHistory: (params?: { limit?: number }) =>
    client.get('/api/v2/harness/playground/history', { params }),

  getStats: () =>
    client.get('/api/v2/harness/playground/stats'),
};
