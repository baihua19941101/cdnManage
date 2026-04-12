import { Button, Result } from 'antd'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router-dom'

export function NotFoundPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()

  return (
    <Result
      status="404"
      title={t('system.notFound.title')}
      subTitle={t('system.notFound.subtitle')}
      extra={
        <Button type="primary" onClick={() => navigate('/')}>
          {t('system.notFound.backHome')}
        </Button>
      }
    />
  )
}
