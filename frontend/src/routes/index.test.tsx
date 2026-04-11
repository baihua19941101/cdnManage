import { act, cleanup, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it } from 'vitest'

import { AppRouter } from './index'
import { useAuthStore } from '../store/auth'
import { useShellStore } from '../store/shell'

const resetStores = () => {
  act(() => {
    useAuthStore.getState().clearSession()
    useAuthStore.getState().setInitialized(false)
    useShellStore.setState({ collapsed: false, themeMode: 'dark' })
  })
}

describe('AppRouter route guards', () => {
  beforeEach(() => {
    resetStores()
    window.history.pushState({}, '', '/')
  })

  afterEach(() => {
    cleanup()
    resetStores()
    window.history.pushState({}, '', '/')
  })

  it('redirects unauthenticated users from protected routes to login', async () => {
    window.history.pushState({}, '', '/storage')

    render(<AppRouter />)

    expect(await screen.findByText('Sign in to the control deck')).toBeInTheDocument()
  })

  it('redirects authenticated users from /login to overview', async () => {
    act(() => {
      useAuthStore.getState().setSession({
        token: 'test-token',
        user: {
          id: 1,
          email: 'admin@example.com',
          platformRole: 'platform_admin',
          status: 'active',
        },
      })
    })
    window.history.pushState({}, '', '/login')

    render(<AppRouter />)

    expect(await screen.findByText('Welcome to CDN Manage workspace')).toBeInTheDocument()
  })
})
