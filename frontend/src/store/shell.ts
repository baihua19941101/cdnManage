import type { ThemeMode } from '../app/themes'
import { create } from 'zustand'

type ShellState = {
  collapsed: boolean
  themeMode: ThemeMode
  setCollapsed: (collapsed: boolean) => void
  setThemeMode: (themeMode: ThemeMode) => void
}

export const useShellStore = create<ShellState>((set) => ({
  collapsed: false,
  themeMode: 'dark',
  setCollapsed: (collapsed) => set({ collapsed }),
  setThemeMode: (themeMode) => set({ themeMode }),
}))
