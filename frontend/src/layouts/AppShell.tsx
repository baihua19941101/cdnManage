import {
  CloudServerOutlined,
  DatabaseOutlined,
  DeploymentUnitOutlined,
  FileSearchOutlined,
  FolderOpenOutlined,
  TeamOutlined,
} from '@ant-design/icons'
import { Button, Layout, Menu, Segmented, Select, Space, Tag, Typography } from 'antd'
import type { ItemType } from 'antd/es/menu/interface'
import { useEffect, useMemo, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { Outlet, useLocation, useNavigate } from 'react-router-dom'

import { themePresets, type ThemeMode } from '../app/themes'
import { useShellStore, type Locale } from '../store/shell'
import { useAuthStore, type PlatformRole } from '../store/auth'

const { Content, Header, Sider } = Layout

type ShellNavigationItem = {
  key: string
  icon: ReactNode
  labelKey: string
  allowedRoles: PlatformRole[]
}

const roleLabelKeys: Record<PlatformRole, string> = {
  super_admin: 'shell.role.super_admin',
  platform_admin: 'shell.role.platform_admin',
  standard_user: 'shell.role.standard_user',
}

const navigationItems: ShellNavigationItem[] = [
  {
    key: '/overview',
    icon: <DeploymentUnitOutlined />,
    labelKey: 'shell.nav.overview',
    allowedRoles: ['super_admin', 'platform_admin', 'standard_user'],
  },
  {
    key: '/projects',
    icon: <FolderOpenOutlined />,
    labelKey: 'shell.nav.projects',
    allowedRoles: ['super_admin', 'platform_admin'],
  },
  {
    key: '/users',
    icon: <TeamOutlined />,
    labelKey: 'shell.nav.users',
    allowedRoles: ['super_admin', 'platform_admin'],
  },
  {
    key: '/storage',
    icon: <DatabaseOutlined />,
    labelKey: 'shell.nav.storage',
    allowedRoles: ['super_admin', 'platform_admin', 'standard_user'],
  },
  {
    key: '/cdn',
    icon: <CloudServerOutlined />,
    labelKey: 'shell.nav.cdn',
    allowedRoles: ['super_admin', 'platform_admin', 'standard_user'],
  },
  {
    key: '/audits',
    icon: <FileSearchOutlined />,
    labelKey: 'shell.nav.audits',
    allowedRoles: ['super_admin', 'platform_admin', 'standard_user'],
  },
]

export function AppShell() {
  const { t, i18n } = useTranslation()
  const location = useLocation()
  const navigate = useNavigate()
  const collapsed = useShellStore((state) => state.collapsed)
  const themeMode = useShellStore((state) => state.themeMode)
  const language = useShellStore((state) => state.language)
  const setCollapsed = useShellStore((state) => state.setCollapsed)
  const setThemeMode = useShellStore((state) => state.setThemeMode)
  const setLanguage = useShellStore((state) => state.setLanguage)
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
        label: t(item.labelKey),
      })),
    [accessibleNavigationItems, t],
  )
  const selectedMenuKey =
    location.pathname === '/' ? '/overview' : `/${location.pathname.split('/')[1]}`
  const currentSection = accessibleNavigationItems.find((item) => item.key === selectedMenuKey)
  const currentSectionLabel = currentSection ? t(currentSection.labelKey) : t('shell.fallbackSection')
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

  useEffect(() => {
    if (i18n.language === language) {
      return
    }
    void i18n.changeLanguage(language)
  }, [i18n, language])

  const handleLogout = () => {
    clearSession()
    navigate('/login', { replace: true })
  }

  const handleLanguageChange = (value: Locale) => {
    setLanguage(value)
    void i18n.changeLanguage(value)
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
            {t('shell.commandCenter')}
          </Typography.Text>
          {!collapsed ? (
            <Typography.Title
              level={4}
              className="app-shell-brand"
              style={{
                margin: '8px 0 0',
              }}
            >
              {t('shell.brand')}
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
                  ([value]) => ({
                    label: t(`shell.theme.${value}`),
                    value,
                  }),
                )
              }
              onChange={(value) => setThemeMode(value)}
            />
            <Select<Locale>
              className="app-shell-language-select"
              value={language}
              style={{ width: 108 }}
              options={[
                { label: t('shell.language.zhCN'), value: 'zh-CN' },
                { label: t('shell.language.enUS'), value: 'en-US' },
              ]}
              onChange={(value) => handleLanguageChange(value)}
            />
            <Space size="small">
              <Typography.Text style={{ color: 'var(--nt-text-primary)' }}>{userEmail}</Typography.Text>
              <Tag className="nt-role-tag">{t(roleLabelKeys[platformRole])}</Tag>
            </Space>
            <Button onClick={handleLogout}>{t('shell.signOut')}</Button>
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
