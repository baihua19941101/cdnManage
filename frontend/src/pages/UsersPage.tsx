import {
  EditOutlined,
  KeyOutlined,
  PlusOutlined,
  ReloadOutlined,
  StopOutlined,
  TeamOutlined,
} from '@ant-design/icons'
import {
  Alert,
  Button,
  Card,
  Form,
  Input,
  message,
  Modal,
  Popconfirm,
  Select,
  Space,
  Table,
  Tag,
  Typography,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import axios from 'axios'
import { useEffect, useState } from 'react'

import { apiClient } from '../services/api/client'
import { hasDuplicateProjectBindings } from './managementValidation'
import { isPlatformAdminRole, useAuthStore } from '../store/auth'

type UserStatus = 'active' | 'disabled'
type PlatformRole = 'super_admin' | 'platform_admin' | 'standard_user'
type ProjectRole = 'project_admin' | 'project_read_only'

type User = {
  id: number
  username: string
  email: string
  status: UserStatus
  platformRole: PlatformRole
}

type Project = {
  id: number
  name: string
}

type ApiResponse<T> = {
  code: string
  message: string
  data: T
}

type CreateUserFormValues = {
  username: string
  email: string
  password: string
  status: UserStatus
  platformRole: PlatformRole
}

type EditUserFormValues = {
  username: string
  email: string
  status: UserStatus
  platformRole: PlatformRole
}

type BindingFormValues = {
  bindings: Array<{
    projectId: number
    projectRole: ProjectRole
  }>
}

type ResetPasswordFormValues = {
  newPassword: string
  confirmPassword: string
}

const platformRoleOptions = [
  { label: 'Super Admin', value: 'super_admin' },
  { label: 'Platform Admin', value: 'platform_admin' },
  { label: 'Standard User', value: 'standard_user' },
]

const statusOptions = [
  { label: 'Active', value: 'active' },
  { label: 'Disabled', value: 'disabled' },
]

const projectRoleOptions = [
  { label: 'Project Admin', value: 'project_admin' },
  { label: 'Project Read Only', value: 'project_read_only' },
]

const roleTagColor: Record<PlatformRole, string> = {
  super_admin: 'magenta',
  platform_admin: 'geekblue',
  standard_user: 'cyan',
}

const resolveErrorMessage = (error: unknown, fallback: string) => {
  if (!axios.isAxiosError(error)) {
    return fallback
  }
  const payload = error.response?.data
  if (
    payload &&
    typeof payload === 'object' &&
    'message' in payload &&
    typeof payload.message === 'string' &&
    payload.message.trim().length > 0
  ) {
    return payload.message
  }
  return fallback
}

export function UsersPage() {
  const platformRole = useAuthStore((state) => state.user?.platformRole)
  const canWrite = isPlatformAdminRole(platformRole)
  const [messageApi, messageContext] = message.useMessage()
  const [createForm] = Form.useForm<CreateUserFormValues>()
  const [editForm] = Form.useForm<EditUserFormValues>()
  const [bindingForm] = Form.useForm<BindingFormValues>()
  const [resetPasswordForm] = Form.useForm<ResetPasswordFormValues>()

  const [users, setUsers] = useState<User[]>([])
  const [projects, setProjects] = useState<Project[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  const [createVisible, setCreateVisible] = useState(false)
  const [editVisible, setEditVisible] = useState(false)
  const [bindingVisible, setBindingVisible] = useState(false)
  const [resetPasswordVisible, setResetPasswordVisible] = useState(false)
  const [activeUser, setActiveUser] = useState<User | null>(null)

  const fetchUsers = async () => {
    setLoading(true)
    setError(null)
    try {
      const response = await apiClient.get<ApiResponse<User[]>>('/users')
      setUsers(Array.isArray(response.data.data) ? response.data.data : [])
    } catch (e) {
      setError(resolveErrorMessage(e, '用户列表加载失败，请稍后重试。'))
    } finally {
      setLoading(false)
    }
  }

  const fetchProjects = async () => {
    try {
      const response = await apiClient.get<ApiResponse<Project[]>>('/projects')
      setProjects(Array.isArray(response.data.data) ? response.data.data : [])
    } catch {
      setProjects([])
    }
  }

  useEffect(() => {
    void fetchUsers()
    void fetchProjects()
  }, [])

  const openCreate = () => {
    createForm.setFieldsValue({
      status: 'active',
      platformRole: 'standard_user',
    })
    setCreateVisible(true)
  }

  const submitCreate = async () => {
    if (!canWrite) {
      return
    }
    const values = await createForm.validateFields()
    setSubmitting(true)
    try {
      await apiClient.post('/users', values)
      messageApi.success('用户创建成功。')
      setCreateVisible(false)
      await fetchUsers()
    } catch (e) {
      messageApi.error(resolveErrorMessage(e, '用户创建失败。'))
    } finally {
      setSubmitting(false)
    }
  }

  const openEdit = (user: User) => {
    setActiveUser(user)
    editForm.setFieldsValue({
      username: user.username,
      email: user.email,
      status: user.status,
      platformRole: user.platformRole,
    })
    setEditVisible(true)
  }

  const submitEdit = async () => {
    if (!activeUser || !canWrite) {
      return
    }
    const values = await editForm.validateFields()
    setSubmitting(true)
    try {
      await apiClient.put(`/users/${activeUser.id}`, values)
      messageApi.success('用户信息已更新。')
      setEditVisible(false)
      await fetchUsers()
    } catch (e) {
      messageApi.error(resolveErrorMessage(e, '用户更新失败。'))
    } finally {
      setSubmitting(false)
    }
  }

  const disableUser = async (user: User) => {
    if (!canWrite) {
      return
    }
    setSubmitting(true)
    try {
      await apiClient.put(`/users/${user.id}`, {
        username: user.username,
        email: user.email,
        status: 'disabled',
        platformRole: user.platformRole,
      })
      messageApi.success('用户已禁用。')
      await fetchUsers()
    } catch (e) {
      messageApi.error(resolveErrorMessage(e, '禁用用户失败。'))
    } finally {
      setSubmitting(false)
    }
  }

  const openBindings = (user: User) => {
    setActiveUser(user)
    bindingForm.setFieldsValue({ bindings: [] })
    setBindingVisible(true)
  }

  const openResetPassword = (user: User) => {
    setActiveUser(user)
    resetPasswordForm.resetFields()
    setResetPasswordVisible(true)
  }

  const submitResetPassword = async () => {
    if (!activeUser || !canWrite) {
      return
    }
    const values = await resetPasswordForm.validateFields()

    setSubmitting(true)
    try {
      await apiClient.put(`/users/${activeUser.id}/password`, {
        newPassword: values.newPassword,
      })
      messageApi.success('密码重置成功。')
      setResetPasswordVisible(false)
      resetPasswordForm.resetFields()
    } catch (e) {
      messageApi.error(resolveErrorMessage(e, '重置密码失败。'))
    } finally {
      setSubmitting(false)
    }
  }

  const submitBindings = async () => {
    if (!activeUser || !canWrite) {
      return
    }
    const values = await bindingForm.validateFields()
    const bindings = values.bindings ?? []

    if (hasDuplicateProjectBindings(bindings)) {
      messageApi.error('同一个项目不能重复绑定。')
      return
    }

    setSubmitting(true)
    try {
      await apiClient.put(`/users/${activeUser.id}/project-bindings`, { bindings })
      messageApi.success('项目角色绑定已更新。')
      setBindingVisible(false)
    } catch (e) {
      messageApi.error(resolveErrorMessage(e, '项目角色绑定更新失败。'))
    } finally {
      setSubmitting(false)
    }
  }

  const columns: ColumnsType<User> = [
    { title: 'ID', dataIndex: 'id', width: 90 },
    { title: '用户名', dataIndex: 'username', width: 180 },
    { title: '邮箱', dataIndex: 'email' },
    {
      title: '状态',
      dataIndex: 'status',
      width: 120,
      render: (value: UserStatus) =>
        value === 'active' ? <Tag color="green">active</Tag> : <Tag color="red">disabled</Tag>,
    },
    {
      title: '平台角色',
      dataIndex: 'platformRole',
      width: 160,
      render: (value: PlatformRole) => <Tag color={roleTagColor[value]}>{value}</Tag>,
    },
    {
      title: '操作',
      key: 'actions',
      width: 320,
      render: (_, record) => (
        <Space>
          <Button icon={<EditOutlined />} onClick={() => openEdit(record)} disabled={!canWrite}>
            编辑
          </Button>
          <Button
            icon={<KeyOutlined />}
            onClick={() => openResetPassword(record)}
            disabled={!canWrite}
          >
            重置密码
          </Button>
          <Button icon={<TeamOutlined />} onClick={() => openBindings(record)} disabled={!canWrite}>
            项目角色绑定
          </Button>
          <Popconfirm
            title="确认禁用该用户？"
            onConfirm={() => void disableUser(record)}
            okButtonProps={{ loading: submitting }}
            disabled={!canWrite || record.status === 'disabled'}
          >
            <Button
              danger
              icon={<StopOutlined />}
              disabled={!canWrite || record.status === 'disabled'}
            >
              禁用
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  return (
    <>
      {messageContext}
      <Card
        title="Users"
        extra={
          <Space>
            <Button icon={<ReloadOutlined />} onClick={() => void fetchUsers()}>
              刷新
            </Button>
            <Button
              type="primary"
              icon={<PlusOutlined />}
              onClick={openCreate}
              disabled={!canWrite}
            >
              新建用户
            </Button>
          </Space>
        }
      >
        {!canWrite ? (
          <Alert
            type="warning"
            showIcon
            style={{ marginBottom: 12 }}
            message="当前账号为只读权限，用户写操作入口已禁用。"
          />
        ) : null}
        {error ? (
          <Alert type="error" showIcon message={error} />
        ) : (
          <Table<User> rowKey="id" loading={loading} columns={columns} dataSource={users} />
        )}
      </Card>

      <Modal
        title="新建用户"
        open={createVisible}
        onCancel={() => setCreateVisible(false)}
        onOk={() => void submitCreate()}
        okButtonProps={{ loading: submitting, disabled: !canWrite }}
        destroyOnHidden
      >
        <Form<CreateUserFormValues> form={createForm} layout="vertical">
          <Form.Item name="username" label="用户名" rules={[{ required: true, message: '请输入用户名' }]}>
            <Input />
          </Form.Item>
          <Form.Item
            name="email"
            label="邮箱"
            rules={[
              { required: true, message: '请输入邮箱' },
              { type: 'email', message: '邮箱格式不正确' },
            ]}
          >
            <Input />
          </Form.Item>
          <Form.Item
            name="password"
            label="初始密码"
            rules={[
              { required: true, message: '请输入初始密码' },
              { min: 8, message: '密码长度至少 8 位' },
            ]}
          >
            <Input.Password />
          </Form.Item>
          <Form.Item name="status" label="状态" rules={[{ required: true }]}>
            <Select options={statusOptions} />
          </Form.Item>
          <Form.Item name="platformRole" label="平台角色" rules={[{ required: true }]}>
            <Select options={platformRoleOptions} />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title={`编辑用户${activeUser ? ` - ${activeUser.username}` : ''}`}
        open={editVisible}
        onCancel={() => setEditVisible(false)}
        onOk={() => void submitEdit()}
        okButtonProps={{ loading: submitting, disabled: !canWrite }}
        destroyOnHidden
      >
        <Form<EditUserFormValues> form={editForm} layout="vertical">
          <Form.Item name="username" label="用户名" rules={[{ required: true, message: '请输入用户名' }]}>
            <Input />
          </Form.Item>
          <Form.Item
            name="email"
            label="邮箱"
            rules={[
              { required: true, message: '请输入邮箱' },
              { type: 'email', message: '邮箱格式不正确' },
            ]}
          >
            <Input />
          </Form.Item>
          <Form.Item name="status" label="状态" rules={[{ required: true }]}>
            <Select options={statusOptions} />
          </Form.Item>
          <Form.Item name="platformRole" label="平台角色" rules={[{ required: true }]}>
            <Select options={platformRoleOptions} />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title={`项目角色绑定${activeUser ? ` - ${activeUser.username}` : ''}`}
        open={bindingVisible}
        onCancel={() => setBindingVisible(false)}
        onOk={() => void submitBindings()}
        okButtonProps={{ loading: submitting, disabled: !canWrite }}
        destroyOnHidden
      >
        <Typography.Paragraph type="secondary">
          每次提交会替换该用户当前全部项目绑定关系。
        </Typography.Paragraph>
        <Form<BindingFormValues> form={bindingForm} layout="vertical">
          <Form.List name="bindings">
            {(fields, { add, remove }) => (
              <Space direction="vertical" style={{ width: '100%' }} size={12}>
                {fields.map((field, index) => (
                  <Card
                    key={field.key}
                    size="small"
                    title={`绑定 #${index + 1}`}
                    extra={
                      <Button
                        danger
                        type="link"
                        onClick={() => remove(field.name)}
                        disabled={!canWrite}
                      >
                        删除
                      </Button>
                    }
                  >
                    <Form.Item
                      name={[field.name, 'projectId']}
                      label="项目"
                      rules={[{ required: true, message: '请选择项目' }]}
                    >
                      <Select
                        showSearch
                        optionFilterProp="label"
                        options={projects.map((project) => ({
                          label: `${project.name} (#${project.id})`,
                          value: project.id,
                        }))}
                      />
                    </Form.Item>
                    <Form.Item
                      name={[field.name, 'projectRole']}
                      label="项目角色"
                      rules={[{ required: true, message: '请选择项目角色' }]}
                    >
                      <Select options={projectRoleOptions} />
                    </Form.Item>
                  </Card>
                ))}
                <Button type="dashed" onClick={() => add()} disabled={!canWrite}>
                  添加项目角色绑定
                </Button>
              </Space>
            )}
          </Form.List>
        </Form>
      </Modal>

      <Modal
        title={`重置密码${activeUser ? ` - ${activeUser.username}` : ''}`}
        open={resetPasswordVisible}
        onCancel={() => setResetPasswordVisible(false)}
        onOk={() => void submitResetPassword()}
        okButtonProps={{ loading: submitting, disabled: !canWrite }}
        destroyOnHidden
      >
        <Typography.Paragraph type="secondary">
          新密码长度至少 8 位。
        </Typography.Paragraph>
        <Form<ResetPasswordFormValues> form={resetPasswordForm} layout="vertical">
          <Form.Item
            name="newPassword"
            label="新密码"
            rules={[
              { required: true, message: '请输入新密码' },
              { min: 8, message: '密码长度至少 8 位' },
            ]}
          >
            <Input.Password />
          </Form.Item>
          <Form.Item
            name="confirmPassword"
            label="确认新密码"
            dependencies={['newPassword']}
            rules={[
              { required: true, message: '请再次输入新密码' },
              ({ getFieldValue }) => ({
                validator(_, value) {
                  if (!value || getFieldValue('newPassword') === value) {
                    return Promise.resolve()
                  }
                  return Promise.reject(new Error('两次输入的密码不一致'))
                },
              }),
            ]}
          >
            <Input.Password />
          </Form.Item>
        </Form>
      </Modal>
    </>
  )
}
