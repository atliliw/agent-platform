import { create } from 'zustand';
import { playgroundApi, AVAILABLE_MODELS } from '../api/playground';
import type {
  PlaygroundMessage,
  PlaygroundStreamChunk,
  PlaygroundResult,
  PlaygroundStats,
  PlaygroundHistory,
  ModelComparison,
} from '../api/playground';

const generateId = () => `msg-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;

interface PlaygroundState {
  // Configuration
  model: string;
  temperature: number;
  maxTokens: number;
  topP: number;
  systemPrompt: string;

  // Messages (multi-turn conversation)
  messages: PlaygroundMessage[];
  sessionId: string | null;

  // Execution state
  isLoading: boolean;
  isStreaming: boolean;
  abortController: AbortController | null;

  // History & Stats
  history: PlaygroundHistory[];
  stats: PlaygroundStats | null;

  // Compare mode
  compareModels: string[];
  compareResults: PlaygroundResult[];
  comparison: ModelComparison | null;
  isComparing: boolean;

  // UI state
  showComparePanel: boolean;

  // Actions — Configuration
  setModel: (model: string) => void;
  setTemperature: (temp: number) => void;
  setMaxTokens: (tokens: number) => void;
  setTopP: (topP: number) => void;
  setSystemPrompt: (prompt: string) => void;

  // Actions — Messages
  sendMessage: (content: string) => Promise<void>;
  stopStreaming: () => void;
  clearMessages: () => void;

  // Actions — Compare
  executeCompare: () => Promise<void>;
  setCompareModels: (models: string[]) => void;
  setShowComparePanel: (show: boolean) => void;

  // Actions — History & Stats
  loadHistory: () => Promise<void>;
  loadStats: () => Promise<void>;
  loadHistorySession: (history: PlaygroundHistory) => void;
}

export const usePlaygroundStore = create<PlaygroundState>((set, get) => ({
  // Configuration defaults
  model: 'qwen-plus',
  temperature: 0.7,
  maxTokens: 2048,
  topP: 1.0,
  systemPrompt: 'You are a helpful assistant.',

  // Messages
  messages: [],
  sessionId: null,

  // Execution state
  isLoading: false,
  isStreaming: false,
  abortController: null,

  // History & Stats
  history: [],
  stats: null,

  // Compare mode
  compareModels: ['qwen-plus', 'gpt-4o'],
  compareResults: [],
  comparison: null,
  isComparing: false,

  // UI state
  showComparePanel: false,

  // ===== Configuration Actions =====

  setModel: (model) => set({ model }),
  setTemperature: (temperature) => set({ temperature }),
  setMaxTokens: (maxTokens) => set({ maxTokens }),
  setTopP: (topP) => set({ topP }),
  setSystemPrompt: (systemPrompt) => set({ systemPrompt }),

  // ===== Message Actions =====

  sendMessage: async (content: string) => {
    const {
      model, temperature, maxTokens, topP, systemPrompt,
      messages, sessionId, abortController,
    } = get();

    // Prevent double execution
    if (abortController) return;

    // Build user message
    const userMessage: PlaygroundMessage = {
      id: generateId(),
      role: 'user',
      content,
      timestamp: Date.now(),
    };

    // Build assistant placeholder (streaming)
    const assistantMessage: PlaygroundMessage = {
      id: generateId(),
      role: 'assistant',
      content: '',
      model,
      timestamp: Date.now(),
      isStreaming: true,
    };

    const updatedMessages = [...messages, userMessage, assistantMessage];
    set({ messages: updatedMessages, isLoading: true, isStreaming: true });

    // Build messages for API (include system prompt + all prior messages)
    const apiMessages: Array<{ role: string; content: string }> = [];
    if (systemPrompt) {
      apiMessages.push({ role: 'system', content: systemPrompt });
    }
    for (const msg of updatedMessages.slice(0, -1)) {
      if (msg.role !== 'system') {
        apiMessages.push({ role: msg.role, content: msg.content });
      }
    }

    // Create abort controller
    const controller = new AbortController();
    set({ abortController: controller });

    // Accumulated content for streaming
    let accumulatedContent = '';

    playgroundApi.streamExecute(
      {
        model,
        messages: apiMessages,
        temperature,
        max_tokens: maxTokens,
        topP,
        session_id: sessionId || undefined,
      },
      // onChunk
      (chunk: PlaygroundStreamChunk) => {
        accumulatedContent += chunk.content;
        const currentMessages = get().messages;
        const lastIdx = currentMessages.length - 1;
        if (lastIdx < 0) return;

        const updated = [...currentMessages];
        updated[lastIdx] = {
          ...updated[lastIdx],
          content: accumulatedContent,
          log_id: chunk.log_id || updated[lastIdx].log_id,
        };
        set({ messages: updated });
      },
      // onError
      (error: Error) => {
        const currentMessages = get().messages;
        const lastIdx = currentMessages.length - 1;
        if (lastIdx >= 0) {
          const updated = [...currentMessages];
          const currentContent = updated[lastIdx].content;
          updated[lastIdx] = {
            ...updated[lastIdx],
            content: currentContent + `\n\n⚠️ Error: ${error.message}`,
            isStreaming: false,
          };
          set({ messages: updated });
        }
        set({ isLoading: false, isStreaming: false, abortController: null });
      },
      // onDone
      () => {
        const currentMessages = get().messages;
        const lastIdx = currentMessages.length - 1;
        if (lastIdx >= 0) {
          const updated = [...currentMessages];
          updated[lastIdx] = {
            ...updated[lastIdx],
            isStreaming: false,
          };
          set({ messages: updated });
        }
        set({ isLoading: false, isStreaming: false, abortController: null });

        // Refresh history/stats in background
        get().loadHistory();
        get().loadStats();
      },
      // signal
      controller.signal,
    );
  },

  stopStreaming: () => {
    const { abortController } = get();
    if (abortController) {
      abortController.abort();
    }
    // Finalize the last assistant message
    const currentMessages = get().messages;
    const lastIdx = currentMessages.length - 1;
    if (lastIdx >= 0 && currentMessages[lastIdx].isStreaming) {
      const updated = [...currentMessages];
      updated[lastIdx] = {
        ...updated[lastIdx],
        isStreaming: false,
      };
      set({ messages: updated });
    }
    set({ isLoading: false, isStreaming: false, abortController: null });
  },

  clearMessages: () => {
    set({ messages: [], sessionId: null });
  },

  // ===== Compare Actions =====

  executeCompare: async () => {
    const {
      model, temperature, maxTokens, topP, systemPrompt,
      messages, compareModels,
    } = get();

    if (compareModels.length < 2) return;

    // Use the last user message as the prompt for comparison
    const lastUserMsg = [...messages].reverse().find((m) => m.role === 'user');
    const promptContent = lastUserMsg?.content || '';

    if (!promptContent.trim()) return;

    set({ isComparing: true, compareResults: [], comparison: null, showComparePanel: true });

    try {
      const apiMessages: Array<{ role: string; content: string }> = [];
      if (systemPrompt) {
        apiMessages.push({ role: 'system', content: systemPrompt });
      }
      apiMessages.push({ role: 'user', content: promptContent });

      const res = (await playgroundApi.compareModels({
        models: compareModels,
        messages: apiMessages,
        temperature,
        max_tokens: maxTokens,
      })) as any;

      const results = res?.results || [];
      const comparison = res?.comparison || null;
      set({ compareResults: results, comparison });
    } catch {
      // Error handled by UI
    } finally {
      set({ isComparing: false });
    }
  },

  setCompareModels: (models) => set({ compareModels: models }),
  setShowComparePanel: (show) => set({ showComparePanel: show }),

  // ===== History & Stats Actions =====

  loadHistory: async () => {
    try {
      const res = (await playgroundApi.getHistory({ limit: 50 })) as any;
      const histories = res?.histories || [];
      set({ history: histories });
    } catch {
      // Silent fail
    }
  },

  loadStats: async () => {
    try {
      const res = (await playgroundApi.getStats()) as any;
      set({ stats: res || null });
    } catch {
      // Silent fail
    }
  },

  loadHistorySession: (history: PlaygroundHistory) => {
    const restoredMessages: PlaygroundMessage[] = [];

    // Convert history messages
    if (history.messages) {
      for (const msg of history.messages) {
        if (msg.role === 'system') continue;
        restoredMessages.push({
          id: generateId(),
          role: msg.role as 'user' | 'assistant',
          content: msg.content,
          model: history.model,
          timestamp: Number(history.created_at) * 1000 || Date.now(),
        });
      }
    }

    // If there's a result, update the last assistant message with metadata
    if (history.result && restoredMessages.length > 0) {
      const lastIdx = restoredMessages.length - 1;
      if (restoredMessages[lastIdx].role === 'assistant') {
        restoredMessages[lastIdx] = {
          ...restoredMessages[lastIdx],
          model: history.result.model || history.model,
          total_tokens: history.result.total_tokens,
          input_tokens: history.result.input_tokens,
          output_tokens: history.result.output_tokens,
          cost: history.result.cost,
          latency: history.result.latency,
          finish_reason: history.result.finish_reason,
        };
      }
    }

    set({
      messages: restoredMessages,
      model: history.model || 'qwen-plus',
      temperature: history.temperature || 0.7,
      maxTokens: Number(history.max_tokens) || 2048,
      topP: history.topP || 1.0,
      sessionId: history.session_id || null,
    });
  },
}));
