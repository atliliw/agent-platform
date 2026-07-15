import { useState, useEffect } from 'react';
import {
  Card,
  Table,
  Tag,
  Input,
  Button,
  Space,
  message,
  Statistic,
  Row,
  Col,
  Empty,
  Popconfirm,
  Select,
  Progress,
} from 'antd';
import {
  ReloadOutlined,
  DeleteOutlined,
  PlusOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons';
import dayjs from 'dayjs';
import client from '../../api/client';

interface WorkingMessage {
  id: string;
  type: string;
  content: string;
  role: string;
  importance: number;
  is_key: boolean;
  tokens: number;
  created_at: number;
}

interface WorkingContext {
  session_id: string;
  messages: WorkingMessage[];
  total_tokens: number;
  max_tokens: number;
  usage_percent: number;
}

export default function WorkingMemoryPanel() {
  const [sessionId, setSessionId] = useState('');
  const [context, setContext] = useState<WorkingContext | null>(null);
  const [loading, setLoading] = useState(false);
  const [addModalVisible, setAddModalVisible] = useState(false);
  const [newMessage, setNewMessage] = useState('');
  const [newRole, setNewRole] = useState('user');
  const [newImportance, setNewImportance] = useState(0.5);

  const loadContext = async () => {
    if (!sessionId.trim()) {
      message.warning('Please enter a session ID');
      return;
    }
    setLoading(true);
    try {
      const res = await client.get(`/api/v2/memory/working/${sessionId}`) as WorkingContext;
      setContext(res);
    } catch (error) {
      console.error('Failed to load working context', error);
      setContext(null);
      message.error('Failed to load working memory');
    } finally {
      setLoading(false);
    }
  };

  const addMessage = async () => {
    if (!sessionId.trim()) {
      message.warning('Please enter a session ID');
      return;
    }
    if (!newMessage.trim()) {
      message.warning('Please enter message content');
      return;
    }
    try {
      await client.post('/api/v2/memory/working', {
        session_id: sessionId,
        content: newMessage,
        role: newRole,
        importance: newImportance,
        is_key: newImportance >= 0.7,
      });
      message.success('Message added');
      setAddModalVisible(false);
      setNewMessage('');
      loadContext();
    } catch (error) {
      message.error('Failed to add message');
    }
  };

  const clearContext = async () => {
    if (!sessionId.trim()) return;
    try {
      await client.delete(`/api/v2/memory/working/${sessionId}`);
      message.success('Working context cleared');
      setContext(null);
    } catch (error) {
      message.error('Failed to clear context');
    }
  };

  useEffect(() => {
    if (sessionId) {
      loadContext();
    }
  }, []);

  const usagePercent = context ? Math.min(context.usage_percent, 100) : 0;
  const usageColor = usagePercent >= 90 ? '#ff4d4f' : usagePercent >= 70 ? '#faad14' : '#52c41a';

  const columns = [
    {
      title: 'Role',
      dataIndex: 'role',
      key: 'role',
      width: 90,
      render: (role: string) => {
        const colorMap: Record<string, string> = {
          user: 'blue',
          assistant: 'green',
          system: 'orange',
        };
        return <Tag color={colorMap[role] || 'default'}>{role}</Tag>;
      },
    },
    {
      title: 'Content',
      dataIndex: 'content',
      key: 'content',
      ellipsis: true,
      render: (content: string) => (
        <span style={{ maxWidth: 400, display: 'inline-block' }}>
          {content.length > 100 ? `${content.substring(0, 100)}...` : content}
        </span>
      ),
    },
    {
      title: 'Tokens',
      dataIndex: 'tokens',
      key: 'tokens',
      width: 80,
      render: (tokens: number) => tokens || '-',
    },
    {
      title: 'Importance',
      dataIndex: 'importance',
      key: 'importance',
      width: 100,
      render: (val: number) => (
        <Tag color={val >= 0.7 ? 'green' : val >= 0.4 ? 'blue' : 'default'}>
          {(val * 100).toFixed(0)}%
        </Tag>
      ),
    },
    {
      title: 'Key',
      dataIndex: 'is_key',
      key: 'is_key',
      width: 60,
      render: (isKey: boolean) => isKey ? <Tag color="gold">Key</Tag> : '-',
    },
    {
      title: 'Time',
      dataIndex: 'created_at',
      key: 'created_at',
      width: 140,
      render: (ts: number) => ts ? dayjs.unix(ts).format('HH:mm:ss') : '-',
    },
  ];

  return (
    <div>
      <Space style={{ marginBottom: 16, width: '100%' }} direction="vertical">
        <Space>
          <span>Session ID:</span>
          <Input
            style={{ width: 240 }}
            value={sessionId}
            onChange={(e) => setSessionId(e.target.value)}
            placeholder="Enter session ID to view working memory"
            onPressEnter={loadContext}
          />
          <Button type="primary" icon={<ReloadOutlined />} onClick={loadContext} loading={loading}>
            Load
          </Button>
          <Button icon={<PlusOutlined />} onClick={() => setAddModalVisible(true)} disabled={!sessionId}>
            Add Message
          </Button>
          <Popconfirm
            title="Clear all working memory for this session?"
            onConfirm={clearContext}
            okText="Clear"
            cancelText="Cancel"
          >
            <Button danger icon={<DeleteOutlined />} disabled={!sessionId}>
              Clear
            </Button>
          </Popconfirm>
        </Space>
      </Space>

      {context && (
        <Row gutter={16} style={{ marginBottom: 16 }}>
          <Col span={6}>
            <Card size="small">
              <Statistic title="Messages" value={context.messages?.length || 0} />
            </Card>
          </Col>
          <Col span={6}>
            <Card size="small">
              <Statistic title="Tokens Used" value={context.total_tokens} />
            </Card>
          </Col>
          <Col span={6}>
            <Card size="small">
              <Statistic title="Max Tokens" value={context.max_tokens} />
            </Card>
          </Col>
          <Col span={6}>
            <Card size="small">
              <Statistic
                title="Key Messages"
                value={context.messages?.filter(m => m.is_key).length || 0}
              />
            </Card>
          </Col>
        </Row>
      )}

      {context && (
        <Card title="Token Usage" size="small" style={{ marginBottom: 16 }}>
          <Progress
            percent={usagePercent}
            strokeColor={usageColor}
            format={() => `${usagePercent.toFixed(1)}%`}
          />
          <div style={{ marginTop: 8, fontSize: 12, color: '#888' }}>
            {context.total_tokens} / {context.max_tokens} tokens used
            {usagePercent >= 90 && (
              <Tag color="red" style={{ marginLeft: 8 }}>
                <ThunderboltOutlined /> Near limit - compression will trigger
              </Tag>
            )}
          </div>
        </Card>
      )}

      <Table
        columns={columns}
        dataSource={context?.messages || []}
        rowKey="id"
        loading={loading}
        pagination={{ pageSize: 15 }}
        locale={{ emptyText: <Empty description={sessionId ? "No messages in this session" : "Enter a session ID to view working memory"} /> }}
      />

      <Card
        title="Add Message to Working Memory"
        size="small"
        style={{ marginTop: 16, display: addModalVisible ? 'block' : 'none' }}
      >
        <Space direction="vertical" style={{ width: '100%' }}>
          <Space>
            <span>Role:</span>
            <Select value={newRole} onChange={setNewRole} style={{ width: 120 }}>
              <Select.Option value="user">User</Select.Option>
              <Select.Option value="assistant">Assistant</Select.Option>
              <Select.Option value="system">System</Select.Option>
            </Select>
            <span>Importance:</span>
            <Select value={newImportance} onChange={setNewImportance} style={{ width: 120 }}>
              <Select.Option value={0.3}>Low (30%)</Select.Option>
              <Select.Option value={0.5}>Medium (50%)</Select.Option>
              <Select.Option value={0.7}>High (70%)</Select.Option>
              <Select.Option value={0.9}>Critical (90%)</Select.Option>
            </Select>
          </Space>
          <Space.Compact style={{ width: '100%' }}>
            <Input
              placeholder="Enter message content"
              value={newMessage}
              onChange={(e) => setNewMessage(e.target.value)}
              onPressEnter={addMessage}
              style={{ width: 'calc(100% - 80px)' }}
            />
            <Button type="primary" onClick={addMessage}>Add</Button>
          </Space.Compact>
        </Space>
      </Card>
    </div>
  );
}
