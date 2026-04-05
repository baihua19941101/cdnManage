import { Button, Result } from 'antd'
import { useNavigate } from 'react-router-dom'

export function UnauthorizedPage() {
  const navigate = useNavigate()

  return (
    <Result
      status="403"
      title="Permission denied"
      subTitle="This route is outside the current role scope."
      extra={
        <Button type="primary" onClick={() => navigate('/')}>
          Back to dashboard
        </Button>
      }
    />
  )
}
