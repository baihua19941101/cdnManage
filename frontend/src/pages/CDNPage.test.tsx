import { act } from 'react'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { CDNPage } from './CDNPage'
import { apiClient } from '../services/api/client'
import { useAuthStore } from '../store/auth'

describe('CDNPage refresh interactions', () => {
  beforeEach(() => {
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
  })

  afterEach(() => {
    vi.restoreAllMocks()
    cleanup()
    act(() => {
      useAuthStore.getState().clearSession()
      useAuthStore.getState().setInitialized(false)
    })
  })

  it('submits URL refresh request with normalized urls', async () => {
    const postMock = vi.spyOn(apiClient, 'post').mockResolvedValueOnce({
      data: {
        code: 'success',
        message: 'ok',
        data: {
          taskId: 'task-url-1',
          status: 'accepted',
        },
      },
    } as never)

    render(<CDNPage />)

    fireEvent.change(screen.getByPlaceholderText('例如 1'), { target: { value: '8' } })
    fireEvent.change(screen.getByLabelText('URLs（每行一个）'), {
      target: { value: ' https://cdn.example.com/a.js \n\nhttps://cdn.example.com/b.css  ' },
    })
    fireEvent.click(screen.getByRole('button', { name: /提交 URL 刷新/ }))

    await waitFor(() => {
      expect(postMock).toHaveBeenCalledWith('/projects/8/cdns/refresh-url', {
        cdnEndpoint: '',
        urls: ['https://cdn.example.com/a.js', 'https://cdn.example.com/b.css'],
      })
    })

    expect(await screen.findByText('task-url-1')).toBeInTheDocument()
    expect(screen.getByText('accepted')).toBeInTheDocument()
  })

  it('submits directory refresh request with normalized directories', async () => {
    const postMock = vi
      .spyOn(apiClient, 'post')
      .mockResolvedValueOnce({
        data: {
          code: 'success',
          message: 'ok',
          data: {
            taskId: 'task-dir-1',
            status: 'accepted',
          },
        },
      } as never)

    render(<CDNPage />)

    fireEvent.change(screen.getByPlaceholderText('例如 1'), { target: { value: '8' } })
    fireEvent.change(screen.getByLabelText('Directories（每行一个）'), {
      target: { value: ' /static/ \n\n/assets/images/ ' },
    })
    fireEvent.click(screen.getByRole('button', { name: /提交目录刷新/ }))

    await waitFor(() => {
      expect(postMock).toHaveBeenCalledWith('/projects/8/cdns/refresh-directory', {
        cdnEndpoint: '',
        directories: ['/static/', '/assets/images/'],
      })
    })

    expect(await screen.findByText('task-dir-1')).toBeInTheDocument()
  })
})
