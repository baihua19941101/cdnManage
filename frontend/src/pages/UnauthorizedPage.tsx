import { Button, Result } from 'antd'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router-dom'

export function UnauthorizedPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()

  return (
    <Result
      status="403"
      title={t('system.unauthorized.title')}
      subTitle={t('system.unauthorized.subtitle')}
      extra={
        <Button type="primary" onClick={() => navigate('/')}>
          {t('system.unauthorized.backHome')}
        </Button>
      }
    />
  )
}
