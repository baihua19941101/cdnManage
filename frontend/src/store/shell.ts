import type { ThemeMode } from '../app/themes'
import { create } from 'zustand'
import { createJSONStorage, persist } from 'zustand/middleware'

export type Locale = 'zh-CN' | 'en-US'

type ShellState = {
  collapsed: boolean
  themeMode: ThemeMode
  language: Locale
  setCollapsed: (collapsed: boolean) => void
  setThemeMode: (themeMode: ThemeMode) => void
  setLanguage: (language: Locale) => void
}

export const useShellStore = create<ShellState>()(
  persist(
    (set) => ({
      collapsed: false,
      themeMode: 'light',
      language: 'zh-CN',
      setCollapsed: (collapsed) => set({ collapsed }),
      setThemeMode: (themeMode) => set({ themeMode }),
      setLanguage: (language) => set({ language }),
    }),
    {
      name: 'cdnmanage-shell',
      storage: createJSONStorage(() => localStorage),
      partialize: (state) => ({
        collapsed: state.collapsed,
        themeMode: state.themeMode,
        language: state.language,
      }),
    },
  ),
)
