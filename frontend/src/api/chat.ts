import client from './client';
import type { ApiResponse, ChatRequest, ChatResponse, Session, MultiAgentRequest, MultiAgentResponse, PaginationParams } from '../types';

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

  // 流式对话 - 返回 EventSource
  chatStream: (params: ChatRequest, onMessage: (data: unknown) => void, onError?: (error: Error) => void) => {
    const baseUrl = import.meta.env.VITE_API_URL || window.location.origin;
    const url = new URL('/api/v2/chat/stream', baseUrl);

    const body = JSON.stringify({ ...params, stream: true });

    fetch(url.toString(), {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-Tenant-ID': localStorage.getItem('tenantId') || 'default',
      },
      body,
    }).then(async (response) => {
      const reader = response.body?.getReader();
      const decoder = new TextDecoder();

      if (!reader) return;

      while (true) {
        const { done, value } = await reader.read();
        if (done) break;

        const chunk = decoder.decode(value);
        const lines = chunk.split('\n');

        for (const line of lines) {
          if (line.startsWith('data:')) {
            const data = line.slice(5).trim();
            if (data) {
              try {
                onMessage(JSON.parse(data));
              } catch {
                onMessage(data);
              }
            }
          }
        }
      }
    }).catch((error) => {
      onError?.(error);
    });
  },
};