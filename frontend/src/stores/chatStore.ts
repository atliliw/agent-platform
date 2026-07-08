import { create } from 'zustand';
import type { Session, Message, AgentState } from '../types';
import { chatApi } from '../api';
import client from '../api/client';
import type { StreamChunk } from '../api/chat';

interface ChatState {
  // 当前会话
  currentSessionId: string | null;
  messages: Message[];

  // 会话列表
  sessions: Session[];

  // 状态
  isLoading: boolean;
  isStreaming: boolean;
  isPaused: boolean;
  isRunning: boolean;

  // Agent 执行轨迹
  agentStates: AgentState[];

  // System Prompt (模板选择)
  systemPrompt: string;
  promptTemplateKey: string | null;
  showSystemPrompt: boolean;

  // Abort controller for cancelling stream
  streamController: AbortController | null;

  // Actions
  loadSessions: () => Promise<void>;
  createSession: () => void;
  selectSession: (id: string) => Promise<void>;
  deleteSession: (id: string) => Promise<void>;
  sendMessage: (content: string) => void;
  stopStreaming: () => void;
  addMessage: (message: Message) => void;
  updateLastMessage: (updater: (msg: Message) => Message) => void;
  clearMessages: () => void;
  setLoading: (loading: boolean) => void;
  setStreaming: (streaming: boolean) => void;
  setIsPaused: (paused: boolean) => void;
  setIsRunning: (running: boolean) => void;
  setSystemPrompt: (prompt: string) => void;
  setPromptTemplateKey: (key: string | null) => void;
  setShowSystemPrompt: (show: boolean) => void;
  pauseAgent: () => Promise<void>;
  resumeAgent: () => Promise<void>;
  stopAgent: () => Promise<void>;
  injectMessage: (message: string) => Promise<void>;
}

