import { act } from 'react'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { StoragePage } from './StoragePage'
import { apiClient } from '../services/api/client'
import { useAuthStore } from '../store/auth'

const queryButtonLabel = '查询对象'

const selectProjectAndBucket = async (projectLabel: string, bucketLabel: string) => {
  const comboboxes = screen.getAllByRole('combobox')
  fireEvent.mouseDown(comboboxes[0])
  fireEvent.click(await screen.findByText(projectLabel))
  await waitFor(() => {
    expect(screen.getAllByText(bucketLabel).length).toBeGreaterThan(0)
  })
}

describe('StoragePage rename interactions', () => {
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

  it('renames file object then refreshes list', async () => {
    let objectListRequestCount = 0
    const getMock = vi.spyOn(apiClient, 'get').mockImplementation(async (url, config) => {
      if (url === '/storage/upload-policy') {
        return {
          data: {
            code: 'success',
            message: 'ok',
            data: { maxUploadSizeBytes: 20 * 1024 * 1024 },
          },
        } as never
      }
      if (url === '/projects') {
        return {
          data: { code: 'success', message: 'ok', data: [{ id: 3, name: 'Demo Project' }] },
        } as never
      }
      if (url === '/projects/3') {
        return {
          data: {
            code: 'success',
            message: 'ok',
            data: { id: 3, buckets: [{ bucketName: 'demo-bucket' }] },
          },
        } as never
      }
      if (url === '/projects/3/storage/objects') {
        objectListRequestCount += 1
        if (objectListRequestCount === 1) {
          return {
            data: {
              code: 'success',
              message: 'ok',
              data: { objects: [{ key: 'old.js', size: 11, isDir: false }] },
            },
          } as never
        }
        return {
          data: {
            code: 'success',
            message: 'ok',
            data: { objects: [{ key: 'new.js', size: 22, isDir: false }] },
          },
        } as never
      }
      throw new Error(`unexpected GET ${String(url)} with ${JSON.stringify(config)}`)
    })
    const putMock = vi.spyOn(apiClient, 'put').mockResolvedValueOnce({
      data: { code: 'success', message: 'ok', data: { message: 'object renamed' } },
    } as never)

    render(<StoragePage />)
    await selectProjectAndBucket('3 - Demo Project', 'demo-bucket')

    fireEvent.click(screen.getByText(queryButtonLabel))
    await screen.findByText('old.js')

    fireEvent.click(screen.getByRole('button', { name: /重命名/ }))
    fireEvent.change(screen.getByLabelText('目标对象 Key'), { target: { value: 'new.js' } })
    fireEvent.click(screen.getByRole('button', { name: /OK|确定|确 定/ }))

    await waitFor(() => {
      expect(putMock).toHaveBeenCalledTimes(1)
    })
    expect(putMock).toHaveBeenCalledWith('/projects/3/storage/rename', {
      bucketName: 'demo-bucket',
      sourceKey: 'old.js',
      targetKey: 'new.js',
    })

    await waitFor(() => {
      expect(objectListRequestCount).toBe(2)
    })
    expect(getMock).toHaveBeenCalledWith('/projects/3/storage/objects', {
      params: {
        bucketName: 'demo-bucket',
        prefix: undefined,
        maxKeys: 200,
      },
    })
    expect(await screen.findByText('new.js')).toBeInTheDocument()
  }, 15000)

  it('shows directory summary when directory rename returns partial failure', async () => {
    let objectListRequestCount = 0
    vi.spyOn(apiClient, 'get').mockImplementation(async (url) => {
      if (url === '/storage/upload-policy') {
        return {
          data: {
            code: 'success',
            message: 'ok',
            data: { maxUploadSizeBytes: 20 * 1024 * 1024 },
          },
        } as never
      }
      if (url === '/projects') {
        return {
          data: { code: 'success', message: 'ok', data: [{ id: 5, name: 'Dir Project' }] },
        } as never
      }
      if (url === '/projects/5') {
        return {
          data: {
            code: 'success',
            message: 'ok',
            data: { id: 5, buckets: [{ bucketName: 'demo-bucket' }] },
          },
        } as never
      }
      if (url === '/projects/5/storage/objects') {
        objectListRequestCount += 1
        if (objectListRequestCount === 1) {
          return {
            data: {
              code: 'success',
              message: 'ok',
              data: { objects: [{ key: 'assets/', size: 0, isDir: true }] },
            },
          } as never
        }
        return {
          data: {
            code: 'success',
            message: 'ok',
            data: { objects: [{ key: 'release/', size: 0, isDir: true }] },
          },
        } as never
      }
      throw new Error(`unexpected GET ${String(url)}`)
    })

    const putMock = vi.spyOn(apiClient, 'put').mockResolvedValueOnce({
      data: {
        code: 'success',
        message: 'ok',
        data: {
          message: 'directory rename partially failed',
          summary: {
            sourceKey: 'assets/',
            targetKey: 'release/',
            targetType: 'directory',
            result: 'failure',
            migratedObjects: 3,
            failedObjects: 1,
            failureReasons: ['assets/logo.png: object rename failed'],
          },
        },
      },
    } as never)

    render(<StoragePage />)
    await selectProjectAndBucket('5 - Dir Project', 'demo-bucket')

    fireEvent.click(screen.getByText(queryButtonLabel))
    await screen.findByText('assets')

    fireEvent.click(screen.getByRole('button', { name: /重命名/ }))
    expect(screen.getByText('目录重命名会迁移该目录前缀下全部对象到目标目录前缀。')).toBeInTheDocument()

    fireEvent.change(screen.getByLabelText('目标目录前缀'), { target: { value: 'release/' } })
    fireEvent.click(screen.getByRole('button', { name: /OK|确定|确 定/ }))

    await waitFor(() => {
      expect(putMock).toHaveBeenCalledTimes(1)
    })
    expect(putMock).toHaveBeenCalledWith('/projects/5/storage/rename', {
      bucketName: 'demo-bucket',
      sourceKey: 'assets/',
      targetKey: 'release/',
    })

    expect(await screen.findByText(/目录重命名完成：迁移 3，失败 1/)).toBeInTheDocument()
    await waitFor(() => {
      expect(objectListRequestCount).toBe(2)
    })
  }, 15000)
})
