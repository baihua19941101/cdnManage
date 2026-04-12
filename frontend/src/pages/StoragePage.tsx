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
  Progress,
  Select,
  Space,
  Table,
  Tag,
  Tooltip,
  Typography,
  Upload,
  message,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import type { TableRowSelection } from 'antd/es/table/interface'
import type { UploadFile } from 'antd/es/upload/interface'
import axios from 'axios'
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useSearchParams } from 'react-router-dom'

import i18n from '../i18n'
import { apiClient } from '../services/api/client'
import { resolveAPIErrorMessage } from '../services/api/error'
import {
  listStorageAuditLogs,
  type StorageAuditLog,
} from '../services/storage/uploadSessions'
import { isPlatformAdminRole, useAuthStore } from '../store/auth'

type ObjectItem = {
  key: string
  etag?: string
  contentType?: string
  size: number
  lastModified?: string
  isDir: boolean
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
  currentProjectRole?: 'project_admin' | 'project_read_only'
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

type UploadFailureReasonItem = {
  fileName: string
  reason: string
}

type BatchDeleteItemResult = {
  key: string
  targetType?: string
  result: string
  deletedObjects?: number
  failedObjects?: number
  errorCode?: string
  reason?: string
}

type BatchDeletePayload = {
  message?: string
  summary?: {
    total?: number
    success?: number
    failure?: number
  }
  results?: BatchDeleteItemResult[]
}

type DeleteObjectSummary = {
  key?: string
  targetType?: string
  result?: string
  deletedObjects?: number
  failedObjects?: number
  errorCode?: string
  reason?: string
}

type DeleteObjectPayload = {
  message?: string
  summary?: DeleteObjectSummary
}

type RenameObjectSummary = {
  sourceKey?: string
  targetKey?: string
  targetType?: string
  result?: string
  migratedObjects?: number
  failedObjects?: number
  errorCode?: string
  failureReasons?: string[]
}

type RenameObjectPayload = {
  message?: string
  summary?: RenameObjectSummary
}

type UploadSessionSummary = {
  sessionId: string
  startedAt: string
  finishedAt: string
  durationMs: number
  totalEntries: number
  successEntries: number
  failedEntries: number
  processedEntries: number
  progressPercent: number
  result: string
  createdAt: string
}

const DEFAULT_UPLOAD_SIZE_LIMIT_BYTES = 20 * 1024 * 1024
const UPLOAD_STAGE_A_TIMEOUT_MS = 10 * 60 * 1000
const DELETE_REQUEST_TIMEOUT_MS = 5 * 60 * 1000
const UPLOAD_STAGE_B_POLL_INTERVAL_MS = 1200
const UPLOAD_STAGE_B_POLL_TIMEOUT_MS = 2 * 60 * 1000
const OBJECT_PAGE_SIZE_OPTIONS = [10, 20, 50, 100] as const
const MIN_OBJECT_FETCH_LIMIT = 200
const OBJECT_FETCH_LIMIT_MULTIPLIER = 10
const trGlobal = (zh: string, en: string) => (i18n.language === 'en-US' ? en : zh)

const normalizeDirectoryPrefix = (value: string): string => {
  const trimmed = value.trim().replace(/^\/+/, '')
  if (!trimmed) {
    return ''
  }
  return trimmed.endsWith('/') ? trimmed : `${trimmed}/`
}

const parentDirectoryPrefix = (prefix: string): string => {
  const normalized = normalizeDirectoryPrefix(prefix)
  if (!normalized) {
    return ''
  }
  const withoutTrailingSlash = normalized.slice(0, -1)
  const lastSlashIndex = withoutTrailingSlash.lastIndexOf('/')
  if (lastSlashIndex < 0) {
    return ''
  }
  return withoutTrailingSlash.slice(0, lastSlashIndex + 1)
}

const remapCurrentPrefixAfterDirectoryRename = (
  currentPrefix: string,
  sourceKey: string,
  targetKey: string,
): string => {
  const normalizedCurrent = normalizeDirectoryPrefix(currentPrefix)
  const normalizedSource = normalizeDirectoryPrefix(sourceKey)
  const normalizedTarget = normalizeDirectoryPrefix(targetKey)
  if (!normalizedCurrent || !normalizedSource || !normalizedTarget) {
    return normalizedCurrent
  }
  if (normalizedCurrent === normalizedSource) {
    return normalizedTarget
  }
  if (normalizedCurrent.startsWith(normalizedSource)) {
    return `${normalizedTarget}${normalizedCurrent.slice(normalizedSource.length)}`
  }
  return normalizedCurrent
}

const objectDisplayName = (key: string, currentPrefix: string, isDir: boolean): string => {
  const normalizedPrefix = currentPrefix.trim()
  let relative = key
  if (normalizedPrefix && key.startsWith(normalizedPrefix)) {
    relative = key.slice(normalizedPrefix.length)
  }

  relative = relative.replace(/^\/+/, '')
  if (isDir) {
    relative = relative.replace(/\/+$/, '')
  }

  if (!relative) {
    const fallback = key.replace(/\/+$/, '')
    const segments = fallback.split('/').filter(Boolean)
    return segments[segments.length - 1] ?? key
  }

  const segments = relative.split('/').filter(Boolean)
  return segments[0] ?? relative
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

const normalizeString = (value: unknown): string =>
  typeof value === 'string' ? value.trim() : ''

const parseFailureReasonItem = (item: unknown): UploadFailureReasonItem | null => {
  if (!isRecord(item)) {
    return null
  }

  const success = item.success
  if (typeof success === 'boolean' && success) {
    return null
  }

  const status = normalizeString(item.status) || normalizeString(item.result)
  const isFailureStatus =
    status === 'failure' ||
    status === 'failed' ||
    status === 'error' ||
    status === 'partial_failure' ||
    status === 'partial-failure'
  if (!isFailureStatus && status === 'success') {
    return null
  }

  const reasonCandidates: unknown[] = [
    item.reason,
    item.errorMessage,
    item.message,
    item.error,
    item.detail,
  ]

  let reason = ''
  for (const candidate of reasonCandidates) {
    if (typeof candidate === 'string' && candidate.trim()) {
      reason = candidate.trim()
      break
    }
  }

  if (!reason) {
    const code = normalizeString(item.errorCode)
    reason = code || 'unknown error'
  } else {
    const code = normalizeString(item.errorCode)
    if (code) {
      reason = `${reason} (${code})`
    }
  }

  const fileName = normalizeString(item.fileName) || normalizeString(item.key)
  return { fileName, reason }
}

const parseFailureReasonSummary = (data: unknown, limit = 3): string => {
  if (!isRecord(data)) {
    return ''
  }

  const resultListCandidates: unknown[] = [data.results, data.files, data.items, data.uploads]
  const failures: UploadFailureReasonItem[] = []

  for (const candidate of resultListCandidates) {
    if (!Array.isArray(candidate)) {
      continue
    }

    for (const item of candidate) {
      const parsed = parseFailureReasonItem(item)
      if (parsed) {
        failures.push(parsed)
      }
    }

    if (failures.length > 0) {
      break
    }
  }

  if (failures.length === 0) {
    return ''
  }

  return failures
    .slice(0, Math.max(1, limit))
    .map((item, index) => {
      if (item.fileName) {
        return `${index + 1}. ${item.fileName}: ${item.reason}`
      }
      return `${index + 1}. ${item.reason}`
    })
    .join('；')
}

const parseFailureReasonSummaryFromError = (error: unknown, limit = 3): string => {
  if (!isRecord(error)) {
    return ''
  }

  const response = error.response
  if (!isRecord(response)) {
    return ''
  }

  const payload = response.data
  if (!isRecord(payload)) {
    return ''
  }

  const nestedData = payload.data
  const fromNested = parseFailureReasonSummary(nestedData, limit)
  if (fromNested) {
    return fromNested
  }
  return parseFailureReasonSummary(payload, limit)
}

const parseSessionIdFromAuditLog = (log: StorageAuditLog): string => {
  if (isRecord(log.metadata)) {
    const metadataSessionID = normalizeString(log.metadata.sessionId)
    if (metadataSessionID) {
      return metadataSessionID
    }
  }
  return normalizeString(log.targetIdentifier)
}

const toMetadataNonNegativeNumber = (metadata: Record<string, unknown>, field: string): number =>
  toNonNegativeNumber(metadata[field]) ?? 0

const parseArchiveSessionSummary = (log: StorageAuditLog): UploadSessionSummary | null => {
  const sessionID = parseSessionIdFromAuditLog(log)
  if (!sessionID) {
    return null
  }

  const metadata = isRecord(log.metadata) ? log.metadata : {}
  const startedAt = normalizeString(metadata.startedAt) || normalizeString(log.createdAt)
  const finishedAt = normalizeString(metadata.finishedAt)
  const durationMs = toMetadataNonNegativeNumber(metadata, 'durationMs')
  const successEntriesRaw = toMetadataNonNegativeNumber(metadata, 'successEntries')
  const failedEntriesRaw = toMetadataNonNegativeNumber(metadata, 'failedEntries')
  const totalEntriesRaw = toMetadataNonNegativeNumber(metadata, 'totalEntries')

  const successEntries =
    successEntriesRaw > 0 ? successEntriesRaw : toMetadataNonNegativeNumber(metadata, 'uploaded')
  const failedEntries =
    failedEntriesRaw > 0 ? failedEntriesRaw : toMetadataNonNegativeNumber(metadata, 'failed')
  const processedEntries = successEntries + failedEntries
  const totalEntries = totalEntriesRaw > 0 ? totalEntriesRaw : processedEntries
  const progressPercent =
    totalEntries > 0 ? Math.min(100, Math.round((processedEntries / totalEntries) * 100)) : 0

  return {
    sessionId: sessionID,
    startedAt,
    finishedAt,
    durationMs,
    totalEntries,
    successEntries,
    failedEntries,
    processedEntries,
    progressPercent,
    result: normalizeString(log.result),
    createdAt: normalizeString(log.createdAt),
  }
}

const parseFailureReasonFromAuditLog = (log: StorageAuditLog): UploadFailureReasonItem | null => {
  if (normalizeString(log.result) === 'success') {
    return null
  }

  const metadata = isRecord(log.metadata) ? log.metadata : null
  const reasonCandidates: unknown[] = metadata
    ? [metadata.reason, metadata.error, metadata.message, metadata.detail]
    : []

  let reason = ''
  for (const candidate of reasonCandidates) {
    const text = normalizeString(candidate)
    if (text) {
      reason = text
      break
    }
  }

  if (!reason) {
    reason = 'unknown error'
  }

  const fileName = metadata
    ? normalizeString(metadata.fileName) ||
      normalizeString(metadata.archiveEntry) ||
      normalizeString(log.targetIdentifier)
    : normalizeString(log.targetIdentifier)

  return { fileName, reason }
}

const buildFailureSummaryFromAuditLogs = (logs: StorageAuditLog[], limit = 3): string => {
  const failures = logs
    .map((log) => parseFailureReasonFromAuditLog(log))
    .filter((item): item is UploadFailureReasonItem => item !== null)

  if (failures.length === 0) {
    return ''
  }

  return failures
    .slice(0, Math.max(1, limit))
    .map((item, index) => {
      if (item.fileName) {
        return `${index + 1}. ${item.fileName}: ${item.reason}`
      }
      return `${index + 1}. ${item.reason}`
    })
    .join('；')
}

const formatDateTimeText = (value: string): string => {
  const normalized = normalizeString(value)
  if (!normalized) {
    return '-'
  }
  const date = new Date(normalized)
  if (Number.isNaN(date.getTime())) {
    return normalized
  }
  return date.toLocaleString('zh-CN', { hour12: false })
}

const formatDurationText = (durationMs: number): string => {
  const ms = Math.max(0, Math.floor(durationMs))
  if (ms < 1000) {
    return `${ms}ms`
  }
  const totalSeconds = Math.floor(ms / 1000)
  const milliseconds = ms % 1000
  const seconds = totalSeconds % 60
  const minutes = Math.floor(totalSeconds / 60) % 60
  const hours = Math.floor(totalSeconds / 3600)

  const segments: string[] = []
  if (hours > 0) {
    segments.push(`${hours}h`)
  }
  if (minutes > 0 || hours > 0) {
    segments.push(`${minutes}m`)
  }
  segments.push(`${seconds}s`)
  if (milliseconds > 0) {
    segments.push(`${milliseconds}ms`)
  }
  return segments.join(' ')
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
  trGlobal(
    `解压与上传摘要：压缩包 ${summary.archivesProcessed}，解压 ${summary.extracted}，上传 ${summary.uploaded}，跳过 ${summary.skipped}，失败 ${summary.failed}`,
    `Extract and upload summary: archives ${summary.archivesProcessed}, extracted ${summary.extracted}, uploaded ${summary.uploaded}, skipped ${summary.skipped}, failed ${summary.failed}`,
  )

const parseUploadSessionId = (data: unknown): string => {
  if (!isRecord(data)) {
    return ''
  }
  return normalizeString(data.sessionId)
}

const isSessionSummaryTerminal = (summary: UploadSessionSummary): boolean => {
  const result = normalizeString(summary.result)
  if (result === 'success' || result === 'failure') {
    return true
  }
  if (normalizeString(summary.finishedAt)) {
    return true
  }
  return summary.totalEntries > 0 && summary.processedEntries >= summary.totalEntries
}

const formatUploadLimitText = (bytes: number): string => {
  const mb = bytes / (1024 * 1024)
  const mbText = Number.isInteger(mb) ? String(mb) : mb.toFixed(2).replace(/\.?0+$/, '')
  const bytesText = new Intl.NumberFormat('en-US').format(bytes)
  return `${mbText} MB (${bytesText} bytes)`
}

const toPositiveInteger = (value: unknown): number | null => {
  const numberValue = toNonNegativeNumber(value)
  if (numberValue === null || numberValue <= 0) {
    return null
  }
  return Math.floor(numberValue)
}

const parseUploadPolicyLimitBytes = (data: unknown): number | null => {
  const directBytes = toPositiveInteger(data)
  if (directBytes !== null) {
    return directBytes
  }

  if (!isRecord(data)) {
    return null
  }

  const byteCandidates: unknown[] = [
    data.maxUploadSizeBytes,
    data.maxSizeBytes,
    data.uploadLimitBytes,
    data.limitBytes,
  ]

  for (const candidate of byteCandidates) {
    const parsed = toPositiveInteger(candidate)
    if (parsed !== null) {
      return parsed
    }
  }

  const mbCandidates: unknown[] = [
    data.maxUploadSizeMB,
    data.maxSizeMB,
    data.uploadLimitMB,
    data.limitMB,
  ]

  for (const candidate of mbCandidates) {
    const parsedMB = toPositiveInteger(candidate)
    if (parsedMB !== null) {
      return parsedMB * 1024 * 1024
    }
  }

  const nestedCandidates: unknown[] = [data.policy, data.uploadPolicy, data.limit]
  for (const candidate of nestedCandidates) {
    const nested = parseUploadPolicyLimitBytes(candidate)
    if (nested !== null) {
      return nested
    }
  }

  return null
}

const isTimeoutError = (error: unknown): boolean => {
  if (!axios.isAxiosError(error)) {
    return false
  }
  if (error.code === 'ECONNABORTED') {
    return true
  }
  const message = normalizeString(error.message).toLowerCase()
  return message.includes('timeout')
}

const isCanceledError = (error: unknown): boolean =>
  axios.isAxiosError(error) && error.code === 'ERR_CANCELED'

const fetchUploadPolicyLimitBytes = async (): Promise<number> => {
  const response = await apiClient.get<ApiResponse<unknown>>('/storage/upload-policy')
  return parseUploadPolicyLimitBytes(response.data?.data) ?? DEFAULT_UPLOAD_SIZE_LIMIT_BYTES
}

export function StoragePage() {
  const { t, i18n } = useTranslation()
  const tr = (zh: string, en: string) => (i18n.language === 'en-US' ? en : zh)
  const [searchParams] = useSearchParams()
  const [messageApi, messageContext] = message.useMessage()
  const [queryForm] = Form.useForm<QueryFormValues>()
  const [renameForm] = Form.useForm<RenameFormValues>()
  const platformRole = useAuthStore((state) => state.user?.platformRole)
  const isPlatformAdmin = isPlatformAdminRole(platformRole)
  const [currentProjectRole, setCurrentProjectRole] = useState<'project_admin' | 'project_read_only' | ''>('')
  const canWrite = isPlatformAdmin || currentProjectRole === 'project_admin'

  const [objects, setObjects] = useState<ObjectItem[]>([])
  const [currentPrefix, setCurrentPrefix] = useState('')
  const [objectPageSize, setObjectPageSize] = useState<number>(OBJECT_PAGE_SIZE_OPTIONS[0])
  const [objectCurrentPage, setObjectCurrentPage] = useState(1)
  const [loading, setLoading] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [queryError, setQueryError] = useState<string | null>(null)
  const [selectedObjectKeys, setSelectedObjectKeys] = useState<React.Key[]>([])

  const [pendingUploadFiles, setPendingUploadFiles] = useState<UploadFile[]>([])
  const [uploadKey, setUploadKey] = useState('')
  const [uploadSizeLimitBytes, setUploadSizeLimitBytes] = useState(DEFAULT_UPLOAD_SIZE_LIMIT_BYTES)
  const [uploadStageAProgress, setUploadStageAProgress] = useState(0)
  const [uploadStageAActive, setUploadStageAActive] = useState(false)
  const uploadAbortControllerRef = useRef<AbortController | null>(null)
  const deleteAbortControllerRef = useRef<AbortController | null>(null)
  const [deleting, setDeleting] = useState(false)
  const [deletingScopeLabel, setDeletingScopeLabel] = useState('')
  const [uploadStageBActive, setUploadStageBActive] = useState(false)
  const [uploadStageBSessionID, setUploadStageBSessionID] = useState('')
  const [uploadStageBSummary, setUploadStageBSummary] = useState<UploadSessionSummary | null>(null)
  const [uploadStageBFailureSummary, setUploadStageBFailureSummary] = useState('')

  const [renameVisible, setRenameVisible] = useState(false)
  const [renamingSourceKey, setRenamingSourceKey] = useState('')
  const [renamingTargetType, setRenamingTargetType] = useState<'file' | 'directory'>('file')

  const [auditVisible, setAuditVisible] = useState(false)
  const [auditLogs, setAuditLogs] = useState<StorageAuditLog[]>([])
  const [auditLoading, setAuditLoading] = useState(false)
  const [sessionSummaries, setSessionSummaries] = useState<UploadSessionSummary[]>([])
  const [sessionSummariesLoading, setSessionSummariesLoading] = useState(false)
  const [sessionFailureSummaryMap, setSessionFailureSummaryMap] = useState<Record<string, string>>(
    {},
  )
  const [sessionDetailVisible, setSessionDetailVisible] = useState(false)
  const [sessionDetailLoading, setSessionDetailLoading] = useState(false)
  const [sessionDetailLogs, setSessionDetailLogs] = useState<StorageAuditLog[]>([])
  const [sessionDetailCurrentPage, setSessionDetailCurrentPage] = useState(1)
  const [sessionDetailPageSize, setSessionDetailPageSize] = useState(12)
  const [activeSession, setActiveSession] = useState<UploadSessionSummary | null>(null)
  const [projectOptions, setProjectOptions] = useState<ProjectOption[]>([])
  const [projectOptionsLoading, setProjectOptionsLoading] = useState(false)
  const [bucketOptions, setBucketOptions] = useState<string[]>([])
  const [bucketOptionsLoading, setBucketOptionsLoading] = useState(false)
  const queryProjectInitializedRef = useRef(false)

  useEffect(() => {
    let cancelled = false

    const loadUploadPolicy = async () => {
      try {
        const limit = await fetchUploadPolicyLimitBytes()
        if (!cancelled) {
          setUploadSizeLimitBytes(limit)
        }
      } catch (error) {
        if (!cancelled) {
          setUploadSizeLimitBytes(DEFAULT_UPLOAD_SIZE_LIMIT_BYTES)
          messageApi.warning(
            `${resolveAPIErrorMessage(error, '上传策略读取失败。')} 已回退默认限制 ${formatUploadLimitText(DEFAULT_UPLOAD_SIZE_LIMIT_BYTES)}。`,
          )
        }
      }
    }

    void loadUploadPolicy()
    return () => {
      cancelled = true
    }
  }, [messageApi])

  useEffect(
    () => () => {
      uploadAbortControllerRef.current?.abort()
      uploadAbortControllerRef.current = null
      deleteAbortControllerRef.current?.abort()
      deleteAbortControllerRef.current = null
    },
    [],
  )

  const startDeleteRequest = (scopeLabel: string): AbortController => {
    deleteAbortControllerRef.current?.abort()
    const controller = new AbortController()
    deleteAbortControllerRef.current = controller
    setDeleting(true)
    setDeletingScopeLabel(scopeLabel)
    return controller
  }

  const finishDeleteRequest = (controller: AbortController) => {
    if (deleteAbortControllerRef.current !== controller) {
      return
    }
    deleteAbortControllerRef.current = null
    setDeleting(false)
    setDeletingScopeLabel('')
  }

  const cancelDelete = () => {
    deleteAbortControllerRef.current?.abort()
  }

  const notifyDeleteRequestError = (error: unknown, scopeLabel: string) => {
    if (isCanceledError(error)) {
      messageApi.warning(tr(`${scopeLabel}已取消。`, `${scopeLabel} canceled.`))
      return
    }
    if (isTimeoutError(error)) {
      messageApi.error(tr(`${scopeLabel}请求超时，请重试或缩小删除范围。`, `${scopeLabel} timed out. Please retry or reduce delete scope.`))
      return
    }
    messageApi.error(resolveAPIErrorMessage(error, tr(`${scopeLabel}失败（后端失败）。`, `${scopeLabel} failed (backend failure).`)))
  }

  const getQuery = () => {
    const values = queryForm.getFieldsValue()
    const projectID = Number(values.projectId)
    return {
      projectID,
      bucketName: values.bucketName?.trim(),
      prefix: values.prefix?.trim() || '',
    }
  }

  const objectFetchLimit = (pageSize: number): number =>
    Math.max(MIN_OBJECT_FETCH_LIMIT, pageSize * OBJECT_FETCH_LIMIT_MULTIPLIER)

  const queryObjects = async (overridePageSize?: number, prefixOverride?: string) => {
    const values = await queryForm.validateFields()
    const projectID = Number(values.projectId)
    if (!Number.isFinite(projectID) || projectID <= 0) {
      messageApi.error(tr('Project ID 必须是正整数。', 'Project ID must be a positive integer.'))
      return
    }
    const requestPrefix =
      typeof prefixOverride === 'string' ? prefixOverride.trim() : values.prefix?.trim() || ''

    setLoading(true)
    setQueryError(null)
    setSelectedObjectKeys([])
    try {
      const response = await apiClient.get<ApiResponse<{ objects: ObjectItem[] }>>(
        `/projects/${projectID}/storage/objects`,
        {
          params: {
            bucketName: values.bucketName.trim(),
            prefix: requestPrefix || undefined,
            maxKeys: objectFetchLimit(overridePageSize ?? objectPageSize),
          },
        },
      )
      setObjects(response.data.data?.objects ?? [])
      setCurrentPrefix(requestPrefix)
      setObjectCurrentPage(1)
    } catch (error) {
      setQueryError(resolveAPIErrorMessage(error, tr('对象列表加载失败。', 'Failed to load object list.')))
      setObjects([])
    } finally {
      setLoading(false)
    }
  }

  const loadProjectOptions = async () => {
    setProjectOptionsLoading(true)
    try {
      const response = await apiClient.get<ApiResponse<ProjectOption[]>>('/projects/accessible')
      const items = Array.isArray(response.data.data) ? response.data.data : []
      setProjectOptions(items)
      return items
    } catch (error) {
      messageApi.error(resolveAPIErrorMessage(error, tr('项目列表加载失败。', 'Failed to load project list.')))
      setProjectOptions([])
      return [] as ProjectOption[]
    } finally {
      setProjectOptionsLoading(false)
    }
  }

  const loadBucketsByProject = async (projectID: number) => {
    if (!Number.isFinite(projectID) || projectID <= 0) {
      setCurrentProjectRole('')
      setBucketOptions([])
      queryForm.setFieldValue('bucketName', '')
      queryForm.setFieldValue('prefix', '')
      setCurrentPrefix('')
      return
    }

    setCurrentProjectRole('')
    setBucketOptionsLoading(true)
    try {
      const response = await apiClient.get<ApiResponse<ProjectDetail>>(`/projects/${projectID}/context`)
      const project = response.data.data
      const role = project?.currentProjectRole?.trim()
      setCurrentProjectRole(role === 'project_admin' || role === 'project_read_only' ? role : '')
      const names =
        Array.isArray(project?.buckets)
          ? project.buckets
              .map((bucket) => bucket.bucketName?.trim())
              .filter((name): name is string => Boolean(name))
          : []
      setBucketOptions(names)
      queryForm.setFieldValue('bucketName', names[0] ?? '')
      queryForm.setFieldValue('prefix', '')
      setCurrentPrefix('')
    } catch (error) {
      messageApi.error(resolveAPIErrorMessage(error, tr('项目存储桶加载失败。', 'Failed to load project buckets.')))
      setCurrentProjectRole('')
      setBucketOptions([])
      queryForm.setFieldValue('bucketName', '')
      queryForm.setFieldValue('prefix', '')
      setCurrentPrefix('')
    } finally {
      setBucketOptionsLoading(false)
    }
  }

  useEffect(() => {
    const bootstrapProjectFromQuery = async () => {
      if (queryProjectInitializedRef.current) {
        return
      }
      queryProjectInitializedRef.current = true

      const projectIDFromQuery = Number(searchParams.get('projectId'))
      if (!Number.isFinite(projectIDFromQuery) || projectIDFromQuery <= 0) {
        return
      }

      const items = await loadProjectOptions()
      if (!items.some((project) => project.id === projectIDFromQuery)) {
        return
      }

      queryForm.setFieldValue('projectId', String(projectIDFromQuery))
      await loadBucketsByProject(projectIDFromQuery)
      await loadUploadSessionSummaries(projectIDFromQuery)
      queryForm.setFieldValue('prefix', '')
      setCurrentPrefix('')
      setObjects([])
      setSelectedObjectKeys([])
      setActiveSession(null)
      setSessionDetailLogs([])
      setUploadStageBSessionID('')
      setUploadStageBSummary(null)
      setUploadStageBFailureSummary('')
      setUploadStageBActive(false)
    }

    void bootstrapProjectFromQuery()
  }, [queryForm, searchParams])

  const enterDirectory = async (directoryKey: string) => {
    const nextPrefix = normalizeDirectoryPrefix(directoryKey)
    queryForm.setFieldValue('prefix', nextPrefix)
    await queryObjects(undefined, nextPrefix)
  }

  const goToParentDirectory = async () => {
    const parentPrefix = parentDirectoryPrefix(currentPrefix)
    queryForm.setFieldValue('prefix', parentPrefix)
    await queryObjects(undefined, parentPrefix)
  }

  const uploadObject = async () => {
    if (!canWrite) {
      return
    }

    let currentLimitBytes = uploadSizeLimitBytes
    try {
      currentLimitBytes = await fetchUploadPolicyLimitBytes()
      setUploadSizeLimitBytes(currentLimitBytes)
    } catch {
      currentLimitBytes = DEFAULT_UPLOAD_SIZE_LIMIT_BYTES
      setUploadSizeLimitBytes(DEFAULT_UPLOAD_SIZE_LIMIT_BYTES)
    }

    const { projectID, bucketName } = getQuery()
    if (!projectID || !bucketName) {
      messageApi.error(tr('请先填写并查询 Project ID 与 BucketName。', 'Please set and query Project ID and BucketName first.'))
      return
    }

    const files = pendingUploadFiles.flatMap((item) =>
      item.originFileObj ? [item.originFileObj] : [],
    )
    if (files.length === 0) {
      messageApi.error(tr('请选择待上传文件。', 'Please select files to upload.'))
      return
    }

    const oversizedFiles = files.filter((file) => file.size > currentLimitBytes)
    if (oversizedFiles.length > 0) {
      const limitText = formatUploadLimitText(currentLimitBytes)
      oversizedFiles.forEach((file) => {
        messageApi.error(tr(`文件 ${file.name} 超过当前大小限制（${limitText}），已拒绝上传。`, `File ${file.name} exceeds size limit (${limitText}) and was rejected.`))
      })
      setPendingUploadFiles((prev) =>
        prev.filter((item) => {
          const originFile = item.originFileObj
          return originFile ? originFile.size <= currentLimitBytes : false
        }),
      )
    }

    const uploadableFiles = files.filter((file) => file.size <= currentLimitBytes)
    if (uploadableFiles.length === 0) {
      messageApi.error(tr('当前待上传文件均超过大小限制，请重新选择。', 'All selected files exceed size limit. Please select again.'))
      return
    }

    const formData = new FormData()
    formData.append('bucketName', bucketName)

    const trimmedKey = uploadKey.trim()
    if (uploadableFiles.length === 1) {
      const singleFile = uploadableFiles[0]
      if (!singleFile) {
        messageApi.error(tr('请选择待上传文件。', 'Please select files to upload.'))
        return
      }
      if (trimmedKey) {
        formData.append('key', trimmedKey)
      }
      formData.append('file', singleFile)
    } else {
      uploadableFiles.forEach((file) => formData.append('files', file))
      if (trimmedKey) {
        formData.append('keyPrefix', trimmedKey)
      }
    }

    const abortController = new AbortController()
    uploadAbortControllerRef.current = abortController
    setUploadStageAProgress(0)
    setUploadStageAActive(true)
    setUploadStageBSessionID('')
    setUploadStageBSummary(null)
    setUploadStageBFailureSummary('')
    setUploadStageBActive(false)
    setSubmitting(true)
    try {
      const response = await apiClient.post<ApiResponse<unknown>>(
        `/projects/${projectID}/storage/upload`,
        formData,
        {
          headers: { 'Content-Type': 'multipart/form-data' },
          timeout: UPLOAD_STAGE_A_TIMEOUT_MS,
          signal: abortController.signal,
          onUploadProgress: (event) => {
            const total = typeof event.total === 'number' && event.total > 0 ? event.total : undefined
            const loaded = typeof event.loaded === 'number' && event.loaded >= 0 ? event.loaded : 0
            if (!total) {
              return
            }
            const percent = Math.max(0, Math.min(100, Math.round((loaded / total) * 100)))
            setUploadStageAProgress(percent)
          },
        },
      )
      setUploadStageAProgress(100)
      const summary = parseUploadSummary(response.data?.data, uploadableFiles.length)
      const archiveSummary = parseArchiveSummary(response.data?.data)
      const failureReasonSummary = parseFailureReasonSummary(response.data?.data)
      const sessionID = parseUploadSessionId(response.data?.data)
      const archiveSummaryText = archiveSummary ? `；${formatArchiveSummary(archiveSummary)}` : ''
      let finalSuccessCount = summary.successCount
      let finalFailureCount = summary.failureCount
      let mergedFailureSummary = failureReasonSummary

      if (sessionID) {
        const finalSummary = await pollUploadSessionStageB(projectID, sessionID)
        if (finalSummary) {
          finalSuccessCount = finalSummary.successEntries
          finalFailureCount = finalSummary.failedEntries
        }
        if (finalFailureCount > 0) {
          try {
            const sessionFailureSummary = await loadFailureSummaryForSession(projectID, sessionID)
            if (sessionFailureSummary) {
              mergedFailureSummary = sessionFailureSummary
              setUploadStageBFailureSummary(sessionFailureSummary)
            }
          } catch {
            // ignore session failure summary fetch errors on upload success path
          }
        }
      }

      const mergedFailureReasonText = mergedFailureSummary
        ? tr(` 失败原因摘要：${mergedFailureSummary}。`, ` Failure summary: ${mergedFailureSummary}.`)
        : ''
      if (finalFailureCount > 0) {
        messageApi.warning(tr(`上传完成：成功 ${finalSuccessCount}，失败 ${finalFailureCount}${archiveSummaryText}。${mergedFailureReasonText}`, `Upload finished: success ${finalSuccessCount}, failure ${finalFailureCount}${archiveSummaryText}.${mergedFailureReasonText}`))
      } else {
        messageApi.success(tr(`上传完成：成功 ${finalSuccessCount}，失败 0${archiveSummaryText}。`, `Upload finished: success ${finalSuccessCount}, failure 0${archiveSummaryText}.`))
      }

      if (archiveSummary && finalSuccessCount > 0) {
        const currentPrefix = queryForm.getFieldValue('prefix')
        if (typeof currentPrefix === 'string' && currentPrefix.trim()) {
          queryForm.setFieldValue('prefix', '')
          messageApi.info(tr('已清空前缀以展示最新上传对象', 'Prefix cleared to show latest uploaded objects.'))
        }
      }

      setPendingUploadFiles([])
      setUploadKey('')
      await queryObjects()
      await loadUploadSessionSummaries(projectID)
    } catch (error) {
      if (axios.isAxiosError(error) && error.code === 'ERR_CANCELED') {
        messageApi.warning(tr('上传已取消。', 'Upload canceled.'))
        return
      }
      if (axios.isAxiosError(error) && error.code === 'ECONNABORTED') {
        messageApi.error(resolveAPIErrorMessage(error, tr('上传阶段 A 超时，请重试。', 'Upload stage A timed out, please retry.')))
        return
      }
      const failureReasonSummary = parseFailureReasonSummaryFromError(error)
      const baseErrorMessage = resolveAPIErrorMessage(error, tr('上传失败。', 'Upload failed.'))
      if (failureReasonSummary) {
        messageApi.error(tr(`${baseErrorMessage} 失败原因摘要：${failureReasonSummary}。`, `${baseErrorMessage} Failure summary: ${failureReasonSummary}.`))
      } else {
        messageApi.error(baseErrorMessage)
      }
    } finally {
      uploadAbortControllerRef.current = null
      setUploadStageAActive(false)
      setUploadStageAProgress(0)
      setSubmitting(false)
    }
  }

  const cancelUpload = () => {
    uploadAbortControllerRef.current?.abort()
  }

  const downloadObject = async (key: string) => {
    const { projectID, bucketName } = getQuery()
    if (!projectID || !bucketName) {
      messageApi.error(tr('请先填写并查询 Project ID 与 BucketName。', 'Please set and query Project ID and BucketName first.'))
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
      messageApi.error(resolveAPIErrorMessage(error, tr('下载失败。', 'Download failed.')))
    }
  }

  const deleteObject = async (key: string, isDir: boolean) => {
    if (!canWrite) {
      return
    }
    const { projectID, bucketName } = getQuery()
    if (!projectID || !bucketName) {
      messageApi.error(tr('请先填写并查询 Project ID 与 BucketName。', 'Please set and query Project ID and BucketName first.'))
      return
    }

    const scopeLabel = isDir ? tr('目录删除', 'Directory Delete') : tr('对象删除', 'Object Delete')
    const abortController = startDeleteRequest(scopeLabel)
    setSubmitting(true)
    try {
      const response = await apiClient.delete<ApiResponse<DeleteObjectPayload>>(`/projects/${projectID}/storage/objects`, {
        params: { bucketName, key },
        timeout: DELETE_REQUEST_TIMEOUT_MS,
        signal: abortController.signal,
      })
      const payload = response.data?.data
      const summary = payload?.summary
      const deletedObjects = toNonNegativeNumber(summary?.deletedObjects) ?? 0
      const failedObjects = toNonNegativeNumber(summary?.failedObjects) ?? 0
      if (isDir) {
        if (failedObjects > 0) {
          messageApi.warning(
            tr(`目录删除完成：成功删除 ${deletedObjects}，失败 ${failedObjects}${summary?.reason ? `。${summary.reason}` : ''}`, `Directory delete completed: deleted ${deletedObjects}, failed ${failedObjects}${summary?.reason ? `. ${summary.reason}` : ''}`),
          )
        } else {
          messageApi.success(tr(`目录删除完成：成功删除 ${deletedObjects} 个对象。`, `Directory delete completed: deleted ${deletedObjects} objects.`))
        }
      } else {
        messageApi.success(tr('删除成功。', 'Delete succeeded.'))
      }

      if (isDir && normalizeDirectoryPrefix(key) === normalizeDirectoryPrefix(currentPrefix)) {
        await goToParentDirectory()
      } else {
        await queryObjects()
      }
    } catch (error) {
      notifyDeleteRequestError(error, scopeLabel)
    } finally {
      finishDeleteRequest(abortController)
      setSubmitting(false)
    }
  }

  const deleteSelectedObjects = async () => {
    if (!canWrite) {
      return
    }

    const { projectID, bucketName } = getQuery()
    if (!projectID || !bucketName) {
      messageApi.error(tr('请先填写并查询 Project ID 与 BucketName。', 'Please set and query Project ID and BucketName first.'))
      return
    }

    const keys = selectedObjectKeys
      .map((item) => String(item).trim())
      .filter((item) => item !== '')
    if (keys.length === 0) {
      messageApi.warning(tr('请先选择要删除的文件或目录。', 'Please select files or directories to delete first.'))
      return
    }

    const scopeLabel = tr('批量删除', 'Batch Delete')
    const abortController = startDeleteRequest(scopeLabel)
    setSubmitting(true)
    try {
      const response = await apiClient.delete<ApiResponse<BatchDeletePayload>>(
        `/projects/${projectID}/storage/objects/batch`,
        {
          data: {
            bucketName,
            keys,
          },
          timeout: DELETE_REQUEST_TIMEOUT_MS,
          signal: abortController.signal,
        },
      )
      const payload = response.data?.data
      const summary = payload?.summary
      const success = toNonNegativeNumber(summary?.success) ?? 0
      const failure = toNonNegativeNumber(summary?.failure) ?? 0
      const allResults = Array.isArray(payload?.results) ? payload.results : []
      const selectedDirectories = allResults.filter(
        (item) => normalizeString(item.targetType) === 'directory',
      ).length
      const selectedFiles = keys.length - selectedDirectories
      const directoryDeletedObjects = allResults.reduce((acc, item) => {
        if (normalizeString(item.targetType) !== 'directory') {
          return acc
        }
        return acc + (toNonNegativeNumber(item.deletedObjects) ?? 0)
      }, 0)
      const directoryFailedObjects = allResults.reduce((acc, item) => {
        if (normalizeString(item.targetType) !== 'directory') {
          return acc
        }
        return acc + (toNonNegativeNumber(item.failedObjects) ?? 0)
      }, 0)

      const failedItems = allResults
        .filter((item) => normalizeString(item.result) !== 'success')
        .slice(0, 3)
      const failedText = failedItems
        .map((item, index) => {
          const reason = normalizeString(item.reason) || normalizeString(item.errorCode) || 'unknown error'
          return `${index + 1}. ${item.key}: ${reason}`
        })
        .join('；')
      const scopeText = tr(`（文件 ${selectedFiles}，目录 ${selectedDirectories}）`, ` (files ${selectedFiles}, directories ${selectedDirectories})`)
      const directorySummaryText =
        selectedDirectories > 0
          ? tr(`。目录递归删除统计：成功删除对象 ${directoryDeletedObjects}，失败 ${directoryFailedObjects}`, `. Directory recursive delete stats: deleted ${directoryDeletedObjects}, failed ${directoryFailedObjects}`)
          : ''

      if (failure > 0) {
        messageApi.warning(
          tr(`批量删除完成${scopeText}：成功 ${success}，失败 ${failure}${directorySummaryText}${failedText ? `。失败摘要：${failedText}` : ''}`, `Batch delete completed${scopeText}: success ${success}, failure ${failure}${directorySummaryText}${failedText ? `. Failure summary: ${failedText}` : ''}`),
        )
      } else {
        messageApi.success(tr(`批量删除完成${scopeText}：成功 ${success}，失败 0${directorySummaryText}`, `Batch delete completed${scopeText}: success ${success}, failure 0${directorySummaryText}`))
      }

      setSelectedObjectKeys([])
      await queryObjects()
    } catch (error) {
      notifyDeleteRequestError(error, scopeLabel)
    } finally {
      finishDeleteRequest(abortController)
      setSubmitting(false)
    }
  }

  const openRename = (sourceKey: string, isDirectory: boolean) => {
    if (!canWrite) {
      return
    }
    const normalizedSourceKey = isDirectory ? normalizeDirectoryPrefix(sourceKey) : sourceKey
    setRenamingTargetType(isDirectory ? 'directory' : 'file')
    setRenamingSourceKey(normalizedSourceKey)
    renameForm.setFieldsValue({ targetKey: normalizedSourceKey })
    setRenameVisible(true)
  }

  const submitRename = async () => {
    if (!canWrite) {
      return
    }
    const { projectID, bucketName } = getQuery()
    if (!projectID || !bucketName) {
      messageApi.error(tr('请先填写并查询 Project ID 与 BucketName。', 'Please set and query Project ID and BucketName first.'))
      return
    }
    const values = await renameForm.validateFields()
    const targetKey =
      renamingTargetType === 'directory'
        ? normalizeDirectoryPrefix(values.targetKey)
        : values.targetKey.trim()
    if (!targetKey) {
      messageApi.error(
        renamingTargetType === 'directory' ? tr('目标目录前缀不能为空。', 'Target directory prefix cannot be empty.') : tr('目标对象 Key 不能为空。', 'Target object key cannot be empty.'),
      )
      return
    }
    if (renamingTargetType === 'directory') {
      renameForm.setFieldsValue({ targetKey })
    }

    setSubmitting(true)
    try {
      const response = await apiClient.put<ApiResponse<RenameObjectPayload>>(`/projects/${projectID}/storage/rename`, {
        bucketName,
        sourceKey: renamingSourceKey,
        targetKey,
      })
      const payload = response.data?.data
      const summary = payload?.summary
      const targetType =
        normalizeString(summary?.targetType) || renamingTargetType
      const migratedObjects = toNonNegativeNumber(summary?.migratedObjects) ?? 0
      const failedObjects = toNonNegativeNumber(summary?.failedObjects) ?? 0
      const failureReasons = Array.isArray(summary?.failureReasons)
        ? summary?.failureReasons
            .map((item) => normalizeString(item))
            .filter((item) => item.length > 0)
        : []
      const failedReasonText = failureReasons.slice(0, 3).join('；')

      if (targetType === 'directory') {
        if (failedObjects > 0 || normalizeString(summary?.result) === 'failure') {
          messageApi.warning(
            tr(`目录重命名完成：迁移 ${migratedObjects}，失败 ${failedObjects}${failedReasonText ? `。失败摘要：${failedReasonText}` : ''}`, `Directory rename completed: migrated ${migratedObjects}, failed ${failedObjects}${failedReasonText ? `. Failure summary: ${failedReasonText}` : ''}`),
          )
        } else {
          messageApi.success(tr(`目录重命名成功：迁移 ${migratedObjects} 个对象。`, `Directory rename succeeded: migrated ${migratedObjects} objects.`))
        }
      } else {
        messageApi.success(payload?.message || tr('重命名成功。', 'Rename succeeded.'))
      }

      setRenameVisible(false)
      if (targetType === 'directory') {
        const nextPrefix = remapCurrentPrefixAfterDirectoryRename(
          currentPrefix,
          renamingSourceKey,
          targetKey,
        )
        queryForm.setFieldValue('prefix', nextPrefix)
        await queryObjects(undefined, nextPrefix)
      } else {
        await queryObjects()
      }
    } catch (error) {
      messageApi.error(resolveAPIErrorMessage(error, tr('重命名失败。', 'Rename failed.')))
    } finally {
      setSubmitting(false)
    }
  }

  const openAudit = async (path: string) => {
    const { projectID } = getQuery()
    if (!projectID) {
      messageApi.error(tr('请先填写并查询 Project ID。', 'Please set and query Project ID first.'))
      return
    }
    setAuditVisible(true)
    setAuditLoading(true)
    try {
      const logs = await listStorageAuditLogs(projectID, { path, limit: 20, offset: 0 })
      setAuditLogs(logs)
    } catch (error) {
      messageApi.error(resolveAPIErrorMessage(error, tr('审计日志查询失败。', 'Failed to query audit logs.')))
      setAuditLogs([])
    } finally {
      setAuditLoading(false)
    }
  }

  const loadFailureSummaryForSession = async (
    projectID: number,
    sessionID: string,
  ): Promise<string> => {
    const logs = await listStorageAuditLogs(projectID, {
      action: 'object.upload',
      sessionId: sessionID,
      result: 'failure',
      limit: 5,
      offset: 0,
    })
    return buildFailureSummaryFromAuditLogs(logs)
  }

  const loadUploadSessionSummaryByID = async (
    projectID: number,
    sessionID: string,
  ): Promise<UploadSessionSummary | null> => {
    const logs = await listStorageAuditLogs(projectID, {
      action: 'object.upload_archive',
      sessionId: sessionID,
      limit: 1,
      offset: 0,
    })
    const log = logs[0]
    if (!log) {
      return null
    }
    return parseArchiveSessionSummary(log)
  }

  const pollUploadSessionStageB = async (
    projectID: number,
    sessionID: string,
  ): Promise<UploadSessionSummary | null> => {
    const deadline = Date.now() + UPLOAD_STAGE_B_POLL_TIMEOUT_MS
    let latestSummary: UploadSessionSummary | null = null

    setUploadStageBSessionID(sessionID)
    setUploadStageBActive(true)
    setUploadStageBSummary(null)
    setUploadStageBFailureSummary('')

    try {
      while (Date.now() <= deadline) {
        const summary = await loadUploadSessionSummaryByID(projectID, sessionID)
        if (summary) {
          latestSummary = summary
          setUploadStageBSummary(summary)
          if (summary.failedEntries > 0) {
            try {
              const failureSummary = await loadFailureSummaryForSession(projectID, sessionID)
              setUploadStageBFailureSummary(failureSummary)
            } catch {
              setUploadStageBFailureSummary('')
            }
          } else {
            setUploadStageBFailureSummary('')
          }
          if (isSessionSummaryTerminal(summary)) {
            return summary
          }
        }
        await new Promise((resolve) => {
          window.setTimeout(resolve, UPLOAD_STAGE_B_POLL_INTERVAL_MS)
        })
      }
    } finally {
      setUploadStageBActive(false)
    }

    return latestSummary
  }

  const loadUploadSessionSummaries = async (projectIDInput?: number) => {
    const projectID = projectIDInput ?? getQuery().projectID
    if (!Number.isFinite(projectID) || projectID <= 0) {
      setSessionSummaries([])
      setSessionFailureSummaryMap({})
      return
    }

    setSessionSummariesLoading(true)
    try {
      const logs = await listStorageAuditLogs(projectID, {
        action: 'object.upload_archive',
        limit: 20,
        offset: 0,
      })
      const seenSessionIDs = new Set<string>()
      const summaries: UploadSessionSummary[] = []

      logs.forEach((log) => {
        const summary = parseArchiveSessionSummary(log)
        if (!summary || seenSessionIDs.has(summary.sessionId)) {
          return
        }
        seenSessionIDs.add(summary.sessionId)
        summaries.push(summary)
      })

      setSessionSummaries(summaries)
      setSessionFailureSummaryMap({})

      const failedSessions = summaries.filter((item) => item.failedEntries > 0)
      if (failedSessions.length > 0) {
        const entries = await Promise.all(
          failedSessions.map(async (item) => {
            try {
              const failureSummary = await loadFailureSummaryForSession(projectID, item.sessionId)
              return [item.sessionId, failureSummary] as const
            } catch {
              return [item.sessionId, ''] as const
            }
          }),
        )
        const map: Record<string, string> = {}
        entries.forEach(([sessionID, summary]) => {
          if (summary) {
            map[sessionID] = summary
          }
        })
        setSessionFailureSummaryMap(map)
      }
    } catch (error) {
      messageApi.error(resolveAPIErrorMessage(error, tr('上传会话汇总加载失败。', 'Failed to load upload session summaries.')))
      setSessionSummaries([])
      setSessionFailureSummaryMap({})
    } finally {
      setSessionSummariesLoading(false)
    }
  }

  const openSessionDetail = async (sessionSummary: UploadSessionSummary) => {
    const projectID = getQuery().projectID
    if (!Number.isFinite(projectID) || projectID <= 0) {
      messageApi.error(tr('请先填写并查询 Project ID。', 'Please set and query Project ID first.'))
      return
    }

    setActiveSession(sessionSummary)
    setSessionDetailCurrentPage(1)
    setSessionDetailVisible(true)
    setSessionDetailLoading(true)
    try {
      const logs = await listStorageAuditLogs(projectID, {
        action: 'object.upload',
        sessionId: sessionSummary.sessionId,
        limit: 200,
        offset: 0,
      })
      setSessionDetailLogs(logs)
      if (sessionSummary.failedEntries > 0) {
        const failureSummary = buildFailureSummaryFromAuditLogs(
          logs.filter((item) => normalizeString(item.result) === 'failure'),
        )
        if (failureSummary) {
          setSessionFailureSummaryMap((prev) => ({
            ...prev,
            [sessionSummary.sessionId]: failureSummary,
          }))
        }
      }
    } catch (error) {
      messageApi.error(resolveAPIErrorMessage(error, tr('上传会话明细加载失败。', 'Failed to load upload session details.')))
      setSessionDetailLogs([])
    } finally {
      setSessionDetailLoading(false)
    }
  }

  const pagedSessionDetailLogs = sessionDetailLogs.slice(
    (sessionDetailCurrentPage - 1) * sessionDetailPageSize,
    sessionDetailCurrentPage * sessionDetailPageSize,
  )

  const columns: ColumnsType<ObjectItem> = [
    {
      title: tr('名称', 'Name'),
      dataIndex: 'key',
      render: (value: string, record) => {
        const displayName = objectDisplayName(value, currentPrefix, record.isDir)
        return (
        <Space size={8}>
          {record.isDir ? (
            <Button
              type="link"
              size="small"
              style={{ padding: 0 }}
              onClick={() => void enterDirectory(record.key)}
            >
              {displayName}
            </Button>
          ) : (
            <Typography.Text>{displayName}</Typography.Text>
          )}
          {record.isDir ? <Tag color="geekblue">DIR</Tag> : null}
        </Space>
      )},
    },
    { title: tr('大小', 'Size'), dataIndex: 'size', width: 120 },
    { title: tr('类型', 'Type'), dataIndex: 'contentType', width: 180 },
    { title: tr('更新时间', 'Updated At'), dataIndex: 'lastModified', width: 220 },
    {
      title: tr('操作', 'Actions'),
      width: 360,
      render: (_, record) => (
        <Space>
          {!record.isDir ? (
            <Button
              icon={<DownloadOutlined />}
              onClick={() => void downloadObject(record.key)}
              size="small"
            >
              {tr('下载', 'Download')}
            </Button>
          ) : null}
          <Button
            icon={<EditOutlined />}
            size="small"
            onClick={() => openRename(record.key, record.isDir)}
            disabled={!canWrite}
          >
            {tr('重命名', 'Rename')}
          </Button>
          <Popconfirm
            title={record.isDir ? tr('确认删除该目录及其下全部对象？', 'Delete this directory and all nested objects?') : tr('确认删除该对象？', 'Delete this object?')}
            onConfirm={() => void deleteObject(record.key, record.isDir)}
            okButtonProps={{ loading: submitting }}
            disabled={!canWrite}
          >
            <Button
              danger
              icon={<DeleteOutlined />}
              size="small"
              disabled={!canWrite}
            >
              {tr('删除', 'Delete')}
            </Button>
          </Popconfirm>
          <Button
            icon={<FileSearchOutlined />}
            size="small"
            onClick={() => void openAudit(record.key)}
          >
            {tr('审计', 'Audit')}
          </Button>
        </Space>
      ),
    },
  ]

  const rowSelection: TableRowSelection<ObjectItem> = {
    selectedRowKeys: selectedObjectKeys,
    onChange: (keys) => setSelectedObjectKeys(keys),
    getCheckboxProps: () => ({
      disabled: !canWrite,
    }),
    preserveSelectedRowKeys: false,
  }

  const sessionColumns: ColumnsType<UploadSessionSummary> = [
    {
      title: tr('会话 ID', 'Session ID'),
      dataIndex: 'sessionId',
      width: 260,
      render: (value: string) => (
        <Tooltip title={value} placement="topLeft">
          <Typography.Text
            copyable={{ text: value }}
            ellipsis
            style={{ display: 'inline-block', maxWidth: '100%', verticalAlign: 'bottom' }}
          >
            {value}
          </Typography.Text>
        </Tooltip>
      ),
    },
    {
      title: tr('整体进展', 'Progress'),
      width: 210,
      render: (_, record) => (
        <Space direction="vertical" size={2} style={{ width: '100%' }}>
          <Progress percent={record.progressPercent} size="small" />
          <Typography.Text type="secondary">
            {record.processedEntries}/{record.totalEntries || record.processedEntries}
          </Typography.Text>
        </Space>
      ),
    },
    {
      title: tr('开始时间', 'Started At'),
      dataIndex: 'startedAt',
      width: 190,
      render: (value: string) => formatDateTimeText(value),
    },
    {
      title: tr('结束时间', 'Finished At'),
      dataIndex: 'finishedAt',
      width: 190,
      render: (value: string) => formatDateTimeText(value),
    },
    {
      title: tr('耗时', 'Duration'),
      dataIndex: 'durationMs',
      width: 140,
      render: (value: number) => formatDurationText(value),
    },
    {
      title: tr('成功/失败', 'Success/Failure'),
      width: 120,
      render: (_, record) => (
        <Typography.Text>
          {record.successEntries}/{record.failedEntries}
        </Typography.Text>
      ),
    },
    {
      title: tr('失败摘要', 'Failure Summary'),
      width: 340,
      render: (_, record) => {
        if (record.failedEntries <= 0) {
          return <Typography.Text type="secondary">-</Typography.Text>
        }

        const summary = sessionFailureSummaryMap[record.sessionId]
        return (
          <Typography.Text
            ellipsis={{ tooltip: summary || tr('点击“查看明细”生成失败摘要。', 'Click "View Details" to generate failure summary.') }}
            style={{ display: 'inline-block', maxWidth: 320, verticalAlign: 'bottom' }}
          >
            {summary || tr('点击“查看明细”生成失败摘要。', 'Click "View Details" to generate failure summary.')}
          </Typography.Text>
        )
      },
    },
    {
      title: tr('结果', 'Result'),
      dataIndex: 'result',
      width: 110,
      render: (value: string) =>
        value === 'success' ? (
          <Tag color="green">success</Tag>
        ) : value === 'failure' ? (
          <Tag color="red">failure</Tag>
        ) : (
          <Tag>{value || '-'}</Tag>
        ),
    },
    {
      title: tr('操作', 'Actions'),
      width: 120,
      render: (_, record) => (
        <Button size="small" onClick={() => void openSessionDetail(record)}>
          {tr('查看明细', 'View Details')}
        </Button>
      ),
    },
  ]

  return (
    <>
      {messageContext}
      <Space direction="vertical" size={16} style={{ width: '100%' }}>
        <Card title={tr('存储对象', 'Storage Objects')}>
          {!canWrite ? (
            <Alert
              type="warning"
              showIcon
              message={t('pages.storage.readOnlyHint')}
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
              rules={[{ required: true, message: tr('请输入 Project ID', 'Please input Project ID') }]}
            >
              <Select
                showSearch
                placeholder={tr('请选择或搜索 Project ID', 'Select or search Project ID')}
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
                  const projectID = Number(value)
                  void loadBucketsByProject(projectID)
                  void loadUploadSessionSummaries(projectID)
                  queryForm.setFieldValue('prefix', '')
                  setCurrentPrefix('')
                  setObjects([])
                  setSelectedObjectKeys([])
                  setActiveSession(null)
                  setSessionDetailLogs([])
                  setUploadStageBSessionID('')
                  setUploadStageBSummary(null)
                  setUploadStageBFailureSummary('')
                  setUploadStageBActive(false)
                }}
              />
            </Form.Item>
            <Form.Item
              name="bucketName"
              label="Bucket"
              rules={[{ required: true, message: tr('请输入 BucketName', 'Please input BucketName') }]}
            >
              <Select
                showSearch
                placeholder={tr('请选择 Bucket（选项目后自动加载）', 'Select Bucket (loaded after project selected)')}
                style={{ width: 280 }}
                loading={bucketOptionsLoading}
                options={bucketOptions.map((bucketName) => ({
                  value: bucketName,
                  label: bucketName,
                }))}
                notFoundContent={tr('该项目暂无存储桶绑定', 'No bucket binding for this project')}
                filterOption={(input, option) =>
                  String(option?.label ?? '')
                    .toLowerCase()
                    .includes(input.toLowerCase())
                }
                onChange={() => {
                  queryForm.setFieldValue('prefix', '')
                  setCurrentPrefix('')
                  setObjects([])
                  setSelectedObjectKeys([])
                  setUploadStageBSessionID('')
                  setUploadStageBSummary(null)
                  setUploadStageBFailureSummary('')
                  setUploadStageBActive(false)
                }}
              />
            </Form.Item>
            <Form.Item name="prefix" label="Prefix">
              <Input placeholder={tr('可选目录前缀', 'Optional prefix')} style={{ width: 220 }} />
            </Form.Item>
            <Form.Item>
              <Button
                type="primary"
                icon={<ReloadOutlined />}
                onClick={() => void queryObjects()}
                loading={loading}
              >
                {t('pages.storage.queryObjects')}
              </Button>
            </Form.Item>
          </Form>
          {queryError ? (
            <Alert type="error" showIcon style={{ marginTop: 12 }} message={queryError} />
          ) : null}
        </Card>

        <Card
          title={tr('上传', 'Upload')}
          extra={
            <Space>
              <Upload
                multiple
                beforeUpload={(file) => {
                  if (file.size > uploadSizeLimitBytes) {
                    messageApi.error(
                      tr(`文件 ${file.name} 超过当前大小限制（${formatUploadLimitText(uploadSizeLimitBytes)}），已拒绝加入列表。`, `File ${file.name} exceeds size limit (${formatUploadLimitText(uploadSizeLimitBytes)}), rejected.`),
                    )
                    return Upload.LIST_IGNORE
                  }
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
                disabled={!canWrite || uploadStageAActive}
              >
                <Button icon={<UploadOutlined />} disabled={!canWrite || uploadStageAActive}>
                  {tr('选择文件', 'Select Files')}
                </Button>
              </Upload>
              <Button
                type="primary"
                loading={submitting}
                onClick={() => void uploadObject()}
                disabled={!canWrite || uploadStageAActive}
              >
                {t('pages.storage.upload')}
              </Button>
              {uploadStageAActive ? (
                <Button danger onClick={cancelUpload}>
                  {t('pages.storage.cancelUpload')}
                </Button>
              ) : null}
            </Space>
          }
        >
          <Space direction="vertical" size={8} style={{ width: '100%' }}>
            {uploadStageAActive ? (
              <Space direction="vertical" size={4} style={{ width: '100%' }}>
                <Progress
                  percent={uploadStageAProgress}
                  status="active"
                  size="small"
                  data-testid="upload-stage-a-progress"
                />
              </Space>
            ) : null}
            {uploadStageBActive || uploadStageBSummary ? (
              <Card size="small" title={tr('上传阶段 B（后端处理）', 'Upload Stage B (Backend Processing)')}>
                <Space direction="vertical" size={4} style={{ width: '100%' }}>
                  <Typography.Text type="secondary">
                    {tr('会话 ID：', 'Session ID: ')}{uploadStageBSessionID || uploadStageBSummary?.sessionId || '-'}
                  </Typography.Text>
                  <Progress
                    percent={uploadStageBSummary?.progressPercent ?? 0}
                    status={uploadStageBActive ? 'active' : undefined}
                    size="small"
                    data-testid="upload-stage-b-progress"
                  />
                  <Typography.Text>
                    {tr('处理进度：', 'Progress: ')}{uploadStageBSummary?.processedEntries ?? 0}/
                    {uploadStageBSummary?.totalEntries || uploadStageBSummary?.processedEntries || 0}
                  </Typography.Text>
                  <Typography.Text>
                    {tr('开始：', 'Start: ')}{formatDateTimeText(uploadStageBSummary?.startedAt || '')}{tr('，结束：', ', End: ')}
                    {formatDateTimeText(uploadStageBSummary?.finishedAt || '')}{tr('，耗时：', ', Duration: ')}
                    {formatDurationText(uploadStageBSummary?.durationMs ?? 0)}
                  </Typography.Text>
                  <Typography.Text>
                    {tr('成功：', 'Success: ')}{uploadStageBSummary?.successEntries ?? 0}{tr('，失败：', ', Failure: ')}
                    {uploadStageBSummary?.failedEntries ?? 0}
                  </Typography.Text>
                  <Typography.Text type="secondary">
                    {tr('失败摘要：', 'Failure summary: ')}{uploadStageBFailureSummary || tr('当前会话无失败摘要。', 'No failure summary in current session.')}
                  </Typography.Text>
                  {uploadStageBSummary ? (
                    <Button size="small" onClick={() => void openSessionDetail(uploadStageBSummary)}>
                      {tr('查看会话明细', 'View Session Details')}
                    </Button>
                  ) : null}
                </Space>
              </Card>
            ) : null}
            <Input
              placeholder={tr('可选：单文件时为对象 Key，多文件时作为 keyPrefix', 'Optional: object Key for single file, keyPrefix for multiple files')}
              value={uploadKey}
              onChange={(event) => setUploadKey(event.target.value)}
              disabled={!canWrite}
            />
            <Typography.Text type="secondary">
              {tr('当前大小限制：', 'Current size limit: ')}{formatUploadLimitText(uploadSizeLimitBytes)}
            </Typography.Text>
            <Typography.Text type="secondary">
              {tr('待上传文件：', 'Pending files: ')}{pendingUploadFiles.length}
              {pendingUploadFiles.length > 1 && uploadKey.trim()
                ? tr('（将使用当前输入作为 keyPrefix）', ' (current input will be used as keyPrefix)')
                : ''}
            </Typography.Text>
            <Typography.Text type="secondary">
              {tr('支持 zip/tar/tar.gz/tgz 自动解压上传。', 'Supports auto-extract upload for zip/tar/tar.gz/tgz.')}
            </Typography.Text>
          </Space>
        </Card>

        <Card
          title={tr('对象列表', 'Object List')}
          extra={
            <Space>
              {deleting ? (
                <>
                  <Typography.Text type="secondary">
                    {(deletingScopeLabel || tr('删除', 'Delete'))}{tr('进行中...', ' in progress...')}
                  </Typography.Text>
                  <Button danger onClick={cancelDelete} size="small">
                    {tr('取消删除', 'Cancel Delete')}
                  </Button>
                </>
              ) : null}
              <Typography.Text type="secondary">{t('pages.storage.currentDir', { dir: currentPrefix || '/' })}</Typography.Text>
              <Button onClick={() => void goToParentDirectory()} disabled={!currentPrefix || loading}>
                {tr('返回上一级', 'Back to Parent')}
              </Button>
              <Popconfirm
                title={tr(`确认删除已选 ${selectedObjectKeys.length} 个条目？目录将递归删除其下全部对象。`, `Delete ${selectedObjectKeys.length} selected entries? Directories will be deleted recursively.`)}
                onConfirm={() => void deleteSelectedObjects()}
                okButtonProps={{ loading: submitting }}
                disabled={!canWrite || selectedObjectKeys.length === 0}
              >
                <Button
                  danger
                  icon={<DeleteOutlined />}
                  disabled={!canWrite || selectedObjectKeys.length === 0}
                >
                  {tr('批量删除', 'Batch Delete')}
                </Button>
              </Popconfirm>
            </Space>
          }
        >
          <Table<ObjectItem>
            rowKey="key"
            columns={columns}
            dataSource={objects}
            rowSelection={rowSelection}
            loading={loading}
            pagination={{
              current: objectCurrentPage,
              pageSize: objectPageSize,
              showSizeChanger: true,
              pageSizeOptions: OBJECT_PAGE_SIZE_OPTIONS.map(String),
              onChange: (page, size) => {
                if (size && size !== objectPageSize) {
                  setObjectPageSize(size)
                  setObjectCurrentPage(1)
                  void queryObjects(size)
                  return
                }
                setObjectCurrentPage(page)
              },
            }}
          />
        </Card>

        <Card
          title={tr('压缩包上传会话', 'Archive Upload Sessions')}
          extra={
            <Button
              icon={<ReloadOutlined />}
              loading={sessionSummariesLoading}
              onClick={() => void loadUploadSessionSummaries()}
            >
              {t('pages.storage.refreshSessions')}
            </Button>
          }
        >
          <Space direction="vertical" size={8} style={{ width: '100%' }}>
            <Typography.Text type="secondary">
              {tr('仅展示压缩包上传会话汇总（action = object.upload_archive），可查看对应 object.upload 明细。', 'Only archive upload session summaries are shown (action = object.upload_archive).')}
            </Typography.Text>
            <div style={{ width: '100%', maxWidth: '100%', overflowX: 'auto' }}>
              <Table<UploadSessionSummary>
                rowKey="sessionId"
                loading={sessionSummariesLoading}
                columns={sessionColumns}
                dataSource={sessionSummaries}
                pagination={{ pageSize: 5 }}
                locale={{ emptyText: tr('暂无压缩包上传会话记录。', 'No archive upload sessions.') }}
                tableLayout="fixed"
                scroll={{ x: 1600 }}
                style={{ width: '100%', minWidth: 0 }}
              />
            </div>
          </Space>
        </Card>
      </Space>

      <Modal
        title={renamingTargetType === 'directory' ? tr('重命名目录', 'Rename Directory') : tr('重命名对象', 'Rename Object')}
        open={renameVisible}
        onCancel={() => setRenameVisible(false)}
        onOk={() => void submitRename()}
        okButtonProps={{ loading: submitting, disabled: !canWrite }}
        destroyOnHidden
      >
        <Typography.Paragraph type="secondary">{tr('源对象：', 'Source: ')}{renamingSourceKey}</Typography.Paragraph>
        {renamingTargetType === 'directory' ? (
          <Typography.Paragraph type="secondary">
            {tr('目录重命名会迁移该目录前缀下全部对象到目标目录前缀。', 'Directory rename migrates all objects under source prefix to target prefix.')}
          </Typography.Paragraph>
        ) : null}
        <Form<RenameFormValues> form={renameForm} layout="vertical">
          <Form.Item
            name="targetKey"
            label={renamingTargetType === 'directory' ? tr('目标目录前缀', 'Target Directory Prefix') : tr('目标对象 Key', 'Target Object Key')}
            rules={[
              {
                required: true,
                message: renamingTargetType === 'directory' ? tr('请输入目标目录前缀', 'Please input target directory prefix') : tr('请输入目标对象 Key', 'Please input target object key'),
              },
              {
                min: 1,
                message:
                  renamingTargetType === 'directory'
                    ? tr('目标目录前缀不能为空', 'Target directory prefix cannot be empty')
                    : tr('目标对象 Key 不能为空', 'Target object key cannot be empty'),
              },
            ]}
          >
            <Input />
          </Form.Item>
        </Form>
      </Modal>

      <Drawer
        title={tr('对象审计日志', 'Object Audit Logs')}
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
            { title: tr('时间', 'Time'), dataIndex: 'createdAt', width: 210 },
            { title: tr('动作', 'Action'), dataIndex: 'action', width: 160 },
            {
              title: tr('结果', 'Result'),
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
            { title: tr('对象', 'Object'), dataIndex: 'targetIdentifier' },
            { title: tr('操作者', 'Actor'), dataIndex: 'actorUsername', width: 140 },
          ]}
        />
      </Drawer>

      <Drawer
        title={activeSession ? `${tr('上传会话明细：', 'Upload Session Detail: ')}${activeSession.sessionId}` : tr('上传会话明细', 'Upload Session Detail')}
        open={sessionDetailVisible}
        onClose={() => setSessionDetailVisible(false)}
        width={980}
      >
        <Space direction="vertical" size={12} style={{ width: '100%' }}>
          {activeSession ? (
            <Card size="small">
              <Space direction="vertical" size={4}>
                <Typography.Text>
                  {tr('会话进展：', 'Session progress: ')}{activeSession.processedEntries}/
                  {activeSession.totalEntries || activeSession.processedEntries} (
                  {activeSession.progressPercent}%)
                </Typography.Text>
                <Typography.Text>
                  {tr('开始：', 'Start: ')}{formatDateTimeText(activeSession.startedAt)}{tr('，结束：', ', End: ')}
                  {formatDateTimeText(activeSession.finishedAt)}{tr('，耗时：', ', Duration: ')}
                  {formatDurationText(activeSession.durationMs)}
                </Typography.Text>
                <Typography.Text>
                  {tr('成功：', 'Success: ')}{activeSession.successEntries}{tr('，失败：', ', Failure: ')}{activeSession.failedEntries}
                </Typography.Text>
                <Typography.Text type="secondary">
                  {tr('失败摘要：', 'Failure summary: ')}
                  {sessionFailureSummaryMap[activeSession.sessionId] || tr('当前会话无失败摘要。', 'No failure summary in current session.')}
                </Typography.Text>
              </Space>
            </Card>
          ) : null}

          <Table<StorageAuditLog>
            rowKey="id"
            loading={sessionDetailLoading}
            pagination={{
              current: sessionDetailCurrentPage,
              pageSize: sessionDetailPageSize,
              total: sessionDetailLogs.length,
              showSizeChanger: true,
              pageSizeOptions: ['12', '24', '50', '100'],
              onChange: (page, size) => {
                setSessionDetailCurrentPage(page)
                if (size && size !== sessionDetailPageSize) {
                  setSessionDetailPageSize(size)
                  setSessionDetailCurrentPage(1)
                }
              },
            }}
            dataSource={pagedSessionDetailLogs}
            columns={[
              { title: tr('时间', 'Time'), dataIndex: 'createdAt', width: 210 },
              { title: tr('动作', 'Action'), dataIndex: 'action', width: 140 },
              {
                title: tr('结果', 'Result'),
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
              { title: tr('对象', 'Object'), dataIndex: 'targetIdentifier' },
              {
                title: tr('失败原因', 'Failure Reason'),
                width: 260,
                render: (_, record) =>
                  normalizeString(record.result) === 'failure'
                    ? parseFailureReasonFromAuditLog(record)?.reason ?? '-'
                    : '-',
              },
              { title: tr('操作者', 'Actor'), dataIndex: 'actorUsername', width: 120 },
            ]}
          />
        </Space>
      </Drawer>
    </>
  )
}
