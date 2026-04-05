import { Card, Result, Typography } from 'antd'

type ResourcePageProps = {
  title: string
  description: string
}

export function ResourcePage({ title, description }: ResourcePageProps) {
  return (
    <Card>
      <Result
        status="info"
        title={title}
        subTitle={description}
        extra={
          <Typography.Text type="secondary">
            Detailed workflows are scheduled in the next tasks.
          </Typography.Text>
        }
      />
    </Card>
  )
}
