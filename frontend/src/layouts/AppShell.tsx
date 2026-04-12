import {
  CloudServerOutlined,
  DatabaseOutlined,
  DeploymentUnitOutlined,
  FileSearchOutlined,
  FolderOpenOutlined,
  TeamOutlined,
} from '@ant-design/icons'
import { Button, Layout, Menu, Segmented, Space, Tag, Typography } from 'antd'
import type { ItemType } from 'antd/es/menu/interface'
import { useEffect, useMemo, type ReactNode } from 'react'
import { Outlet, useLocation, useNavigate } from 'react-router-dom'

import { themePresets, type ThemeMode } from '../app/themes'
import { useShellStore } from '../store/shell'
import { useAuthStore, type PlatformRole } from '../store/auth'

const { Content, Header, Sider } = Layout

type ShellNavigationItem = {
  key: string
  icon: ReactNode
  label: string
  allowedRoles: PlatformRole[]
}

const roleLabels: Record<PlatformRole, string> = {
  super_admin: '超级管理员',
  platform_admin: '平台管理员',
  standard_user: '标准用户',
}

const navigationItems: ShellNavigationItem[] = [
  {
    key: '/overview',
    icon: <DeploymentUnitOutlined />,
    label: '总览',
    allowedRoles: ['super_admin', 'platform_admin', 'standard_user'],
  },
  {
    key: '/projects',
    icon: <FolderOpenOutlined />,
    label: '项目管理',
    allowedRoles: ['super_admin', 'platform_admin'],
  },
  {
    key: '/users',
    icon: <TeamOutlined />,
    label: '用户管理',
    allowedRoles: ['super_admin', 'platform_admin'],
  },
  {
    key: '/storage',
    icon: <DatabaseOutlined />,
    label: '存储管理',
    allowedRoles: ['super_admin', 'platform_admin', 'standard_user'],
  },
  {
    key: '/cdn',
    icon: <CloudServerOutlined />,
    label: 'CDN管理',
    allowedRoles: ['super_admin', 'platform_admin', 'standard_user'],
  },
  {
    key: '/audits',
    icon: <FileSearchOutlined />,
    label: '审计管理',
    allowedRoles: ['super_admin', 'platform_admin', 'standard_user'],
  },
]

export function AppShell() {
  const location = useLocation()
  const navigate = useNavigate()
  const collapsed = useShellStore((state) => state.collapsed)
  const themeMode = useShellStore((state) => state.themeMode)
  const setCollapsed = useShellStore((state) => state.setCollapsed)
  const setThemeMode = useShellStore((state) => state.setThemeMode)
  const user = useAuthStore((state) => state.user)
  const clearSession = useAuthStore((state) => state.clearSession)

  const platformRole: PlatformRole = user?.platformRole ?? 'standard_user'
  const userEmail = user?.email ?? 'unknown@cdnmanage.local'

  const accessibleNavigationItems = useMemo(
    () => navigationItems.filter((item) => item.allowedRoles.includes(platformRole)),
    [platformRole],
  )
  const menuItems = useMemo<ItemType[]>(
    () =>
      accessibleNavigationItems.map((item) => ({
        key: item.key,
        icon: item.icon,
        label: item.label,
      })),
    [accessibleNavigationItems],
  )
  const selectedMenuKey =
    location.pathname === '/' ? '/overview' : `/${location.pathname.split('/')[1]}`
  const currentSectionLabel =
    accessibleNavigationItems.find((item) => item.key === selectedMenuKey)?.label ?? '控制台'
  const fallbackPath = accessibleNavigationItems[0]?.key
  const hasAccessToCurrentPath = accessibleNavigationItems.some(
    (item) => item.key === selectedMenuKey,
  )

  useEffect(() => {
    if (!fallbackPath) {
      return
    }
    if (!hasAccessToCurrentPath) {
      navigate(fallbackPath, { replace: true })
    }
  }, [fallbackPath, hasAccessToCurrentPath, navigate])

  const handleLogout = () => {
    clearSession()
    navigate('/login', { replace: true })
  }

  return (
    <Layout
      style={{
        minHeight: '100vh',
        height: '100vh',
        overflow: 'hidden',
        background: 'transparent',
      }}
      className="app-shell-surface"
    >
      <Sider
        breakpoint="lg"
        collapsible
        collapsed={collapsed}
        onCollapse={setCollapsed}
        width={256}
        className="app-shell-sider"
        style={{
          borderRight: '1px solid var(--nt-shell-border)',
          position: 'sticky',
          top: 0,
          left: 0,
          height: '100vh',
          overflow: 'auto',
        }}
      >
        <div style={{ padding: 20 }}>
          <Typography.Text className="app-shell-mark" style={{ display: 'block', fontSize: 12 }}>
            COMMAND CENTER
          </Typography.Text>
          {!collapsed ? (
            <Typography.Title
              level={4}
              className="app-shell-brand"
              style={{
                margin: '8px 0 0',
              }}
            >
              CDN管理平台
            </Typography.Title>
          ) : null}
        </div>
        <Menu
          mode="inline"
          selectedKeys={[selectedMenuKey]}
          items={menuItems}
          onClick={({ key }) => navigate(String(key))}
          style={{ background: 'transparent', borderInlineEnd: 'none' }}
        />
      </Sider>

      <Layout style={{ background: 'transparent', minWidth: 0, height: '100vh' }}>
        <Header
          className="app-shell-header"
          style={{
            background: 'transparent',
            borderBottom: '1px solid var(--nt-shell-border)',
            padding: '0 24px',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            flex: '0 0 auto',
          }}
        >
          <Typography.Title
            level={3}
            className="app-shell-title"
            style={{
              margin: 0,
            }}
          >
            {currentSectionLabel}
          </Typography.Title>
          <Space size="middle">
            <Segmented<ThemeMode>
              size="middle"
              value={themeMode}
              options={
                (Object.entries(themePresets) as [ThemeMode, (typeof themePresets)[ThemeMode]][]).map(
                  ([value, preset]) => ({
                    label: preset.label,
                    value,
                  }),
                )
              }
              onChange={(value) => setThemeMode(value)}
            />
            <Space size="small">
              <Typography.Text style={{ color: 'var(--nt-text-primary)' }}>{userEmail}</Typography.Text>
              <Tag className="nt-role-tag">{roleLabels[platformRole]}</Tag>
            </Space>
            <Button onClick={handleLogout}>Sign Out</Button>
          </Space>
        </Header>

        <Content
          style={{
            padding: 24,
            overflowY: 'auto',
            overflowX: 'hidden',
            minHeight: 0,
          }}
        >
          <Outlet />
        </Content>
      </Layout>
    </Layout>
  )
}
