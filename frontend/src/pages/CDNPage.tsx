import { CloudSyncOutlined, LinkOutlined, ReloadOutlined } from '@ant-design/icons'
import {
  Alert,
  Button,
  Card,
  Descriptions,
  Form,
  Input,
  Space,
  Typography,
  message,
} from 'antd'
import axios from 'axios'
import { useState } from 'react'

import { apiClient } from '../services/api/client'
import { isPlatformAdminRole, useAuthStore } from '../store/auth'

type ApiResponse<T> = {
  code: string
  message: string
  requestId?: string
  data: T
}

type CDNTaskResult = {
  providerRequestId?: string
  taskId?: string
  status?: string
  submittedAt?: string
  completedAt?: string
  metadata?: Record<string, string>
}

type BaseFormValues = {
  projectId: string
  cdnEndpoint?: string
}

type URLRefreshFormValues = {
  urls: string
}

type DirectoryRefreshFormValues = {
  directories: string
}

type SyncFormValues = {
  bucketName: string
  paths: string
}

const resolveErrorMessage = (error: unknown, fallback: string) => {
  if (!axios.isAxiosError(error)) {
    return fallback
  }
  const payload = error.response?.data
  if (
    payload &&
    typeof payload === 'object' &&
    'message' in payload &&
    typeof payload.message === 'string' &&
    payload.message.trim().length > 0
  ) {
    return payload.message
  }
  return fallback
}

const splitByLines = (value: string) =>
  value
    .split('\n')
    .map((item) => item.trim())
    .filter((item) => item.length > 0)

function TaskResultCard({ result }: { result: CDNTaskResult | null }) {
  if (!result) {
    return (
      <Typography.Text type="secondary">
        暂无任务结果，提交后会展示 providerRequestId/taskId/status/submittedAt/completedAt/metadata。
      </Typography.Text>
    )
  }

  return (
    <Descriptions
      size="small"
      column={1}
      bordered
      items={[
        {
          label: 'providerRequestId',
          children: result.providerRequestId || '-',
        },
        {
          label: 'taskId',
          children: result.taskId || '-',
        },
        {
          label: 'status',
          children: result.status || '-',
        },
        {
          label: 'submittedAt',
          children: result.submittedAt || '-',
        },
        {
          label: 'completedAt',
          children: result.completedAt || '-',
        },
        {
          label: 'metadata',
          children: result.metadata ? (
            <pre style={{ margin: 0, whiteSpace: 'pre-wrap' }}>
              {JSON.stringify(result.metadata, null, 2)}
            </pre>
          ) : (
            '-'
          ),
        },
      ]}
    />
  )
}

