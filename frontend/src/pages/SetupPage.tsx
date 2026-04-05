import { Card, Result } from 'antd'

export function SetupPage() {
  return (
    <Card>
      <Result
        status="warning"
        title="Initial setup is pending"
        subTitle="This placeholder route is reserved for the first-run initialization flow."
      />
    </Card>
  )
}
