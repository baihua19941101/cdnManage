import { ReloadOutlined } from '@ant-design/icons'
import { useSearchParams } from 'react-router-dom'
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

type PlatformAuditFilterOptions = {
  actions?: string[]
  targetTypes?: string[]
}

type ProjectAuditFilterProjectOption = {
  projectId: number
  projectName: string
}

type ProjectAuditFilterOptions = {
  projects?: ProjectAuditFilterProjectOption[]
  actions?: string[]
  targetTypes?: string[]
}

type ProjectSummary = {
  id: number
  name: string
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

type QuerySearchParams = {
  scope?: QueryScope
  projectId?: string
  action?: string
  result?: AuditResult
  targetType?: string
  targetIdentifier?: string
  actorUserId?: string
  limit?: number
  offset?: number
  autoQuery: boolean
}

type BuildQueryParamsOptions = {
  includeActorUserId: boolean
  availableActions: string[]
  availableTargetTypes: string[]
}

const normalizeSelectValue = (value: string | undefined, availableOptions: string[]) => {
  const normalized = value?.trim()
  if (!normalized) {
    return undefined
  }
  if (availableOptions.length > 0 && !availableOptions.includes(normalized)) {
    return undefined
  }
  return normalized
}

const buildQueryParams = (values: QueryFormValues, options: BuildQueryParamsOptions) => {
  const params: Record<string, string | number> = {
    limit: values.limit,
    offset: values.offset,
  }

  const action = normalizeSelectValue(values.action, options.availableActions)
  const result = values.result?.trim()
  const targetType = normalizeSelectValue(values.targetType, options.availableTargetTypes)
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

  if (options.includeActorUserId) {
    const actorUserId = values.actorUserId?.trim()
    if (actorUserId) {
      params.actorUserId = actorUserId
    }
  }

  return params
}

const normalizePositiveInteger = (value: string | null) => {
  if (!value) {
    return undefined
  }
  const normalized = value.trim()
  if (!/^[1-9]\d*$/.test(normalized)) {
    return undefined
  }
  return normalized
}

const normalizeNonNegativeInteger = (value: string | null) => {
  if (!value) {
    return undefined
  }
  const normalized = value.trim()
  if (!/^\d+$/.test(normalized)) {
    return undefined
  }
  return normalized
}

const normalizeAuditResult = (value: string | null): AuditResult | undefined => {
  if (value === 'success' || value === 'failure' || value === 'denied') {
    return value
  }
  return undefined
}

const parseSearchParams = (
  searchParams: URLSearchParams,
  canQueryPlatformScope: boolean,
): QuerySearchParams => {
  const scopeParam = searchParams.get('scope')
  const parsedScope: QueryScope | undefined =
    scopeParam === 'platform' || scopeParam === 'project'
      ? scopeParam
      : undefined

  const requestedScope: QueryScope = parsedScope ?? (canQueryPlatformScope ? 'platform' : 'project')
  const scope: QueryScope = !canQueryPlatformScope && requestedScope === 'platform'
    ? 'project'
    : requestedScope

  const projectId = normalizePositiveInteger(searchParams.get('projectId'))
  const action = searchParams.get('action')?.trim() || undefined
  const result = normalizeAuditResult(searchParams.get('result'))
  const targetType = searchParams.get('targetType')?.trim() || undefined
  const targetIdentifier = searchParams.get('targetIdentifier')?.trim() || undefined
  const actorUserId = normalizePositiveInteger(searchParams.get('actorUserId'))
  const limit = Number(normalizePositiveInteger(searchParams.get('limit')) ?? '20')
  const offset = Number(normalizeNonNegativeInteger(searchParams.get('offset')) ?? '0')

  return {
    scope,
    projectId,
    action,
    result,
    targetType,
    targetIdentifier,
    actorUserId,
    limit,
    offset,
    autoQuery: searchParams.get('autoQuery') === '1',
  }
}

export function AuditsPage() {
  const [messageApi, messageContext] = message.useMessage()
  const [queryForm] = Form.useForm<QueryFormValues>()
  const [searchParams] = useSearchParams()
  const userID = useAuthStore((state) => state.user?.id)
  const platformRole = useAuthStore((state) => state.user?.platformRole)
  const canQuery = Boolean(userID)
  const canQueryPlatformScope = isPlatformAdminRole(platformRole)

  const [loading, setLoading] = useState(false)
  const [queryError, setQueryError] = useState<string | null>(null)
  const [logs, setLogs] = useState<AuditLog[]>([])
  const [hasSearched, setHasSearched] = useState(false)
  const [platformActions, setPlatformActions] = useState<string[]>([])
  const [platformTargetTypes, setPlatformTargetTypes] = useState<string[]>([])
  const [platformFilterOptionsLoading, setPlatformFilterOptionsLoading] = useState(false)
  const [platformFilterOptionsError, setPlatformFilterOptionsError] = useState<string | null>(null)
  const [projectOptions, setProjectOptions] = useState<ProjectAuditFilterProjectOption[]>([])
  const [projectActions, setProjectActions] = useState<string[]>([])
  const [projectTargetTypes, setProjectTargetTypes] = useState<string[]>([])
  const [projectFilterOptionsLoading, setProjectFilterOptionsLoading] = useState(false)
  const [projectFilterOptionsError, setProjectFilterOptionsError] = useState<string | null>(null)
  const [projectFieldHint, setProjectFieldHint] = useState<string | null>(null)
  const [actionFieldHint, setActionFieldHint] = useState<string | null>(null)
  const [targetTypeFieldHint, setTargetTypeFieldHint] = useState<string | null>(null)
  const [scopeFieldHint, setScopeFieldHint] = useState<string | null>(null)
  const [pendingAutoQuery, setPendingAutoQuery] = useState(false)

  const scope = Form.useWatch('scope', queryForm) ?? 'platform'
  const selectedProjectID = Form.useWatch('projectId', queryForm)

  const toNonEmptyStrings = (input: unknown): string[] =>
    Array.isArray(input)
      ? input.filter((option): option is string => typeof option === 'string' && option.trim().length > 0)
      : []

  const loadProjectList = async () => {
    setProjectFilterOptionsError(null)
    setProjectFieldHint(null)
    try {
      const response = await apiClient.get<ApiResponse<ProjectSummary[]>>('/projects')
      const items = Array.isArray(response.data.data) ? response.data.data : []
      const projects = items
        .filter((item) => Number.isFinite(item.id) && item.id > 0)
        .map((item) => ({
          projectId: item.id,
          projectName: item.name?.trim() || `Project-${item.id}`,
        }))
      setProjectOptions(projects)
      const currentProjectID = queryForm.getFieldValue('projectId')
      if (
        typeof currentProjectID === 'string' &&
        currentProjectID.trim().length > 0 &&
        !projects.some((project) => String(project.projectId) === currentProjectID)
      ) {
        queryForm.setFieldsValue({
          projectId: '',
          action: undefined,
          targetType: undefined,
        })
        setProjectActions([])
        setProjectTargetTypes([])
      }
    } catch (error) {
      setProjectOptions([])
      setProjectActions([])
      setProjectTargetTypes([])
      setProjectFieldHint('Project ID 选项加载失败，请稍后重试。')
      setProjectFilterOptionsError(resolveAPIErrorMessage(error, '项目级审计筛选项目列表加载失败。'))
    }
  }

  useEffect(() => {
    if (!canQueryPlatformScope) {
      queryForm.setFieldValue('scope', 'project')
    }
  }, [canQueryPlatformScope, queryForm])

  useEffect(() => {
    const parsed = parseSearchParams(searchParams, canQueryPlatformScope)
    const shouldApply =
      parsed.autoQuery ||
      Boolean(parsed.projectId || parsed.action || parsed.result || parsed.targetType || parsed.targetIdentifier || parsed.actorUserId)
    if (!shouldApply) {
      return
    }

    queryForm.setFieldsValue({
      scope: parsed.scope,
      projectId: parsed.projectId ?? '',
      action: parsed.action,
      result: parsed.result,
      targetType: parsed.targetType,
      targetIdentifier: parsed.targetIdentifier,
      actorUserId: parsed.actorUserId ?? '',
      limit: parsed.limit ?? 20,
      offset: parsed.offset ?? 0,
    })

    if (parsed.autoQuery) {
      const projectScopeMissingProjectID = parsed.scope === 'project' && !parsed.projectId
      if (!projectScopeMissingProjectID) {
        setPendingAutoQuery(true)
      }
    }
  }, [canQueryPlatformScope, queryForm, searchParams])

  useEffect(() => {
    const loadPlatformFilterOptions = async () => {
      if (!canQuery || !canQueryPlatformScope || scope !== 'platform') {
        return
      }

      setPlatformFilterOptionsLoading(true)
      setPlatformFilterOptionsError(null)
      setActionFieldHint(null)
      setTargetTypeFieldHint(null)
      try {
        const response = await apiClient.get<ApiResponse<PlatformAuditFilterOptions>>(
          '/audits/filter-options',
        )
        const actions = toNonEmptyStrings(response.data.data?.actions)
        const targetTypes = toNonEmptyStrings(response.data.data?.targetTypes)
        setPlatformActions(actions)
        setPlatformTargetTypes(targetTypes)
      } catch (error) {
        setPlatformActions([])
        setPlatformTargetTypes([])
        setActionFieldHint('Action 选项加载失败，请稍后重试。')
        setTargetTypeFieldHint('Target Type 选项加载失败，请稍后重试。')
        setPlatformFilterOptionsError(
          resolveAPIErrorMessage(error, '审计筛选选项加载失败，可直接查询全部日志。'),
        )
      } finally {
        setPlatformFilterOptionsLoading(false)
      }
    }

    void loadPlatformFilterOptions()
  }, [canQuery, canQueryPlatformScope, scope])

  useEffect(() => {
    const bootstrapProjectScope = async () => {
      if (!canQuery || scope !== 'project') {
        return
      }
      await loadProjectList()
    }

    void bootstrapProjectScope()
  }, [canQuery, scope])

  useEffect(() => {
    const loadProjectScopedFilterOptions = async () => {
      if (!canQuery || scope !== 'project') {
        return
      }
      const projectID = Number(selectedProjectID)
      if (!Number.isFinite(projectID) || projectID <= 0) {
        setProjectActions([])
        setProjectTargetTypes([])
        setProjectFilterOptionsError(null)
        setActionFieldHint(null)
        setTargetTypeFieldHint(null)
        return
      }

      setProjectFilterOptionsLoading(true)
      setProjectFilterOptionsError(null)
      setActionFieldHint(null)
      setTargetTypeFieldHint(null)
      try {
        const response = await apiClient.get<ApiResponse<ProjectAuditFilterOptions>>(
          `/projects/${projectID}/audits/filter-options`,
        )
        const actions = toNonEmptyStrings(response.data.data?.actions)
        const targetTypes = toNonEmptyStrings(response.data.data?.targetTypes)
        const projects = Array.isArray(response.data.data?.projects)
          ? response.data.data.projects.filter(
              (project): project is ProjectAuditFilterProjectOption =>
                Number.isFinite(project.projectId) && project.projectId > 0,
            )
          : []

        if (projects.length > 0) {
          setProjectOptions(projects)
        }
        setProjectActions(actions)
        setProjectTargetTypes(targetTypes)
        setActionFieldHint(null)
        setTargetTypeFieldHint(null)

        const currentAction = queryForm.getFieldValue('action')
        if (typeof currentAction === 'string' && currentAction.trim().length > 0 && !actions.includes(currentAction)) {
          queryForm.setFieldValue('action', undefined)
        }
        const currentTargetType = queryForm.getFieldValue('targetType')
        if (
          typeof currentTargetType === 'string' &&
          currentTargetType.trim().length > 0 &&
          !targetTypes.includes(currentTargetType)
        ) {
          queryForm.setFieldValue('targetType', undefined)
        }
      } catch (error) {
        setProjectActions([])
        setProjectTargetTypes([])
        setActionFieldHint('Action 选项加载失败，请稍后重试。')
        setTargetTypeFieldHint('Target Type 选项加载失败，请稍后重试。')
        setProjectFilterOptionsError(
          resolveAPIErrorMessage(error, '项目级审计筛选选项加载失败，可直接查询全部日志。'),
        )
      } finally {
        setProjectFilterOptionsLoading(false)
      }
    }

    void loadProjectScopedFilterOptions()
  }, [canQuery, scope, selectedProjectID, queryForm])

  const submitQuery = async () => {
    if (!canQuery) {
      return
    }

    setScopeFieldHint(null)
    setProjectFieldHint((current) =>
      current === '筛选上下文无效，请先选择有效的 Project ID。' || current === '查询失败，请检查 Project ID 后重试。'
        ? null
        : current,
    )
    const values = await queryForm.validateFields().catch(() => null)
    if (!values) {
      setQueryError('筛选条件校验失败，请修正带提示的字段后重试。')
      messageApi.error('筛选条件校验失败，请检查表单提示。')
      return
    }

    setLoading(true)
    setQueryError(null)
    setHasSearched(true)

    try {
      if (values.scope === 'project') {
        const projectID = Number(values.projectId)
        if (!Number.isFinite(projectID) || projectID <= 0) {
          setProjectFieldHint('筛选上下文无效，请先选择有效的 Project ID。')
          setQueryError('筛选上下文无效，请先选择有效的 Project ID。')
          messageApi.error('Project ID 必须是正整数。')
          setLoading(false)
          return
        }

        const response = await apiClient.get<ApiResponse<ListAuditLogsPayload>>(
          `/projects/${projectID}/audits`,
          {
            params: buildQueryParams(values, {
              includeActorUserId: false,
              availableActions: projectActions,
              availableTargetTypes: projectTargetTypes,
            }),
          },
        )
        setLogs(Array.isArray(response.data.data?.logs) ? response.data.data.logs : [])
        return
      }

      if (!canQueryPlatformScope) {
        setScopeFieldHint('筛选上下文无效，当前账号仅支持项目级审计查询。')
        setQueryError('筛选上下文无效，当前账号仅支持项目级审计查询。')
        messageApi.error('当前账号仅支持项目级审计查询。')
        setLogs([])
        setLoading(false)
        return
      }

      const response = await apiClient.get<ApiResponse<ListAuditLogsPayload>>('/audits', {
        params: buildQueryParams(values, {
          includeActorUserId: true,
          availableActions: platformActions,
          availableTargetTypes: platformTargetTypes,
        }),
      })
      setLogs(Array.isArray(response.data.data?.logs) ? response.data.data.logs : [])
    } catch (error) {
      setLogs([])
      if (values.scope === 'project') {
        setProjectFieldHint('查询失败，请检查 Project ID 后重试。')
      } else {
        setScopeFieldHint('查询失败，请检查筛选条件后重试。')
      }
      setQueryError(resolveAPIErrorMessage(error, '审计日志查询失败，请稍后重试。'))
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    if (!pendingAutoQuery || loading) {
      return
    }
    setPendingAutoQuery(false)
    void submitQuery()
  }, [loading, pendingAutoQuery])

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
            <Form.Item
              name="scope"
              label="查询范围"
              validateStatus={scopeFieldHint ? 'error' : undefined}
              help={scopeFieldHint ?? undefined}
            >
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
                rules={[{ required: true, message: '请选择 Project ID' }]}
                validateStatus={projectFieldHint ? 'error' : undefined}
                help={projectFieldHint ?? undefined}
              >
                <Select
                  showSearch
                  allowClear
                  placeholder="请选择或搜索 Project ID"
                  style={{ width: 280 }}
                  options={projectOptions.map((project) => ({
                    value: String(project.projectId),
                    label: `${project.projectId} - ${project.projectName}`,
                  }))}
                  filterOption={(input, option) =>
                    String(option?.label ?? '')
                      .toLowerCase()
                      .includes(input.toLowerCase())
                  }
                  onChange={(value) => {
                    setProjectFieldHint(null)
                    if (!value) {
                      setProjectActions([])
                      setProjectTargetTypes([])
                    }
                    queryForm.setFieldsValue({
                      action: undefined,
                      targetType: undefined,
                    })
                  }}
                  onOpenChange={(open) => {
                    if (open && projectOptions.length === 0) {
                      void loadProjectList()
                    }
                  }}
                />
              </Form.Item>
            ) : null}

            {scope === 'platform' ? (
              <Form.Item
                name="action"
                label="Action"
                validateStatus={actionFieldHint ? 'error' : undefined}
                help={actionFieldHint ?? undefined}
              >
                <Select
                  showSearch
                  allowClear
                  placeholder="全部 Action"
                  style={{ width: 220 }}
                  loading={platformFilterOptionsLoading}
                  options={platformActions.map((value) => ({ label: value, value }))}
                  notFoundContent="暂无可选 Action，可直接查询全部。"
                  onChange={() => setActionFieldHint(null)}
                />
              </Form.Item>
            ) : (
              <Form.Item
                name="action"
                label="Action"
                validateStatus={actionFieldHint ? 'error' : undefined}
                help={actionFieldHint ?? undefined}
              >
                <Select
                  showSearch
                  allowClear
                  placeholder="全部 Action"
                  style={{ width: 220 }}
                  loading={projectFilterOptionsLoading}
                  options={projectActions.map((value) => ({ label: value, value }))}
                  notFoundContent={
                    selectedProjectID
                      ? '暂无可选 Action，可直接查询全部。'
                      : '请先选择 Project ID。'
                  }
                  onChange={() => setActionFieldHint(null)}
                />
              </Form.Item>
            )}

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

            {scope === 'platform' ? (
              <Form.Item
                name="targetType"
                label="Target Type"
                validateStatus={targetTypeFieldHint ? 'error' : undefined}
                help={targetTypeFieldHint ?? undefined}
              >
                <Select
                  showSearch
                  allowClear
                  placeholder="全部 Target Type"
                  style={{ width: 220 }}
                  loading={platformFilterOptionsLoading}
                  options={platformTargetTypes.map((value) => ({ label: value, value }))}
                  notFoundContent="暂无可选 Target Type，可直接查询全部。"
                  onChange={() => setTargetTypeFieldHint(null)}
                />
              </Form.Item>
            ) : (
              <Form.Item
                name="targetType"
                label="Target Type"
                validateStatus={targetTypeFieldHint ? 'error' : undefined}
                help={targetTypeFieldHint ?? undefined}
              >
                <Select
                  showSearch
                  allowClear
                  placeholder="全部 Target Type"
                  style={{ width: 220 }}
                  loading={projectFilterOptionsLoading}
                  options={projectTargetTypes.map((value) => ({ label: value, value }))}
                  notFoundContent={
                    selectedProjectID
                      ? '暂无可选 Target Type，可直接查询全部。'
                      : '请先选择 Project ID。'
                  }
                  onChange={() => setTargetTypeFieldHint(null)}
                />
              </Form.Item>
            )}

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
          {scope === 'platform' && !platformFilterOptionsLoading && !platformFilterOptionsError ? (
            <>
              {platformActions.length === 0 ? (
                <Typography.Text type="secondary">
                  当前暂无 Action 可选值，可直接查询全部日志。
                </Typography.Text>
              ) : null}
              {platformTargetTypes.length === 0 ? (
                <Typography.Text type="secondary" style={{ marginLeft: 12 }}>
                  当前暂无 Target Type 可选值，可直接查询全部日志。
                </Typography.Text>
              ) : null}
            </>
          ) : null}
          {scope === 'platform' && platformFilterOptionsError ? (
            <Alert type="warning" showIcon style={{ marginTop: 12 }} message={platformFilterOptionsError} />
          ) : null}
          {scope === 'project' &&
          selectedProjectID &&
          !projectFilterOptionsLoading &&
          !projectFilterOptionsError ? (
            <>
              {projectActions.length === 0 ? (
                <Typography.Text type="secondary">
                  当前项目暂无 Action 可选值，可直接查询全部日志。
                </Typography.Text>
              ) : null}
              {projectTargetTypes.length === 0 ? (
                <Typography.Text type="secondary" style={{ marginLeft: 12 }}>
                  当前项目暂无 Target Type 可选值，可直接查询全部日志。
                </Typography.Text>
              ) : null}
            </>
          ) : null}
          {scope === 'project' && projectFilterOptionsError ? (
            <Alert type="warning" showIcon style={{ marginTop: 12 }} message={projectFilterOptionsError} />
          ) : null}
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
