import axios from 'axios'
import { Alert, Button, Card, Form, Input, Space, Typography } from 'antd'
import type { AxiosError } from 'axios'
import { useState } from 'react'
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

const resolveApiErrorMessage = (error: unknown): string => {
  if (!axios.isAxiosError(error)) {
    return '登录失败，请稍后重试。'
  }

  const status = error.response?.status
  if (status === 401) {
    return '邮箱或密码错误，请检查后重试。'
  }

  if (status === 403) {
    return '当前账号没有登录权限，或账号已被禁用。'
  }

  if (status === 400 || status === 422) {
    const payload = error.response?.data
    if (isRecord(payload) && typeof payload.message === 'string' && payload.message.trim().length > 0) {
      return `登录参数校验失败：${payload.message}`
    }

    return '登录参数校验失败，请确认邮箱格式和密码是否正确。'
  }

  const fallback = (error as AxiosError<LoginApiResponse>).response?.data?.message
  if (typeof fallback === 'string' && fallback.trim().length > 0) {
    return fallback
  }

  return '登录失败，请稍后重试。'
}

export function LoginPage() {
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
        setErrorMessage('登录响应格式不符合预期，请联系管理员。')
        return
      }

      setSession({ token: accessToken, user: authUser })
      navigate('/', { replace: true })
    } catch (error) {
      setErrorMessage(resolveApiErrorMessage(error))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div
      style={{
        minHeight: '100vh',
        display: 'grid',
        placeItems: 'center',
        padding: 24,
        background:
          'radial-gradient(circle at top, rgba(226, 175, 111, 0.28), transparent 30%), linear-gradient(180deg, #f7efe1 0%, #efe1c8 52%, #e6d4b4 100%)',
      }}
    >
      <Card style={{ width: 'min(440px, 100%)', borderRadius: 24 }}>
        <Space direction="vertical" size={16} style={{ width: '100%' }}>
          <Typography.Text style={{ color: '#9c5b1f', letterSpacing: 2 }}>
            AUTH ENTRY
          </Typography.Text>
          <Typography.Title
            level={2}
            style={{
              margin: 0,
              fontFamily: '"Iowan Old Style", "Palatino Linotype", serif',
              color: '#2d2118',
            }}
          >
            Sign in to the control deck
          </Typography.Title>

          {errorMessage ? <Alert type="error" showIcon message={errorMessage} /> : null}

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
              label="Email"
              rules={[
                { required: true, message: '请输入邮箱地址。' },
                { type: 'email', message: '请输入有效的邮箱地址。' },
              ]}
            >
              <Input autoComplete="email" placeholder="admin@example.com" />
            </Form.Item>
            <Form.Item
              name="password"
              label="Password"
              rules={[
                { required: true, message: '请输入密码。' },
                { min: 6, message: '密码长度至少为 6 位。' },
              ]}
            >
              <Input.Password autoComplete="current-password" placeholder="••••••••" />
            </Form.Item>
            <Button type="primary" htmlType="submit" loading={submitting} block>
              Continue
            </Button>
          </Form>
        </Space>
      </Card>
    </div>
  )
}
