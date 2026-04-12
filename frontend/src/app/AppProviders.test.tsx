import { act } from 'react'
import { render, waitFor } from '@testing-library/react'
import { BrowserRouter } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it } from 'vitest'

import { AppProviders } from './AppProviders'
import { useShellStore } from '../store/shell'

const resetShellStore = () => {
  useShellStore.setState({
    collapsed: false,
    themeMode: 'dark',
  })
}

const renderWithRouterAt = (path: string) => {
  window.history.pushState({}, '', path)
  return render(
    <BrowserRouter>
      <AppProviders>
        <div>content</div>
      </AppProviders>
    </BrowserRouter>,
  )
}

describe('AppProviders theme dataset', () => {
  beforeEach(() => {
    resetShellStore()
    delete document.documentElement.dataset.theme
  })

  afterEach(() => {
    resetShellStore()
    delete document.documentElement.dataset.theme
  })

  it('在业务路由下根据 shell 主题切换 light/dark', async () => {
    renderWithRouterAt('/dashboard')

    await waitFor(() => {
      expect(document.documentElement.dataset.theme).toBe('dark')
    })

    act(() => {
      useShellStore.getState().setThemeMode('light')
    })

    await waitFor(() => {
      expect(document.documentElement.dataset.theme).toBe('light')
    })

    act(() => {
      useShellStore.getState().setThemeMode('dark')
    })

    await waitFor(() => {
      expect(document.documentElement.dataset.theme).toBe('dark')
    })
  })

  it('登录与初始化路由优先级高于 shell 主题（/login 与 /setup 均强制 dark）', async () => {
    act(() => {
      useShellStore.getState().setThemeMode('light')
    })
    const firstRender = renderWithRouterAt('/login')

    await waitFor(() => {
      expect(document.documentElement.dataset.theme).toBe('dark')
    })

    act(() => {
      useShellStore.getState().setThemeMode('dark')
    })

    await waitFor(() => {
      expect(document.documentElement.dataset.theme).toBe('dark')
    })

    firstRender.unmount()

    act(() => {
      useShellStore.getState().setThemeMode('light')
    })
    renderWithRouterAt('/setup')

    await waitFor(() => {
      expect(document.documentElement.dataset.theme).toBe('dark')
    })
  })
})
