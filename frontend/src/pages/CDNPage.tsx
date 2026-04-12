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
import { type ReactNode, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useSearchParams } from 'react-router-dom'

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

type DirectoryQueryResult = {
  directories?: string[]
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
  currentProjectRole?: 'project_admin' | 'project_read_only'
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

const getOperationMeta = (
  t: (key: string) => string,
): Record<
  OperationType,
  {
    tabLabel: string
    actionText: string
    fieldLabel: string
    placeholder: string
    emptyInputMessage: string
    successMessage: string
    errorMessage: string
    icon: ReactNode
  }
> => ({
  url: {
    tabLabel: t('cdn.tabUrlRefresh'),
    actionText: t('cdn.tabUrlRefresh'),
    fieldLabel: t('cdn.urlFieldLabel'),
    placeholder: 'https://cdn.example.com/a.js\nhttps://cdn.example.com/b.css',
    emptyInputMessage: t('cdn.urlEmpty'),
    successMessage: t('cdn.urlSuccess'),
    errorMessage: t('cdn.urlError'),
    icon: <LinkOutlined />,
  },
  directory: {
    tabLabel: t('cdn.tabDirectoryRefresh'),
    actionText: t('cdn.tabDirectoryRefresh'),
    fieldLabel: t('cdn.directoryFieldLabel'),
    placeholder: '/static/\n/assets/images/',
    emptyInputMessage: t('cdn.directoryEmpty'),
    successMessage: t('cdn.directorySuccess'),
    errorMessage: t('cdn.directoryError'),
    icon: <ReloadOutlined />,
  },
  sync: {
    tabLabel: t('cdn.tabResourceSync'),
    actionText: t('cdn.tabResourceSync'),
    fieldLabel: t('cdn.syncFieldLabel'),
    placeholder: 'dist/app.js\ndist/app.css',
    emptyInputMessage: t('cdn.syncEmpty'),
    successMessage: t('cdn.syncSuccess'),
    errorMessage: t('cdn.syncError'),
    icon: <CloudSyncOutlined />,
  },
})

function TaskResultCard({ result }: { result: CDNTaskResult | null }) {
  const { t } = useTranslation()
  if (!result) {
    return (
      <Typography.Text type="secondary">
        {t('cdn.taskResultPlaceholder')}
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
  const { t } = useTranslation()
  const [searchParams] = useSearchParams()
  const [messageApi, messageContext] = message.useMessage()
  const [baseForm] = Form.useForm<BaseFormValues>()
  const [operationForm] = Form.useForm<{ operationInput: string }>()
  const platformRole = useAuthStore((state) => state.user?.platformRole)
  const isPlatformAdmin = isPlatformAdminRole(platformRole)
  const [currentProjectRole, setCurrentProjectRole] = useState<'project_admin' | 'project_read_only' | ''>('')
  const canWrite = isPlatformAdmin || currentProjectRole === 'project_admin'

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
  const [directoryQueryPrefix, setDirectoryQueryPrefix] = useState('')
  const [directoryQueryLoading, setDirectoryQueryLoading] = useState(false)
  const [directoryOptions, setDirectoryOptions] = useState<string[]>([])
  const [selectedDirectory, setSelectedDirectory] = useState<string>()
  const [queryProjectInitialized, setQueryProjectInitialized] = useState(false)
  const selectedProjectID = Form.useWatch('projectId', baseForm)

  const hasCDNBindings = cdnOptions.length > 0
  const hasBucketBindings = bucketOptions.length > 0
  const hasSelectedProject = Boolean(selectedProjectID)
  const disableURLAndDirectorySubmit = !canWrite || (hasSelectedProject && !hasCDNBindings)
  const disableSyncSubmit = !canWrite || (hasSelectedProject && !hasBucketBindings)
  const operationMeta = getOperationMeta(t)
  const activeMeta = operationMeta[activeOperation]
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
      const response = await apiClient.get<ApiResponse<ProjectOption[]>>('/projects/accessible')
      const items = Array.isArray(response.data.data) ? response.data.data : []
      setProjectOptions(items)
      return items
    } catch (error) {
      setProjectOptions([])
      messageApi.error(resolveAPIErrorMessage(error, t('cdn.loadProjectsFailed')))
      return [] as ProjectOption[]
    } finally {
      setProjectOptionsLoading(false)
    }
  }

  const loadBindingsByProject = async (projectID: number) => {
    if (!Number.isFinite(projectID) || projectID <= 0) {
      setCurrentProjectRole('')
      setCDNOptions([])
      setBucketOptions([])
      setDirectoryOptions([])
      setSelectedDirectory(undefined)
      baseForm.setFieldsValue({
        cdnEndpoint: '',
        bucketName: '',
      })
      return
    }

    setCurrentProjectRole('')
    setBindingsLoading(true)
    try {
      const response = await apiClient.get<ApiResponse<ProjectDetail>>(`/projects/${projectID}/context`)
      const project = response.data.data
      const role = project?.currentProjectRole?.trim()
      setCurrentProjectRole(role === 'project_admin' || role === 'project_read_only' ? role : '')
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
      setDirectoryOptions([])
      setSelectedDirectory(undefined)
      baseForm.setFieldsValue({
        cdnEndpoint: nextCDN,
        bucketName: nextBucket,
      })
    } catch (error) {
      setCurrentProjectRole('')
      setCDNOptions([])
      setBucketOptions([])
      setDirectoryOptions([])
      setSelectedDirectory(undefined)
      baseForm.setFieldsValue({
        cdnEndpoint: '',
        bucketName: '',
      })
      messageApi.error(resolveAPIErrorMessage(error, t('cdn.loadBindingsFailed')))
    } finally {
      setBindingsLoading(false)
    }
  }

  useEffect(() => {
    void loadProjectOptions()
  }, [])

  useEffect(() => {
    const bootstrapProjectFromQuery = async () => {
      if (queryProjectInitialized) {
        return
      }
      setQueryProjectInitialized(true)

      const projectIDFromQuery = Number(searchParams.get('projectId'))
      if (!Number.isFinite(projectIDFromQuery) || projectIDFromQuery <= 0) {
        return
      }

      const hasProjectOption = projectOptions.some((project) => project.id === projectIDFromQuery)
      const options = hasProjectOption ? projectOptions : await loadProjectOptions()
      if (!options.some((project) => project.id === projectIDFromQuery)) {
        return
      }

      baseForm.setFieldValue('projectId', String(projectIDFromQuery))
      await loadBindingsByProject(projectIDFromQuery)
    }

    void bootstrapProjectFromQuery()
  }, [baseForm, projectOptions, queryProjectInitialized, searchParams])

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

  const queryDirectoryCandidates = async () => {
    let basePayload: Awaited<ReturnType<typeof getBasePayload>>
    try {
      basePayload = await getBasePayload()
    } catch {
      return
    }
    const bucketName = basePayload.bucketName.trim()
    if (!bucketName) {
      messageApi.error(t('cdn.selectBucketBeforeQuery'))
      return
    }

    setDirectoryQueryLoading(true)
    try {
      const response = await apiClient.get<ApiResponse<DirectoryQueryResult>>(
        `/projects/${basePayload.projectID}/cdns/directories`,
        {
          params: {
            bucketName,
            prefix: directoryQueryPrefix.trim() || undefined,
          },
        },
      )
      const directories = Array.isArray(response.data.data?.directories)
        ? response.data.data?.directories?.filter(
            (item): item is string => typeof item === 'string' && item.trim().length > 0,
          ) ?? []
        : []
      setDirectoryOptions(directories)
      setSelectedDirectory(undefined)
      if (directories.length === 0) {
        messageApi.info(t('cdn.noDirectories'))
      } else {
        messageApi.success(t('cdn.directoriesFound', { count: directories.length }))
      }
    } catch (error) {
      setDirectoryOptions([])
      setSelectedDirectory(undefined)
      messageApi.error(resolveAPIErrorMessage(error, t('cdn.queryDirectoriesFailed')))
    } finally {
      setDirectoryQueryLoading(false)
    }
  }

  const appendSelectedDirectory = () => {
    const directory = (selectedDirectory ?? '').trim()
    if (!directory) {
      return
    }
    const current = operationInputs.directory.trim()
    const nextValue = current ? `${current}\n${directory}` : directory
    setOperationInputs((prev) => ({
      ...prev,
      directory: nextValue,
    }))
    if (activeOperation === 'directory') {
      operationForm.setFieldValue('operationInput', nextValue)
    }
  }

  const activeResult = results[activeOperation]

  return (
    <>
      {messageContext}
      <Space direction="vertical" size={16} style={{ width: '100%' }}>
        <Card title={t('pages.cdn.title')}>
          {!canWrite ? (
            <Alert
              type="warning"
              showIcon
              style={{ marginBottom: 12 }}
              message={t('cdn.readOnlyDisabled')}
            />
          ) : null}
          <Form<BaseFormValues>
            form={baseForm}
            layout="inline"
            style={{ rowGap: 12 }}
            initialValues={{ projectId: '', cdnEndpoint: '', bucketName: '' }}
          >
            <Form.Item
              label={t('cdn.projectLabel')}
              name="projectId"
              rules={[
                { required: true, message: t('cdn.selectProject') },
                {
                  validator: (_, value) => {
                    if (!value) {
                      return Promise.resolve()
                    }
                    const projectID = Number(value)
                    if (Number.isFinite(projectID) && projectID > 0) {
                      return Promise.resolve()
                    }
                    return Promise.reject(new Error(t('cdn.projectIdInvalid')))
                  },
                },
              ]}
            >
              <Select
                placeholder={t('cdn.selectProject')}
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
                  ? t('cdn.noCdnBindingShort')
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
                        new Error(t('cdn.noCdnBindingShort')),
                      )
                    }
                    const endpoint = typeof value === 'string' ? value.trim() : ''
                    if (!endpoint) {
                      return Promise.reject(new Error(t('cdn.selectCdn')))
                    }
                    return Promise.resolve()
                  },
                },
              ]}
            >
              <Select
                placeholder={t('cdn.cdnOptionalPlaceholder')}
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
                  ? t('cdn.noBucketBindingShort')
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
                        new Error(t('cdn.noBucketBindingShort')),
                      )
                    }
                    const bucketName = typeof value === 'string' ? value.trim() : ''
                    if (!bucketName) {
                      return Promise.reject(new Error(t('cdn.selectBucket')))
                    }
                    return Promise.resolve()
                  },
                },
              ]}
            >
              <Select
                placeholder={t('cdn.syncBucketRequired')}
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
              message={t('cdn.noCdnBindingLong')}
            />
          ) : null}
          {hasSelectedProject && !hasBucketBindings ? (
            <Alert
              type="info"
              showIcon
              style={{ marginTop: 12 }}
              message={t('cdn.noBucketBindingLong')}
            />
          ) : null}
        </Card>

        <Card
          title={t('pages.cdn.operationTitle')}
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
          {activeOperation === 'directory' ? (
            <Card
              size="small"
              title={t('pages.cdn.queryDirectory')}
              style={{ marginBottom: 16 }}
              extra={
                <Button loading={directoryQueryLoading} onClick={() => void queryDirectoryCandidates()}>
                  {t('pages.cdn.queryDirectory')}
                </Button>
              }
            >
              <Space wrap>
                <Input
                  placeholder={t('cdn.prefixPlaceholder')}
                  style={{ width: 260 }}
                  value={directoryQueryPrefix}
                  onChange={(event) => {
                    setDirectoryQueryPrefix(event.target.value)
                  }}
                />
                <Select
                  placeholder={t('cdn.selectDirectory')}
                  style={{ width: 320 }}
                  value={selectedDirectory}
                  onChange={(value) => {
                    setSelectedDirectory(value)
                  }}
                  options={directoryOptions.map((directory) => ({
                    value: directory,
                    label: directory,
                  }))}
                  allowClear
                  showSearch
                  optionFilterProp="label"
                />
                <Button onClick={appendSelectedDirectory} disabled={!selectedDirectory}>
                  {t('pages.cdn.addDirectoryInput')}
                </Button>
              </Space>
              <Typography.Paragraph type="secondary" style={{ marginTop: 12, marginBottom: 0 }}>
                {t('cdn.readonlyHint')}
              </Typography.Paragraph>
            </Card>
          ) : null}
          <Tabs
            activeKey={activeOperation}
            onChange={(key) => {
              setActiveOperation(key as OperationType)
            }}
            items={[
              { key: 'url', label: operationMeta.url.tabLabel },
              { key: 'directory', label: operationMeta.directory.tabLabel },
              { key: 'sync', label: operationMeta.sync.tabLabel },
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
                        return Promise.reject(new Error(t('cdn.urlInvalid', { value: invalidURL })))
                      }
                    }
                    if (activeOperation === 'directory') {
                      const invalidDirectory = entries.find((entry) => !entry.startsWith('/'))
                      if (invalidDirectory) {
                        return Promise.reject(
                          new Error(t('cdn.directoryMustStartSlash', { value: invalidDirectory })),
                        )
                      }
                    }
                    if (activeOperation === 'sync') {
                      const invalidPath = entries.find((entry) => entry.startsWith('/'))
                      if (invalidPath) {
                        return Promise.reject(
                          new Error(t('cdn.syncPathNoSlash', { value: invalidPath })),
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
