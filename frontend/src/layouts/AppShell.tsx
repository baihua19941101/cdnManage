import {
  CloudServerOutlined,
  DatabaseOutlined,
  DeploymentUnitOutlined,
  FileSearchOutlined,
  FolderOpenOutlined,
  HighlightOutlined,
  TeamOutlined,
} from '@ant-design/icons'
import { Layout, Menu, Segmented, Space, Tag, Typography } from 'antd'
import type { ItemType } from 'antd/es/menu/interface'
import { Outlet, useLocation, useNavigate } from 'react-router-dom'

import { themePresets, type ThemeMode } from '../app/themes'
import { useShellStore } from '../store/shell'

const { Content, Footer, Header, Sider } = Layout

const menuItems: ItemType[] = [
  { key: '/', icon: <DeploymentUnitOutlined />, label: 'Overview' },
  { key: '/projects', icon: <FolderOpenOutlined />, label: 'Projects' },
  { key: '/users', icon: <TeamOutlined />, label: 'Users' },
  { key: '/storage', icon: <DatabaseOutlined />, label: 'Storage' },
  { key: '/cdn', icon: <CloudServerOutlined />, label: 'CDN' },
  { key: '/audits', icon: <FileSearchOutlined />, label: 'Audits' },
]

export function AppShell() {
  const location = useLocation()
  const navigate = useNavigate()
  const collapsed = useShellStore((state) => state.collapsed)
  const themeMode = useShellStore((state) => state.themeMode)
  const setCollapsed = useShellStore((state) => state.setCollapsed)
  const setThemeMode = useShellStore((state) => state.setThemeMode)

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
          selectedKeys={[location.pathname === '/' ? '/' : `/${location.pathname.split('/')[1]}`]}
          items={menuItems}
          onClick={({ key }) => navigate(key)}
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
