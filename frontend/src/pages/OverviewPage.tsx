import { Link } from 'react-router-dom'
import {
  AuditOutlined,
  CopyOutlined,
  CloudServerOutlined,
  DeploymentUnitOutlined,
  FolderOpenOutlined,
  ReloadOutlined,
  SyncOutlined,
  UploadOutlined,
} from '@ant-design/icons'
import { Alert, Button, Card, Col, Row, Segmented, Space, Spin, Table, Tag, Typography } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { resolveAPIErrorMessage } from '../services/api/error'
import { apiClient } from '../services/api/client'
import { isPlatformAdminRole, useAuthStore } from '../store/auth'

type TimeRange = '24h' | '7d'

type ApiResponse<T> = {
  code: string
  message: string
  requestId?: string
  data: T
}

type ProjectOption = {
  id: number
  name: string
}

type ProjectContext = {
  id: number
  currentProjectRole?: 'project_admin' | 'project_read_only'
  buckets?: Array<{
    bucketName?: string
    isPrimary?: boolean
  }>
  cdns?: Array<{
    cdnEndpoint?: string
    isPrimary?: boolean
  }>
}

type AuditLog = {
  id: number
  actorUserId?: number
  actorUsername?: string
  action: string
  result: string
  createdAt?: string
  targetType?: string
  targetIdentifier?: string
  requestId?: string
}

type ListAuditLogsPayload = {
  logs: AuditLog[]
}

type OverviewCoreMetrics = {
  uploadSessionTotal: number
  uploadFailureTotal: number
  cdnOperationTotal: number
  cdnFailureTotal: number
  visibleProjectTotal: number
}

type ProjectWorkbenchRow = {
  key: number
  projectId: number
  projectName: string
  primaryBucket: string
  primaryCDN: string
  latestUploadAt: string
  latestRefreshAt: string
  failureCount: number
}

type RiskItem = {
  key: string
  title: string
  summary: string
  occurredAt: string
  auditLink: string
}

type RecentActivityItem = {
  key: string
  actor: string
  project: string
  action: string
  result: string
  time: string
  requestId: string
  traceLink: string
}

const AUDIT_PAGE_SIZE = 200
const RECENT_ACTIVITY_LIMIT = 20
const UPLOAD_SESSION_ACTION = 'object.upload_archive'
const CDN_ACTIONS = new Set(['cdn.refresh_url', 'cdn.refresh_directory', 'cdn.sync_resources'])
const FAILURE_RESULT = 'failure'

const quickActionItems = [
  {
    titleKey: 'overview.quickActions.uploadTitle',
    descriptionKey: 'overview.quickActions.uploadDescription',
    to: '/storage',
    icon: <UploadOutlined />,
    requiresWriteAccess: true,
  },
  {
    titleKey: 'overview.quickActions.urlRefreshTitle',
    descriptionKey: 'overview.quickActions.urlRefreshDescription',
    to: '/cdn',
    icon: <CloudServerOutlined />,
    requiresWriteAccess: true,
  },
  {
    titleKey: 'overview.quickActions.syncTitle',
    descriptionKey: 'overview.quickActions.syncDescription',
    to: '/cdn',
    icon: <SyncOutlined />,
    requiresWriteAccess: true,
  },
  {
    titleKey: 'overview.quickActions.auditsTitle',
    descriptionKey: 'overview.quickActions.auditsDescription',
    to: '/audits',
    icon: <AuditOutlined />,
    requiresWriteAccess: false,
  },
] as const

const emptyMetrics: OverviewCoreMetrics = {
  uploadSessionTotal: 0,
  uploadFailureTotal: 0,
  cdnOperationTotal: 0,
  cdnFailureTotal: 0,
  visibleProjectTotal: 0,
}

const pickPrimaryOrFirst = <T,>(
  items: T[] | undefined,
  valueGetter: (item: T) => string | undefined,
  primaryGetter: (item: T) => boolean | undefined,
) => {
  if (!Array.isArray(items) || items.length === 0) {
    return '-'
  }
  const normalized = items
    .map((item) => valueGetter(item)?.trim())
    .filter((value): value is string => Boolean(value))
  if (normalized.length === 0) {
    return '-'
  }
  const primary = items.find((item) => {
    const value = valueGetter(item)?.trim()
    return Boolean(value) && primaryGetter(item)
  })
  const primaryValue = primary ? valueGetter(primary)?.trim() : ''
  return primaryValue || normalized[0]
}

