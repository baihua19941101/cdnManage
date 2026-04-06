import { Button, Card, Space, Typography } from 'antd'
import { useNavigate } from 'react-router-dom'

export function SetupPage() {
  const navigate = useNavigate()

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
      <Card style={{ width: 'min(560px, 100%)', borderRadius: 24 }}>
        <Space direction="vertical" size={16} style={{ width: '100%' }}>
          <Typography.Text style={{ color: '#9c5b1f', letterSpacing: 2 }}>
            INITIAL SETUP
          </Typography.Text>
          <Typography.Title
            level={2}
            style={{
              margin: 0,
              fontFamily: '"Iowan Old Style", "Palatino Linotype", serif',
              color: '#2d2118',
            }}
          >
            首次启动前需要完成系统初始化
          </Typography.Title>
          <Typography.Paragraph style={{ marginBottom: 0 }}>
            当前环境检测到平台仍处于首次启动状态。请先完成管理员账号、基础配置与鉴权参数初始化，再进行登录。
          </Typography.Paragraph>
          <Typography.Paragraph style={{ marginBottom: 0 }}>
            初始化完成后，登录接口将返回标准会话信息（accessToken 与 user），前端会按现有 Bearer Token 契约建立登录态。
          </Typography.Paragraph>
          <Button type="primary" onClick={() => navigate('/login')}>
            返回登录页
          </Button>
        </Space>
      </Card>
    </div>
  )
}
