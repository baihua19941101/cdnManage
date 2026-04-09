import {
  DeleteOutlined,
  EditOutlined,
  PlusOutlined,
  ReloadOutlined,
} from '@ant-design/icons'
import {
  Alert,
  Button,
  Card,
  Col,
  Divider,
  Empty,
  Form,
  Input,
  List,
  message,
  Modal,
  Row,
  Select,
  Space,
  Spin,
  Switch,
  Table,
  Tag,
  Typography,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import axios from 'axios'
import { useEffect, useState } from 'react'

import { apiClient } from '../services/api/client'
import { validateProjectBindingCounts } from './managementValidation'
import { isPlatformAdminRole, useAuthStore } from '../store/auth'

type ProviderType = 'aliyun' | 'tencent_cloud' | 'huawei_cloud' | 'qiniu'
type PurgeScope = 'url' | 'directory'
type CredentialOperation = 'KEEP' | 'REPLACE'

type ProjectBucket = {
  id: number
  providerType: string
  bucketName: string
  region: string
  credentialMasked?: string
  isPrimary: boolean
}

type ProjectCDN = {
  id: number
  providerType: string
  cdnEndpoint: string
  region?: string
  credentialMasked?: string
  purgeScope: string
  isPrimary: boolean
}

type Project = {
  id: number
  name: string
  description: string
  createdAt: string
  buckets?: ProjectBucket[]
  cdns?: ProjectCDN[]
}

type ApiResponse<T> = {
  code: string
  message: string
  data: T
  details?: unknown
}

type EditProjectBucketInput = {
  id?: number
  providerType: ProviderType
  originalProviderType?: ProviderType
  bucketName: string
  region: string
  accessKeyId: string
  accessKeySecret: string
  securityToken?: string
  replaceCredential?: boolean
  credentialOperation?: CredentialOperation
  isPrimary: boolean
}

type EditProjectCDNInput = {
  id?: number
  providerType: ProviderType
  originalProviderType?: ProviderType
  cdnEndpoint: string
  region: string
  accessKeyId: string
  accessKeySecret: string
  securityToken?: string
  replaceCredential?: boolean
  credentialOperation?: CredentialOperation
  purgeScope: PurgeScope
  isPrimary: boolean
}

type EditProjectFormValues = {
  name: string
  description: string
  buckets: EditProjectBucketInput[]
  cdns: EditProjectCDNInput[]
}

type ProjectUpdatePayload = {
  name: string
  description: string
  buckets: Array<{
    id: number
    providerType: ProviderType
    bucketName: string
    region: string
    accessKeyId: string
    accessKeySecret: string
    securityToken?: string
    credentialOperation: CredentialOperation
    isPrimary: boolean
  }>
  cdns: Array<{
    id: number
    providerType: ProviderType
    cdnEndpoint: string
    region: string
    accessKeyId: string
    accessKeySecret: string
    securityToken?: string
    purgeScope: PurgeScope
    credentialOperation: CredentialOperation
    isPrimary: boolean
  }>
}

const providerOptions = [
  { label: 'Aliyun', value: 'aliyun' },
  { label: 'Tencent Cloud', value: 'tencent_cloud' },
  { label: 'Huawei Cloud', value: 'huawei_cloud' },
  { label: 'Qiniu', value: 'qiniu' },
]

const purgeScopeOptions = [
  { label: 'URL', value: 'url' },
  { label: 'Directory', value: 'directory' },
]

const providerTagColor: Record<string, string> = {
  aliyun: 'blue',
  tencent_cloud: 'cyan',
  huawei_cloud: 'geekblue',
  qiniu: 'purple',
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

type BindingFieldError = {
  fieldPath: (string | number)[]
  fieldMessage: string
  toastMessage: string
}

const parseBindingPath = (rawPath: string): (string | number)[] | null => {
  const normalized = rawPath.trim()
  if (!normalized) {
    return null
  }

  const match = normalized.match(/^(buckets|cdns)\[(\d+)\]\.([A-Za-z0-9_]+)$/)
  if (!match) {
    return null
  }

  const index = Number(match[2])
  if (!Number.isFinite(index) || index < 0) {
    return null
  }

  return [match[1], index, match[3]]
}

const resolveProviderRegistrationFieldError = (error: unknown): BindingFieldError | null => {
  if (!axios.isAxiosError(error)) {
    return null
  }

  const payload = error.response?.data
  if (!payload || typeof payload !== 'object') {
    return null
  }

  const code = (payload as Record<string, unknown>).code
  if (code !== 'provider_not_registered') {
    return null
  }

  const details = (payload as Record<string, unknown>).details
  if (!details || typeof details !== 'object') {
    return null
  }

  const detailsRecord = details as Record<string, unknown>
  const bindingType = detailsRecord.bindingType === 'cdns' ? 'cdns' : 'buckets'
  const bindingIndex =
    typeof detailsRecord.bindingIndex === 'number' && detailsRecord.bindingIndex >= 0
      ? Math.floor(detailsRecord.bindingIndex)
      : 0
  const providerType =
    typeof detailsRecord.providerType === 'string' && detailsRecord.providerType.trim()
      ? detailsRecord.providerType.trim()
      : 'unknown'
  const bindingPathText =
    typeof detailsRecord.bindingPath === 'string' ? detailsRecord.bindingPath : ''
  const parsedPath = parseBindingPath(bindingPathText) ?? [bindingType, bindingIndex, 'providerType']

  const bindingLabel = bindingType === 'cdns' ? 'CDN 绑定' : '存储桶绑定'
  return {
    fieldPath: parsedPath,
    fieldMessage: `${bindingLabel} #${bindingIndex + 1} 的 Provider（${providerType}）未在当前服务实例注册。`,
    toastMessage: `${bindingLabel} #${bindingIndex + 1} Provider 未注册，请检查后端 Provider 注册配置后重试。`,
  }
}

const resolveCredentialOperationFieldError = (error: unknown): BindingFieldError | null => {
  if (!axios.isAxiosError(error)) {
    return null
  }

  const payload = error.response?.data
  if (!payload || typeof payload !== 'object') {
    return null
  }

  const code = (payload as Record<string, unknown>).code
  if (
    code !== 'provider_change_requires_credential_replace' &&
    code !== 'credential_missing_for_new_binding' &&
    code !== 'credential_not_found_for_keep'
  ) {
    return null
  }

  const details = (payload as Record<string, unknown>).details
  if (!details || typeof details !== 'object') {
    return null
  }

  const detailsRecord = details as Record<string, unknown>
  const bindingType = detailsRecord.bindingType === 'cdns' ? 'cdns' : 'buckets'
  const bindingIndex =
    typeof detailsRecord.bindingIndex === 'number' && detailsRecord.bindingIndex >= 0
      ? Math.floor(detailsRecord.bindingIndex)
      : 0
  const bindingPathText =
    typeof detailsRecord.bindingPath === 'string' ? detailsRecord.bindingPath : ''
  const parsedPath = parseBindingPath(bindingPathText) ?? [bindingType, bindingIndex, 'credentialOperation']
  const bindingLabel = bindingType === 'cdns' ? 'CDN 绑定' : '存储桶绑定'

  if (code === 'provider_change_requires_credential_replace') {
    return {
      fieldPath: [bindingType, bindingIndex, 'providerType'],
      fieldMessage: `${bindingLabel} #${bindingIndex + 1} 修改了 Provider，需开启“更新凭据”并填写 AK/SK。`,
      toastMessage: `${bindingLabel} #${bindingIndex + 1} 已修改 Provider，请开启“更新凭据”后再提交。`,
    }
  }

  if (code === 'credential_missing_for_new_binding') {
    return {
      fieldPath: [bindingType, bindingIndex, 'accessKeyId'],
      fieldMessage: `${bindingLabel} #${bindingIndex + 1} 为新增绑定，必须填写 AK/SK。`,
      toastMessage: `${bindingLabel} #${bindingIndex + 1} 是新增绑定，请填写 AK/SK 后重试。`,
    }
  }

  return {
    fieldPath: parsedPath[0] === bindingType ? [bindingType, bindingIndex, 'replaceCredential'] : parsedPath,
    fieldMessage: `${bindingLabel} #${bindingIndex + 1} 无可用历史凭据，需开启“更新凭据”并填写 AK/SK。`,
    toastMessage: `${bindingLabel} #${bindingIndex + 1} 无法保留历史凭据，请切换为“更新凭据”。`,
  }
}

const isExistingBinding = (bindingId: number | undefined): bindingId is number =>
  typeof bindingId === 'number' && Number.isFinite(bindingId) && bindingId > 0

const shouldReplaceCredential = (
  mode: 'create' | 'edit',
  bindingId: number | undefined,
  replaceCredential: boolean | undefined,
) => mode === 'create' || !isExistingBinding(bindingId) || Boolean(replaceCredential)

export function ProjectsPage() {
  const platformRole = useAuthStore((state) => state.user?.platformRole)
  const canWrite = isPlatformAdminRole(platformRole)
  const [messageApi, messageContext] = message.useMessage()
  const [form] = Form.useForm<EditProjectFormValues>()
  const [projects, setProjects] = useState<Project[]>([])
  const [loadingList, setLoadingList] = useState(false)
  const [loadingDetail, setLoadingDetail] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [queryName, setQueryName] = useState('')
  const [selectedProjectId, setSelectedProjectId] = useState<number | null>(null)
  const [selectedProject, setSelectedProject] = useState<Project | null>(null)
  const [listError, setListError] = useState<string | null>(null)
  const [detailError, setDetailError] = useState<string | null>(null)
  const [editVisible, setEditVisible] = useState(false)
  const [projectModalMode, setProjectModalMode] = useState<'create' | 'edit'>('edit')

  const fetchProjects = async (name?: string) => {
    setLoadingList(true)
    setListError(null)
    try {
      const response = await apiClient.get<ApiResponse<Project[]>>('/projects', {
        params: name ? { name } : undefined,
      })
      const items = Array.isArray(response.data.data) ? response.data.data : []
      setProjects(items)

      if (items.length === 0) {
        setSelectedProjectId(null)
        setSelectedProject(null)
        return
      }

      if (!selectedProjectId || !items.some((item) => item.id === selectedProjectId)) {
        setSelectedProjectId(items[0].id)
      }
    } catch (error) {
      setListError(resolveErrorMessage(error, '项目列表加载失败，请稍后重试。'))
    } finally {
      setLoadingList(false)
    }
  }

  const fetchProjectDetail = async (projectId: number) => {
    setLoadingDetail(true)
    setDetailError(null)
    try {
      const response = await apiClient.get<ApiResponse<Project>>(`/projects/${projectId}`)
      setSelectedProject(response.data.data)
    } catch (error) {
      setSelectedProject(null)
      setDetailError(resolveErrorMessage(error, '项目详情加载失败，请稍后重试。'))
    } finally {
      setLoadingDetail(false)
    }
  }

  useEffect(() => {
    void fetchProjects()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  useEffect(() => {
    if (!selectedProjectId) {
      return
    }
    void fetchProjectDetail(selectedProjectId)
  }, [selectedProjectId])

  const openEditModal = () => {
    if (!selectedProject || !canWrite) {
      return
    }

    setProjectModalMode('edit')
    form.setFieldsValue({
      name: selectedProject.name,
      description: selectedProject.description,
      buckets:
        selectedProject.buckets?.map((bucket) => ({
          id: bucket.id,
          providerType: bucket.providerType as ProviderType,
          originalProviderType: bucket.providerType as ProviderType,
          bucketName: bucket.bucketName,
          region: bucket.region,
          accessKeyId: '',
          accessKeySecret: '',
          securityToken: '',
          replaceCredential: false,
          credentialOperation: 'KEEP' as CredentialOperation,
          isPrimary: bucket.isPrimary,
        })) ?? [],
      cdns:
        selectedProject.cdns?.map((cdn) => ({
          id: cdn.id,
          providerType: cdn.providerType as ProviderType,
          originalProviderType: cdn.providerType as ProviderType,
          cdnEndpoint: cdn.cdnEndpoint,
          region: cdn.region || '',
          accessKeyId: '',
          accessKeySecret: '',
          securityToken: '',
          replaceCredential: false,
          credentialOperation: 'KEEP' as CredentialOperation,
          purgeScope: (cdn.purgeScope || 'url') as PurgeScope,
          isPrimary: cdn.isPrimary,
        })) ?? [],
    })
    setEditVisible(true)
  }

  const openCreateModal = () => {
    if (!canWrite) {
      return
    }

    setProjectModalMode('create')
    form.setFieldsValue({
      name: '',
      description: '',
      buckets: [],
      cdns: [],
    })
    setEditVisible(true)
  }

  const submitProject = async () => {
    if (!canWrite) {
      return
    }
    if (projectModalMode === 'edit' && !selectedProjectId) {
      return
    }

    const values = await form.validateFields()
    const buckets = Array.isArray(values.buckets) ? values.buckets : []
    const cdns = Array.isArray(values.cdns) ? values.cdns : []

    const validationError = validateProjectBindingCounts({
      bucketCount: buckets.length,
      cdnCount: cdns.length,
      primaryBucketCount: buckets.filter((bucket) => bucket.isPrimary).length,
      primaryCDNCount: cdns.filter((cdn) => cdn.isPrimary).length,
    })
    if (validationError) {
      messageApi.error(validationError)
      return
    }

    const existingBucketProviders = new Map(
      (selectedProject?.buckets ?? []).map((bucket) => [bucket.id, bucket.providerType]),
    )
    const existingCDNProviders = new Map(
      (selectedProject?.cdns ?? []).map((cdn) => [cdn.id, cdn.providerType]),
    )
    const providerChangeFieldErrors: Parameters<typeof form.setFields>[0] = []

    const payload: ProjectUpdatePayload = {
      name: values.name,
      description: values.description,
      buckets: buckets.map((bucket, index) => {
        const id = isExistingBinding(bucket.id) ? Math.floor(bucket.id) : 0
        const replaceCredential = shouldReplaceCredential(projectModalMode, id, bucket.replaceCredential)
        const credentialOperation: CredentialOperation = replaceCredential ? 'REPLACE' : 'KEEP'
        const originalProviderType = (
          bucket.originalProviderType ?? existingBucketProviders.get(id) ?? ''
        ).trim()
        const currentProviderType = (bucket.providerType ?? '').trim()

        if (
          projectModalMode === 'edit' &&
          id > 0 &&
          !replaceCredential &&
          originalProviderType &&
          currentProviderType &&
          originalProviderType.localeCompare(currentProviderType, undefined, { sensitivity: 'accent' }) !==
            0
        ) {
          providerChangeFieldErrors.push({
            name: ['buckets', index, 'providerType'],
            errors: ['Provider 已变更，请开启“更新凭据”并填写 AK/SK 后再提交。'],
          })
        }

        return {
          id,
          providerType: bucket.providerType,
          bucketName: bucket.bucketName,
          region: bucket.region,
          accessKeyId: replaceCredential ? bucket.accessKeyId : '',
          accessKeySecret: replaceCredential ? bucket.accessKeySecret : '',
          securityToken: replaceCredential ? bucket.securityToken : '',
          credentialOperation,
          isPrimary: bucket.isPrimary,
        }
      }),
      cdns: cdns.map((cdn, index) => {
        const id = isExistingBinding(cdn.id) ? Math.floor(cdn.id) : 0
        const replaceCredential = shouldReplaceCredential(projectModalMode, id, cdn.replaceCredential)
        const credentialOperation: CredentialOperation = replaceCredential ? 'REPLACE' : 'KEEP'
        const originalProviderType = (cdn.originalProviderType ?? existingCDNProviders.get(id) ?? '').trim()
        const currentProviderType = (cdn.providerType ?? '').trim()

        if (
          projectModalMode === 'edit' &&
          id > 0 &&
          !replaceCredential &&
          originalProviderType &&
          currentProviderType &&
          originalProviderType.localeCompare(currentProviderType, undefined, { sensitivity: 'accent' }) !==
            0
        ) {
          providerChangeFieldErrors.push({
            name: ['cdns', index, 'providerType'],
            errors: ['Provider 已变更，请开启“更新凭据”并填写 AK/SK 后再提交。'],
          })
        }

        return {
          id,
          providerType: cdn.providerType,
          cdnEndpoint: cdn.cdnEndpoint,
          region: cdn.region,
          accessKeyId: replaceCredential ? cdn.accessKeyId : '',
          accessKeySecret: replaceCredential ? cdn.accessKeySecret : '',
          securityToken: replaceCredential ? cdn.securityToken : '',
          purgeScope: cdn.purgeScope,
          credentialOperation,
          isPrimary: cdn.isPrimary,
        }
      }),
    }

    if (providerChangeFieldErrors.length > 0) {
      form.setFields(providerChangeFieldErrors as Parameters<typeof form.setFields>[0])
      messageApi.error('检测到 Provider 已变更且仍为“保留凭据”，请开启“更新凭据”并填写 AK/SK。')
      return
    }

    setSubmitting(true)
    try {
      if (projectModalMode === 'create') {
        const response = await apiClient.post<ApiResponse<Project>>('/projects', payload)
        const createdProject = response.data.data
        messageApi.success('项目已创建。')
        setEditVisible(false)
        await fetchProjects(queryName.trim() || undefined)
        if (createdProject?.id) {
          setSelectedProjectId(createdProject.id)
          await fetchProjectDetail(createdProject.id)
        }
        return
      }

      await apiClient.put<ApiResponse<Project>>(`/projects/${selectedProjectId}`, payload)
      messageApi.success('项目配置已更新。')
      setEditVisible(false)
      await fetchProjects(queryName.trim() || undefined)
      if (selectedProjectId) {
        await fetchProjectDetail(selectedProjectId)
      }
    } catch (error) {
      const bindingError = resolveProviderRegistrationFieldError(error)
      if (bindingError) {
        form.setFields([
          {
            name: bindingError.fieldPath as never,
            errors: [bindingError.fieldMessage],
          },
        ] as Parameters<typeof form.setFields>[0])
        messageApi.error(bindingError.toastMessage)
        return
      }

      const credentialError = resolveCredentialOperationFieldError(error)
      if (credentialError) {
        form.setFields([
          {
            name: credentialError.fieldPath as never,
            errors: [credentialError.fieldMessage],
          },
        ] as Parameters<typeof form.setFields>[0])
        messageApi.error(credentialError.toastMessage)
        return
      }

      messageApi.error(
        resolveErrorMessage(
          error,
          projectModalMode === 'create'
            ? '项目创建失败，请检查输入后重试。'
            : '项目更新失败，请检查输入后重试。',
        ),
      )
    } finally {
      setSubmitting(false)
    }
  }

  const columns: ColumnsType<Project> = [
    {
      title: 'ID',
      dataIndex: 'id',
      width: 90,
    },
    {
      title: '项目名称',
      dataIndex: 'name',
      render: (value: string) => <Typography.Text strong>{value}</Typography.Text>,
    },
    {
      title: '描述',
      dataIndex: 'description',
      ellipsis: true,
      render: (value: string) =>
        value?.trim().length > 0 ? value : <Typography.Text type="secondary">未填写</Typography.Text>,
    },
    {
      title: '创建时间',
      dataIndex: 'createdAt',
      width: 220,
    },
  ]

  return (
    <>
      {messageContext}
      <Space direction="vertical" size={16} style={{ width: '100%' }}>
        <Card
          title="Projects"
          extra={
            <Space>
              <Input.Search
                placeholder="按名称过滤项目"
                allowClear
                onSearch={(value) => {
                  setQueryName(value)
                  void fetchProjects(value.trim() || undefined)
                }}
                style={{ width: 240 }}
              />
              <Button
                icon={<ReloadOutlined />}
                onClick={() => void fetchProjects(queryName.trim() || undefined)}
              >
                刷新
              </Button>
              <Button
                type="primary"
                icon={<PlusOutlined />}
                onClick={openCreateModal}
                disabled={!canWrite}
              >
                新建项目
              </Button>
            </Space>
          }
        >
          {listError ? (
            <Alert type="error" showIcon message={listError} />
          ) : (
            <Table<Project>
              rowKey="id"
              loading={loadingList}
              columns={columns}
              dataSource={projects}
              pagination={{ pageSize: 8 }}
              rowSelection={{
                type: 'radio',
                selectedRowKeys: selectedProjectId ? [selectedProjectId] : [],
                onChange: (selectedRowKeys) => {
                  const projectId = Number(selectedRowKeys[0])
                  if (Number.isFinite(projectId)) {
                    setSelectedProjectId(projectId)
                  }
                },
              }}
            />
          )}
        </Card>

        <Card
          title="Project Detail"
          extra={
            <Button
              type="primary"
              icon={<EditOutlined />}
              onClick={openEditModal}
              disabled={!selectedProject || !canWrite}
            >
              编辑项目
            </Button>
          }
        >
          {!canWrite ? (
            <Alert
              type="warning"
              showIcon
              style={{ marginBottom: 12 }}
              message="当前账号为只读权限，项目写操作入口已禁用。"
            />
          ) : null}
          {loadingDetail ? (
            <Spin />
          ) : detailError ? (
            <Alert type="error" showIcon message={detailError} />
          ) : !selectedProject ? (
            <Empty description="请选择一个项目查看详情" />
          ) : (
            <Space direction="vertical" size={16} style={{ width: '100%' }}>
              <div>
                <Typography.Title level={4} style={{ margin: 0 }}>
                  {selectedProject.name}
                </Typography.Title>
                <Typography.Paragraph style={{ marginTop: 8, marginBottom: 4 }}>
                  {selectedProject.description?.trim().length
                    ? selectedProject.description
                    : '该项目尚未填写描述。'}
                </Typography.Paragraph>
                <Typography.Text type="secondary">
                  创建时间：{selectedProject.createdAt}
                </Typography.Text>
              </div>

              <Divider style={{ margin: '8px 0' }} />

              <Row gutter={[16, 16]}>
                <Col xs={24} lg={12}>
                  <Card size="small" title="存储桶绑定">
                    {selectedProject.buckets?.length ? (
                      <List
                        size="small"
                        dataSource={selectedProject.buckets}
                        renderItem={(bucket) => (
                          <List.Item>
                            <Space direction="vertical" size={0}>
                              <Space size={8}>
                                <Typography.Text strong>{bucket.bucketName}</Typography.Text>
                                <Tag color={providerTagColor[bucket.providerType] ?? 'default'}>
                                  {bucket.providerType}
                                </Tag>
                                {bucket.isPrimary ? <Tag color="gold">Primary</Tag> : null}
                              </Space>
                              <Typography.Text type="secondary">
                                Region: {bucket.region || '-'}
                              </Typography.Text>
                              <Typography.Text type="secondary">
                                Credential: {bucket.credentialMasked || '-'}
                              </Typography.Text>
                            </Space>
                          </List.Item>
                        )}
                      />
                    ) : (
                      <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无存储桶绑定" />
                    )}
                  </Card>
                </Col>

                <Col xs={24} lg={12}>
                  <Card size="small" title="CDN 绑定">
                    {selectedProject.cdns?.length ? (
                      <List
                        size="small"
                        dataSource={selectedProject.cdns}
                        renderItem={(cdn) => (
                          <List.Item>
                            <Space direction="vertical" size={0}>
                              <Space size={8}>
                                <Typography.Text strong>{cdn.cdnEndpoint}</Typography.Text>
                                <Tag color={providerTagColor[cdn.providerType] ?? 'default'}>
                                  {cdn.providerType}
                                </Tag>
                                {cdn.isPrimary ? <Tag color="gold">Primary</Tag> : null}
                              </Space>
                              <Typography.Text type="secondary">
                                PurgeScope: {cdn.purgeScope || 'url'}
                              </Typography.Text>
                              <Typography.Text type="secondary">
                                Region: {cdn.region || '-'}
                              </Typography.Text>
                              <Typography.Text type="secondary">
                                Credential: {cdn.credentialMasked || '-'}
                              </Typography.Text>
                            </Space>
                          </List.Item>
                        )}
                      />
                    ) : (
                      <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无 CDN 绑定" />
                    )}
                  </Card>
                </Col>
              </Row>
            </Space>
          )}
        </Card>
      </Space>

      <Modal
        title={projectModalMode === 'create' ? '新建项目' : '编辑项目配置'}
        open={editVisible}
        onCancel={() => setEditVisible(false)}
        onOk={() => void submitProject()}
        okButtonProps={{ loading: submitting, disabled: !canWrite }}
        width={900}
        destroyOnHidden
      >
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 16 }}
          message={
            projectModalMode === 'create'
              ? '项目可先空配置创建；新增绑定默认需要填写 AK/SK 并执行凭据替换。'
              : '编辑模式下已有绑定默认“保留凭据”。如需更换凭据，请开启“更新凭据”并填写 AK/SK。'
          }
        />
        <Form<EditProjectFormValues> form={form} layout="vertical">
          <Form.Item
            label="项目名称"
            name="name"
            rules={[{ required: true, message: '请输入项目名称' }]}
          >
            <Input />
          </Form.Item>
          <Form.Item label="项目描述" name="description">
            <Input.TextArea rows={3} />
          </Form.Item>

          <Divider>存储桶绑定（0~2）</Divider>
          <Form.List name="buckets">
            {(fields, { add, remove }) => (
              <Space direction="vertical" style={{ width: '100%' }} size={12}>
                <Button
                  icon={<PlusOutlined />}
                  onClick={() =>
                    add({
                      id: 0,
                      providerType: 'aliyun',
                      originalProviderType: 'aliyun',
                      bucketName: '',
                      region: '',
                      accessKeyId: '',
                      accessKeySecret: '',
                      securityToken: '',
                      replaceCredential: true,
                      credentialOperation: 'REPLACE' as CredentialOperation,
                      isPrimary: fields.length === 0,
                    })
                  }
                  disabled={fields.length >= 2}
                >
                  添加存储桶绑定
                </Button>
                {fields.map((field, index) => (
                  <Form.Item
                    key={field.key}
                    noStyle
                    shouldUpdate={(prevValues, curValues) => {
                      const previous = prevValues?.buckets?.[field.name]
                      const current = curValues?.buckets?.[field.name]
                      return (
                        previous?.id !== current?.id ||
                        previous?.replaceCredential !== current?.replaceCredential
                      )
                    }}
                  >
                    {({ getFieldValue }) => {
                      const bindingId = getFieldValue(['buckets', field.name, 'id']) as number | undefined
                      const existingBindingInEdit = projectModalMode === 'edit' && isExistingBinding(bindingId)
                      const replaceCredential = shouldReplaceCredential(
                        projectModalMode,
                        bindingId,
                        getFieldValue(['buckets', field.name, 'replaceCredential']) as
                          | boolean
                          | undefined,
                      )

                      return (
                        <Card
                          size="small"
                          title={`Bucket #${index + 1}`}
                          extra={
                            <Space size={8}>
                              <Form.Item
                                noStyle
                                name={[field.name, 'isPrimary']}
                                valuePropName="checked"
                                initialValue={index === 0}
                              >
                                <Switch checkedChildren="Primary" unCheckedChildren="Secondary" />
                              </Form.Item>
                              <Button
                                danger
                                icon={<DeleteOutlined />}
                                onClick={() => remove(field.name)}
                                size="small"
                              >
                                删除
                              </Button>
                            </Space>
                          }
                        >
                          <Form.Item hidden name={[field.name, 'id']}>
                            <Input />
                          </Form.Item>
                          <Form.Item hidden name={[field.name, 'originalProviderType']}>
                            <Input />
                          </Form.Item>
                          <Row gutter={12}>
                            <Col span={8}>
                              <Form.Item
                                label="Provider"
                                name={[field.name, 'providerType']}
                                rules={[{ required: true, message: '请选择 Provider' }]}
                              >
                                <Select options={providerOptions} />
                              </Form.Item>
                            </Col>
                            <Col span={8}>
                              <Form.Item
                                label="BucketName"
                                name={[field.name, 'bucketName']}
                                rules={[{ required: true, message: '请输入 BucketName' }]}
                              >
                                <Input />
                              </Form.Item>
                            </Col>
                            <Col span={8}>
                              <Form.Item
                                label="Region"
                                name={[field.name, 'region']}
                                rules={[{ required: true, message: '请输入 Region' }]}
                                extra="阿里云请填写 Region ID（如 cn-beijing）。可从 OSS 外网域名提取，例如 oss-cn-beijing.aliyuncs.com 对应 cn-beijing。"
                              >
                                <Input />
                              </Form.Item>
                            </Col>
                          </Row>

                          {existingBindingInEdit ? (
                            <Form.Item
                              label="更新凭据"
                              name={[field.name, 'replaceCredential']}
                              valuePropName="checked"
                              extra="默认保留历史凭据。开启后将提交 REPLACE，并要求填写 AK/SK。"
                            >
                              <Switch checkedChildren="开启" unCheckedChildren="保留" />
                            </Form.Item>
                          ) : (
                            <Alert
                              type="info"
                              showIcon
                              style={{ marginBottom: 12 }}
                              message="新增绑定默认使用 REPLACE，请填写 AK/SK。"
                            />
                          )}

                          {replaceCredential ? (
                            <Row gutter={12}>
                              <Col span={8}>
                                <Form.Item
                                  label="AccessKeyId"
                                  name={[field.name, 'accessKeyId']}
                                  rules={[{ required: true, message: '请输入 AccessKeyId' }]}
                                >
                                  <Input />
                                </Form.Item>
                              </Col>
                              <Col span={8}>
                                <Form.Item
                                  label="AccessKeySecret"
                                  name={[field.name, 'accessKeySecret']}
                                  rules={[{ required: true, message: '请输入 AccessKeySecret' }]}
                                >
                                  <Input.Password />
                                </Form.Item>
                              </Col>
                              <Col span={8}>
                                <Form.Item label="SecurityToken" name={[field.name, 'securityToken']}>
                                  <Input />
                                </Form.Item>
                              </Col>
                            </Row>
                          ) : (
                            <Alert
                              type="success"
                              showIcon
                              message="当前为保留凭据（KEEP）模式，无需填写 AK/SK。"
                            />
                          )}
                        </Card>
                      )
                    }}
                  </Form.Item>
                ))}
                {fields.length === 0 ? (
                  <Alert
                    type="info"
                    showIcon
                    message="当前未绑定存储桶。可以先创建项目，后续再补充绑定。"
                  />
                ) : null}
              </Space>
            )}
          </Form.List>

          <Divider>CDN 绑定（0~2）</Divider>
          <Form.List name="cdns">
            {(fields, { add, remove }) => (
              <Space direction="vertical" style={{ width: '100%' }} size={12}>
                <Button
                  icon={<PlusOutlined />}
                  onClick={() =>
                    add({
                      id: 0,
                      providerType: 'aliyun',
                      originalProviderType: 'aliyun',
                      cdnEndpoint: '',
                      region: '',
                      accessKeyId: '',
                      accessKeySecret: '',
                      securityToken: '',
                      replaceCredential: true,
                      credentialOperation: 'REPLACE' as CredentialOperation,
                      purgeScope: 'url',
                      isPrimary: fields.length === 0,
                    })
                  }
                  disabled={fields.length >= 2}
                >
                  添加 CDN 绑定
                </Button>
                {fields.map((field, index) => (
                  <Form.Item
                    key={field.key}
                    noStyle
                    shouldUpdate={(prevValues, curValues) => {
                      const previous = prevValues?.cdns?.[field.name]
                      const current = curValues?.cdns?.[field.name]
                      return (
                        previous?.id !== current?.id ||
                        previous?.replaceCredential !== current?.replaceCredential
                      )
                    }}
                  >
                    {({ getFieldValue }) => {
                      const bindingId = getFieldValue(['cdns', field.name, 'id']) as number | undefined
                      const existingBindingInEdit = projectModalMode === 'edit' && isExistingBinding(bindingId)
                      const replaceCredential = shouldReplaceCredential(
                        projectModalMode,
                        bindingId,
                        getFieldValue(['cdns', field.name, 'replaceCredential']) as
                          | boolean
                          | undefined,
                      )

                      return (
                        <Card
                          size="small"
                          title={`CDN #${index + 1}`}
                          extra={
                            <Space size={8}>
                              <Form.Item
                                noStyle
                                name={[field.name, 'isPrimary']}
                                valuePropName="checked"
                                initialValue={index === 0}
                              >
                                <Switch checkedChildren="Primary" unCheckedChildren="Secondary" />
                              </Form.Item>
                              <Button
                                danger
                                icon={<DeleteOutlined />}
                                onClick={() => remove(field.name)}
                                size="small"
                              >
                                删除
                              </Button>
                            </Space>
                          }
                        >
                          <Form.Item hidden name={[field.name, 'id']}>
                            <Input />
                          </Form.Item>
                          <Form.Item hidden name={[field.name, 'originalProviderType']}>
                            <Input />
                          </Form.Item>
                          <Row gutter={12}>
                            <Col span={6}>
                              <Form.Item
                                label="Provider"
                                name={[field.name, 'providerType']}
                                rules={[{ required: true, message: '请选择 Provider' }]}
                              >
                                <Select options={providerOptions} />
                              </Form.Item>
                            </Col>
                            <Col span={8}>
                              <Form.Item
                                label="CDN Endpoint"
                                name={[field.name, 'cdnEndpoint']}
                                rules={[{ required: true, message: '请输入 CDN Endpoint' }]}
                              >
                                <Input />
                              </Form.Item>
                            </Col>
                            <Col span={4}>
                              <Form.Item
                                label="Region"
                                name={[field.name, 'region']}
                                rules={[{ required: true, message: '请输入 Region' }]}
                                extra="阿里云请填写 Region ID（如 cn-beijing）。若有 OSS 外网域名（如 oss-cn-beijing.aliyuncs.com），可提取 cn-beijing。"
                              >
                                <Input />
                              </Form.Item>
                            </Col>
                            <Col span={6}>
                              <Form.Item
                                label="PurgeScope"
                                name={[field.name, 'purgeScope']}
                                rules={[{ required: true, message: '请选择 PurgeScope' }]}
                                initialValue="url"
                              >
                                <Select options={purgeScopeOptions} />
                              </Form.Item>
                            </Col>
                          </Row>

                          {existingBindingInEdit ? (
                            <Form.Item
                              label="更新凭据"
                              name={[field.name, 'replaceCredential']}
                              valuePropName="checked"
                              extra="默认保留历史凭据。开启后将提交 REPLACE，并要求填写 AK/SK。"
                            >
                              <Switch checkedChildren="开启" unCheckedChildren="保留" />
                            </Form.Item>
                          ) : (
                            <Alert
                              type="info"
                              showIcon
                              style={{ marginBottom: 12 }}
                              message="新增绑定默认使用 REPLACE，请填写 AK/SK。"
                            />
                          )}

                          {replaceCredential ? (
                            <Row gutter={12}>
                              <Col span={8}>
                                <Form.Item
                                  label="AccessKeyId"
                                  name={[field.name, 'accessKeyId']}
                                  rules={[{ required: true, message: '请输入 AccessKeyId' }]}
                                >
                                  <Input />
                                </Form.Item>
                              </Col>
                              <Col span={8}>
                                <Form.Item
                                  label="AccessKeySecret"
                                  name={[field.name, 'accessKeySecret']}
                                  rules={[{ required: true, message: '请输入 AccessKeySecret' }]}
                                >
                                  <Input.Password />
                                </Form.Item>
                              </Col>
                              <Col span={8}>
                                <Form.Item label="SecurityToken" name={[field.name, 'securityToken']}>
                                  <Input />
                                </Form.Item>
                              </Col>
                            </Row>
                          ) : (
                            <Alert
                              type="success"
                              showIcon
                              message="当前为保留凭据（KEEP）模式，无需填写 AK/SK。"
                            />
                          )}
                        </Card>
                      )
                    }}
                  </Form.Item>
                ))}
                {fields.length === 0 ? (
                  <Alert
                    type="info"
                    showIcon
                    message="当前未绑定 CDN。可以先创建项目，后续再补充绑定。"
                  />
                ) : null}
              </Space>
            )}
          </Form.List>
        </Form>
      </Modal>
    </>
  )
}