const formatAuditTime = (value: string | undefined) => {
  if (!value) {
    return '-'
  }
  const timestamp = Date.parse(value)
  if (!Number.isFinite(timestamp)) {
    return '-'
  }
  return new Date(timestamp).toLocaleString()
}

const buildAuditTimeRangeParams = (range: TimeRange) => {
  const now = new Date()
  const durationHours = range === '24h' ? 24 : 7 * 24
  const createdAfter = new Date(now.getTime() - durationHours * 60 * 60 * 1000)
  return {
    createdAfter: createdAfter.toISOString(),
    createdBefore: now.toISOString(),
  }
}

const aggregateMetrics = (logs: AuditLog[], visibleProjectTotal: number): OverviewCoreMetrics => {
  let uploadSessionTotal = 0
  let uploadFailureTotal = 0
  let cdnOperationTotal = 0
  let cdnFailureTotal = 0

  logs.forEach((log) => {
    if (log.action === UPLOAD_SESSION_ACTION) {
      uploadSessionTotal += 1
      if (log.result === 'failure') {
        uploadFailureTotal += 1
      }
      return
    }

    if (CDN_ACTIONS.has(log.action)) {
      cdnOperationTotal += 1
      if (log.result === 'failure') {
        cdnFailureTotal += 1
      }
    }
  })

  return {
    uploadSessionTotal,
    uploadFailureTotal,
    cdnOperationTotal,
    cdnFailureTotal,
    visibleProjectTotal,
  }
}

