import axios from 'axios'

type ErrorPayload = {
  message?: unknown
  requestId?: unknown
}

const extractRequestIDFromHeaders = (headers: unknown): string | undefined => {
  if (!headers || typeof headers !== 'object') {
    return undefined
  }

  const rawValue =
    (headers as Record<string, unknown>)['x-request-id'] ??
    (headers as Record<string, unknown>)['X-Request-ID']

  if (typeof rawValue === 'string' && rawValue.trim().length > 0) {
    return rawValue
  }

  if (Array.isArray(rawValue) && typeof rawValue[0] === 'string' && rawValue[0].trim().length > 0) {
    return rawValue[0]
  }

  return undefined
}

export const resolveAPIErrorMessage = (error: unknown, fallback: string): string => {
  if (!axios.isAxiosError(error)) {
    return fallback
  }

  const payload = error.response?.data as ErrorPayload | undefined
  const payloadMessage =
    typeof payload?.message === 'string' && payload.message.trim().length > 0
      ? payload.message
      : fallback

  const payloadRequestID =
    typeof payload?.requestId === 'string' && payload.requestId.trim().length > 0
      ? payload.requestId
      : undefined

  const headerRequestID = extractRequestIDFromHeaders(error.response?.headers)
  const requestID = payloadRequestID ?? headerRequestID

  if (!requestID) {
    return payloadMessage
  }

  return `${payloadMessage}（请求追踪号: ${requestID}）`
}
