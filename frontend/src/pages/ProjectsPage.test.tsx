import { AxiosError } from 'axios'
import { act } from 'react'
import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { ProjectsPage } from './ProjectsPage'
import { apiClient } from '../services/api/client'
import { useAuthStore } from '../store/auth'

describe('ProjectsPage provider registration errors', () => {
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

  it('shows field-level error for provider_not_registered details', async () => {
    vi.spyOn(apiClient, 'get').mockImplementation(async (url) => {
      if (url === '/projects') {
        return {
          data: {
            code: 'success',
            message: 'ok',
            data: [],
          },
        } as never
      }
      throw new Error(`unexpected GET ${String(url)}`)
    })

    const postMock = vi.spyOn(apiClient, 'post').mockRejectedValueOnce(
      new AxiosError(
        'bad request',
        'ERR_BAD_REQUEST',
        undefined,
        undefined,
        {
          status: 400,
          statusText: 'Bad Request',
          headers: {},
          config: {} as never,
          data: {
            code: 'provider_not_registered',
            message: 'binding provider is not registered',
            details: {
              bindingType: 'buckets',
              bindingIndex: 0,
              bindingPath: 'buckets[0].providerType',
              providerType: 'aliyun',
              providerService: 'object_storage',
            },
          },
        },
      ),
    )

    render(<ProjectsPage />)

    const createButton = await screen.findByText('新建项目')
    fireEvent.click(createButton.closest('button') as HTMLButtonElement)

    const modal = await screen.findByRole('dialog')
    fireEvent.change(within(modal).getByLabelText('项目名称'), { target: { value: 'demo project' } })
    fireEvent.change(within(modal).getByLabelText('项目描述'), { target: { value: 'demo desc' } })

    const addBucketBinding = await within(modal).findByText('添加存储桶绑定')
    fireEvent.click(addBucketBinding.closest('button') as HTMLButtonElement)

    fireEvent.change(within(modal).getByLabelText('BucketName'), { target: { value: 'bucket-demo' } })
    fireEvent.change(within(modal).getByLabelText('Region'), { target: { value: 'cn-hangzhou' } })
    fireEvent.change(within(modal).getByLabelText('AccessKeyId'), { target: { value: 'AKID_TEST' } })
    fireEvent.change(within(modal).getByLabelText('AccessKeySecret'), { target: { value: 'AK_SECRET_TEST' } })
    fireEvent.click(within(modal).getByRole('button', { name: 'OK' }))

    await waitFor(() => {
      expect(postMock).toHaveBeenCalledTimes(1)
    })

    expect(await screen.findByText('存储桶绑定 #1 的 Provider（aliyun）未在当前服务实例注册。')).toBeInTheDocument()
  }, 20000)

  it('shows field-level error for provider_change_requires_credential_replace details', async () => {
    vi.spyOn(apiClient, 'get').mockImplementation(async (url) => {
      if (url === '/projects') {
        return {
          data: {
            code: 'success',
            message: 'ok',
            data: [],
          },
        } as never
      }
      throw new Error(`unexpected GET ${String(url)}`)
    })

    const postMock = vi.spyOn(apiClient, 'post').mockRejectedValueOnce(
      new AxiosError(
        'bad request',
        'ERR_BAD_REQUEST',
        undefined,
        undefined,
        {
          status: 400,
          statusText: 'Bad Request',
          headers: {},
          config: {} as never,
          data: {
            code: 'provider_change_requires_credential_replace',
            message: 'provider type change requires credential replacement',
            details: {
              bindingType: 'buckets',
              bindingIndex: 0,
              bindingPath: 'buckets[0].credentialOperation',
            },
          },
        },
      ),
    )

    render(<ProjectsPage />)

    const createButton = await screen.findByText('新建项目')
    fireEvent.click(createButton.closest('button') as HTMLButtonElement)

    const modal = await screen.findByRole('dialog')
    fireEvent.change(within(modal).getByLabelText('项目名称'), { target: { value: 'demo project' } })
    fireEvent.change(within(modal).getByLabelText('项目描述'), { target: { value: 'demo desc' } })
    const addBucketBinding = await within(modal).findByText('添加存储桶绑定')
    fireEvent.click(addBucketBinding.closest('button') as HTMLButtonElement)
    fireEvent.change(within(modal).getByLabelText('BucketName'), { target: { value: 'bucket-demo' } })
    fireEvent.change(within(modal).getByLabelText('Region'), { target: { value: 'cn-hangzhou' } })
    fireEvent.change(within(modal).getByLabelText('AccessKeyId'), { target: { value: 'AKID_TEST' } })
    fireEvent.change(within(modal).getByLabelText('AccessKeySecret'), { target: { value: 'AK_SECRET_TEST' } })
    fireEvent.click(within(modal).getByRole('button', { name: 'OK' }))

    await waitFor(() => {
      expect(postMock).toHaveBeenCalledTimes(1)
    })

    expect(await screen.findByText('存储桶绑定 #1 修改了 Provider，需开启“更新凭据”并填写 AK/SK。')).toBeInTheDocument()
  }, 20000)

  it('shows field-level error for credential_missing_for_new_binding details', async () => {
    vi.spyOn(apiClient, 'get').mockImplementation(async (url) => {
      if (url === '/projects') {
        return {
          data: {
            code: 'success',
            message: 'ok',
            data: [],
          },
        } as never
      }
      throw new Error(`unexpected GET ${String(url)}`)
    })

    const postMock = vi.spyOn(apiClient, 'post').mockRejectedValueOnce(
      new AxiosError(
        'bad request',
        'ERR_BAD_REQUEST',
        undefined,
        undefined,
        {
          status: 400,
          statusText: 'Bad Request',
          headers: {},
          config: {} as never,
          data: {
            code: 'credential_missing_for_new_binding',
            message: 'new binding requires credential replacement',
            details: {
              bindingType: 'buckets',
              bindingIndex: 0,
              bindingPath: 'buckets[0].credentialOperation',
            },
          },
        },
      ),
    )

    render(<ProjectsPage />)

    const createButton = await screen.findByText('新建项目')
    fireEvent.click(createButton.closest('button') as HTMLButtonElement)

    const modal = await screen.findByRole('dialog')
    fireEvent.change(within(modal).getByLabelText('项目名称'), { target: { value: 'demo project' } })
    fireEvent.change(within(modal).getByLabelText('项目描述'), { target: { value: 'demo desc' } })

    const addBucketBinding = await within(modal).findByText('添加存储桶绑定')
    fireEvent.click(addBucketBinding.closest('button') as HTMLButtonElement)
    fireEvent.change(within(modal).getByLabelText('BucketName'), { target: { value: 'bucket-demo' } })
    fireEvent.change(within(modal).getByLabelText('Region'), { target: { value: 'cn-hangzhou' } })
    fireEvent.change(within(modal).getByLabelText('AccessKeyId'), { target: { value: 'AKID_TEST' } })
    fireEvent.change(within(modal).getByLabelText('AccessKeySecret'), { target: { value: 'AK_SECRET_TEST' } })
    fireEvent.click(within(modal).getByRole('button', { name: 'OK' }))

    await waitFor(() => {
      expect(postMock).toHaveBeenCalledTimes(1)
    })

    expect(await screen.findByText('存储桶绑定 #1 为新增绑定，必须填写 AK/SK。')).toBeInTheDocument()
  }, 20000)

  it('shows field-level error for credential_not_found_for_keep details', async () => {
    vi.spyOn(apiClient, 'get').mockImplementation(async (url) => {
      if (url === '/projects') {
        return {
          data: {
            code: 'success',
            message: 'ok',
            data: [],
          },
        } as never
      }
      throw new Error(`unexpected GET ${String(url)}`)
    })

    const postMock = vi.spyOn(apiClient, 'post').mockRejectedValueOnce(
      new AxiosError(
        'bad request',
        'ERR_BAD_REQUEST',
        undefined,
        undefined,
        {
          status: 400,
          statusText: 'Bad Request',
          headers: {},
          config: {} as never,
          data: {
            code: 'credential_not_found_for_keep',
            message: 'historical credential was not found for keep operation',
            details: {
              bindingType: 'buckets',
              bindingIndex: 0,
              bindingPath: 'buckets[0].credentialOperation',
            },
          },
        },
      ),
    )

    render(<ProjectsPage />)

    const createButton = await screen.findByText('新建项目')
    fireEvent.click(createButton.closest('button') as HTMLButtonElement)

    const modal = await screen.findByRole('dialog')
    fireEvent.change(within(modal).getByLabelText('项目名称'), { target: { value: 'demo project' } })
    fireEvent.change(within(modal).getByLabelText('项目描述'), { target: { value: 'demo desc' } })

    const addBucketBinding = await within(modal).findByText('添加存储桶绑定')
    fireEvent.click(addBucketBinding.closest('button') as HTMLButtonElement)
    fireEvent.change(within(modal).getByLabelText('BucketName'), { target: { value: 'bucket-demo' } })
    fireEvent.change(within(modal).getByLabelText('Region'), { target: { value: 'cn-hangzhou' } })
    fireEvent.change(within(modal).getByLabelText('AccessKeyId'), { target: { value: 'AKID_TEST' } })
    fireEvent.change(within(modal).getByLabelText('AccessKeySecret'), { target: { value: 'AK_SECRET_TEST' } })
    fireEvent.click(within(modal).getByRole('button', { name: 'OK' }))

    await waitFor(() => {
      expect(postMock).toHaveBeenCalledTimes(1)
    })

    expect(await screen.findByText('存储桶绑定 #1 无法保留历史凭据，请切换为“更新凭据”。')).toBeInTheDocument()
  }, 20000)
})
