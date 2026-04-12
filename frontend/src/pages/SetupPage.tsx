import { Button, Card, Space, Typography } from 'antd'
import { useNavigate } from 'react-router-dom'

export function SetupPage() {
  const navigate = useNavigate()

  return (
    <div className="auth-scene">
      <Card className="auth-card">
        <Space direction="vertical" size={16} style={{ width: '100%' }}>
          <Typography.Text className="auth-label">
            INITIAL SETUP
          </Typography.Text>
          <Typography.Title level={2} className="auth-title">
            首次启动前需要完成系统初始化
          </Typography.Title>
          <Typography.Paragraph className="auth-paragraph">
            当前环境检测到平台仍处于首次启动状态。请先完成管理员账号、基础配置与鉴权参数初始化，再进行登录。
          </Typography.Paragraph>
          <Typography.Paragraph className="auth-paragraph">
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
