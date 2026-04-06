import { act } from 'react'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { StoragePage } from './StoragePage'
import { apiClient } from '../services/api/client'
import { useAuthStore } from '../store/auth'

const queryButtonLabel = '查询对象'

describe('StoragePage core interactions', () => {
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

  it('queries object list with expected params', async () => {
    const getMock = vi.spyOn(apiClient, 'get').mockResolvedValueOnce({
      data: {
        code: 'success',
        message: 'ok',
        data: {
          objects: [
            { key: 'dist/app.js', size: 12, isDir: false },
            { key: 'dist/app.css', size: 18, isDir: false },
          ],
        },
      },
    } as never)

    render(<StoragePage />)

    fireEvent.change(screen.getByPlaceholderText('例如 1'), { target: { value: '9' } })
    fireEvent.change(screen.getByPlaceholderText('bucket-name'), { target: { value: 'demo-bucket' } })
    fireEvent.change(screen.getByPlaceholderText('可选目录前缀'), { target: { value: 'dist/' } })
    fireEvent.click(screen.getByText(queryButtonLabel))

    await waitFor(() => {
      expect(getMock).toHaveBeenCalledWith('/projects/9/storage/objects', {
        params: {
          bucketName: 'demo-bucket',
          prefix: 'dist/',
        },
      })
    })

    expect(await screen.findByText('dist/app.js')).toBeInTheDocument()
    expect(screen.getByText('dist/app.css')).toBeInTheDocument()
  })

  it('renames object then refreshes list', async () => {
    const getMock = vi
      .spyOn(apiClient, 'get')
      .mockResolvedValueOnce({
        data: {
          code: 'success',
          message: 'ok',
          data: { objects: [{ key: 'dist/old.js', size: 11, isDir: false }] },
        },
      } as never)
      .mockResolvedValueOnce({
        data: {
          code: 'success',
          message: 'ok',
          data: { objects: [{ key: 'dist/new.js', size: 22, isDir: false }] },
        },
      } as never)
    const putMock = vi.spyOn(apiClient, 'put').mockResolvedValueOnce({
      data: { code: 'success', message: 'ok', data: { message: 'object renamed' } },
    } as never)

    render(<StoragePage />)

    fireEvent.change(screen.getByPlaceholderText('例如 1'), { target: { value: '3' } })
    fireEvent.change(screen.getByPlaceholderText('bucket-name'), { target: { value: 'demo-bucket' } })
    fireEvent.click(screen.getByText(queryButtonLabel))
    await screen.findByText('dist/old.js')

    fireEvent.click(screen.getByRole('button', { name: /重命名/ }))
    fireEvent.change(screen.getByLabelText('目标对象 Key'), { target: { value: 'dist/new.js' } })
    fireEvent.click(screen.getByRole('button', { name: /OK|确定|确 定/ }))

    await waitFor(() => {
      expect(putMock).toHaveBeenCalledTimes(1)
    })

    const putArgs = putMock.mock.calls[0]
    expect(putArgs[0]).toBe('/projects/3/storage/rename')
    expect(putArgs[1]).toEqual({
      bucketName: 'demo-bucket',
      sourceKey: 'dist/old.js',
      targetKey: 'dist/new.js',
    })

    await waitFor(() => {
      expect(getMock).toHaveBeenCalledTimes(2)
    })
    expect(await screen.findByText('dist/new.js')).toBeInTheDocument()
  }, 15000)
})
