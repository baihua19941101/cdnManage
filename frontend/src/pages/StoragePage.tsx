import {
  DeleteOutlined,
  DownloadOutlined,
  EditOutlined,
  FileSearchOutlined,
  ReloadOutlined,
  UploadOutlined,
} from '@ant-design/icons'
import {
  Alert,
  Button,
  Card,
  Drawer,
  Form,
  Input,
  Modal,
  Popconfirm,
  Select,
  Space,
  Table,
  Tag,
  Typography,
  Upload,
  message,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import type { UploadFile } from 'antd/es/upload/interface'
import { useState } from 'react'

import { apiClient } from '../services/api/client'
import { resolveAPIErrorMessage } from '../services/api/error'
import { isPlatformAdminRole, useAuthStore } from '../store/auth'

type ObjectItem = {
  key: string
  etag?: string
  contentType?: string
  size: number
  lastModified?: string
  isDir: boolean
}

type StorageAuditLog = {
  id: number
  actorUserId: number
  actorUsername?: string
  action: string
  targetType: string
  targetIdentifier: string
  result: string
  requestId: string
  createdAt: string
}

type ApiResponse<T> = {
  code: string
  message: string
  data: T
}

type ProjectOption = {
  id: number
  name: string
}

type ProjectDetail = {
  id: number
  buckets?: Array<{
    bucketName: string
  }>
}

type QueryFormValues = {
  projectId: string
  bucketName: string
  prefix?: string
}

type RenameFormValues = {
  targetKey: string
}

type UploadSummary = {
  successCount: number
  failureCount: number
}

type ArchiveSummary = {
  archivesProcessed: number
  extracted: number
  uploaded: number
  failed: number
  skipped: number
}

const isRecord = (value: unknown): value is Record<string, unknown> =>
  typeof value === 'object' && value !== null

const toNonNegativeNumber = (value: unknown): number | null => {
  if (typeof value === 'number' && Number.isFinite(value)) {
    return Math.max(0, value)
  }
  if (typeof value === 'string' && value.trim()) {
    const numericValue = Number(value)
    if (Number.isFinite(numericValue)) {
      return Math.max(0, numericValue)
    }
  }
  return null
}

const parseUploadSummary = (data: unknown, fallbackTotal: number): UploadSummary => {
  if (isRecord(data)) {
    const successCount = data.successCount
    const failureCount = data.failureCount ?? data.failedCount
    if (typeof successCount === 'number' && typeof failureCount === 'number') {
      return {
        successCount: Math.max(0, successCount),
        failureCount: Math.max(0, failureCount),
      }
    }

    const resultListCandidates = [data.results, data.files, data.items, data.uploads]
    for (const candidate of resultListCandidates) {
      if (Array.isArray(candidate)) {
        let success = 0
        let failure = 0

        for (const item of candidate) {
          if (!isRecord(item)) {
            success += 1
            continue
          }

          if (typeof item.success === 'boolean') {
            if (item.success) {
              success += 1
            } else {
              failure += 1
            }
            continue
          }

          const status =
            typeof item.status === 'string'
              ? item.status.toLowerCase()
              : typeof item.result === 'string'
                ? item.result.toLowerCase()
                : ''

          if (status === 'failure' || status === 'failed' || status === 'error') {
            failure += 1
            continue
          }
          success += 1
        }

        return { successCount: success, failureCount: failure }
      }
    }

    if (Array.isArray(data.successes) || Array.isArray(data.failures)) {
      const success = Array.isArray(data.successes) ? data.successes.length : 0
      const failure = Array.isArray(data.failures) ? data.failures.length : 0
      return { successCount: success, failureCount: failure }
    }
  }

  return {
    successCount: fallbackTotal,
    failureCount: 0,
  }
}

const parseArchiveSummary = (data: unknown): ArchiveSummary | null => {
  if (!isRecord(data)) {
    return null
  }

  const candidates: unknown[] = [data.archiveSummary, data.extractSummary, data]

  for (const candidate of candidates) {
    if (!isRecord(candidate)) {
      continue
    }

    const archivesProcessed = toNonNegativeNumber(candidate.archivesProcessed)
    const extracted = toNonNegativeNumber(candidate.extracted)
    const uploaded = toNonNegativeNumber(candidate.uploaded)
    const failed = toNonNegativeNumber(candidate.failed)
    const skipped = toNonNegativeNumber(candidate.skipped)

    const hasAnyArchiveField =
      archivesProcessed !== null ||
      extracted !== null ||
      uploaded !== null ||
      failed !== null ||
      skipped !== null

    if (!hasAnyArchiveField) {
      continue
    }

    return {
      archivesProcessed: archivesProcessed ?? 0,
      extracted: extracted ?? 0,
      uploaded: uploaded ?? 0,
      failed: failed ?? 0,
      skipped: skipped ?? 0,
    }
  }

  return null
}

const formatArchiveSummary = (summary: ArchiveSummary): string =>
  `解压与上传摘要：压缩包 ${summary.archivesProcessed}，解压 ${summary.extracted}，上传 ${summary.uploaded}，跳过 ${summary.skipped}，失败 ${summary.failed}`

export function StoragePage() {
  const [messageApi, messageContext] = message.useMessage()
  const [queryForm] = Form.useForm<QueryFormValues>()
  const [renameForm] = Form.useForm<RenameFormValues>()
  const platformRole = useAuthStore((state) => state.user?.platformRole)
  const canWrite = isPlatformAdminRole(platformRole)

  const [objects, setObjects] = useState<ObjectItem[]>([])
  const [loading, setLoading] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [queryError, setQueryError] = useState<string | null>(null)

  const [pendingUploadFiles, setPendingUploadFiles] = useState<UploadFile[]>([])
  const [uploadKey, setUploadKey] = useState('')

  const [renameVisible, setRenameVisible] = useState(false)
  const [renamingSourceKey, setRenamingSourceKey] = useState('')

  const [auditVisible, setAuditVisible] = useState(false)
  const [auditLogs, setAuditLogs] = useState<StorageAuditLog[]>([])
  const [auditLoading, setAuditLoading] = useState(false)
  const [projectOptions, setProjectOptions] = useState<ProjectOption[]>([])
  const [projectOptionsLoading, setProjectOptionsLoading] = useState(false)
  const [bucketOptions, setBucketOptions] = useState<string[]>([])
  const [bucketOptionsLoading, setBucketOptionsLoading] = useState(false)

  const getQuery = () => {
    const values = queryForm.getFieldsValue()
    const projectID = Number(values.projectId)
    return {
      projectID,
      bucketName: values.bucketName?.trim(),
      prefix: values.prefix?.trim() || '',
    }
  }

  const queryObjects = async () => {
    const values = await queryForm.validateFields()
    const projectID = Number(values.projectId)
    if (!Number.isFinite(projectID) || projectID <= 0) {
      messageApi.error('Project ID 必须是正整数。')
      return
    }

    setLoading(true)
    setQueryError(null)
    try {
      const response = await apiClient.get<ApiResponse<{ objects: ObjectItem[] }>>(
        `/projects/${projectID}/storage/objects`,
        {
          params: {
            bucketName: values.bucketName.trim(),
            prefix: values.prefix?.trim() || undefined,
          },
        },
      )
      setObjects(response.data.data?.objects ?? [])
    } catch (error) {
      setQueryError(resolveAPIErrorMessage(error, '对象列表加载失败。'))
      setObjects([])
    } finally {
      setLoading(false)
    }
  }

  const loadProjectOptions = async () => {
    setProjectOptionsLoading(true)
    try {
      const response = await apiClient.get<ApiResponse<ProjectOption[]>>('/projects')
      const items = Array.isArray(response.data.data) ? response.data.data : []
      setProjectOptions(items)
    } catch (error) {
      messageApi.error(resolveAPIErrorMessage(error, '项目列表加载失败。'))
      setProjectOptions([])
    } finally {
      setProjectOptionsLoading(false)
    }
  }

  const loadBucketsByProject = async (projectID: number) => {
    if (!Number.isFinite(projectID) || projectID <= 0) {
      setBucketOptions([])
      queryForm.setFieldValue('bucketName', '')
      return
    }

    setBucketOptionsLoading(true)
    try {
      const response = await apiClient.get<ApiResponse<ProjectDetail>>(`/projects/${projectID}`)
      const project = response.data.data
      const names =
        Array.isArray(project?.buckets)
          ? project.buckets
              .map((bucket) => bucket.bucketName?.trim())
              .filter((name): name is string => Boolean(name))
          : []
      setBucketOptions(names)
      queryForm.setFieldValue('bucketName', names[0] ?? '')
    } catch (error) {
      messageApi.error(resolveAPIErrorMessage(error, '项目存储桶加载失败。'))
      setBucketOptions([])
      queryForm.setFieldValue('bucketName', '')
    } finally {
      setBucketOptionsLoading(false)
    }
  }

  const uploadObject = async () => {
    if (!canWrite) {
      return
    }
    const { projectID, bucketName } = getQuery()
    if (!projectID || !bucketName) {
      messageApi.error('请先填写并查询 Project ID 与 BucketName。')
      return
    }

    const files = pendingUploadFiles.flatMap((item) =>
      item.originFileObj ? [item.originFileObj] : [],
    )
    if (files.length === 0) {
      messageApi.error('请选择待上传文件。')
      return
    }

    const formData = new FormData()
    formData.append('bucketName', bucketName)

    const trimmedKey = uploadKey.trim()
    if (files.length === 1) {
      const singleFile = files[0]
      if (!singleFile) {
        messageApi.error('请选择待上传文件。')
        return
      }
      if (trimmedKey) {
        formData.append('key', trimmedKey)
      }
      formData.append('file', singleFile)
    } else {
      files.forEach((file) => formData.append('files', file))
      if (trimmedKey) {
        formData.append('keyPrefix', trimmedKey)
      }
    }

    setSubmitting(true)
    try {
      const response = await apiClient.post<ApiResponse<unknown>>(
        `/projects/${projectID}/storage/upload`,
        formData,
        {
          headers: { 'Content-Type': 'multipart/form-data' },
        },
      )
      const summary = parseUploadSummary(response.data?.data, files.length)
      const archiveSummary = parseArchiveSummary(response.data?.data)
      const archiveSummaryText = archiveSummary ? `；${formatArchiveSummary(archiveSummary)}` : ''
      if (summary.failureCount > 0) {
        messageApi.warning(
          `上传完成：成功 ${summary.successCount}，失败 ${summary.failureCount}${archiveSummaryText}。`,
        )
      } else {
        messageApi.success(`上传完成：成功 ${summary.successCount}，失败 0${archiveSummaryText}。`)
      }
      setPendingUploadFiles([])
      setUploadKey('')
      await queryObjects()
    } catch (error) {
      messageApi.error(resolveAPIErrorMessage(error, '上传失败。'))
    } finally {
      setSubmitting(false)
    }
  }

  const downloadObject = async (key: string) => {
    const { projectID, bucketName } = getQuery()
    if (!projectID || !bucketName) {
      messageApi.error('请先填写并查询 Project ID 与 BucketName。')
      return
    }

    try {
      const response = await apiClient.get<Blob>(`/projects/${projectID}/storage/download`, {
        params: { bucketName, key },
        responseType: 'blob',
      })
      const url = window.URL.createObjectURL(response.data)
      const anchor = document.createElement('a')
      anchor.href = url
      anchor.download = key.split('/').pop() || key
      document.body.appendChild(anchor)
      anchor.click()
      anchor.remove()
      window.URL.revokeObjectURL(url)
    } catch (error) {
      messageApi.error(resolveAPIErrorMessage(error, '下载失败。'))
    }
  }

  const deleteObject = async (key: string) => {
    if (!canWrite) {
      return
    }
    const { projectID, bucketName } = getQuery()
    if (!projectID || !bucketName) {
      messageApi.error('请先填写并查询 Project ID 与 BucketName。')
      return
    }

    setSubmitting(true)
    try {
      await apiClient.delete(`/projects/${projectID}/storage/objects`, {
        params: { bucketName, key },
      })
      messageApi.success('删除成功。')
      await queryObjects()
    } catch (error) {
      messageApi.error(resolveAPIErrorMessage(error, '删除失败。'))
    } finally {
      setSubmitting(false)
    }
  }

  const openRename = (sourceKey: string) => {
    if (!canWrite) {
      return
    }
    setRenamingSourceKey(sourceKey)
    renameForm.setFieldsValue({ targetKey: sourceKey })
    setRenameVisible(true)
  }

  const submitRename = async () => {
    if (!canWrite) {
      return
    }
    const { projectID, bucketName } = getQuery()
    if (!projectID || !bucketName) {
      messageApi.error('请先填写并查询 Project ID 与 BucketName。')
      return
    }
    const values = await renameForm.validateFields()

    setSubmitting(true)
    try {
      await apiClient.put(`/projects/${projectID}/storage/rename`, {
        bucketName,
        sourceKey: renamingSourceKey,
        targetKey: values.targetKey.trim(),
      })
      messageApi.success('重命名成功。')
      setRenameVisible(false)
      await queryObjects()
    } catch (error) {
      messageApi.error(resolveAPIErrorMessage(error, '重命名失败。'))
    } finally {
      setSubmitting(false)
    }
  }

  const openAudit = async (path: string) => {
    const { projectID } = getQuery()
    if (!projectID) {
      messageApi.error('请先填写并查询 Project ID。')
      return
    }
    setAuditVisible(true)
    setAuditLoading(true)
    try {
      const response = await apiClient.get<ApiResponse<{ logs: StorageAuditLog[] }>>(
        `/projects/${projectID}/storage/audits`,
        {
          params: { path, limit: 20, offset: 0 },
        },
      )
      setAuditLogs(response.data.data?.logs ?? [])
    } catch (error) {
      messageApi.error(resolveAPIErrorMessage(error, '审计日志查询失败。'))
      setAuditLogs([])
    } finally {
      setAuditLoading(false)
    }
  }

  const columns: ColumnsType<ObjectItem> = [
    {
      title: '对象路径',
      dataIndex: 'key',
      render: (value: string, record) => (
        <Space size={8}>
          <Typography.Text>{value}</Typography.Text>
          {record.isDir ? <Tag color="geekblue">DIR</Tag> : null}
        </Space>
      ),
    },
    { title: '大小', dataIndex: 'size', width: 120 },
    { title: '类型', dataIndex: 'contentType', width: 180 },
    { title: '更新时间', dataIndex: 'lastModified', width: 220 },
    {
      title: '操作',
      width: 360,
      render: (_, record) => (
        <Space>
          {!record.isDir ? (
            <Button
              icon={<DownloadOutlined />}
              onClick={() => void downloadObject(record.key)}
              size="small"
            >
              下载
            </Button>
          ) : null}
          <Button
            icon={<EditOutlined />}
            size="small"
            onClick={() => openRename(record.key)}
            disabled={!canWrite}
          >
            重命名
          </Button>
          <Popconfirm
            title="确认删除该对象？"
            onConfirm={() => void deleteObject(record.key)}
            okButtonProps={{ loading: submitting }}
            disabled={!canWrite}
          >
            <Button
              danger
              icon={<DeleteOutlined />}
              size="small"
              disabled={!canWrite}
            >
              删除
            </Button>
          </Popconfirm>
          <Button
            icon={<FileSearchOutlined />}
            size="small"
            onClick={() => void openAudit(record.key)}
          >
            审计
          </Button>
        </Space>
      ),
    },
  ]

  return (
    <>
      {messageContext}
      <Space direction="vertical" size={16} style={{ width: '100%' }}>
        <Card title="Storage Objects">
          {!canWrite ? (
            <Alert
              type="warning"
              showIcon
              message="当前账号为只读权限，上传/删除/重命名入口已禁用。"
              style={{ marginBottom: 12 }}
            />
          ) : null}
          <Form<QueryFormValues>
            form={queryForm}
            layout="inline"
            initialValues={{ projectId: '', bucketName: '', prefix: '' }}
            style={{ rowGap: 12 }}
          >
            <Form.Item
              name="projectId"
              label="Project ID"
              rules={[{ required: true, message: '请输入 Project ID' }]}
            >
              <Select
                showSearch
                placeholder="请选择或搜索 Project ID"
                style={{ width: 280 }}
                loading={projectOptionsLoading}
                filterOption={(input, option) =>
                  String(option?.label ?? '')
                    .toLowerCase()
                    .includes(input.toLowerCase())
                }
                options={projectOptions.map((project) => ({
                  value: String(project.id),
                  label: `${project.id} - ${project.name}`,
                }))}
                onDropdownVisibleChange={(open) => {
                  if (open && projectOptions.length === 0) {
                    void loadProjectOptions()
                  }
                }}
                onChange={(value) => {
                  void loadBucketsByProject(Number(value))
                }}
              />
            </Form.Item>
            <Form.Item
              name="bucketName"
              label="Bucket"
              rules={[{ required: true, message: '请输入 BucketName' }]}
            >
              <Select
                showSearch
                placeholder="请选择 Bucket（选项目后自动加载）"
                style={{ width: 280 }}
                loading={bucketOptionsLoading}
                options={bucketOptions.map((bucketName) => ({
                  value: bucketName,
                  label: bucketName,
                }))}
                notFoundContent="该项目暂无存储桶绑定"
                filterOption={(input, option) =>
                  String(option?.label ?? '')
                    .toLowerCase()
                    .includes(input.toLowerCase())
                }
              />
            </Form.Item>
            <Form.Item name="prefix" label="Prefix">
              <Input placeholder="可选目录前缀" style={{ width: 220 }} />
            </Form.Item>
            <Form.Item>
              <Button
                type="primary"
                icon={<ReloadOutlined />}
                onClick={() => void queryObjects()}
                loading={loading}
              >
                查询对象
              </Button>
            </Form.Item>
          </Form>
          {queryError ? (
            <Alert type="error" showIcon style={{ marginTop: 12 }} message={queryError} />
          ) : null}
        </Card>

        <Card
          title="Upload"
          extra={
            <Space>
              <Upload
                multiple
                beforeUpload={(file) => {
                  setPendingUploadFiles((prev) => {
                    if (prev.some((item) => item.uid === file.uid)) {
                      return prev
                    }
                    return [
                      ...prev,
                      {
                        uid: file.uid,
                        name: file.name,
                        status: 'done',
                        type: file.type,
                        size: file.size,
                        originFileObj: file,
                      } satisfies UploadFile,
                    ]
                  })
                  return false
                }}
                onRemove={(file) => {
                  setPendingUploadFiles((prev) => prev.filter((item) => item.uid !== file.uid))
                }}
                showUploadList
                fileList={pendingUploadFiles}
                disabled={!canWrite}
              >
                <Button icon={<UploadOutlined />} disabled={!canWrite}>
                  选择文件
                </Button>
              </Upload>
              <Button
                type="primary"
                loading={submitting}
                onClick={() => void uploadObject()}
                disabled={!canWrite}
              >
                上传
              </Button>
            </Space>
          }
        >
          <Space direction="vertical" size={8} style={{ width: '100%' }}>
            <Input
              placeholder="可选：单文件时为对象 Key，多文件时作为 keyPrefix"
              value={uploadKey}
              onChange={(event) => setUploadKey(event.target.value)}
              disabled={!canWrite}
            />
            <Typography.Text type="secondary">
              待上传文件：{pendingUploadFiles.length}
              {pendingUploadFiles.length > 1 && uploadKey.trim()
                ? '（将使用当前输入作为 keyPrefix）'
                : ''}
            </Typography.Text>
            <Typography.Text type="secondary">
              支持 zip/tar/tar.gz/tgz 自动解压上传。
            </Typography.Text>
          </Space>
        </Card>

        <Card title="Object List">
          <Table<ObjectItem>
            rowKey="key"
            columns={columns}
            dataSource={objects}
            loading={loading}
            pagination={{ pageSize: 10 }}
          />
        </Card>
      </Space>

      <Modal
        title="重命名对象"
        open={renameVisible}
        onCancel={() => setRenameVisible(false)}
        onOk={() => void submitRename()}
        okButtonProps={{ loading: submitting, disabled: !canWrite }}
        destroyOnHidden
      >
        <Typography.Paragraph type="secondary">源对象：{renamingSourceKey}</Typography.Paragraph>
        <Form<RenameFormValues> form={renameForm} layout="vertical">
          <Form.Item
            name="targetKey"
            label="目标对象 Key"
            rules={[
              { required: true, message: '请输入目标对象 Key' },
              { min: 1, message: '目标对象 Key 不能为空' },
            ]}
          >
            <Input />
          </Form.Item>
        </Form>
      </Modal>

      <Drawer
        title="对象审计日志"
        open={auditVisible}
        onClose={() => setAuditVisible(false)}
        width={760}
      >
        <Table<StorageAuditLog>
          rowKey="id"
          loading={auditLoading}
          pagination={{ pageSize: 8 }}
          dataSource={auditLogs}
          columns={[
            { title: '时间', dataIndex: 'createdAt', width: 210 },
            { title: '动作', dataIndex: 'action', width: 160 },
            {
              title: '结果',
              dataIndex: 'result',
              width: 120,
              render: (value: string) =>
                value === 'success' ? (
                  <Tag color="green">success</Tag>
                ) : value === 'failure' ? (
                  <Tag color="red">failure</Tag>
                ) : (
                  <Tag>{value}</Tag>
                ),
            },
            { title: '对象', dataIndex: 'targetIdentifier' },
            { title: '操作者', dataIndex: 'actorUsername', width: 140 },
          ]}
        />
      </Drawer>
    </>
  )
}
