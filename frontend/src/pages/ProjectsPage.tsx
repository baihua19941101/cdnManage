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
  providerType: ProviderType
  bucketName: string
  region: string
  accessKeyId: string
  accessKeySecret: string
  securityToken?: string
  isPrimary: boolean
}

type EditProjectCDNInput = {
  providerType: ProviderType
  cdnEndpoint: string
  region: string
  accessKeyId: string
  accessKeySecret: string
  securityToken?: string
  purgeScope: PurgeScope
  isPrimary: boolean
}

type EditProjectFormValues = {
  name: string
  description: string
  buckets: EditProjectBucketInput[]
  cdns: EditProjectCDNInput[]
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
          providerType: bucket.providerType as ProviderType,
          bucketName: bucket.bucketName,
          region: bucket.region,
          accessKeyId: '',
          accessKeySecret: '',
          securityToken: '',
          isPrimary: bucket.isPrimary,
        })) ?? [],
      cdns:
        selectedProject.cdns?.map((cdn) => ({
          providerType: cdn.providerType as ProviderType,
          cdnEndpoint: cdn.cdnEndpoint,
          region: cdn.region || '',
          accessKeyId: '',
          accessKeySecret: '',
          securityToken: '',
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

    const validationError = validateProjectBindingCounts({
      bucketCount: values.buckets.length,
      cdnCount: values.cdns.length,
      primaryBucketCount: values.buckets.filter((bucket) => bucket.isPrimary).length,
      primaryCDNCount: values.cdns.filter((cdn) => cdn.isPrimary).length,
    })
    if (validationError) {
      messageApi.error(validationError)
      return
    }

    setSubmitting(true)
    try {
      if (projectModalMode === 'create') {
        const response = await apiClient.post<ApiResponse<Project>>('/projects', values)
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

      await apiClient.put<ApiResponse<Project>>(`/projects/${selectedProjectId}`, values)
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
            name: bindingError.fieldPath,
            errors: [bindingError.fieldMessage],
          },
        ])
        messageApi.error(bindingError.toastMessage)
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
              ? '项目可先空配置创建；可按需绑定 0~2 个存储桶与 0~2 个 CDN，并在每类存在绑定时仅设置一个 Primary。'
              : '后端更新接口要求每个存储桶提供有效凭证，本次编辑请重新填写每个存储桶的 Credential。'
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
                      providerType: 'aliyun',
                      bucketName: '',
                      region: '',
                      accessKeyId: '',
                      accessKeySecret: '',
                      securityToken: '',
                      isPrimary: fields.length === 0,
                    })
                  }
                  disabled={fields.length >= 2}
                >
                  添加存储桶绑定
                </Button>
                {fields.map((field, index) => (
                  <Card
                    key={field.key}
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
                      <Col span={6}>
                        <Form.Item
                          label="BucketName"
                          name={[field.name, 'bucketName']}
                          rules={[{ required: true, message: '请输入 BucketName' }]}
                        >
                          <Input />
                        </Form.Item>
                      </Col>
                      <Col span={6}>
                        <Form.Item
                          label="Region"
                          name={[field.name, 'region']}
                          rules={[{ required: true, message: '请输入 Region' }]}
                          extra="阿里云请填写 Region ID（如 cn-beijing）。可从 OSS 外网域名提取，例如 oss-cn-beijing.aliyuncs.com 对应 cn-beijing。"
                        >
                          <Input />
                        </Form.Item>
                      </Col>
                      <Col span={6}>
                        <Form.Item
                          label="SecurityToken"
                          name={[field.name, 'securityToken']}
                        >
                          <Input />
                        </Form.Item>
                      </Col>
                    </Row>
                    <Row gutter={12}>
                      <Col span={12}>
                        <Form.Item
                          label="AccessKeyId"
                          name={[field.name, 'accessKeyId']}
                          rules={[{ required: true, message: '请输入 AccessKeyId' }]}
                        >
                          <Input />
                        </Form.Item>
                      </Col>
                      <Col span={12}>
                        <Form.Item
                          label="AccessKeySecret"
                          name={[field.name, 'accessKeySecret']}
                          rules={[{ required: true, message: '请输入 AccessKeySecret' }]}
                        >
                          <Input.Password />
                        </Form.Item>
                      </Col>
                    </Row>
                  </Card>
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
                      providerType: 'aliyun',
                      cdnEndpoint: '',
                      region: '',
                      accessKeyId: '',
                      accessKeySecret: '',
                      securityToken: '',
                      purgeScope: 'url',
                      isPrimary: fields.length === 0,
                    })
                  }
                  disabled={fields.length >= 2}
                >
                  添加 CDN 绑定
                </Button>
                {fields.map((field, index) => (
                  <Card
                    key={field.key}
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
                        <Form.Item
                          label="SecurityToken"
                          name={[field.name, 'securityToken']}
                        >
                          <Input />
                        </Form.Item>
                      </Col>
                    </Row>
                  </Card>
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
