import { create } from 'zustand'

export type PlatformRole = 'super_admin' | 'platform_admin' | 'standard_user'
export type UserStatus = 'active' | 'disabled'

export type AuthUser = {
  id: number
  email: string
  platformRole: PlatformRole
  status: UserStatus
}

type AuthSessionPayload = {
  token: string | null
  user: AuthUser | null
}

type AuthState = {
  token: string | null
  user: AuthUser | null
  isLoggedIn: boolean
  isInitialized: boolean
  setSession: (payload: AuthSessionPayload) => void
  setToken: (token: string | null) => void
  setUser: (user: AuthUser | null) => void
  setInitialized: (value: boolean) => void
  clearSession: () => void
}

export const useAuthStore = create<AuthState>((set) => ({
  token: null,
  user: null,
  isLoggedIn: false,
  isInitialized: false,
  setSession: ({ token, user }) =>
    set({
      token,
      user,
      isLoggedIn: Boolean(token),
      isInitialized: true,
    }),
  setToken: (token) =>
    set((state) => ({
      token,
      isLoggedIn: Boolean(token),
      user: token ? state.user : null,
    })),
  setUser: (user) =>
    set((state) => ({
      user,
      isLoggedIn: Boolean(state.token),
    })),
  setInitialized: (value) => set({ isInitialized: value }),
  clearSession: () =>
    set({
      token: null,
      user: null,
      isLoggedIn: false,
    }),
}))
