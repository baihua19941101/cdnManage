import { act } from 'react'
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { UsersPage } from './UsersPage'
import { apiClient } from '../services/api/client'
import { useAuthStore } from '../store/auth'

describe('UsersPage readonly interactions', () => {
  beforeEach(() => {
    vi.spyOn(apiClient, 'get').mockImplementation(async (url) => {
      if (url === '/users') {
        return {
          data: {
            code: 'success',
            message: 'ok',
            data: [
              {
                id: 1,
                username: 'reader',
                email: 'reader@example.com',
                status: 'active',
                platformRole: 'standard_user',
              },
            ],
          },
        } as never
      }
      if (url === '/projects') {
        return {
          data: {
            code: 'success',
            message: 'ok',
            data: [],
          },
        } as never
      }
      throw new Error(`unexpected request: ${url}`)
    })

    act(() => {
      useAuthStore.getState().setSession({
        token: 'token',
        user: {
          id: 1,
          email: 'reader@example.com',
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

  it('disables write operation entries for readonly role', async () => {
    render(<UsersPage />)

    expect(
      await screen.findByText('当前账号为只读权限，用户写操作入口已禁用。'),
    ).toBeInTheDocument()

    await waitFor(() => {
      const createButton = screen.getByText('新建用户').closest('button')
      expect(createButton).toBeDisabled()
    })

    const editButton = screen.getByText('编辑').closest('button')
    const bindButton = screen.getByText('项目角色绑定').closest('button')
    const disableButton = screen.getByText('禁用').closest('button')
    expect(editButton).toBeDisabled()
    expect(bindButton).toBeDisabled()
    expect(disableButton).toBeDisabled()
  })
})
