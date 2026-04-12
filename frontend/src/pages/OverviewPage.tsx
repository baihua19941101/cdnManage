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

type TimeRange = '24h' | '7d' | '30d'

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
  projectCount: number
  bucketCount: number
  cdnCount: number
  uploadSessionTotal: number
  cdnOperationTotal: number
  failureTotal: number
  uploadAvgDurationMs: number
}

type OverviewTrendPoint = {
  time: string
  success: number
  failed: number
}

type OverviewRatioPoint = {
  name: string
  value: number
}

type Translate = (key: string, options?: Record<string, unknown>) => string

type OverviewMetricsPayload = {
  timeWindow: TimeRange
  kpis?: OverviewCoreMetrics
  trends?: {
    uploadSessions?: OverviewTrendPoint[]
    cdnOperations?: OverviewTrendPoint[]
  }
  ratios?: {
    providerResourceShare?: OverviewRatioPoint[]
    operationTypeShare?: OverviewRatioPoint[]
  }
  emptyState?: {
    hasKpiData?: boolean
    hasTrendData?: boolean
    hasRatioData?: boolean
  }
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
  rawResult: string
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

type MetricTone =
  | 'cyan'
  | 'blue'
  | 'green'
  | 'purple'
  | 'orange'
  | 'indigo'
  | 'red'
  | 'teal'

const emptyMetrics: OverviewCoreMetrics = {
  projectCount: 0,
  bucketCount: 0,
  cdnCount: 0,
  uploadSessionTotal: 0,
  cdnOperationTotal: 0,
  failureTotal: 0,
  uploadAvgDurationMs: 0,
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
  const durationHours = range === '24h' ? 24 : range === '7d' ? 7 * 24 : 30 * 24
  const createdAfter = new Date(now.getTime() - durationHours * 60 * 60 * 1000)
  return {
    createdAfter: createdAfter.toISOString(),
    createdBefore: now.toISOString(),
  }
}

const chartColors = ['#00d4ff', '#4f8cff', '#36cfc9', '#ff9f43', '#ff6b6b', '#b37feb', '#95de64']

const buildPieGradient = (data: OverviewRatioPoint[]) => {
  const total = data.reduce((sum, item) => sum + item.value, 0)
  if (total <= 0) {
    return 'conic-gradient(#1f2a3a 0deg 360deg)'
  }
  let cursor = 0
  const segments = data.map((item, index) => {
    const value = Math.max(0, item.value)
    const degree = (value / total) * 360
    const start = cursor
    const end = cursor + degree
    cursor = end
    return `${chartColors[index % chartColors.length]} ${start}deg ${end}deg`
  })
  return `conic-gradient(${segments.join(', ')})`
}

const RatioPie = ({
  data,
  emptyText,
}: {
  data: OverviewRatioPoint[]
  emptyText: string
}) => {
  const total = data.reduce((sum, item) => sum + item.value, 0)
  if (total <= 0 || data.length === 0) {
    return <Typography.Text type="secondary">{emptyText}</Typography.Text>
  }

  return (
    <Row gutter={[12, 12]}>
      <Col xs={24} sm={10} md={8}>
        <div className="overview-donut-wrap">
          <div
            className="overview-donut"
            style={{
              background: buildPieGradient(data),
            }}
          >
            <div className="overview-donut-center">
              <Typography.Text type="secondary">Total</Typography.Text>
              <Typography.Title level={4} style={{ margin: 0 }}>
                {total}
              </Typography.Title>
            </div>
          </div>
        </div>
      </Col>
      <Col xs={24} sm={14} md={16}>
        <div className="overview-ratio-table">
          <div className="overview-ratio-head">
            <span>Name</span>
            <span>Count</span>
            <span>Share</span>
          </div>
          {data.map((item, index) => {
            const percent = ((item.value / total) * 100).toFixed(1)
            return (
              <div key={`${item.name}-${index}`} className="overview-ratio-row">
                <span className="overview-ratio-name">
                  <i
                    style={{
                      background: chartColors[index % chartColors.length],
                    }}
                  />
                  {item.name}
                </span>
                <span>{item.value}</span>
                <span>{percent}%</span>
              </div>
            )
          })}
        </div>
      </Col>
    </Row>
  )
}

const TrendLine = ({
  data,
  emptyText,
  successLabel,
  failedLabel,
}: {
  data: OverviewTrendPoint[]
  emptyText: string
  successLabel: string
  failedLabel: string
}) => {
  const [hoverIndex, setHoverIndex] = useState<number | null>(null)
  if (data.length === 0) {
    return <Typography.Text type="secondary">{emptyText}</Typography.Text>
  }

  const width = 520
  const height = 210
  const padding = 30
  const maxValue = Math.max(
    1,
    ...data.map((item) => Math.max(item.success, item.failed)),
  )
  const stepX = data.length > 1 ? (width - padding * 2) / (data.length - 1) : 0
  const y = (value: number) => height - padding - (value / maxValue) * (height - padding * 2)
  const pointsOf = (values: number[]) =>
    values
      .map((value, index) => `${padding + stepX * index},${y(value)}`)
      .join(' ')

  return (
    <Space direction="vertical" size={8} style={{ width: '100%' }}>
      <div className="overview-trend-wrap">
        <svg width="100%" viewBox={`0 0 ${width} ${height}`} preserveAspectRatio="xMidYMid meet">
          {Array.from({ length: 5 }).map((_, index) => {
            const value = (maxValue / 4) * (4 - index)
            const yPos = y(value)
            return (
              <g key={`grid-${index}`}>
                <line
                  x1={padding}
                  y1={yPos}
                  x2={width - padding}
                  y2={yPos}
                  stroke="#314056"
                  strokeOpacity={0.35}
                  strokeWidth="1"
                />
                <text x={2} y={yPos + 4} fill="#8ea6bf" fontSize="10">
                  {Math.round(value)}
                </text>
              </g>
            )
          })}
          <line x1={padding} y1={padding} x2={padding} y2={height - padding} stroke="#314056" strokeWidth="1" />
          <line x1={padding} y1={height - padding} x2={width - padding} y2={height - padding} stroke="#314056" strokeWidth="1" />
          {data.map((_, index) => {
            const x = padding + stepX * index
            return (
              <rect
                key={`hover-${index}`}
                x={x - 10}
                y={padding}
                width={20}
                height={height - padding * 2}
                fill="transparent"
                onMouseEnter={() => setHoverIndex(index)}
                onMouseLeave={() => setHoverIndex((previous) => (previous === index ? null : previous))}
              />
            )
          })}
          {hoverIndex !== null ? (
            <line
              x1={padding + stepX * hoverIndex}
              y1={padding}
              x2={padding + stepX * hoverIndex}
              y2={height - padding}
              stroke="#4f8cff"
              strokeDasharray="4 4"
              strokeOpacity={0.65}
            />
          ) : null}
        <polyline
          fill="none"
          stroke="#00d4ff"
          strokeWidth="2.2"
          strokeLinecap="round"
          strokeLinejoin="round"
          points={pointsOf(data.map((item) => item.success))}
        />
        <polyline
          fill="none"
          stroke="#ff6b6b"
          strokeWidth="2.2"
          strokeLinecap="round"
          strokeLinejoin="round"
          points={pointsOf(data.map((item) => item.failed))}
        />
          {data.map((item, index) => {
            const cx = padding + stepX * index
            return (
              <g key={`dot-${index}`}>
                <circle cx={cx} cy={y(item.success)} r={3} fill="#00d4ff" />
                <circle cx={cx} cy={y(item.failed)} r={3} fill="#ff6b6b" />
              </g>
            )
          })}
        </svg>
        {hoverIndex !== null ? (
          <div className="overview-trend-tooltip">
            <div className="overview-trend-tooltip-title">{data[hoverIndex]?.time || '-'}</div>
            <div>{successLabel}: {data[hoverIndex]?.success ?? 0}</div>
            <div>{failedLabel}: {data[hoverIndex]?.failed ?? 0}</div>
          </div>
        ) : null}
      </div>
      <Space style={{ justifyContent: 'space-between', width: '100%' }}>
        <Typography.Text type="secondary">{data[0]?.time || '-'}</Typography.Text>
        <Space size={12}>
          <Space size={4}>
            <span style={{ width: 8, height: 8, borderRadius: '50%', display: 'inline-block', background: '#00d4ff' }} />
            <Typography.Text type="secondary">{successLabel}</Typography.Text>
          </Space>
          <Space size={4}>
            <span style={{ width: 8, height: 8, borderRadius: '50%', display: 'inline-block', background: '#ff6b6b' }} />
            <Typography.Text type="secondary">{failedLabel}</Typography.Text>
          </Space>
        </Space>
        <Typography.Text type="secondary">{data[data.length - 1]?.time || '-'}</Typography.Text>
      </Space>
    </Space>
  )
}

const formatPercent = (value: number) => `${value.toFixed(1)}%`

const formatProviderLabel = (provider: string, t: Translate) => {
  const normalized = provider?.trim().toLowerCase()
  switch (normalized) {
    case 'aliyun':
      return t('overview.providers.aliyun')
    case 'tencent_cloud':
      return t('overview.providers.tencent')
    case 'huawei_cloud':
      return t('overview.providers.huawei')
    case 'qiniu':
      return t('overview.providers.qiniu')
    case 'unknown':
      return t('overview.providers.unknown')
    default:
      return provider
  }
}

const formatResultLabel = (result: string, t: Translate) => {
  switch (result) {
    case 'success':
      return t('overview.status.success')
    case 'failure':
      return t('overview.status.failure')
    case 'denied':
      return t('overview.status.denied')
    default:
      return result || '-'
  }
}

const formatActionLabel = (action: string, t: Translate) => {
  switch (action) {
    case 'object.upload_archive':
      return t('overview.actions.objectUploadArchive')
    case 'object.upload':
      return t('overview.actions.objectUpload')
    case 'object.delete':
      return t('overview.actions.objectDelete')
    case 'object.rename':
      return t('overview.actions.objectRename')
    case 'cdn.refresh_url':
      return t('overview.actions.cdnRefreshURL')
    case 'cdn.refresh_directory':
      return t('overview.actions.cdnRefreshDirectory')
    case 'cdn.sync_resources':
      return t('overview.actions.cdnSyncResources')
    default:
      return action || '-'
  }
}

export function OverviewPage() {
  const { t } = useTranslation()
  const [range, setRange] = useState<TimeRange>('24h')
  const platformRole = useAuthStore((state) => state.user?.platformRole)
  const isPlatformAdmin = isPlatformAdminRole(platformRole)
  const [metrics, setMetrics] = useState<OverviewCoreMetrics>(emptyMetrics)
  const [uploadTrend, setUploadTrend] = useState<OverviewTrendPoint[]>([])
  const [cdnTrend, setCDNTrend] = useState<OverviewTrendPoint[]>([])
  const [providerRatio, setProviderRatio] = useState<OverviewRatioPoint[]>([])
  const [operationRatio, setOperationRatio] = useState<OverviewRatioPoint[]>([])
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
      const overviewResponse = await apiClient.get<ApiResponse<OverviewMetricsPayload>>('/overview/metrics', {
        params: { timeWindow: range },
      })
      const overviewData = overviewResponse.data.data
      const serverMetrics = overviewData?.kpis ?? emptyMetrics
      setMetrics({
        projectCount: Number(serverMetrics.projectCount ?? 0),
        bucketCount: Number(serverMetrics.bucketCount ?? 0),
        cdnCount: Number(serverMetrics.cdnCount ?? 0),
        uploadSessionTotal: Number(serverMetrics.uploadSessionTotal ?? 0),
        cdnOperationTotal: Number(serverMetrics.cdnOperationTotal ?? 0),
        failureTotal: Number(serverMetrics.failureTotal ?? 0),
        uploadAvgDurationMs: Number(serverMetrics.uploadAvgDurationMs ?? 0),
      })
      setUploadTrend(Array.isArray(overviewData?.trends?.uploadSessions) ? overviewData.trends.uploadSessions : [])
      setCDNTrend(Array.isArray(overviewData?.trends?.cdnOperations) ? overviewData.trends.cdnOperations : [])
      setProviderRatio(
        Array.isArray(overviewData?.ratios?.providerResourceShare)
          ? overviewData.ratios.providerResourceShare.map((item) => ({
            ...item,
            name: formatProviderLabel(item.name, t),
          }))
          : [],
      )
      setOperationRatio(
        Array.isArray(overviewData?.ratios?.operationTypeShare)
          ? overviewData.ratios.operationTypeShare.map((item) => ({
            ...item,
            name:
              item.name === 'other'
                ? t('overview.actions.other')
                : formatActionLabel(item.name, t),
          }))
          : [],
      )

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

      if (requestSequenceRef.current !== requestID) {
        return
      }

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
              ? `${t('overview.cards.requestId')}: ${item.log.requestId.trim()}`
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
              ? `${t('overview.cards.requestId')}: ${item.log.requestId.trim()}`
              : formatActionLabel(item.log.action, t),
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
            action: formatActionLabel(log.action?.trim() || '-', t),
            result: formatResultLabel(log.result?.trim() || '-', t),
            rawResult: log.result?.trim() || '-',
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
        .map(({ key, actor, project, action, result, rawResult, time, requestId, traceLink }) => ({
          key,
          actor,
          project,
          action,
          result,
          rawResult,
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
      setMetrics(emptyMetrics)
      setUploadTrend([])
      setCDNTrend([])
      setProviderRatio([])
      setOperationRatio([])
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
        : range === '7d'
          ? t('overview.subtitle7d')
          : t('overview.subtitle30d'),
    [range, t],
  )

  const trendGranularityLabel = useMemo(
    () => (range === '24h' ? t('overview.controls.byHour') : t('overview.controls.byDay')),
    [range, t],
  )

  const coreMetricCards = useMemo(
    () => {
      const totalOps = metrics.uploadSessionTotal + metrics.cdnOperationTotal
      const successRate = totalOps > 0 ? ((totalOps - metrics.failureTotal) / totalOps) * 100 : 0

      return [
      {
        key: 'projects-total',
        title: t('overview.cards.projectTotal'),
        value: metrics.projectCount,
        icon: <AuditOutlined />,
        tone: 'cyan' as MetricTone,
        footnote: t('overview.cards.scopeHint'),
      },
      {
        key: 'buckets-total',
        title: t('overview.cards.bucketTotal'),
        value: metrics.bucketCount,
        icon: <FolderOpenOutlined />,
        tone: 'blue' as MetricTone,
        footnote: t('overview.cards.scopeHint'),
      },
      {
        key: 'cdns-total',
        title: t('overview.cards.cdnBindingTotal'),
        value: metrics.cdnCount,
        icon: <DeploymentUnitOutlined />,
        tone: 'indigo' as MetricTone,
        footnote: t('overview.cards.scopeHint'),
      },
      {
        key: 'upload-total',
        title: t('overview.cards.uploadTotal'),
        value: metrics.uploadSessionTotal,
        icon: <UploadOutlined />,
        tone: 'green' as MetricTone,
        footnote: headerSubtitle,
      },
      {
        key: 'cdn-total',
        title: t('overview.cards.cdnTotal'),
        value: metrics.cdnOperationTotal,
        icon: <CloudServerOutlined />,
        tone: 'purple' as MetricTone,
        footnote: headerSubtitle,
      },
      {
        key: 'failure-total',
        title: t('overview.cards.failureTotal'),
        value: metrics.failureTotal,
        icon: <DeploymentUnitOutlined />,
        tone: 'red' as MetricTone,
        footnote: t('overview.cards.failureHint'),
      },
      {
        key: 'upload-avg-duration',
        title: t('overview.cards.uploadAvgDurationMs'),
        value: metrics.uploadAvgDurationMs,
        icon: <SyncOutlined />,
        tone: 'orange' as MetricTone,
        footnote: t('overview.cards.performanceHint'),
      },
      {
        key: 'success-rate',
        title: t('overview.cards.successRate'),
        value: formatPercent(successRate),
        icon: <CloudServerOutlined />,
        tone: 'teal' as MetricTone,
        footnote: t('overview.cards.qualityHint'),
      },
    ]
    },
    [headerSubtitle, metrics, t],
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
        render: (_value: string, record) =>
          record.rawResult === 'success' ? (
            <Tag color="green">{record.result}</Tag>
          ) : record.rawResult === 'failure' ? (
            <Tag color="red">{record.result}</Tag>
          ) : record.rawResult === 'denied' ? (
            <Tag color="orange">{record.result}</Tag>
          ) : (
            <Tag>{record.result}</Tag>
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
              {t('overview.mark')}
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
            <Tag color="blue">{headerSubtitle}</Tag>
            <Tag color="cyan">{t('overview.charts.operationRatio')}</Tag>
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
          <Row gutter={[14, 14]} className="overview-metrics-grid">
            {coreMetricCards.map((item) => (
              <Col key={item.key} xs={24} sm={12} lg={12} xl={6}>
                <div className={`overview-kpi-tile overview-kpi-${item.tone}`}>
                  <div className="overview-kpi-icon">{item.icon}</div>
                  <div className="overview-kpi-content">
                    <Typography.Text type="secondary">{item.title}</Typography.Text>
                    <Typography.Title level={3} style={{ margin: 0 }}>
                      {loading ? <Spin size="small" /> : item.value}
                    </Typography.Title>
                    <Typography.Text type="secondary" className="overview-kpi-footnote">
                      {item.footnote}
                    </Typography.Text>
                  </div>
                </div>
              </Col>
            ))}
          </Row>
        </Space>
      </Card>

      <Card className="overview-control-bar">
        <Row align="middle" justify="space-between" gutter={[12, 12]}>
          <Col>
            <Space wrap>
              <Typography.Text strong>{t('overview.controls.timeRange')}</Typography.Text>
              <Segmented<TimeRange>
                value={range}
                options={[
                  { label: '24h', value: '24h' },
                  { label: '7d', value: '7d' },
                  { label: '30d', value: '30d' },
                ]}
                onChange={(value) => setRange(value)}
              />
              <Button icon={<ReloadOutlined />} onClick={() => void loadCoreMetrics()} loading={loading}>
                {t('overview.refresh')}
              </Button>
            </Space>
          </Col>
          <Col>
            <Space>
              <Typography.Text type="secondary">{t('overview.controls.granularity')}</Typography.Text>
              <Tag color="processing">{trendGranularityLabel}</Tag>
            </Space>
          </Col>
        </Row>
      </Card>

      <Row gutter={[16, 16]} className="overview-chart-grid">
        <Col xs={24} lg={12}>
          <Card title={t('overview.charts.uploadTrend')} className="overview-chart-card">
            <TrendLine
              data={uploadTrend}
              emptyText={t('overview.charts.empty')}
              successLabel={t('overview.status.success')}
              failedLabel={t('overview.status.failure')}
            />
          </Card>
        </Col>
        <Col xs={24} lg={12}>
          <Card title={t('overview.charts.cdnTrend')} className="overview-chart-card">
            <TrendLine
              data={cdnTrend}
              emptyText={t('overview.charts.empty')}
              successLabel={t('overview.status.success')}
              failedLabel={t('overview.status.failure')}
            />
          </Card>
        </Col>
      </Row>

      <Row gutter={[16, 16]} className="overview-chart-grid">
        <Col xs={24} lg={12}>
          <Card title={t('overview.charts.providerRatio')} className="overview-chart-card">
            <RatioPie data={providerRatio} emptyText={t('overview.charts.empty')} />
          </Card>
        </Col>
        <Col xs={24} lg={12}>
          <Card title={t('overview.charts.operationRatio')} className="overview-chart-card">
            <RatioPie data={operationRatio} emptyText={t('overview.charts.empty')} />
          </Card>
        </Col>
      </Row>

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
