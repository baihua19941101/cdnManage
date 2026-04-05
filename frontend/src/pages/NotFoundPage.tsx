import { Button, Result } from 'antd'
import { useNavigate } from 'react-router-dom'

export function NotFoundPage() {
  const navigate = useNavigate()

  return (
    <Result
      status="404"
      title="Route not found"
      subTitle="The requested module does not exist in the current frontend shell."
      extra={
        <Button type="primary" onClick={() => navigate('/')}>
          Return home
        </Button>
      }
    />
  )
}
