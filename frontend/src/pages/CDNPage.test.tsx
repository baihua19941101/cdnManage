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

  it('submits URL refresh request with project bindings defaults', async () => {
    const getMock = vi.spyOn(apiClient, 'get').mockImplementation(async (url) => {
      if (url === '/projects') {
        return {
          data: {
            code: 'success',
            message: 'ok',
            data: [{ id: 8, name: 'Demo Project' }],
          },
        } as never
      }
      if (url === '/projects/8') {
        return {
          data: {
            code: 'success',
            message: 'ok',
            data: {
              id: 8,
              cdns: [
                { cdnEndpoint: 'https://primary.cdn.example.com', isPrimary: true },
                { cdnEndpoint: 'https://backup.cdn.example.com', isPrimary: false },
              ],
              buckets: [{ bucketName: 'assets-primary', isPrimary: true }],
            },
          },
        } as never
      }
      throw new Error(`Unexpected GET url: ${url}`)
    })

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

    fireEvent.mouseDown(screen.getAllByRole('combobox')[0])
    fireEvent.click(await screen.findByText('8 - Demo Project'))
    fireEvent.change(screen.getByLabelText('URLs（每行一个）'), {
      target: { value: ' https://cdn.example.com/a.js \n\nhttps://cdn.example.com/b.css  ' },
    })
    fireEvent.click(screen.getByRole('button', { name: /提交 URL 刷新/ }))

    await waitFor(() => {
      expect(postMock).toHaveBeenCalledWith('/projects/8/cdns/refresh-url', {
        cdnEndpoint: 'https://primary.cdn.example.com',
        urls: ['https://cdn.example.com/a.js', 'https://cdn.example.com/b.css'],
      })
    })

    expect(getMock).toHaveBeenCalledWith('/projects')
    expect(getMock).toHaveBeenCalledWith('/projects/8')
    expect(await screen.findByText('task-url-1')).toBeInTheDocument()
    expect(screen.getByText('accepted')).toBeInTheDocument()
  })

  it('submits sync request with project bucket default', async () => {
    vi.spyOn(apiClient, 'get').mockImplementation(async (url) => {
      if (url === '/projects') {
        return {
          data: {
            code: 'success',
            message: 'ok',
            data: [{ id: 8, name: 'Sync Project' }],
          },
        } as never
      }
      if (url === '/projects/8') {
        return {
          data: {
            code: 'success',
            message: 'ok',
            data: {
              id: 8,
              cdns: [{ cdnEndpoint: 'https://cdn.sync.example.com', isPrimary: true }],
              buckets: [
                { bucketName: 'bucket-main', isPrimary: true },
                { bucketName: 'bucket-backup', isPrimary: false },
              ],
            },
          },
        } as never
      }
      throw new Error(`Unexpected GET url: ${url}`)
    })

    const postMock = vi.spyOn(apiClient, 'post').mockResolvedValueOnce({
        data: {
          code: 'success',
          message: 'ok',
          data: {
            taskId: 'task-sync-1',
            status: 'accepted',
          },
        },
      } as never)

    render(<CDNPage />)

    fireEvent.mouseDown(screen.getAllByRole('combobox')[0])
    fireEvent.click(await screen.findByText('8 - Sync Project'))
    fireEvent.change(screen.getByLabelText('Paths（每行一个）'), {
      target: { value: ' dist/app.js \n\ndist/app.css ' },
    })
    fireEvent.click(screen.getByRole('button', { name: /提交资源同步/ }))

    await waitFor(() => {
      expect(postMock).toHaveBeenCalledWith('/projects/8/cdns/sync', {
        cdnEndpoint: 'https://cdn.sync.example.com',
        bucketName: 'bucket-main',
        paths: ['dist/app.js', 'dist/app.css'],
      })
    })

    expect(await screen.findByText('task-sync-1')).toBeInTheDocument()
  })

  it('disables URL refresh button when project has no CDN bindings', async () => {
    vi.spyOn(apiClient, 'get').mockImplementation(async (url) => {
      if (url === '/projects') {
        return {
          data: {
            code: 'success',
            message: 'ok',
            data: [{ id: 9, name: 'No CDN Project' }],
          },
        } as never
      }
      if (url === '/projects/9') {
        return {
          data: {
            code: 'success',
            message: 'ok',
            data: {
              id: 9,
              cdns: [],
              buckets: [{ bucketName: 'bucket-only', isPrimary: true }],
            },
          },
        } as never
      }
      throw new Error(`Unexpected GET url: ${url}`)
    })

    render(<CDNPage />)

    fireEvent.mouseDown(screen.getAllByRole('combobox')[0])
    fireEvent.click(await screen.findByText('9 - No CDN Project'))

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /提交 URL 刷新/ })).toBeDisabled()
    })
  })

  it('disables sync button when project has no bucket bindings', async () => {
    vi.spyOn(apiClient, 'get').mockImplementation(async (url) => {
      if (url === '/projects') {
        return {
          data: {
            code: 'success',
            message: 'ok',
            data: [{ id: 10, name: 'No Bucket Project' }],
          },
        } as never
      }
      if (url === '/projects/10') {
        return {
          data: {
            code: 'success',
            message: 'ok',
            data: {
              id: 10,
              cdns: [{ cdnEndpoint: 'https://cdn.no-bucket.example.com', isPrimary: true }],
              buckets: [],
            },
          },
        } as never
      }
      throw new Error(`Unexpected GET url: ${url}`)
    })

    render(<CDNPage />)

    fireEvent.mouseDown(screen.getAllByRole('combobox')[0])
    fireEvent.click(await screen.findByText('10 - No Bucket Project'))

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /提交资源同步/ })).toBeDisabled()
    })
  })
})
