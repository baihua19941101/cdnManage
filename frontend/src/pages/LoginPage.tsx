import { Button, Card, Form, Input, Space, Typography } from 'antd'

export function LoginPage() {
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
      <Card style={{ width: 'min(440px, 100%)', borderRadius: 24 }}>
        <Space direction="vertical" size={16} style={{ width: '100%' }}>
          <Typography.Text style={{ color: '#9c5b1f', letterSpacing: 2 }}>
            AUTH ENTRY
          </Typography.Text>
          <Typography.Title
            level={2}
            style={{
              margin: 0,
              fontFamily: '"Iowan Old Style", "Palatino Linotype", serif',
              color: '#2d2118',
            }}
          >
            Sign in to the control deck
          </Typography.Title>
          <Form layout="vertical">
            <Form.Item label="Email">
              <Input placeholder="admin@example.com" />
            </Form.Item>
            <Form.Item label="Password">
              <Input.Password placeholder="••••••••" />
            </Form.Item>
            <Button type="primary" block>
              Continue
            </Button>
          </Form>
        </Space>
      </Card>
    </div>
  )
}
