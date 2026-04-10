import { act } from 'react'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { AuditsPage } from './AuditsPage'
import { apiClient } from '../services/api/client'
import { useAuthStore } from '../store/auth'

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

  it('queries project audits with filter params', async () => {
    const getMock = vi.spyOn(apiClient, 'get').mockResolvedValueOnce({
      data: {
        code: 'success',
        message: 'ok',
        data: {
          logs: [
            {
              id: 1,
              actorUserId: 1001,
              actorUsername: 'admin',
              action: 'object.upload',
              targetType: 'object',
              targetIdentifier: 'dist/app.js',
              result: 'success',
              requestId: 'req-001',
              createdAt: '2026-04-06T00:00:00Z',
            },
          ],
        },
      },
    } as never)

    render(<AuditsPage />)

    fireEvent.change(screen.getByPlaceholderText('例如 42'), { target: { value: '42' } })
    fireEvent.change(screen.getByPlaceholderText('例如 object.upload'), {
      target: { value: 'object.upload' },
    })
    fireEvent.change(screen.getByPlaceholderText('例如 object / cdn / project'), {
      target: { value: 'object' },
    })
    fireEvent.change(screen.getByPlaceholderText('支持模糊匹配'), {
      target: { value: 'dist/' },
    })

    fireEvent.click(screen.getByRole('button', { name: /查询审计日志/ }))

    await waitFor(() => {
      expect(getMock).toHaveBeenCalledWith('/projects/42/audits', {
        params: {
          action: 'object.upload',
          targetType: 'object',
          targetIdentifier: 'dist/',
          limit: 20,
          offset: 0,
        },
      })
    })

    expect(await screen.findByText('dist/app.js')).toBeInTheDocument()
    expect(screen.getByText('req-001')).toBeInTheDocument()
  })

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
})
