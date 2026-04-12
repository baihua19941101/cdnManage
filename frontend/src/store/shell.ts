import type { ThemeMode } from '../app/themes'
import { create } from 'zustand'
import { createJSONStorage, persist } from 'zustand/middleware'

type ShellState = {
  collapsed: boolean
  themeMode: ThemeMode
  setCollapsed: (collapsed: boolean) => void
  setThemeMode: (themeMode: ThemeMode) => void
}

export const useShellStore = create<ShellState>()(
  persist(
    (set) => ({
      collapsed: false,
      themeMode: 'light',
      setCollapsed: (collapsed) => set({ collapsed }),
      setThemeMode: (themeMode) => set({ themeMode }),
    }),
    {
      name: 'cdnmanage-shell',
      storage: createJSONStorage(() => localStorage),
      partialize: (state) => ({
        collapsed: state.collapsed,
        themeMode: state.themeMode,
      }),
    },
  ),
)
