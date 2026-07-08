import client from './client';
import type { ApiResponse, ChatRequest, ChatResponse, Session, MultiAgentRequest, MultiAgentResponse, PaginationParams } from '../types';

export interface StreamChunk {
  type: string;       // "think" | "token" | "tool_start" | "tool_result" | "handoff" | "final" | "error" | "done"
  content: string;
  state?: {
    thought: string;
    action: string;
    arguments: string;
    result: string;
    step: number;
  };
}

export interface StreamCallbacks {
  onChunk: (chunk: StreamChunk) => void;
  onError: (error: Error) => void;
  onDone: () => void;
}

export const chatApi = {
  // 对话
  chat: (params: ChatRequest): Promise<ApiResponse<ChatResponse>> =>
    client.post('/api/v2/chat', params),

  // 会话列表
  listSessions: (params?: PaginationParams): Promise<ApiResponse<{ sessions: Session[]; pagination: { total: number; page: number; page_size: number } }>> =>
    client.get('/api/v2/sessions', { params }),

  // 获取会话
  getSession: (id: string): Promise<ApiResponse<Session>> =>
    client.get(`/api/v2/sessions/${id}`),

  // 删除会话
  deleteSession: (id: string): Promise<ApiResponse<null>> =>
    client.delete(`/api/v2/sessions/${id}`),

  // 多 Agent 对话
  multiAgentChat: (params: MultiAgentRequest): Promise<ApiResponse<MultiAgentResponse>> =>
    client.post('/api/v2/multi-agent/chat', params),

  // 流式对话 — real SSE parsing with buffer and AbortController support
  chatStream: (params: ChatRequest, callbacks: StreamCallbacks): AbortController => {
    const controller = new AbortController();
    const baseUrl = import.meta.env.VITE_API_URL || window.location.origin;
    const url = new URL('/api/v2/chat/stream', baseUrl);

    const body = JSON.stringify({ ...params, stream: true });

    fetch(url.toString(), {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-Tenant-ID': localStorage.getItem('tenantId') || 'default',
        ...(localStorage.getItem('token')
          ? { Authorization: `Bearer ${localStorage.getItem('token')}` }
          : {}),
      },
      body,
      signal: controller.signal,
    }).then(async (response) => {
      if (!response.ok) {
        callbacks.onError(new Error(`HTTP ${response.status}: ${response.statusText}`));
        return;
      }

      const reader = response.body?.getReader();
      const decoder = new TextDecoder();

      if (!reader) {
        callbacks.onError(new Error('No readable stream'));
        return;
      }

      let buffer = '';

      while (true) {
        const { done, value } = await reader.read();
        if (done) break;

        buffer += decoder.decode(value, { stream: true });
        const lines = buffer.split('\n');
        // Keep the last (potentially partial) line in the buffer
        buffer = lines.pop() || '';

        for (const line of lines) {
          if (line.startsWith('event:')) {
            // Skip event type lines — we parse data lines
            continue;
          }
          if (line.startsWith('data:')) {
            const data = line.slice(5).trim();
            if (!data) continue;

            try {
              const parsed = JSON.parse(data) as StreamChunk;

              if (parsed.type === 'done') {
                callbacks.onDone();
                return;
              }

              callbacks.onChunk(parsed);
            } catch {
              // If it's not JSON, treat as raw content
              if (data === '[DONE]') {
                callbacks.onDone();
                return;
              }
            }
          }
        }
      }

      // Process any remaining buffer
      if (buffer.startsWith('data:')) {
        const data = buffer.slice(5).trim();
        if (data) {
          try {
            const parsed = JSON.parse(data) as StreamChunk;
            if (parsed.type !== 'done') {
              callbacks.onChunk(parsed);
            }
          } catch {
            // ignore
          }
        }
      }

      callbacks.onDone();
    }).catch((error) => {
      if (error.name === 'AbortError') {
        // User cancelled — not an error
        callbacks.onDone();
        return;
      }
      callbacks.onError(error);
    });

    return controller;
  },
};
