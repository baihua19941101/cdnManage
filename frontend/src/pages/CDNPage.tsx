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
  Tabs,
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

type OperationType = 'url' | 'directory' | 'sync'

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

const OPERATION_META: Record<
  OperationType,
  {
    tabLabel: string
    actionText: string
    fieldLabel: string
    placeholder: string
    emptyInputMessage: string
    successMessage: string
    errorMessage: string
    icon: JSX.Element
  }
> = {
  url: {
    tabLabel: 'URL 刷新',
    actionText: '提交 URL 刷新',
    fieldLabel: 'URLs（每行一个）',
    placeholder: 'https://cdn.example.com/a.js\nhttps://cdn.example.com/b.css',
    emptyInputMessage: '请至少输入一个 URL。',
    successMessage: 'URL 刷新请求已提交。',
    errorMessage: 'URL 刷新请求提交失败。',
    icon: <LinkOutlined />,
  },
  directory: {
    tabLabel: '目录刷新',
    actionText: '提交目录刷新',
    fieldLabel: 'Directories（每行一个）',
    placeholder: '/static/\n/assets/images/',
    emptyInputMessage: '请至少输入一个目录。',
    successMessage: '目录刷新请求已提交。',
    errorMessage: '目录刷新请求提交失败。',
    icon: <ReloadOutlined />,
  },
  sync: {
    tabLabel: '资源同步',
    actionText: '提交资源同步',
    fieldLabel: 'Paths（每行一个）',
    placeholder: 'dist/app.js\ndist/app.css',
    emptyInputMessage: '请至少输入一个资源路径。',
    successMessage: '资源同步请求已提交。',
    errorMessage: '资源同步请求提交失败。',
    icon: <CloudSyncOutlined />,
  },
}

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
  const [operationForm] = Form.useForm<{ operationInput: string }>()
  const platformRole = useAuthStore((state) => state.user?.platformRole)
  const canWrite = isPlatformAdminRole(platformRole)

  const [submitting, setSubmitting] = useState(false)
  const [activeOperation, setActiveOperation] = useState<OperationType>('url')
  const [operationInputs, setOperationInputs] = useState<Record<OperationType, string>>({
    url: '',
    directory: '',
    sync: '',
  })
  const [projectOptions, setProjectOptions] = useState<ProjectOption[]>([])
  const [projectOptionsLoading, setProjectOptionsLoading] = useState(false)
  const [cdnOptions, setCDNOptions] = useState<string[]>([])
  const [bucketOptions, setBucketOptions] = useState<string[]>([])
  const [bindingsLoading, setBindingsLoading] = useState(false)
  const [results, setResults] = useState<Record<OperationType, CDNTaskResult | null>>({
    url: null,
    directory: null,
    sync: null,
  })
  const selectedProjectID = Form.useWatch('projectId', baseForm)

  const hasCDNBindings = cdnOptions.length > 0
  const hasBucketBindings = bucketOptions.length > 0
  const hasSelectedProject = Boolean(selectedProjectID)
  const disableURLAndDirectorySubmit = !canWrite || (hasSelectedProject && !hasCDNBindings)
  const disableSyncSubmit = !canWrite || (hasSelectedProject && !hasBucketBindings)
  const activeMeta = OPERATION_META[activeOperation]
  const disableActiveSubmit =
    activeOperation === 'sync' ? disableSyncSubmit : disableURLAndDirectorySubmit
  const showCDNBindingError = hasSelectedProject && !hasCDNBindings
  const showBucketBindingError = hasSelectedProject && activeOperation === 'sync' && !hasBucketBindings

  const getBasePayload = async () => {
    const values = await baseForm.validateFields()
    const projectID = Number(values.projectId)
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
    const primary = items.find((item) => {
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

  useEffect(() => {
    operationForm.setFieldValue('operationInput', operationInputs[activeOperation])
  }, [activeOperation, operationForm, operationInputs])

  const submitActiveOperation = async () => {
    if (!canWrite) {
      return
    }
    let basePayload: Awaited<ReturnType<typeof getBasePayload>>
    try {
      basePayload = await getBasePayload()
      await operationForm.validateFields()
    } catch {
      return
    }
    if (!basePayload) return
    const operationValue = operationForm.getFieldValue('operationInput') ?? ''
    const entries = splitByLines(operationValue)

    setSubmitting(true)
    try {
      let response: { data: ApiResponse<CDNTaskResult> }
      if (activeOperation === 'url') {
        response = await apiClient.post<ApiResponse<CDNTaskResult>>(
          `/projects/${basePayload.projectID}/cdns/refresh-url`,
          {
            cdnEndpoint: basePayload.cdnEndpoint,
            urls: entries,
          },
        )
      } else if (activeOperation === 'directory') {
        response = await apiClient.post<ApiResponse<CDNTaskResult>>(
          `/projects/${basePayload.projectID}/cdns/refresh-directory`,
          {
            cdnEndpoint: basePayload.cdnEndpoint,
            directories: entries,
          },
        )
      } else {
        response = await apiClient.post<ApiResponse<CDNTaskResult>>(
          `/projects/${basePayload.projectID}/cdns/sync`,
          {
            cdnEndpoint: basePayload.cdnEndpoint,
            bucketName: basePayload.bucketName,
            paths: entries,
          },
        )
      }
      setResults((prev) => ({
        ...prev,
        [activeOperation]: response.data.data ?? null,
      }))
      messageApi.success(activeMeta.successMessage)
    } catch (error) {
      messageApi.error(resolveAPIErrorMessage(error, activeMeta.errorMessage))
    } finally {
      setSubmitting(false)
    }
  }

  const activeResult = results[activeOperation]

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
              rules={[
                { required: true, message: '请选择项目。' },
                {
                  validator: (_, value) => {
                    if (!value) {
                      return Promise.resolve()
                    }
                    const projectID = Number(value)
                    if (Number.isFinite(projectID) && projectID > 0) {
                      return Promise.resolve()
                    }
                    return Promise.reject(new Error('项目 ID 必须是正整数。'))
                  },
                },
              ]}
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
            <Form.Item
              label="CDN Endpoint"
              name="cdnEndpoint"
              validateStatus={showCDNBindingError ? 'error' : undefined}
              help={
                showCDNBindingError
                  ? '当前项目未绑定 CDN 域名，请先在项目配置中新增 CDN 绑定。'
                  : undefined
              }
              rules={[
                {
                  validator: (_, value) => {
                    const projectID = Number(baseForm.getFieldValue('projectId'))
                    if (!Number.isFinite(projectID) || projectID <= 0) {
                      return Promise.resolve()
                    }
                    if (!hasCDNBindings) {
                      return Promise.reject(
                        new Error('当前项目未绑定 CDN 域名，请先在项目配置中新增 CDN 绑定。'),
                      )
                    }
                    const endpoint = typeof value === 'string' ? value.trim() : ''
                    if (!endpoint) {
                      return Promise.reject(new Error('请选择 CDN 域名。'))
                    }
                    return Promise.resolve()
                  },
                },
              ]}
            >
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
            <Form.Item
              label="Bucket Name"
              name="bucketName"
              validateStatus={showBucketBindingError ? 'error' : undefined}
              help={
                showBucketBindingError
                  ? '当前项目未绑定 Bucket，请先在项目配置中新增存储桶绑定。'
                  : undefined
              }
              rules={[
                {
                  validator: (_, value) => {
                    if (activeOperation !== 'sync') {
                      return Promise.resolve()
                    }
                    const projectID = Number(baseForm.getFieldValue('projectId'))
                    if (!Number.isFinite(projectID) || projectID <= 0) {
                      return Promise.resolve()
                    }
                    if (!hasBucketBindings) {
                      return Promise.reject(
                        new Error('当前项目未绑定 Bucket，请先在项目配置中新增存储桶绑定。'),
                      )
                    }
                    const bucketName = typeof value === 'string' ? value.trim() : ''
                    if (!bucketName) {
                      return Promise.reject(new Error('请选择 Bucket Name。'))
                    }
                    return Promise.resolve()
                  },
                },
              ]}
            >
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
          title="操作区"
          extra={
            <Button
              type="primary"
              icon={activeMeta.icon}
              onClick={() => void submitActiveOperation()}
              loading={submitting}
              disabled={disableActiveSubmit}
            >
              {activeMeta.actionText}
            </Button>
          }
        >
          <Tabs
            activeKey={activeOperation}
            onChange={(key) => {
              setActiveOperation(key as OperationType)
            }}
            items={[
              { key: 'url', label: OPERATION_META.url.tabLabel },
              { key: 'directory', label: OPERATION_META.directory.tabLabel },
              { key: 'sync', label: OPERATION_META.sync.tabLabel },
            ]}
          />
          <Form form={operationForm} layout="vertical">
            <Form.Item
              label={activeMeta.fieldLabel}
              name="operationInput"
              validateTrigger={['onSubmit', 'onBlur']}
              rules={[
                {
                  validator: (_, value: string | undefined) => {
                    const entries = splitByLines(value ?? '')
                    if (entries.length === 0) {
                      return Promise.reject(new Error(activeMeta.emptyInputMessage))
                    }
                    if (activeOperation === 'url') {
                      const invalidURL = entries.find((entry) => {
                        try {
                          const parsed = new URL(entry)
                          return parsed.protocol !== 'http:' && parsed.protocol !== 'https:'
                        } catch {
                          return true
                        }
                      })
                      if (invalidURL) {
                        return Promise.reject(new Error(`URL 格式无效：${invalidURL}`))
                      }
                    }
                    if (activeOperation === 'directory') {
                      const invalidDirectory = entries.find((entry) => !entry.startsWith('/'))
                      if (invalidDirectory) {
                        return Promise.reject(
                          new Error(`目录路径必须以 "/" 开头：${invalidDirectory}`),
                        )
                      }
                    }
                    if (activeOperation === 'sync') {
                      const invalidPath = entries.find((entry) => entry.startsWith('/'))
                      if (invalidPath) {
                        return Promise.reject(
                          new Error(`资源路径不能以 "/" 开头：${invalidPath}`),
                        )
                      }
                    }
                    return Promise.resolve()
                  },
                },
              ]}
            >
              <Input.TextArea
                rows={5}
                placeholder={activeMeta.placeholder}
                aria-label={activeMeta.fieldLabel}
                value={operationInputs[activeOperation]}
                onChange={(event) => {
                  const { value } = event.target
                  operationForm.setFieldValue('operationInput', value)
                  setOperationInputs((prev) => ({
                    ...prev,
                    [activeOperation]: value,
                  }))
                }}
              />
            </Form.Item>
          </Form>
          <TaskResultCard result={activeResult} />
        </Card>
      </Space>
    </>
  )
}
