import { create } from 'zustand';

interface SettingsState {
  apiBaseUrl: string;
  theme: 'light' | 'dark';
  defaultModel: string;
  language: 'zh-CN' | 'en-US';

  updateSettings: (settings: Partial<SettingsState>) => void;
  resetSettings: () => void;
}

const defaultSettings: Omit<SettingsState, 'updateSettings' | 'resetSettings'> = {
  apiBaseUrl: 'http://192.168.10.100:9000',
  theme: 'light',
  defaultModel: 'qwen3.7-max-2026-05-17',
  language: 'zh-CN',
};

export const useSettingsStore = create<SettingsState>((set) => ({
  ...defaultSettings,

  updateSettings: (settings) => {
    set((state) => ({ ...state, ...settings }));
  },

  resetSettings: () => {
    set(defaultSettings);
  },
}));