import { create } from 'zustand';
import type { Session, Message, AgentState } from '../types';
import { chatApi } from '../api';

interface ChatState {
  // 当前会话
  currentSessionId: string | null;
  messages: Message[];

  // 会话列表
  sessions: Session[];

  // 状态
  isLoading: boolean;
  isStreaming: boolean;

  // Agent 执行轨迹
  agentStates: AgentState[];

  // Actions
  loadSessions: () => Promise<void>;
  createSession: () => void;
  selectSession: (id: string) => Promise<void>;
  deleteSession: (id: string) => Promise<void>;
  sendMessage: (content: string) => Promise<void>;
  addMessage: (message: Message) => void;
  updateLastMessage: (content: string) => void;
  clearMessages: () => void;
  setLoading: (loading: boolean) => void;
  setStreaming: (streaming: boolean) => void;
}

export const useChatStore = create<ChatState>((set, get) => ({
  currentSessionId: null,
  messages: [],
  sessions: [],
  isLoading: false,
  isStreaming: false,
  agentStates: [],

  loadSessions: async () => {
    try {
      console.log('[ChatStore] Loading sessions...');
      const res = await chatApi.listSessions({ page: 1, page_size: 50 });
      console.log('[ChatStore] Sessions response:', res);
      // API 响应结构: { code: 0, data: { sessions: [...], pagination: {...} } }
      // axios 拦截器已经返回 response.data，所以这里是 { code: 0, data: {...} }
      const sessions = (res as any).data?.sessions || (res as any).sessions || [];
      console.log('[ChatStore] Parsed sessions:', sessions);
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
      // API 响应结构: { code: 0, data: { session: {...} } }
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

  sendMessage: async (content: string) => {
    const { currentSessionId, addMessage, setLoading, setStreaming } = get();

    // 添加用户消息
    const userMessage: Message = {
      id: `user-${Date.now()}`,
      role: 'user',
      content,
      timestamp: Date.now(),
    };
    addMessage(userMessage);

    setLoading(true);

    try {
      const res = await chatApi.chat({
        session_id: currentSessionId || undefined,
        message: content,
      });

      // API 响应结构: { code: 0, data: { session_id, content, ... } }
      const data = (res as any).data || res;

      // 更新会话 ID
      if (!currentSessionId && data.session_id) {
        set({ currentSessionId: data.session_id });
      }

      // 添加助手消息（包含 agent_states）
      const assistantMessage: Message = {
        id: `assistant-${Date.now()}`,
        role: 'assistant',
        content: data.content,
        tool_calls: data.tool_calls,
        agent_trace: data.agent_states,  // 包含 Agent 执行轨迹
        timestamp: Date.now(),
      };
      addMessage(assistantMessage);

      // 更新全局 agentStates
      if (data.agent_states) {
        set({ agentStates: data.agent_states });
      }

      // 刷新会话列表
      get().loadSessions();
    } catch (error) {
      console.error('Send message failed:', error);
      // 添加错误消息
      addMessage({
        id: `error-${Date.now()}`,
        role: 'assistant',
        content: '抱歉，发生了错误，请稍后重试。',
        timestamp: Date.now(),
      });
    } finally {
      setLoading(false);
      setStreaming(false);
    }
  },

  addMessage: (message: Message) => {
    set((state) => ({
      messages: [...state.messages, message],
    }));
  },

  updateLastMessage: (content: string) => {
    set((state) => {
      const messages = [...state.messages];
      if (messages.length > 0) {
        messages[messages.length - 1] = {
          ...messages[messages.length - 1],
          content,
        };
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
}));