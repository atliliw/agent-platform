// Playground API
import client from './client';

// ==================== Types ====================

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

/** Streaming chunk from SSE */
export interface PlaygroundStreamChunk {
  content: string;
  done: boolean;
  error: string;
  log_id: string;
  created_at: number | string;
}

/** Extended message for playground multi-turn context */
export interface PlaygroundMessage {
  id: string;
  role: 'user' | 'assistant' | 'system';
  content: string;
  model?: string;
  total_tokens?: number | string;
  input_tokens?: number | string;
  output_tokens?: number | string;
  cost?: number;
  latency?: number | string;
  finish_reason?: string;
  log_id?: string;
  timestamp: number;
  isStreaming?: boolean;
}

/** Model configuration for sidebar */
export interface ModelConfig {
  id: string;
  name: string;
  provider: string;
}

// ==================== Helpers ====================

/** Convert a protobuf int64 value (serialized as string) to a number */
function toNumber(value: number | string | undefined | null): number {
  if (value == null) return 0;
  return Number(value);
}

export const playgroundHelpers = {
  toNumber,
};

/** Available models — DashScope compatible models */
export const AVAILABLE_MODELS: ModelConfig[] = [
  // Qwen 系列
  { id: 'qwen-turbo', name: 'Qwen Turbo', provider: 'DashScope' },
  { id: 'qwen-plus', name: 'Qwen Plus', provider: 'DashScope' },
  { id: 'qwen-max', name: 'Qwen Max', provider: 'DashScope' },
  { id: 'qwen3.7-max-2026-06-08', name: 'Qwen 3.7 Max', provider: 'DashScope' },
  { id: 'qwen-long', name: 'Qwen Long', provider: 'DashScope' },
  // 多模态
  { id: 'qwen-vl-plus', name: 'Qwen VL Plus', provider: 'DashScope' },
  { id: 'qwen-vl-max', name: 'Qwen VL Max', provider: 'DashScope' },
  // DeepSeek 系列 (DashScope 兼容)
  { id: 'deepseek-r1', name: 'DeepSeek R1', provider: 'DashScope' },
  { id: 'deepseek-v3', name: 'DeepSeek V3', provider: 'DashScope' },
];

// ==================== API ====================

export const playgroundApi = {
  // Single model execution (non-streaming)
  execute: (data: {
    model: string;
    messages: Array<{ role: string; content: string }>;
    temperature?: number;
    max_tokens?: number;
    topP?: number;
  }) => client.post('/api/v2/harness/playground/execute', data),

  // Multi-model comparison
  compareModels: (data: {
    models: string[];
    messages: Array<{ role: string; content: string }>;
    temperature?: number;
    max_tokens?: number;
  }) => client.post('/api/v2/harness/playground/compare', data),

  // Streaming execution via SSE
  streamExecute: (
    data: {
      model: string;
      messages: Array<{ role: string; content: string }>;
      temperature?: number;
      max_tokens?: number;
      topP?: number;
      session_id?: string;
    },
    onChunk: (chunk: PlaygroundStreamChunk) => void,
    onError?: (error: Error) => void,
    onDone?: () => void,
    signal?: AbortSignal,
  ) => {
    const baseUrl = import.meta.env.VITE_API_URL || window.location.origin;
    const url = new URL(
      '/api/v2/harness/playground/stream',
      baseUrl,
    );

    const token = localStorage.getItem('token');
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
      'X-Tenant-ID': localStorage.getItem('tenantId') || 'default',
    };
    if (token) {
      headers['Authorization'] = `Bearer ${token}`;
    }

    fetch(url.toString(), {
      method: 'POST',
      headers,
      body: JSON.stringify(data),
      signal,
    })
      .then(async (response) => {
        if (!response.ok) {
          throw new Error(`HTTP ${response.status}: ${response.statusText}`);
        }

        const reader = response.body?.getReader();
        const decoder = new TextDecoder();
        if (!reader) return;

        let buffer = '';

        while (true) {
          const { done, value } = await reader.read();
          if (done) break;

          buffer += decoder.decode(value, { stream: true });
          const lines = buffer.split('\n');
          // Keep the last incomplete line in the buffer
          buffer = lines.pop() || '';

          for (const line of lines) {
            if (line.startsWith('data:')) {
              const rawData = line.slice(5).trim();
              if (!rawData || rawData === '[DONE]') continue;

              try {
                const parsed = JSON.parse(rawData);
                // c.SSEvent sends { "content": ..., "done": ..., ... }
                // The event name is on a separate "event:" line, data is here
                const chunkData = parsed.message || parsed;
                const chunk = chunkData as PlaygroundStreamChunk;

                if (chunk.error) {
                  onError?.(new Error(chunk.error));
                  return;
                }

                onChunk(chunk);

                if (chunk.done) {
                  onDone?.();
                  return;
                }
              } catch {
                // Skip malformed lines
              }
            }

            if (line.startsWith('event:')) {
              const eventType = line.slice(6).trim();
              if (eventType === 'error') {
                // Next data line will contain the error
              }
            }
          }
        }

        // Stream ended without done signal
        onDone?.();
      })
      .catch((error) => {
        if (error.name !== 'AbortError') {
          onError?.(error);
        }
      });
  },

  // Get execution history
  getHistory: (params?: { limit?: number }) =>
    client.get('/api/v2/harness/playground/history', { params }),

  // Get execution statistics
  getStats: () => client.get('/api/v2/harness/playground/stats'),
};
