import { apiClient } from '../api/client'

export type ApiResponse<T> = {
  code: string
  message: string
  data: T
}

export type StorageAuditLog = {
  id: number
  actorUserId: number
  actorUsername?: string
  action: string
  targetType: string
  targetIdentifier: string
  result: string
  requestId: string
  createdAt: string
  metadata?: Record<string, unknown>
}

type StorageAuditLogsPayload = {
  logs: StorageAuditLog[]
}

type ListStorageAuditLogsParams = {
  path?: string
  action?: string
  sessionId?: string
  result?: string
  limit?: number
  offset?: number
}

export const listStorageAuditLogs = async (
  projectID: number,
  params: ListStorageAuditLogsParams,
): Promise<StorageAuditLog[]> => {
  const response = await apiClient.get<ApiResponse<StorageAuditLogsPayload>>(
    `/projects/${projectID}/storage/audits`,
    {
      params,
    },
  )
  return response.data.data?.logs ?? []
}
