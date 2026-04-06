import { ReloadOutlined } from '@ant-design/icons'
import {
  Alert,
  Button,
  Card,
  Empty,
  Form,
  Input,
  InputNumber,
  Select,
  Segmented,
  Space,
  Table,
  Tag,
  Typography,
  message,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { useEffect, useState } from 'react'

import { apiClient } from '../services/api/client'
import { resolveAPIErrorMessage } from '../services/api/error'
import { isPlatformAdminRole, useAuthStore } from '../store/auth'

type QueryScope = 'platform' | 'project'
type AuditResult = 'success' | 'failure' | 'denied'

type AuditLog = {
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

type ListAuditLogsPayload = {
  logs: AuditLog[]
}

type ApiResponse<T> = {
  code: string
  message: string
  requestId?: string
  data: T
}

type QueryFormValues = {
  scope: QueryScope
  projectId?: string
  action?: string
  result?: AuditResult
  targetType?: string
  targetIdentifier?: string
  actorUserId?: string
  limit: number
  offset: number
}

const buildQueryParams = (values: QueryFormValues, includeActorUserId: boolean) => {
  const params: Record<string, string | number> = {
    limit: values.limit,
    offset: values.offset,
  }

  const action = values.action?.trim()
  const result = values.result?.trim()
  const targetType = values.targetType?.trim()
  const targetIdentifier = values.targetIdentifier?.trim()

  if (action) {
    params.action = action
  }
  if (result) {
    params.result = result
  }
  if (targetType) {
    params.targetType = targetType
  }
  if (targetIdentifier) {
    params.targetIdentifier = targetIdentifier
  }

  if (includeActorUserId) {
    const actorUserId = values.actorUserId?.trim()
    if (actorUserId) {
      params.actorUserId = actorUserId
    }
  }

  return params
}

export function AuditsPage() {
  const [messageApi, messageContext] = message.useMessage()
  const [queryForm] = Form.useForm<QueryFormValues>()
  const userID = useAuthStore((state) => state.user?.id)
  const platformRole = useAuthStore((state) => state.user?.platformRole)
  const canQuery = Boolean(userID)
  const canQueryPlatformScope = isPlatformAdminRole(platformRole)

  const [loading, setLoading] = useState(false)
  const [queryError, setQueryError] = useState<string | null>(null)
  const [logs, setLogs] = useState<AuditLog[]>([])
  const [hasSearched, setHasSearched] = useState(false)

  const scope = Form.useWatch('scope', queryForm) ?? 'platform'

  useEffect(() => {
    if (!canQueryPlatformScope) {
      queryForm.setFieldValue('scope', 'project')
    }
  }, [canQueryPlatformScope, queryForm])

  const submitQuery = async () => {
    if (!canQuery) {
      return
    }

    const values = await queryForm.validateFields()
    setLoading(true)
    setQueryError(null)
    setHasSearched(true)

    try {
      if (values.scope === 'project') {
        const projectID = Number(values.projectId)
        if (!Number.isFinite(projectID) || projectID <= 0) {
          messageApi.error('Project ID 必须是正整数。')
          setLoading(false)
          return
        }

        const response = await apiClient.get<ApiResponse<ListAuditLogsPayload>>(
          `/projects/${projectID}/audits`,
          {
            params: buildQueryParams(values, false),
          },
        )
        setLogs(Array.isArray(response.data.data?.logs) ? response.data.data.logs : [])
        return
      }

      if (!canQueryPlatformScope) {
        messageApi.error('当前账号仅支持项目级审计查询。')
        setLogs([])
        setLoading(false)
        return
      }

      const response = await apiClient.get<ApiResponse<ListAuditLogsPayload>>('/audits', {
        params: buildQueryParams(values, true),
      })
      setLogs(Array.isArray(response.data.data?.logs) ? response.data.data.logs : [])
    } catch (error) {
      setLogs([])
      setQueryError(resolveAPIErrorMessage(error, '审计日志查询失败，请稍后重试。'))
    } finally {
      setLoading(false)
    }
  }

  const columns: ColumnsType<AuditLog> = [
    { title: 'createdAt', dataIndex: 'createdAt', width: 220 },
    { title: 'action', dataIndex: 'action', width: 190 },
    { title: 'targetType', dataIndex: 'targetType', width: 150 },
    { title: 'targetIdentifier', dataIndex: 'targetIdentifier' },
    {
      title: 'result',
      dataIndex: 'result',
      width: 120,
      render: (value: string) =>
        value === 'success' ? (
          <Tag color="green">success</Tag>
        ) : value === 'failure' ? (
          <Tag color="red">failure</Tag>
        ) : value === 'denied' ? (
          <Tag color="orange">denied</Tag>
        ) : (
          <Tag>{value}</Tag>
        ),
    },
    {
      title: 'actorUsername',
      dataIndex: 'actorUsername',
      width: 170,
      render: (value?: string) =>
        value?.trim().length ? value : <Typography.Text type="secondary">unknown</Typography.Text>,
    },
    { title: 'requestId', dataIndex: 'requestId', width: 240 },
  ]

  return (
    <>
      {messageContext}
      <Space direction="vertical" size={16} style={{ width: '100%' }}>
        <Card title="Audit Query">
          {!canQuery ? (
            <Alert
              type="warning"
              showIcon
              style={{ marginBottom: 12 }}
              message="当前账号无权限执行审计查询。请联系平台管理员。"
            />
          ) : !canQueryPlatformScope ? (
            <Alert
              type="info"
              showIcon
              style={{ marginBottom: 12 }}
              message="当前账号仅支持项目级审计查询。请输入 Project ID 后查询。"
            />
          ) : null}
          <Form<QueryFormValues>
            form={queryForm}
            layout="inline"
            disabled={!canQuery}
            initialValues={{
              scope: canQueryPlatformScope ? 'platform' : 'project',
              projectId: '',
              action: '',
              result: undefined,
              targetType: '',
              targetIdentifier: '',
              actorUserId: '',
              limit: 20,
              offset: 0,
            }}
            style={{ rowGap: 12 }}
          >
            <Form.Item name="scope" label="查询范围">
              <Segmented<QueryScope>
                options={[
                  { label: '平台级', value: 'platform' },
                  { label: '项目级', value: 'project' },
                ]}
                disabled={!canQueryPlatformScope}
              />
            </Form.Item>

            {scope === 'project' ? (
              <Form.Item
                name="projectId"
                label="Project ID"
                rules={[
                  { required: true, message: '请输入 Project ID' },
                  { pattern: /^[1-9]\d*$/, message: 'Project ID 必须是正整数' },
                ]}
              >
                <Input placeholder="例如 42" style={{ width: 150 }} />
              </Form.Item>
            ) : null}

            <Form.Item name="action" label="Action">
              <Input placeholder="例如 object.upload" style={{ width: 190 }} />
            </Form.Item>

            <Form.Item name="result" label="Result">
              <Select<AuditResult>
                allowClear
                placeholder="全部"
                style={{ width: 160 }}
                options={[
                  { label: 'success', value: 'success' },
                  { label: 'failure', value: 'failure' },
                  { label: 'denied', value: 'denied' },
                ]}
              />
            </Form.Item>

            <Form.Item name="targetType" label="Target Type">
              <Input placeholder="例如 object / cdn / project" style={{ width: 210 }} />
            </Form.Item>

            <Form.Item name="targetIdentifier" label="Target Identifier">
              <Input placeholder="支持模糊匹配" style={{ width: 240 }} />
            </Form.Item>

            {scope === 'platform' ? (
              <Form.Item
                name="actorUserId"
                label="Actor User ID"
                rules={[{ pattern: /^[1-9]\d*$/, message: 'Actor User ID 必须是正整数' }]}
              >
                <Input placeholder="可选" style={{ width: 160 }} />
              </Form.Item>
            ) : null}

            <Form.Item
              name="limit"
              label="Limit"
              rules={[{ required: true, message: '请输入 Limit' }]}
            >
              <InputNumber min={1} max={200} precision={0} style={{ width: 110 }} />
            </Form.Item>

            <Form.Item
              name="offset"
              label="Offset"
              rules={[{ required: true, message: '请输入 Offset' }]}
            >
              <InputNumber min={0} precision={0} style={{ width: 110 }} />
            </Form.Item>

            <Form.Item>
              <Button
                type="primary"
                icon={<ReloadOutlined />}
                onClick={() => void submitQuery()}
                loading={loading}
                disabled={!canQuery}
              >
                查询审计日志
              </Button>
            </Form.Item>
          </Form>
        </Card>

        <Card title="Audit Logs">
          {queryError ? <Alert type="error" showIcon style={{ marginBottom: 12 }} message={queryError} /> : null}
          <Table<AuditLog>
            rowKey={(record) =>
              record.id ? String(record.id) : `${record.requestId}-${record.createdAt}`
            }
            columns={columns}
            dataSource={logs}
            loading={loading}
            pagination={{ pageSize: 10 }}
            locale={{
              emptyText: hasSearched ? (
                <Empty
                  image={Empty.PRESENTED_IMAGE_SIMPLE}
                  description="未查询到审计日志，请调整筛选条件后重试。"
                />
              ) : (
                <Typography.Text type="secondary">
                  请先设置筛选条件并点击“查询审计日志”。
                </Typography.Text>
              ),
            }}
          />
        </Card>
      </Space>
    </>
  )
}