export const useChatStore = create<ChatState>((set, get) => ({
  currentSessionId: null,
  messages: [],
  sessions: [],
  isLoading: false,
  isStreaming: false,
  isPaused: false,
  isRunning: false,
  agentStates: [],
  systemPrompt: '',
  promptTemplateKey: null,
  showSystemPrompt: false,
  streamController: null,

  loadSessions: async () => {
    try {
      const res = await chatApi.listSessions({ page: 1, page_size: 50 });
      const sessions = (res as any).data?.sessions || (res as any).sessions || [];
      set({ sessions });
    } catch (error) {
      console.error('[ChatStore] Load sessions failed:', error);
    }
  },

  createSession: () => {
    set({
      currentSessionId: null,
      messages: [],
      agentStates: [],
    });
  },

  selectSession: async (id: string) => {
    try {
      set({ isLoading: true });
      const res = await chatApi.getSession(id);
      const session = (res as any).data?.session || (res as any).session || (res as any).data;
      const messages = session?.messages || [];
      set({
        currentSessionId: id,
        messages,
        isLoading: false,
      });
    } catch (error) {
      console.error('Load session failed:', error);
      set({ isLoading: false });
    }
  },

  deleteSession: async (id: string) => {
    try {
      await chatApi.deleteSession(id);
      const sessions = get().sessions.filter((s) => s.id !== id);
      set({ sessions });

      if (get().currentSessionId === id) {
        set({ currentSessionId: null, messages: [], agentStates: [] });
      }
    } catch (error) {
      console.error('Delete session failed:', error);
    }
  },

  sendMessage: (content: string) => {
    const { currentSessionId, systemPrompt, promptTemplateKey } = get();

    // Add user message
    const userMessage: Message = {
      id: `user-${Date.now()}`,
      role: 'user',
      content,
      timestamp: Date.now(),
    };
    set((state) => ({ messages: [...state.messages, userMessage] }));

    // Create placeholder assistant message for streaming
    const assistantId = `assistant-${Date.now()}`;
    const assistantPlaceholder: Message = {
      id: assistantId,
      role: 'assistant',
      content: '',
      timestamp: Date.now(),
    };
    set((state) => ({
      messages: [...state.messages, assistantPlaceholder],
      isStreaming: true,
      isLoading: true,
      isRunning: true,
      isPaused: false,
    }));

    // Start streaming
    const controller = chatApi.chatStream(
      {
        session_id: currentSessionId || undefined,
        message: content,
        system_prompt: systemPrompt || undefined,
        prompt_template_key: promptTemplateKey || undefined,
      },
      {
        onChunk: (chunk: StreamChunk) => {
          const { messages, currentSessionId: sid } = get();

          switch (chunk.type) {
            case 'token': {
              // Append token to the last assistant message
              const updated = messages.map((msg, idx) => {
                if (idx === messages.length - 1 && msg.role === 'assistant') {
                  return { ...msg, content: msg.content + chunk.content };
                }
                return msg;
              });
              set({ messages: updated });
              break;
            }

            case 'think': {
              // Update the placeholder with thinking indicator
              const updated = messages.map((msg, idx) => {
                if (idx === messages.length - 1 && msg.role === 'assistant') {
                  return { ...msg, content: chunk.content || '正在思考...' };
                }
                return msg;
              });
              set({ messages: updated });
              break;
            }

            case 'tool_start': {
              // Show tool call indicator
              const updated = messages.map((msg, idx) => {
                if (idx === messages.length - 1 && msg.role === 'assistant') {
                  const existingContent = msg.content || '';
                  return { ...msg, content: existingContent + '\n\n' + chunk.content };
                }
                return msg;
              });
              set({ messages: updated });
              break;
            }

            case 'tool_result': {
              const updated = messages.map((msg, idx) => {
                if (idx === messages.length - 1 && msg.role === 'assistant') {
                  const existingContent = msg.content || '';
                  return { ...msg, content: existingContent + '\n' + chunk.content };
                }
                return msg;
              });
              set({ messages: updated });
              break;
            }

            case 'handoff': {
              const updated = messages.map((msg, idx) => {
                if (idx === messages.length - 1 && msg.role === 'assistant') {
                  const existingContent = msg.content || '';
                  return { ...msg, content: existingContent + '\n\n🔄 ' + chunk.content };
                }
                return msg;
              });
              set({ messages: updated });
              break;
            }

            case 'final': {
              // Replace the entire content with the final complete response
              const updated = messages.map((msg, idx) => {
                if (idx === messages.length - 1 && msg.role === 'assistant') {
                  return { ...msg, content: chunk.content };
                }
                return msg;
              });
              set({ messages: updated });

              // Update session ID if this is a new session
              if (!sid && chunk.state) {
                // Session ID comes from the done event or we refresh
              }
              break;
            }

            case 'error': {
              const updated = messages.map((msg, idx) => {
                if (idx === messages.length - 1 && msg.role === 'assistant') {
                  return { ...msg, content: '❌ ' + (chunk.content || '发生了错误') };
                }
                return msg;
              });
              set({ messages: updated, isStreaming: false, isLoading: false, isRunning: false, isPaused: false });
              break;
            }
          }
        },

        onError: (error: Error) => {
          console.error('[ChatStore] Stream error:', error);
          const { messages } = get();
          const updated = messages.map((msg, idx) => {
            if (idx === messages.length - 1 && msg.role === 'assistant') {
              return { ...msg, content: '抱歉，发生了错误，请稍后重试。' };
            }
            return msg;
          });
          set({ messages: updated, isStreaming: false, isLoading: false, isRunning: false, isPaused: false });
        },

        onDone: () => {
          set({ isStreaming: false, isLoading: false, isRunning: false, isPaused: false, streamController: null });
          // Refresh session list
          get().loadSessions();
        },
      },
    );

    set({ streamController: controller });
  },

  stopStreaming: () => {
    const { streamController } = get();
    if (streamController) {
      streamController.abort();
      set({ streamController: null, isStreaming: false, isLoading: false, isRunning: false, isPaused: false });
    }
  },

  addMessage: (message: Message) => {
    set((state) => ({
      messages: [...state.messages, message],
    }));
  },

  updateLastMessage: (updater: (msg: Message) => Message) => {
    set((state) => {
      const messages = [...state.messages];
      if (messages.length > 0) {
        const lastIdx = messages.length - 1;
        messages[lastIdx] = updater(messages[lastIdx]);
      }
      return { messages };
    });
  },

  clearMessages: () => {
    set({ messages: [], currentSessionId: null, agentStates: [] });
  },

  setLoading: (loading: boolean) => {
    set({ isLoading: loading });
  },

  setStreaming: (streaming: boolean) => {
    set({ isStreaming: streaming });
  },

  setIsPaused: (paused: boolean) => {
    set({ isPaused: paused });
  },

  setIsRunning: (running: boolean) => {
    set({ isRunning: running });
  },

  setSystemPrompt: (prompt: string) => {
    set({ systemPrompt: prompt });
  },

  setPromptTemplateKey: (key: string | null) => {
    set({ promptTemplateKey: key });
  },

  setShowSystemPrompt: (show: boolean) => {
    set({ showSystemPrompt: show });
  },

  pauseAgent: async () => {
    const { currentSessionId } = get();
    if (!currentSessionId) return;
    try {
      await client.post(`/api/v2/harness/session/${currentSessionId}/intervene`, { type: 'pause' });
      set({ isPaused: true });
    } catch (error) {
      console.error('[ChatStore] Pause agent failed:', error);
    }
  },

  resumeAgent: async () => {
    const { currentSessionId } = get();
    if (!currentSessionId) return;
    try {
      await client.post(`/api/v2/harness/session/${currentSessionId}/resume`);
      set({ isPaused: false });
    } catch (error) {
      console.error('[ChatStore] Resume agent failed:', error);
    }
  },

  stopAgent: async () => {
    const { currentSessionId } = get();
    if (!currentSessionId) return;
    try {
      await client.post(`/api/v2/harness/session/${currentSessionId}/intervene`, { type: 'stop' });
      // Also abort the local stream
      const { streamController } = get();
      if (streamController) {
        streamController.abort();
      }
      set({
        streamController: null,
        isStreaming: false,
        isLoading: false,
        isRunning: false,
        isPaused: false,
      });
    } catch (error) {
      console.error('[ChatStore] Stop agent failed:', error);
    }
  },

  injectMessage: async (message: string) => {
    const { currentSessionId } = get();
    if (!currentSessionId || !message.trim()) return;
    try {
      await client.post(`/api/v2/harness/session/${currentSessionId}/inject`, { message: message.trim() });
    } catch (error) {
      console.error('[ChatStore] Inject message failed:', error);
    }
  },
}));
