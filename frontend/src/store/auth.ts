import { create } from 'zustand'
import { createJSONStorage, persist } from 'zustand/middleware'

export type PlatformRole = 'super_admin' | 'platform_admin' | 'standard_user'
export type UserStatus = 'active' | 'disabled'

export type AuthUser = {
  id: number
  email: string
  platformRole: PlatformRole
  status: UserStatus
}

export const isPlatformAdminRole = (role: PlatformRole | null | undefined) =>
  role === 'super_admin' || role === 'platform_admin'

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

export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
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
    }),
    {
      name: 'cdnmanage-auth',
      storage: createJSONStorage(() => localStorage),
      partialize: (state) => ({
        token: state.token,
        user: state.user,
        isLoggedIn: state.isLoggedIn,
      }),
      onRehydrateStorage: () => (state) => {
        state?.setInitialized(true)
      },
    },
  ),
)
