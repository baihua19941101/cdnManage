import { Card, Col, List, Row, Space, Statistic, Tag, Typography } from 'antd'

const signals = [
  'API client wrapper and error normalization',
  'Route map for auth, dashboard and admin domains',
  'Zustand stores for shell and auth session placeholders',
  'Exception pages for unauthorized and missing routes',
]

export function DashboardPage() {
  return (
    <Space direction="vertical" size={24} style={{ width: '100%' }}>
      <Card
        style={{
          background: 'linear-gradient(135deg, rgba(31,73,89,0.72), rgba(10,23,31,0.92))',
          border: '1px solid rgba(92, 175, 196, 0.18)',
        }}
      >
        <Typography.Text style={{ color: '#75d7c2', letterSpacing: 2 }}>
          SYSTEM ENTRY
        </Typography.Text>
        <Typography.Title
          level={2}
          style={{
            color: '#f4fbfd',
            marginTop: 12,
            fontFamily: '"Iowan Old Style", "Palatino Linotype", serif',
          }}
        >
          Frontend foundation is ready for task-driven expansion.
        </Typography.Title>
        <Typography.Paragraph style={{ color: '#9fbfcb', maxWidth: 720 }}>
          This shell provides the router, API abstraction, shared state, navigation frame, and
          fallback pages needed for the upcoming feature pages.
        </Typography.Paragraph>
      </Card>

      <Row gutter={[16, 16]}>
        <Col xs={24} md={8}>
          <Card>
            <Statistic title="Skeleton Modules" value={6} />
          </Card>
        </Col>
        <Col xs={24} md={8}>
          <Card>
            <Statistic title="Primary Routes" value={8} />
          </Card>
        </Col>
        <Col xs={24} md={8}>
          <Card>
            <Statistic title="Theme Track" value="Task 10.2" />
          </Card>
        </Col>
      </Row>

      <Card title="Bootstrap Signals" extra={<Tag color="cyan">MVP</Tag>}>
        <List
          dataSource={signals}
          renderItem={(item) => (
            <List.Item>
              <Typography.Text style={{ color: '#cbe1e9' }}>{item}</Typography.Text>
            </List.Item>
          )}
        />
      </Card>
    </Space>
  )
}
