import { act } from 'react'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { AuditsPage } from './AuditsPage'
import { apiClient } from '../services/api/client'
import { useAuthStore } from '../store/auth'

const selectByComboboxIndex = async (index: number, optionText: string) => {
  fireEvent.mouseDown(screen.getAllByRole('combobox')[index])
  fireEvent.click(await screen.findByText(optionText))
}

describe('AuditsPage filter query flow', () => {
  beforeEach(() => {
    act(() => {
      useAuthStore.getState().setSession({
        token: 'token',
        user: {
          id: 1,
          email: 'admin@example.com',
          platformRole: 'standard_user',
          status: 'active',
        },
      })
    })
  })

  afterEach(() => {
    vi.restoreAllMocks()
    cleanup()
    act(() => {
      useAuthStore.getState().clearSession()
      useAuthStore.getState().setInitialized(false)
    })
  })

  it(
    'loads project-level dropdown options, updates linked options when project changes, and keeps query params compatible',
    async () => {
    const getMock = vi.spyOn(apiClient, 'get').mockImplementation(async (url, config) => {
      if (url === '/projects') {
        return {
          data: {
            code: 'success',
            message: 'ok',
            data: [
              { id: 42, name: 'Project-A' },
              { id: 43, name: 'Project-B' },
            ],
          },
        } as never
      }
      if (url === '/projects/42/audits/filter-options') {
        return {
          data: {
            code: 'success',
            message: 'ok',
            data: {
              projects: [
                { projectId: 42, projectName: 'Project-A' },
                { projectId: 43, projectName: 'Project-B' },
              ],
              actions: ['object.upload'],
              targetTypes: ['object'],
            },
          },
        } as never
      }
      if (url === '/projects/43/audits/filter-options') {
        return {
          data: {
            code: 'success',
            message: 'ok',
            data: {
              projects: [
                { projectId: 43, projectName: 'Project-B' },
                { projectId: 42, projectName: 'Project-A' },
              ],
              actions: ['cdn.refresh_url'],
              targetTypes: ['cdn'],
            },
          },
        } as never
      }
      if (url === '/projects/43/audits') {
        return {
          data: {
            code: 'success',
            message: 'ok',
            data: {
              logs: [
                {
                  id: 1,
                  actorUserId: 1001,
                  actorUsername: 'admin',
                  action: 'cdn.refresh_url',
                  targetType: 'cdn',
                  targetIdentifier: 'https://cdn.example.com/app.js',
                  result: 'success',
                  requestId: 'req-001',
                  createdAt: '2026-04-06T00:00:00Z',
                },
              ],
            },
          },
        } as never
      }
      throw new Error(`Unexpected GET url: ${String(url)} ${JSON.stringify(config ?? {})}`)
    })

    render(<AuditsPage />)

    await waitFor(() => {
      expect(getMock).toHaveBeenCalledWith('/projects')
    })

    await selectByComboboxIndex(0, '42 - Project-A')
    await waitFor(() => {
      expect(getMock).toHaveBeenCalledWith('/projects/42/audits/filter-options')
    })
    fireEvent.mouseDown(screen.getAllByRole('combobox')[1])
    expect(await screen.findByRole('option', { name: 'object.upload' })).toBeInTheDocument()
    fireEvent.click(screen.getByRole('option', { name: 'object.upload' }))
    fireEvent.keyDown(document.body, { key: 'Escape' })

    await selectByComboboxIndex(0, '43 - Project-B')
    await waitFor(() => {
      expect(getMock).toHaveBeenCalledWith('/projects/43/audits/filter-options')
    })
    fireEvent.mouseDown(screen.getAllByRole('combobox')[1])
    expect(await screen.findByRole('option', { name: 'cdn.refresh_url' })).toBeInTheDocument()
    fireEvent.keyDown(document.body, { key: 'Escape' })

    fireEvent.click(screen.getByRole('button', { name: /查询审计日志/ }))

    await waitFor(() => {
      expect(getMock).toHaveBeenCalledWith('/projects/43/audits', {
        params: {
          limit: 20,
          offset: 0,
        },
      })
    })
    const projectQueryCall = getMock.mock.calls.find((call) => call[0] === '/projects/43/audits')
    expect(projectQueryCall?.[1]).toEqual({
      params: {
        limit: 20,
        offset: 0,
      },
    })

    expect(await screen.findByText('https://cdn.example.com/app.js')).toBeInTheDocument()
    expect(screen.getByText('req-001')).toBeInTheDocument()
    },
    15000,
  )

  it('loads platform filter options and keeps query available when options are empty', async () => {
    act(() => {
      useAuthStore.getState().setSession({
        token: 'token',
        user: {
          id: 1,
          email: 'admin@example.com',
          platformRole: 'platform_admin',
          status: 'active',
        },
      })
    })

    const getMock = vi.spyOn(apiClient, 'get').mockImplementation(async (url, config) => {
      if (url === '/audits/filter-options') {
        return {
          data: {
            code: 'success',
            message: 'ok',
            data: {
              actions: [],
              targetTypes: [],
            },
          },
        } as never
      }
      if (url === '/audits') {
        return {
          data: {
            code: 'success',
            message: 'ok',
            data: {
              logs: [],
            },
          },
        } as never
      }
      throw new Error(`Unexpected GET url: ${url} ${JSON.stringify(config ?? {})}`)
    })

    render(<AuditsPage />)

    await waitFor(() => {
      expect(getMock).toHaveBeenCalledWith('/audits/filter-options')
    })

    expect(screen.getByText('当前暂无 Action 可选值，可直接查询全部日志。')).toBeInTheDocument()
    expect(screen.getByText('当前暂无 Target Type 可选值，可直接查询全部日志。')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: /查询审计日志/ }))

    await waitFor(() => {
      expect(getMock).toHaveBeenCalledWith('/audits', {
        params: {
          limit: 20,
          offset: 0,
        },
      })
    })
  })

  it('shows field-level and global errors when options loading or query fails', async () => {
    act(() => {
      useAuthStore.getState().setSession({
        token: 'token',
        user: {
          id: 1,
          email: 'admin@example.com',
          platformRole: 'platform_admin',
          status: 'active',
        },
      })
    })

    const getMock = vi.spyOn(apiClient, 'get').mockImplementation(async (url) => {
      if (url === '/audits/filter-options') {
        throw new Error('failed')
      }
      if (url === '/audits') {
        throw new Error('failed')
      }
      throw new Error(`Unexpected GET url: ${url}`)
    })

    render(<AuditsPage />)

    expect(await screen.findByText('审计筛选选项加载失败，可直接查询全部日志。')).toBeInTheDocument()
    expect(screen.getByText('Action 选项加载失败，请稍后重试。')).toBeInTheDocument()
    expect(screen.getByText('Target Type 选项加载失败，请稍后重试。')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: /查询审计日志/ }))

    await waitFor(() => {
      expect(getMock).toHaveBeenCalledWith('/audits', {
        params: {
          limit: 20,
          offset: 0,
        },
      })
    })

    expect(await screen.findByText('审计日志查询失败，请稍后重试。')).toBeInTheDocument()
    expect(screen.getByText('查询失败，请检查筛选条件后重试。')).toBeInTheDocument()
  })
})
