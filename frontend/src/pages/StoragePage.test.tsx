import { act } from 'react'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { AxiosError } from 'axios'
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

const chooseSingleUploadFile = async (file: File) => {
  const fileInput = document.querySelector('input[type="file"]') as HTMLInputElement | null
  expect(fileInput).not.toBeNull()
  fireEvent.change(fileInput!, { target: { files: [file] } })
  await waitFor(() => {
    expect(screen.getByText(file.name)).toBeInTheDocument()
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

describe('StoragePage upload stage A interactions', () => {
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

  it('shows stage A upload progress during request', async () => {
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
          data: { code: 'success', message: 'ok', data: [{ id: 7, name: 'Upload Project' }] },
        } as never
      }
      if (url === '/projects/7') {
        return {
          data: {
            code: 'success',
            message: 'ok',
            data: { id: 7, buckets: [{ bucketName: 'demo-bucket' }] },
          },
        } as never
      }
      if (url === '/projects/7/storage/objects') {
        return { data: { code: 'success', message: 'ok', data: { objects: [] } } } as never
      }
      if (url === '/projects/7/storage/audits') {
        return { data: { code: 'success', message: 'ok', data: { logs: [] } } } as never
      }
      throw new Error(`unexpected GET ${String(url)}`)
    })

    let resolveUpload: ((value: unknown) => void) | null = null
    vi.spyOn(apiClient, 'post').mockImplementation(async (_url, _data, config) => {
      config?.onUploadProgress?.({ loaded: 5, total: 10 } as never)
      return await new Promise((resolve) => {
        resolveUpload = resolve
      })
    })

    render(<StoragePage />)
    await selectProjectAndBucket('7 - Upload Project', 'demo-bucket')
    await chooseSingleUploadFile(new File(['hello'], 'demo.txt', { type: 'text/plain' }))

    fireEvent.click(screen.getByRole('button', { name: /上\s*传|上传/ }))

    expect(await screen.findByTestId('upload-stage-a-label')).toBeInTheDocument()
    expect(screen.getByTestId('upload-stage-a-percent')).toHaveTextContent('当前传输进度：50%')

    resolveUpload?.({
      data: {
        code: 'success',
        message: 'ok',
        data: {
          summary: { success: 1, failure: 0 },
          results: [{ fileName: 'demo.txt', result: 'success' }],
        },
      },
    })

    await waitFor(() => {
      expect(screen.queryByTestId('upload-stage-a-label')).not.toBeInTheDocument()
    })
  }, 20000)

  it('polls stage B session summary by sessionId after upload', async () => {
    vi.spyOn(apiClient, 'get').mockImplementation(async (url, config) => {
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
          data: { code: 'success', message: 'ok', data: [{ id: 9, name: 'StageB Project' }] },
        } as never
      }
      if (url === '/projects/9') {
        return {
          data: {
            code: 'success',
            message: 'ok',
            data: { id: 9, buckets: [{ bucketName: 'demo-bucket' }] },
          },
        } as never
      }
      if (url === '/projects/9/storage/objects') {
        return { data: { code: 'success', message: 'ok', data: { objects: [] } } } as never
      }
      if (url === '/projects/9/storage/audits') {
        const params = (config?.params ?? {}) as Record<string, unknown>
        if (params.action === 'object.upload_archive' && params.sessionId === 'archive-123') {
          return {
            data: {
              code: 'success',
              message: 'ok',
              data: {
                logs: [
                  {
                    id: 901,
                    actorUserId: 1,
                    actorUsername: 'admin',
                    action: 'object.upload_archive',
                    targetType: 'object',
                    targetIdentifier: 'archive-123',
                    result: 'success',
                    requestId: 'req-archive-123',
                    createdAt: '2026-04-07T01:00:00Z',
                    metadata: {
                      sessionId: 'archive-123',
                      startedAt: '2026-04-07T01:00:00Z',
                      finishedAt: '2026-04-07T01:00:05Z',
                      durationMs: 5000,
                      totalEntries: 2,
                      successEntries: 2,
                      failedEntries: 0,
                    },
                  },
                ],
              },
            },
          } as never
        }
        return { data: { code: 'success', message: 'ok', data: { logs: [] } } } as never
      }
      throw new Error(`unexpected GET ${String(url)}`)
    })

    vi.spyOn(apiClient, 'post').mockResolvedValueOnce({
      data: {
        code: 'success',
        message: 'ok',
        data: {
          sessionId: 'archive-123',
          startedAt: '2026-04-07T01:00:00Z',
          summary: { success: 2, failure: 0 },
          totalEntries: 2,
          successEntries: 2,
          failedEntries: 0,
        },
      },
    } as never)

    render(<StoragePage />)
    await selectProjectAndBucket('9 - StageB Project', 'demo-bucket')
    await chooseSingleUploadFile(new File(['stage-b'], 'stage-b.txt', { type: 'text/plain' }))

    fireEvent.click(screen.getByRole('button', { name: /上\s*传|上传/ }))

    expect(await screen.findByTestId('upload-stage-b-label')).toBeInTheDocument()
    expect(screen.getByTestId('upload-stage-b-session-id')).toHaveTextContent('archive-123')
    await waitFor(() => {
      expect(screen.getByTestId('upload-stage-b-counts')).toHaveTextContent('进度：2/2；成功：2，失败：0')
    })
  }, 20000)

  it('cancels uploading and shows cancellation message', async () => {
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
          data: { code: 'success', message: 'ok', data: [{ id: 8, name: 'Cancel Project' }] },
        } as never
      }
      if (url === '/projects/8') {
        return {
          data: {
            code: 'success',
            message: 'ok',
            data: { id: 8, buckets: [{ bucketName: 'demo-bucket' }] },
          },
        } as never
      }
      if (url === '/projects/8/storage/objects') {
        return { data: { code: 'success', message: 'ok', data: { objects: [] } } } as never
      }
      if (url === '/projects/8/storage/audits') {
        return { data: { code: 'success', message: 'ok', data: { logs: [] } } } as never
      }
      throw new Error(`unexpected GET ${String(url)}`)
    })

    vi.spyOn(apiClient, 'post').mockImplementation(async (_url, _data, config) => {
      return await new Promise((_, reject) => {
        const signal = config?.signal as AbortSignal | undefined
        signal?.addEventListener('abort', () => {
          reject(new AxiosError('canceled', 'ERR_CANCELED'))
        })
      })
    })

    render(<StoragePage />)
    await selectProjectAndBucket('8 - Cancel Project', 'demo-bucket')
    await chooseSingleUploadFile(new File(['world'], 'cancel.txt', { type: 'text/plain' }))

    fireEvent.click(screen.getByRole('button', { name: /上\s*传|上传/ }))
    expect(await screen.findByRole('button', { name: '取消上传' })).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: '取消上传' }))

    expect(await screen.findByText('上传已取消。')).toBeInTheDocument()
    await waitFor(() => {
      expect(screen.queryByTestId('upload-stage-a-label')).not.toBeInTheDocument()
    })
  }, 20000)
})
