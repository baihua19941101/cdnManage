import axios from 'axios'
import { Alert, Button, Card, Form, Input, Space, Typography } from 'antd'
import type { AxiosError } from 'axios'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Navigate, useNavigate } from 'react-router-dom'

import { apiClient } from '../services/api/client'
import type { AuthUser } from '../store/auth'
import { useAuthStore } from '../store/auth'

type LoginFormValues = {
  email: string
  password: string
}

type LoginApiUser = {
  id: unknown
  username: unknown
  email: unknown
  status: unknown
  platformRole: unknown
}

type LoginApiResponse = {
  code?: unknown
  message?: unknown
  data?: {
    accessToken?: unknown
    user?: LoginApiUser
  }
}

const isRecord = (value: unknown): value is Record<string, unknown> =>
  typeof value === 'object' && value !== null

const isPlatformRole = (value: unknown): value is AuthUser['platformRole'] =>
  value === 'super_admin' || value === 'platform_admin' || value === 'standard_user'

const isUserStatus = (value: unknown): value is AuthUser['status'] =>
  value === 'active' || value === 'disabled'

const normalizeUser = (value: unknown): AuthUser | null => {
  if (!isRecord(value)) {
    return null
  }

  const id = value.id
  const email = value.email
  const status = value.status
  const platformRole = value.platformRole

  if (typeof id !== 'number' || typeof email !== 'string' || !isUserStatus(status) || !isPlatformRole(platformRole)) {
    return null
  }

  return {
    id,
    email,
    status,
    platformRole,
  }
}

const resolveApiErrorMessage = (error: unknown, t: (key: string, options?: Record<string, unknown>) => string): string => {
  if (!axios.isAxiosError(error)) {
    return t('auth.login.failed')
  }

  const status = error.response?.status
  if (status === 401) {
    return t('auth.login.invalidCredential')
  }

  if (status === 403) {
    return t('auth.login.forbidden')
  }

  if (status === 400 || status === 422) {
    const payload = error.response?.data
    if (isRecord(payload) && typeof payload.message === 'string' && payload.message.trim().length > 0) {
      return t('auth.login.validateFailedWithMessage', { message: payload.message })
    }

    return t('auth.login.validateFailed')
  }

  const fallback = (error as AxiosError<LoginApiResponse>).response?.data?.message
  if (typeof fallback === 'string' && fallback.trim().length > 0) {
    return fallback
  }

  return t('auth.login.failed')
}

export function LoginPage() {
  const { t } = useTranslation()
  const [form] = Form.useForm<LoginFormValues>()
  const navigate = useNavigate()
  const isLoggedIn = useAuthStore((state) => state.isLoggedIn)
  const setSession = useAuthStore((state) => state.setSession)
  const [submitting, setSubmitting] = useState(false)
  const [errorMessage, setErrorMessage] = useState<string | null>(null)

  if (isLoggedIn) {
    return <Navigate to="/" replace />
  }

  const handleSubmit = async (values: LoginFormValues) => {
    setErrorMessage(null)
    setSubmitting(true)

    try {
      const response = await apiClient.post<LoginApiResponse>('/auth/login', values)
      const accessToken = response.data?.data?.accessToken
      const authUser = normalizeUser(response.data?.data?.user)

      if (typeof accessToken !== 'string' || accessToken.trim().length === 0 || !authUser) {
        setErrorMessage(t('auth.login.invalidResponse'))
        return
      }

      setSession({ token: accessToken, user: authUser })
      navigate('/', { replace: true })
    } catch (error) {
      setErrorMessage(resolveApiErrorMessage(error, t))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="auth-scene">
      <Card className="auth-card auth-card--login">
        <Space direction="vertical" size={16} style={{ width: '100%' }}>
          <Typography.Text className="auth-label">{t('auth.login.entry')}</Typography.Text>
          <Space direction="vertical" size={6} style={{ width: '100%' }}>
            <Typography.Title level={2} className="auth-title">
              {t('auth.login.title')}
            </Typography.Title>
            <Typography.Text className="auth-subtitle">
              {t('auth.login.subtitle')}
            </Typography.Text>
          </Space>

          {errorMessage ? (
            <Alert className="auth-alert" type="error" showIcon message={errorMessage} />
          ) : null}

          <Form<LoginFormValues>
            form={form}
            layout="vertical"
            requiredMark={false}
            onFinish={handleSubmit}
            onValuesChange={() => {
              if (errorMessage) {
                setErrorMessage(null)
              }
            }}
          >
            <Form.Item
              name="email"
              label={t('auth.login.email')}
              rules={[
                { required: true, message: t('auth.login.emailRequired') },
                { type: 'email', message: t('auth.login.emailInvalid') },
              ]}
            >
              <Input autoComplete="email" placeholder="admin@example.com" size="large" />
            </Form.Item>
            <Form.Item
              name="password"
              label={t('auth.login.password')}
              rules={[
                { required: true, message: t('auth.login.passwordRequired') },
                { min: 6, message: t('auth.login.passwordMin') },
              ]}
            >
              <Input.Password autoComplete="current-password" placeholder="••••••••" size="large" />
            </Form.Item>
            <Button className="auth-submit" type="primary" htmlType="submit" loading={submitting} block size="large">
              {t('auth.login.submit')}
            </Button>
          </Form>
        </Space>
      </Card>
    </div>
  )
}
