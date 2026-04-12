import { Button, Card, Space, Typography } from 'antd'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router-dom'

export function SetupPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()

  return (
    <div className="auth-scene">
      <Card className="auth-card">
        <Space direction="vertical" size={16} style={{ width: '100%' }}>
          <Typography.Text className="auth-label">
            {t('auth.setup.label')}
          </Typography.Text>
          <Typography.Title level={2} className="auth-title">
            {t('auth.setup.title')}
          </Typography.Title>
          <Typography.Paragraph className="auth-paragraph">
            {t('auth.setup.description1')}
          </Typography.Paragraph>
          <Typography.Paragraph className="auth-paragraph">
            {t('auth.setup.description2')}
          </Typography.Paragraph>
          <Button type="primary" onClick={() => navigate('/login')}>
            {t('auth.setup.backToLogin')}
          </Button>
        </Space>
      </Card>
    </div>
  )
}
