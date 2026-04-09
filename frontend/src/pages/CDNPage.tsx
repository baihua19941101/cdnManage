import { CloudSyncOutlined, LinkOutlined, ReloadOutlined } from '@ant-design/icons'
import {
  Alert,
  Button,
  Card,
  Descriptions,
  Form,
  Input,
  Select,
  Space,
  Typography,
  message,
} from 'antd'
import { useEffect, useState } from 'react'

import { apiClient } from '../services/api/client'
import { resolveAPIErrorMessage } from '../services/api/error'
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
  bucketName?: string
}

type URLRefreshFormValues = {
  urls: string
}

type DirectoryRefreshFormValues = {
  directories: string
}

type SyncFormValues = {
  paths: string
}

type ProjectOption = {
  id: number
  name: string
}

type ProjectDetail = {
  id: number
  buckets?: Array<{
    bucketName: string
    isPrimary?: boolean
  }>
  cdns?: Array<{
    cdnEndpoint: string
    isPrimary?: boolean
  }>
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
  const [projectOptions, setProjectOptions] = useState<ProjectOption[]>([])
  const [projectOptionsLoading, setProjectOptionsLoading] = useState(false)
  const [cdnOptions, setCDNOptions] = useState<string[]>([])
  const [bucketOptions, setBucketOptions] = useState<string[]>([])
  const [bindingsLoading, setBindingsLoading] = useState(false)

  const [urlResult, setURLResult] = useState<CDNTaskResult | null>(null)
  const [directoryResult, setDirectoryResult] = useState<CDNTaskResult | null>(null)
  const [syncResult, setSyncResult] = useState<CDNTaskResult | null>(null)
  const selectedProjectID = Form.useWatch('projectId', baseForm)

  const hasCDNBindings = cdnOptions.length > 0
  const hasBucketBindings = bucketOptions.length > 0
  const hasSelectedProject = Boolean(selectedProjectID)
  const disableURLAndDirectorySubmit = !canWrite || (hasSelectedProject && !hasCDNBindings)
  const disableSyncSubmit = !canWrite || (hasSelectedProject && !hasBucketBindings)

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
      bucketName: values.bucketName?.trim() ?? '',
    }
  }

  const pickPrimaryOrFirst = <T,>(
    items: T[],
    valueGetter: (item: T) => string | undefined,
    primaryGetter: (item: T) => boolean | undefined,
  ) => {
    const normalized = items
      .map((item) => valueGetter(item)?.trim())
      .filter((value): value is string => Boolean(value))
    if (normalized.length === 0) {
      return ''
    }
    const primary = items
      .find((item) => {
        const value = valueGetter(item)?.trim()
        return Boolean(value) && primaryGetter(item)
      })
    const primaryValue = primary ? valueGetter(primary)?.trim() : ''
    return primaryValue || normalized[0]
  }

  const loadProjectOptions = async () => {
    setProjectOptionsLoading(true)
    try {
      const response = await apiClient.get<ApiResponse<ProjectOption[]>>('/projects')
      const items = Array.isArray(response.data.data) ? response.data.data : []
      setProjectOptions(items)
    } catch (error) {
      setProjectOptions([])
      messageApi.error(resolveAPIErrorMessage(error, '项目列表加载失败。'))
    } finally {
      setProjectOptionsLoading(false)
    }
  }

  const loadBindingsByProject = async (projectID: number) => {
    if (!Number.isFinite(projectID) || projectID <= 0) {
      setCDNOptions([])
      setBucketOptions([])
      baseForm.setFieldsValue({
        cdnEndpoint: '',
        bucketName: '',
      })
      return
    }

    setBindingsLoading(true)
    try {
      const response = await apiClient.get<ApiResponse<ProjectDetail>>(`/projects/${projectID}`)
      const project = response.data.data
      const cdnList =
        Array.isArray(project?.cdns)
          ? project.cdns
              .map((cdn) => cdn.cdnEndpoint?.trim())
              .filter((endpoint): endpoint is string => Boolean(endpoint))
          : []
      const bucketList =
        Array.isArray(project?.buckets)
          ? project.buckets
              .map((bucket) => bucket.bucketName?.trim())
              .filter((name): name is string => Boolean(name))
          : []

      const nextCDN = Array.isArray(project?.cdns)
        ? pickPrimaryOrFirst(project.cdns, (item) => item.cdnEndpoint, (item) => item.isPrimary)
        : ''
      const nextBucket = Array.isArray(project?.buckets)
        ? pickPrimaryOrFirst(project.buckets, (item) => item.bucketName, (item) => item.isPrimary)
        : ''

      setCDNOptions(cdnList)
      setBucketOptions(bucketList)
      baseForm.setFieldsValue({
        cdnEndpoint: nextCDN,
        bucketName: nextBucket,
      })
    } catch (error) {
      setCDNOptions([])
      setBucketOptions([])
      baseForm.setFieldsValue({
        cdnEndpoint: '',
        bucketName: '',
      })
      messageApi.error(resolveAPIErrorMessage(error, '项目绑定加载失败。'))
    } finally {
      setBindingsLoading(false)
    }
  }

  useEffect(() => {
    void loadProjectOptions()
  }, [])

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
      messageApi.error(resolveAPIErrorMessage(error, 'URL 刷新请求提交失败。'))
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
      messageApi.error(resolveAPIErrorMessage(error, '目录刷新请求提交失败。'))
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
    if (!basePayload.bucketName) {
      messageApi.error('请选择 Bucket Name。')
      return
    }
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
          bucketName: basePayload.bucketName,
          paths,
        },
      )
      setSyncResult(response.data.data ?? null)
      messageApi.success('资源同步请求已提交。')
    } catch (error) {
      messageApi.error(resolveAPIErrorMessage(error, '资源同步请求提交失败。'))
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
            initialValues={{ projectId: '', cdnEndpoint: '', bucketName: '' }}
          >
            <Form.Item
              label="项目"
              name="projectId"
              rules={[{ required: true, message: '请选择项目' }]}
            >
              <Select
                placeholder="请选择项目"
                style={{ width: 220 }}
                loading={projectOptionsLoading}
                options={projectOptions.map((project) => ({
                  value: String(project.id),
                  label: `${project.id} - ${project.name}`,
                }))}
                onChange={(value) => {
                  const projectID = Number(value)
                  void loadBindingsByProject(projectID)
                }}
              />
            </Form.Item>
            <Form.Item label="CDN Endpoint" name="cdnEndpoint">
              <Select
                placeholder="可选，不填时使用后端 primary CDN"
                style={{ width: 320 }}
                loading={bindingsLoading}
                options={cdnOptions.map((endpoint) => ({
                  value: endpoint,
                  label: endpoint,
                }))}
                allowClear
              />
            </Form.Item>
            <Form.Item label="Bucket Name" name="bucketName">
              <Select
                placeholder="同步资源时必填"
                style={{ width: 240 }}
                loading={bindingsLoading}
                options={bucketOptions.map((bucketName) => ({
                  value: bucketName,
                  label: bucketName,
                }))}
                allowClear
              />
            </Form.Item>
          </Form>
          {hasSelectedProject && !hasCDNBindings ? (
            <Alert
              type="info"
              showIcon
              style={{ marginTop: 12 }}
              message="当前项目未绑定 CDN 域名，请先在项目配置中绑定 CDN 后再执行 URL 刷新或目录刷新。"
            />
          ) : null}
          {hasSelectedProject && !hasBucketBindings ? (
            <Alert
              type="info"
              showIcon
              style={{ marginTop: 12 }}
              message="当前项目未绑定 Bucket，请先在项目配置中绑定 Bucket 后再执行资源同步。"
            />
          ) : null}
        </Card>

        <Card
          title="URL 刷新"
          extra={
            <Button
              type="primary"
              icon={<LinkOutlined />}
              onClick={() => void submitURLRefresh()}
              loading={urlSubmitting}
              disabled={disableURLAndDirectorySubmit}
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
              disabled={disableURLAndDirectorySubmit}
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
              disabled={disableSyncSubmit}
            >
              提交资源同步
            </Button>
          }
        >
          <Form<SyncFormValues> form={syncForm} layout="vertical" initialValues={{ paths: '' }}>
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