export function OverviewPage() {
  const { t } = useTranslation()
  const [range, setRange] = useState<TimeRange>('24h')
  const platformRole = useAuthStore((state) => state.user?.platformRole)
  const isPlatformAdmin = isPlatformAdminRole(platformRole)
  const [metrics, setMetrics] = useState<OverviewCoreMetrics>(emptyMetrics)
  const [workbenchRows, setWorkbenchRows] = useState<ProjectWorkbenchRow[]>([])
  const [failedUploadRiskItems, setFailedUploadRiskItems] = useState<RiskItem[]>([])
  const [failedCDNRiskItems, setFailedCDNRiskItems] = useState<RiskItem[]>([])
  const [recentActivityItems, setRecentActivityItems] = useState<RecentActivityItem[]>([])
  const [loading, setLoading] = useState(false)
  const [errorText, setErrorText] = useState<string | null>(null)
  const [lastUpdatedAt, setLastUpdatedAt] = useState<string | null>(null)
  const [hasWritableProjectAccess, setHasWritableProjectAccess] = useState(isPlatformAdmin)
  const requestSequenceRef = useRef(0)

  const listAuditLogs = useCallback(
    async (
      endpoint: string,
      params: Record<string, string>,
    ): Promise<AuditLog[]> => {
      const items: AuditLog[] = []
      let offset = 0

      while (true) {
        const response = await apiClient.get<ApiResponse<ListAuditLogsPayload>>(endpoint, {
          params: {
            ...params,
            limit: AUDIT_PAGE_SIZE,
            offset,
          },
        })
        const pageLogs = Array.isArray(response.data.data?.logs) ? response.data.data.logs : []
        items.push(...pageLogs)

        if (pageLogs.length < AUDIT_PAGE_SIZE) {
          break
        }
        offset += AUDIT_PAGE_SIZE
      }

      return items
    },
    [],
  )

  const buildAuditLinkWithQuery = useCallback(
    (projectID: number, action: string, targetType?: string, targetIdentifier?: string) => {
      const query = new URLSearchParams({
        scope: 'project',
        projectId: String(projectID),
        action,
        result: FAILURE_RESULT,
        limit: '20',
        offset: '0',
        autoQuery: '1',
      })
      if (targetType?.trim()) {
        query.set('targetType', targetType.trim())
      }
      if (targetIdentifier?.trim()) {
        query.set('targetIdentifier', targetIdentifier.trim())
      }
      return `/audits?${query.toString()}`
    },
    [],
  )

  const buildTraceLink = useCallback(
    (
      projectID: number,
      action: string,
      result: string,
      targetType?: string,
      targetIdentifier?: string,
      requestId?: string,
    ) => {
      const query = new URLSearchParams({
        scope: 'project',
        projectId: String(projectID),
        action,
        result,
        limit: '20',
        offset: '0',
        autoQuery: '1',
      })
      if (targetType?.trim()) {
        query.set('targetType', targetType.trim())
      }
      if (targetIdentifier?.trim()) {
        query.set('targetIdentifier', targetIdentifier.trim())
      }
      if (requestId?.trim()) {
        query.set('requestId', requestId.trim())
      }
      return `/audits?${query.toString()}`
    },
    [],
  )

  const loadCoreMetrics = useCallback(async () => {
    const requestID = requestSequenceRef.current + 1
    requestSequenceRef.current = requestID
    setLoading(true)
    setErrorText(null)

    try {
      const accessibleResponse = await apiClient.get<ApiResponse<ProjectOption[]>>('/projects/accessible')
      const projectOptions = Array.isArray(accessibleResponse.data.data)
        ? accessibleResponse.data.data
        : []
      const normalizedProjectOptions = projectOptions
        .filter((project) => Number.isFinite(project.id) && project.id > 0)
        .map((project) => ({
          id: project.id,
          name: project.name?.trim() || t('overview.activity.projectFallback', { projectId: project.id }),
        }))

      const visibleProjectTotal = normalizedProjectOptions.length
      const projectNameByID = new Map(
        normalizedProjectOptions.map((project) => [project.id, project.name] as const),
      )

      const timeParams = buildAuditTimeRangeParams(range)
      const contextEntries = await Promise.all(
        normalizedProjectOptions.map(async (project) => {
          try {
            const response = await apiClient.get<ApiResponse<ProjectContext>>(
              `/projects/${project.id}/context`,
            )
            return {
              projectID: project.id,
              context: response.data.data,
            }
          } catch {
            return {
              projectID: project.id,
              context: null,
            }
          }
        }),
      )

      const writableProjectIDs = contextEntries
        .filter(({ context }) => Boolean(context && context.id > 0))
        .filter(({ context }) => isPlatformAdmin || context?.currentProjectRole === 'project_admin')
        .map((entry) => entry.projectID)
      setHasWritableProjectAccess(isPlatformAdmin || writableProjectIDs.length > 0)

      const projectAuditLogPairs = await Promise.all(
        writableProjectIDs.map(async (projectID) => {
          try {
            const logs = await listAuditLogs(`/projects/${projectID}/audits`, timeParams)
            return [projectID, logs] as const
          } catch {
            return [projectID, [] as AuditLog[]] as const
          }
        }),
      )
      const projectAuditLogMap = new Map<number, AuditLog[]>(projectAuditLogPairs)

      let logs: AuditLog[] = []
      if (isPlatformAdmin) {
        logs = Array.from(projectAuditLogMap.values()).flat()
      } else if (visibleProjectTotal > 0) {
        if (writableProjectIDs.length > 0) {
          logs = Array.from(projectAuditLogMap.values()).flat()
        }
      }

      if (requestSequenceRef.current !== requestID) {
        return
      }

      setMetrics(aggregateMetrics(logs, visibleProjectTotal))
      const failedUploadItems = writableProjectIDs
        .flatMap((projectID) =>
          (projectAuditLogMap.get(projectID) ?? [])
            .filter((log) => log.action === UPLOAD_SESSION_ACTION && log.result === FAILURE_RESULT)
            .map((log) => ({
              projectID,
              projectName:
                projectNameByID.get(projectID) ??
                t('overview.activity.projectFallback', { projectId: projectID }),
              log,
            })),
        )
        .sort(
          (left, right) =>
            Date.parse(right.log.createdAt ?? '') - Date.parse(left.log.createdAt ?? ''),
        )
        .slice(0, 5)
        .map((item, index) => ({
          key: `upload-${item.projectID}-${item.log.id || index}`,
          title: `${item.projectName} (#${item.projectID})`,
          summary: item.log.targetIdentifier?.trim()
            ? item.log.targetIdentifier.trim()
            : item.log.requestId?.trim()
              ? `requestId: ${item.log.requestId.trim()}`
              : t('overview.risks.uploadSessionFailed'),
          occurredAt: formatAuditTime(item.log.createdAt),
          auditLink: buildAuditLinkWithQuery(
            item.projectID,
            item.log.action,
            item.log.targetType,
            item.log.targetIdentifier,
          ),
        }))

      const failedCDNItems = writableProjectIDs
        .flatMap((projectID) =>
          (projectAuditLogMap.get(projectID) ?? [])
            .filter((log) => CDN_ACTIONS.has(log.action) && log.result === FAILURE_RESULT)
            .map((log) => ({
              projectID,
              projectName:
                projectNameByID.get(projectID) ??
                t('overview.activity.projectFallback', { projectId: projectID }),
              log,
            })),
        )
        .sort(
          (left, right) =>
            Date.parse(right.log.createdAt ?? '') - Date.parse(left.log.createdAt ?? ''),
        )
        .slice(0, 5)
        .map((item, index) => ({
          key: `cdn-${item.projectID}-${item.log.id || index}`,
          title: `${item.projectName} (#${item.projectID})`,
          summary: item.log.targetIdentifier?.trim()
            ? item.log.targetIdentifier.trim()
            : item.log.requestId?.trim()
              ? `requestId: ${item.log.requestId.trim()}`
              : item.log.action,
          occurredAt: formatAuditTime(item.log.createdAt),
          auditLink: buildAuditLinkWithQuery(
            item.projectID,
            item.log.action,
            item.log.targetType,
            item.log.targetIdentifier,
          ),
        }))

      setFailedUploadRiskItems(failedUploadItems)
      setFailedCDNRiskItems(failedCDNItems)
      const activityItems = writableProjectIDs
        .flatMap((projectID) =>
          (projectAuditLogMap.get(projectID) ?? []).map((log, index) => ({
            key: `activity-${projectID}-${log.id || index}`,
            actor: log.actorUsername?.trim()
              ? log.actorUsername.trim()
              : Number.isFinite(log.actorUserId)
                ? `user#${log.actorUserId}`
                : t('overview.activity.unknownUser'),
            project: `${projectNameByID.get(projectID) ?? t('overview.activity.projectFallback', { projectId: projectID })} (#${projectID})`,
            action: log.action?.trim() || '-',
            result: log.result?.trim() || '-',
            time: formatAuditTime(log.createdAt),
            requestId: log.requestId?.trim() || '-',
            traceLink: buildTraceLink(
              projectID,
              log.action,
              log.result,
              log.targetType,
              log.targetIdentifier,
              log.requestId,
            ),
            createdAt: log.createdAt,
          })),
        )
        .sort(
          (left, right) =>
            Date.parse(right.createdAt ?? '') - Date.parse(left.createdAt ?? ''),
        )
        .slice(0, RECENT_ACTIVITY_LIMIT)
        .map(({ key, actor, project, action, result, time, requestId, traceLink }) => ({
          key,
          actor,
          project,
          action,
          result,
          time,
          requestId,
          traceLink,
        }))
      setRecentActivityItems(activityItems)
      const rows: ProjectWorkbenchRow[] = normalizedProjectOptions.map((project) => {
        const context =
          contextEntries.find((entry) => entry.projectID === project.id)?.context ?? null
        const logsForProject = projectAuditLogMap.get(project.id) ?? []

        let latestUploadAt = ''
        let latestRefreshAt = ''
        let failureCount = 0

        logsForProject.forEach((log) => {
          if (log.action === UPLOAD_SESSION_ACTION) {
            if (log.result === 'failure') {
              failureCount += 1
            }
            if (Date.parse(log.createdAt ?? '') > Date.parse(latestUploadAt || '')) {
              latestUploadAt = log.createdAt ?? ''
            }
            return
          }
          if (CDN_ACTIONS.has(log.action)) {
            if (log.result === 'failure') {
              failureCount += 1
            }
            if (Date.parse(log.createdAt ?? '') > Date.parse(latestRefreshAt || '')) {
              latestRefreshAt = log.createdAt ?? ''
            }
          }
        })

        return {
          key: project.id,
          projectId: project.id,
          projectName: project.name,
          primaryBucket: pickPrimaryOrFirst(
            context?.buckets,
            (item) => item.bucketName,
            (item) => item.isPrimary,
          ),
          primaryCDN: pickPrimaryOrFirst(
            context?.cdns,
            (item) => item.cdnEndpoint,
            (item) => item.isPrimary,
          ),
          latestUploadAt: formatAuditTime(latestUploadAt),
          latestRefreshAt: formatAuditTime(latestRefreshAt),
          failureCount,
        }
      })
      setWorkbenchRows(rows)
      setLastUpdatedAt(new Date().toISOString())
    } catch (error) {
      if (requestSequenceRef.current !== requestID) {
        return
      }
      setMetrics((previous) => ({
        ...emptyMetrics,
        visibleProjectTotal: previous.visibleProjectTotal,
      }))
      setWorkbenchRows([])
      setFailedUploadRiskItems([])
      setFailedCDNRiskItems([])
      setRecentActivityItems([])
      setErrorText(resolveAPIErrorMessage(error, t('overview.errors.coreLoadFailed')))
      if (!isPlatformAdmin) {
        setHasWritableProjectAccess(false)
      }
    } finally {
      if (requestSequenceRef.current === requestID) {
        setLoading(false)
      }
    }
  }, [buildAuditLinkWithQuery, buildTraceLink, isPlatformAdmin, listAuditLogs, range])

  useEffect(() => {
    void loadCoreMetrics()
  }, [loadCoreMetrics])

  const headerSubtitle = useMemo(
    () =>
      range === '24h'
        ? t('overview.subtitle24h')
        : t('overview.subtitle7d'),
    [range, t],
  )

  const coreMetricCards = useMemo(
    () => [
      {
        key: 'upload-total',
        title: t('overview.cards.uploadTotal'),
        value: metrics.uploadSessionTotal,
        icon: <UploadOutlined />,
      },
      {
        key: 'upload-failure',
        title: t('overview.cards.uploadFailure'),
        value: metrics.uploadFailureTotal,
        icon: <FolderOpenOutlined />,
      },
      {
        key: 'cdn-total',
        title: t('overview.cards.cdnTotal'),
        value: metrics.cdnOperationTotal,
        icon: <CloudServerOutlined />,
      },
      {
        key: 'cdn-failure',
        title: t('overview.cards.cdnFailure'),
        value: metrics.cdnFailureTotal,
        icon: <DeploymentUnitOutlined />,
      },
      {
        key: 'projects-visible',
        title: t('overview.cards.visibleProjects'),
        value: metrics.visibleProjectTotal,
        icon: <AuditOutlined />,
      },
    ],
    [metrics, t],
  )
  const visibleQuickActionItems = useMemo(
    () =>
      quickActionItems.filter(
        (item) => !item.requiresWriteAccess || hasWritableProjectAccess || isPlatformAdmin,
      ),
    [hasWritableProjectAccess, isPlatformAdmin],
  )

  const workbenchColumns = useMemo<ColumnsType<ProjectWorkbenchRow>>(
    () => [
      {
        title: t('overview.cards.projectId'),
        dataIndex: 'projectId',
        width: 120,
      },
      {
        title: t('overview.cards.projectName'),
        dataIndex: 'projectName',
        width: 220,
      },
      {
        title: t('overview.cards.primaryBucket'),
        dataIndex: 'primaryBucket',
        width: 220,
      },
      {
        title: t('overview.cards.primaryCDN'),
        dataIndex: 'primaryCDN',
        width: 260,
      },
      {
        title: t('overview.cards.latestUploadAt'),
        dataIndex: 'latestUploadAt',
        width: 200,
      },
      {
        title: t('overview.cards.latestRefreshAt'),
        dataIndex: 'latestRefreshAt',
        width: 200,
      },
      {
        title: t('overview.cards.failureCount'),
        dataIndex: 'failureCount',
        width: 120,
      },
      {
        title: t('overview.cards.shortcuts'),
        key: 'quick-links',
        width: 180,
        render: (_, record) => (
          <Space size={4}>
            <Button type="link" size="small" style={{ padding: 0 }}>
              <Link to={`/storage?projectId=${record.projectId}`}>{t('overview.cards.shortcutStorage')}</Link>
            </Button>
            <Button type="link" size="small" style={{ padding: 0 }}>
              <Link to={`/cdn?projectId=${record.projectId}`}>{t('overview.cards.shortcutCDN')}</Link>
            </Button>
          </Space>
        ),
      },
    ],
    [t],
  )

  const activityColumns = useMemo<ColumnsType<RecentActivityItem>>(
    () => [
      {
        title: t('overview.cards.actor'),
        dataIndex: 'actor',
        width: 180,
      },
      {
        title: t('overview.cards.project'),
        dataIndex: 'project',
        width: 260,
      },
      {
        title: t('overview.cards.action'),
        dataIndex: 'action',
        width: 220,
      },
      {
        title: t('overview.cards.result'),
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
        title: t('overview.cards.time'),
        dataIndex: 'time',
        width: 190,
      },
      {
        title: t('overview.cards.requestId'),
        dataIndex: 'requestId',
        width: 280,
        render: (value: string, record) =>
          value === '-' ? (
            <Typography.Text type="secondary">-</Typography.Text>
          ) : (
            <Space size={8}>
              <Typography.Text
                code
                copyable={{
                  text: value,
                  icon: [<CopyOutlined key="copy-icon" />, <CopyOutlined key="copied-icon" />],
                  tooltips: [t('overview.cards.copyRequestId'), t('overview.cards.copied')],
                }}
              >
                {value}
              </Typography.Text>
              <Button type="link" size="small" style={{ padding: 0 }}>
                <Link to={record.traceLink}>{t('overview.cards.trace')}</Link>
              </Button>
            </Space>
          ),
      },
    ],
    [t],
  )

  return (
    <Space direction="vertical" size={16} style={{ width: '100%' }}>
      <Card className="nt-hero-card">
        <Space
          direction="vertical"
          size={12}
          style={{ width: '100%', display: 'flex', justifyContent: 'space-between' }}
        >
          <div>
            <Typography.Text style={{ color: 'var(--nt-text-secondary)', letterSpacing: 1.6 }}>
              OVERVIEW
            </Typography.Text>
            <Typography.Title
              level={2}
              style={{
                marginTop: 8,
                marginBottom: 8,
                color: 'var(--nt-text-primary)',
              }}
            >
              {t('overview.heading')}
            </Typography.Title>
            <Typography.Paragraph style={{ marginBottom: 0, color: 'var(--nt-text-secondary)' }}>
              {headerSubtitle}
            </Typography.Paragraph>
          </div>

          <Space wrap>
            <Segmented<TimeRange>
              value={range}
              options={[
                { label: '24h', value: '24h' },
                { label: '7d', value: '7d' },
              ]}
              onChange={(value) => setRange(value)}
            />
            <Button icon={<ReloadOutlined />} onClick={() => void loadCoreMetrics()} loading={loading}>
              {t('overview.refresh')}
            </Button>
          </Space>
        </Space>
      </Card>

      <Card
        title={t('overview.cards.title')}
        extra={
          <Typography.Text type="secondary">
            {lastUpdatedAt
              ? t('overview.cards.updatedAt', { time: new Date(lastUpdatedAt).toLocaleString() })
              : t('overview.cards.waiting')}
          </Typography.Text>
        }
      >
        <Space direction="vertical" size={12} style={{ width: '100%' }}>
          {errorText ? <Alert type="warning" showIcon message={errorText} /> : null}
          <Row gutter={[16, 16]}>
            {coreMetricCards.map((item) => (
              <Col key={item.key} xs={24} sm={12} lg={8} xl={4}>
                <Card size="small" style={{ height: '100%' }}>
                  <Space direction="vertical" size={6}>
                    <Typography.Text type="secondary">
                      {item.icon} {item.title}
                    </Typography.Text>
                    <Typography.Title level={3} style={{ margin: 0 }}>
                      {loading ? <Spin size="small" /> : item.value}
                    </Typography.Title>
                  </Space>
                </Card>
              </Col>
            ))}
          </Row>
        </Space>
      </Card>

      <Card title={t('overview.quickActions.title')}>
        <Row gutter={[16, 16]}>
          {visibleQuickActionItems.map((item) => (
            <Col key={item.titleKey} xs={24} md={12} lg={6}>
              <Card size="small" style={{ height: '100%' }}>
                <Space direction="vertical" size={8} style={{ width: '100%' }}>
                  <Typography.Text strong>
                    {item.icon} {t(item.titleKey)}
                  </Typography.Text>
                  <Typography.Text type="secondary">{t(item.descriptionKey)}</Typography.Text>
                  <Button type="link" style={{ padding: 0 }}>
                    <Link to={item.to}>{t('overview.quickActions.open')}</Link>
                  </Button>
                </Space>
              </Card>
            </Col>
          ))}
        </Row>
      </Card>

      <Card title={t('overview.workbench.title')}>
        <Table<ProjectWorkbenchRow>
          rowKey="projectId"
          size="small"
          loading={loading}
          columns={workbenchColumns}
          dataSource={workbenchRows}
          pagination={{ pageSize: 10, showSizeChanger: false }}
          scroll={{ x: 1500 }}
        />
      </Card>

      <Card title={t('overview.risks.title')}>
        <Row gutter={[16, 16]}>
          <Col xs={24} lg={12}>
            <Space direction="vertical" size={10} style={{ width: '100%' }}>
              <Typography.Text strong>{t('overview.risks.recentUploadFailures')}</Typography.Text>
              {failedUploadRiskItems.length > 0 ? (
                failedUploadRiskItems.map((item) => (
                  <Card key={item.key} size="small">
                    <Space direction="vertical" size={4} style={{ width: '100%' }}>
                      <Typography.Text strong>{item.title}</Typography.Text>
                      <Typography.Text type="secondary">{item.occurredAt}</Typography.Text>
                      <Typography.Text>{item.summary}</Typography.Text>
                      <Button type="link" style={{ padding: 0, width: 'fit-content' }}>
                        <Link to={item.auditLink}>{t('overview.risks.viewAudit')}</Link>
                      </Button>
                    </Space>
                  </Card>
                ))
              ) : (
                <Typography.Text type="secondary">{t('overview.risks.noUploadFailures')}</Typography.Text>
              )}
            </Space>
          </Col>
          <Col xs={24} lg={12}>
            <Space direction="vertical" size={10} style={{ width: '100%' }}>
              <Typography.Text strong>{t('overview.risks.recentCDNFailures')}</Typography.Text>
              {failedCDNRiskItems.length > 0 ? (
                failedCDNRiskItems.map((item) => (
                  <Card key={item.key} size="small">
                    <Space direction="vertical" size={4} style={{ width: '100%' }}>
                      <Typography.Text strong>{item.title}</Typography.Text>
                      <Typography.Text type="secondary">{item.occurredAt}</Typography.Text>
                      <Typography.Text>{item.summary}</Typography.Text>
                      <Button type="link" style={{ padding: 0, width: 'fit-content' }}>
                        <Link to={item.auditLink}>{t('overview.risks.viewAudit')}</Link>
                      </Button>
                    </Space>
                  </Card>
                ))
              ) : (
                <Typography.Text type="secondary">{t('overview.risks.noCDNFailures')}</Typography.Text>
              )}
            </Space>
          </Col>
        </Row>
      </Card>

      <Card title={t('overview.activity.title')}>
        <Table<RecentActivityItem>
          rowKey="key"
          size="small"
          loading={loading}
          columns={activityColumns}
          dataSource={recentActivityItems}
          pagination={{ pageSize: 10, showSizeChanger: false }}
          locale={{ emptyText: t('overview.activity.empty') }}
          scroll={{ x: 1300 }}
        />
      </Card>
    </Space>
  )
}
