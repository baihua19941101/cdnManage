import {
  CloudServerOutlined,
  DatabaseOutlined,
  DeploymentUnitOutlined,
  FileSearchOutlined,
  FolderOpenOutlined,
  HighlightOutlined,
  TeamOutlined,
} from '@ant-design/icons'
import { Button, Layout, Menu, Segmented, Space, Tag, Typography } from 'antd'
import type { ItemType } from 'antd/es/menu/interface'
import { useEffect, useMemo, type ReactNode } from 'react'
import { Outlet, useLocation, useNavigate } from 'react-router-dom'

import { themePresets, type ThemeMode } from '../app/themes'
import { useShellStore } from '../store/shell'
import { useAuthStore, type PlatformRole } from '../store/auth'

const { Content, Footer, Header, Sider } = Layout

type ShellNavigationItem = {
  key: string
  icon: ReactNode
  label: string
  allowedRoles: PlatformRole[]
}

const roleLabels: Record<PlatformRole, string> = {
  super_admin: 'Super Admin',
  platform_admin: 'Platform Admin',
  standard_user: 'Standard User',
}

const roleTagColors: Record<PlatformRole, string> = {
  super_admin: 'magenta',
  platform_admin: 'geekblue',
  standard_user: 'cyan',
}

const navigationItems: ShellNavigationItem[] = [
  {
    key: '/overview',
    icon: <DeploymentUnitOutlined />,
    label: 'Overview',
    allowedRoles: ['super_admin', 'platform_admin', 'standard_user'],
  },
  {
    key: '/projects',
    icon: <FolderOpenOutlined />,
    label: 'Projects',
    allowedRoles: ['super_admin', 'platform_admin'],
  },
  {
    key: '/users',
    icon: <TeamOutlined />,
    label: 'Users',
    allowedRoles: ['super_admin', 'platform_admin'],
  },
  {
    key: '/storage',
    icon: <DatabaseOutlined />,
    label: 'Storage',
    allowedRoles: ['super_admin', 'platform_admin', 'standard_user'],
  },
  {
    key: '/cdn',
    icon: <CloudServerOutlined />,
    label: 'CDN',
    allowedRoles: ['super_admin', 'platform_admin', 'standard_user'],
  },
  {
    key: '/audits',
    icon: <FileSearchOutlined />,
    label: 'Audits',
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
    location.pathname === '/' ? '/' : `/${location.pathname.split('/')[1]}`
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

  const shellBackdrop =
    themeMode === 'light'
      ? 'rgba(236, 245, 247, 0.9)'
      : themeMode === 'auth'
        ? 'rgba(244, 235, 220, 0.88)'
        : 'rgba(5, 17, 24, 0.88)'

  return (
    <Layout style={{ minHeight: '100vh', background: 'transparent' }}>
      <Sider
        breakpoint="lg"
        collapsible
        collapsed={collapsed}
        onCollapse={setCollapsed}
        width={256}
        style={{
          background: shellBackdrop,
          backdropFilter: 'blur(12px)',
          borderRight: '1px solid rgba(125, 176, 196, 0.16)',
        }}
      >
        <div style={{ padding: 20 }}>
          <Typography.Text
            style={{ display: 'block', fontSize: 12, letterSpacing: 2, color: '#6fc9be' }}
          >
            CDN MANAGE
          </Typography.Text>
          {!collapsed ? (
            <Typography.Title
              level={4}
              style={{
                margin: '8px 0 0',
                color: '#f4fbfd',
                fontFamily: '"Iowan Old Style", "Palatino Linotype", serif',
              }}
            >
              Control Deck
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

      <Layout style={{ background: 'transparent' }}>
        <Header
          style={{
            background: 'transparent',
            borderBottom: '1px solid rgba(125, 176, 196, 0.16)',
            padding: '0 24px',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
          }}
        >
          <Space direction="vertical" size={0}>
            <Typography.Text style={{ color: '#7db0c4', letterSpacing: 1.5, fontSize: 12 }}>
              DESKTOP CONTROL SURFACE
            </Typography.Text>
            <Typography.Title
              level={3}
              style={{
                margin: 0,
                color: '#f4fbfd',
                fontFamily: '"Iowan Old Style", "Palatino Linotype", serif',
              }}
            >
              Frontend Bootstrap
            </Typography.Title>
          </Space>
          <Space size="middle">
            <Space size="small">
              <HighlightOutlined style={{ color: '#7db0c4' }} />
              <Segmented<ThemeMode>
                size="middle"
                value={themeMode}
                options={(Object.entries(themePresets) as [ThemeMode, (typeof themePresets)[ThemeMode]][]).map(([value, preset]) => ({
                  label: preset.label,
                  value,
                }))}
                onChange={(value) => setThemeMode(value)}
              />
            </Space>
            <Space size="small">
              <Typography.Text style={{ color: '#d7edf5' }}>{userEmail}</Typography.Text>
              <Tag color={roleTagColors[platformRole]}>{roleLabels[platformRole]}</Tag>
            </Space>
            <Button onClick={handleLogout}>退出登录</Button>
            <Tag color="cyan">React</Tag>
            <Tag color="geekblue">TypeScript</Tag>
            <Tag color="gold">Ant Design</Tag>
          </Space>
        </Header>

        <Content style={{ padding: 24 }}>
          <Outlet />
        </Content>

        <Footer style={{ background: 'transparent', color: '#7db0c4' }}>
          CDN Manage platform shell scaffold for desktop web operations.
        </Footer>
      </Layout>
    </Layout>
  )
}