export function CDNPage() {
  const [messageApi, messageContext] = message.useMessage()
  const [baseForm] = Form.useForm<BaseFormValues>()
  const [urlForm] = Form.useForm<URLRefreshFormValues>()
  const [directoryForm] = Form.useForm<DirectoryRefreshFormValues>()
  const [syncForm] = Form.useForm<SyncFormValues>()
  const platformRole = useAuthStore((state) => state.user?.platformRole)
  const canWrite = isPlatformAdminRole(platformRole)

  const [urlSubmitting, setURLSubmitting] = useState(false)
  const [directorySubmitting, setDirectorySubmitting] = useState(false)
  const [syncSubmitting, setSyncSubmitting] = useState(false)

  const [urlResult, setURLResult] = useState<CDNTaskResult | null>(null)
  const [directoryResult, setDirectoryResult] = useState<CDNTaskResult | null>(null)
  const [syncResult, setSyncResult] = useState<CDNTaskResult | null>(null)

  const getBasePayload = async () => {
    const values = await baseForm.validateFields()
    const projectID = Number(values.projectId)
    if (!Number.isFinite(projectID) || projectID <= 0) {
      messageApi.error('Project ID 必须是正整数。')
      return null
    }
    return {
      projectID,
      cdnEndpoint: values.cdnEndpoint?.trim() ?? '',
    }
  }

  const submitURLRefresh = async () => {
    if (!canWrite) {
      return
    }
    const basePayload = await getBasePayload()
    if (!basePayload) {
      return
    }

    const values = await urlForm.validateFields()
    const urls = splitByLines(values.urls)
    if (urls.length === 0) {
      messageApi.error('请至少输入一个 URL。')
      return
    }

    setURLSubmitting(true)
    try {
      const response = await apiClient.post<ApiResponse<CDNTaskResult>>(
        `/projects/${basePayload.projectID}/cdns/refresh-url`,
        {
          cdnEndpoint: basePayload.cdnEndpoint,
          urls,
        },
      )
      setURLResult(response.data.data ?? null)
      messageApi.success('URL 刷新请求已提交。')
    } catch (error) {
      messageApi.error(resolveErrorMessage(error, 'URL 刷新请求提交失败。'))
    } finally {
      setURLSubmitting(false)
    }
  }

  const submitDirectoryRefresh = async () => {
    if (!canWrite) {
      return
    }
    const basePayload = await getBasePayload()
    if (!basePayload) {
      return
    }

    const values = await directoryForm.validateFields()
    const directories = splitByLines(values.directories)
    if (directories.length === 0) {
      messageApi.error('请至少输入一个目录。')
      return
    }

    setDirectorySubmitting(true)
    try {
      const response = await apiClient.post<ApiResponse<CDNTaskResult>>(
        `/projects/${basePayload.projectID}/cdns/refresh-directory`,
        {
          cdnEndpoint: basePayload.cdnEndpoint,
          directories,
        },
      )
      setDirectoryResult(response.data.data ?? null)
      messageApi.success('目录刷新请求已提交。')
    } catch (error) {
      messageApi.error(resolveErrorMessage(error, '目录刷新请求提交失败。'))
    } finally {
      setDirectorySubmitting(false)
    }
  }

  const submitSyncResources = async () => {
    if (!canWrite) {
      return
    }
    const basePayload = await getBasePayload()
    if (!basePayload) {
      return
    }

    const values = await syncForm.validateFields()
    const paths = splitByLines(values.paths)
    if (paths.length === 0) {
      messageApi.error('请至少输入一个资源路径。')
      return
    }

    setSyncSubmitting(true)
    try {
      const response = await apiClient.post<ApiResponse<CDNTaskResult>>(
        `/projects/${basePayload.projectID}/cdns/sync`,
        {
          cdnEndpoint: basePayload.cdnEndpoint,
          bucketName: values.bucketName.trim(),
          paths,
        },
      )
      setSyncResult(response.data.data ?? null)
      messageApi.success('资源同步请求已提交。')
    } catch (error) {
      messageApi.error(resolveErrorMessage(error, '资源同步请求提交失败。'))
    } finally {
      setSyncSubmitting(false)
    }
  }

  return (
    <>
      {messageContext}
      <Space direction="vertical" size={16} style={{ width: '100%' }}>
        <Card title="CDN Refresh & Sync">
          {!canWrite ? (
            <Alert
              type="warning"
              showIcon
              style={{ marginBottom: 12 }}
              message="当前账号为只读权限，URL 刷新、目录刷新、资源同步提交按钮已禁用。"
            />
          ) : null}
          <Form<BaseFormValues>
            form={baseForm}
            layout="inline"
            style={{ rowGap: 12 }}
            initialValues={{ projectId: '', cdnEndpoint: '' }}
          >
            <Form.Item
              label="Project ID"
              name="projectId"
              rules={[{ required: true, message: '请输入 Project ID' }]}
            >
              <Input placeholder="例如 1" style={{ width: 160 }} />
            </Form.Item>
            <Form.Item label="CDN Endpoint" name="cdnEndpoint">
              <Input
                placeholder="可选，不填时使用后端 primary CDN"
                style={{ width: 420 }}
              />
            </Form.Item>
          </Form>
        </Card>

        <Card
          title="URL 刷新"
          extra={
            <Button
              type="primary"
              icon={<LinkOutlined />}
              onClick={() => void submitURLRefresh()}
              loading={urlSubmitting}
              disabled={!canWrite}
            >
              提交 URL 刷新
            </Button>
          }
        >
          <Form<URLRefreshFormValues> form={urlForm} layout="vertical" initialValues={{ urls: '' }}>
            <Form.Item
              label="URLs（每行一个）"
              name="urls"
              rules={[{ required: true, message: '请至少输入一个 URL' }]}
            >
              <Input.TextArea
                rows={5}
                placeholder={'https://cdn.example.com/a.js\nhttps://cdn.example.com/b.css'}
              />
            </Form.Item>
          </Form>
          <TaskResultCard result={urlResult} />
        </Card>

        <Card
          title="目录刷新"
          extra={
            <Button
              type="primary"
              icon={<ReloadOutlined />}
              onClick={() => void submitDirectoryRefresh()}
              loading={directorySubmitting}
              disabled={!canWrite}
            >
              提交目录刷新
            </Button>
          }
        >
          <Form<DirectoryRefreshFormValues>
            form={directoryForm}
            layout="vertical"
            initialValues={{ directories: '' }}
          >
            <Form.Item
              label="Directories（每行一个）"
              name="directories"
              rules={[{ required: true, message: '请至少输入一个目录' }]}
            >
              <Input.TextArea rows={5} placeholder={'/static/\n/assets/images/'} />
            </Form.Item>
          </Form>
          <TaskResultCard result={directoryResult} />
        </Card>

        <Card
          title="资源同步"
          extra={
            <Button
              type="primary"
              icon={<CloudSyncOutlined />}
              onClick={() => void submitSyncResources()}
              loading={syncSubmitting}
              disabled={!canWrite}
            >
              提交资源同步
            </Button>
          }
        >
          <Form<SyncFormValues> form={syncForm} layout="vertical" initialValues={{ bucketName: '', paths: '' }}>
            <Form.Item
              label="Bucket Name"
              name="bucketName"
              rules={[{ required: true, message: '请输入 Bucket Name' }]}
            >
              <Input placeholder="例如 project-assets" />
            </Form.Item>
            <Form.Item
              label="Paths（每行一个）"
              name="paths"
              rules={[{ required: true, message: '请至少输入一个资源路径' }]}
            >
              <Input.TextArea rows={5} placeholder={'dist/app.js\ndist/app.css'} />
            </Form.Item>
          </Form>
          <TaskResultCard result={syncResult} />
        </Card>
      </Space>
    </>
  )
}
