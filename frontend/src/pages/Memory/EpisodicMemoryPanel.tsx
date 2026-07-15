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
  Timeline,
  Select,
  Form,
  Modal,
} from 'antd';
import {
  SearchOutlined,
  ReloadOutlined,
  PlusOutlined,
  ClockCircleOutlined,
} from '@ant-design/icons';
import dayjs from 'dayjs';
import client from '../../api/client';

interface Episode {
  id: string;
  session_id: string;
  agent_id?: string;
  event_type: string;
  title: string;
  description: string;
  importance: number;
  created_at: number;
}

export default function EpisodicMemoryPanel() {
  const [episodes, setEpisodes] = useState<Episode[]>([]);
  const [loading, setLoading] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');
  const [addModalVisible, setAddModalVisible] = useState(false);
  const [addForm] = Form.useForm();
  const [sessionFilter, setSessionFilter] = useState('');

  const loadEpisodes = async () => {
    setLoading(true);
    try {
      const params = new URLSearchParams();
      if (sessionFilter) params.set('session_id', sessionFilter);
      const res = await client.get(`/api/v2/memory/episodic?${params.toString()}`) as { episodes: Episode[] };
      setEpisodes(res.episodes || []);
    } catch (error) {
      console.error('Failed to load episodes', error);
      message.error('Failed to load episodic memories');
    } finally {
      setLoading(false);
    }
  };

  const searchEpisodes = async () => {
    if (!searchQuery.trim()) {
      loadEpisodes();
      return;
    }
    setLoading(true);
    try {
      const res = await client.post('/api/v2/memory/episodic/similar', {
        query: searchQuery,
        top_k: 20,
      }) as { episodes: Episode[] };
      setEpisodes(res.episodes || []);
    } catch (error) {
      message.error('Search failed');
    } finally {
      setLoading(false);
    }
  };

  const handleAddEpisode = async () => {
    try {
      const values = await addForm.validateFields();
      await client.post('/api/v2/memory/episodic', values);
      message.success('Episode saved');
      setAddModalVisible(false);
      addForm.resetFields();
      loadEpisodes();
    } catch (error) {
      message.error('Failed to save episode');
    }
  };

  useEffect(() => {
    loadEpisodes();
  }, [sessionFilter]);

  const typeColorMap: Record<string, string> = {
    MEMORY_TYPE_IMPORTANT: 'orange',
    MEMORY_TYPE_FACT: 'green',
    MEMORY_TYPE_SUMMARY: 'blue',
    conversation: 'blue',
    action: 'orange',
    decision: 'purple',
    error: 'red',
    success: 'green',
  };

  const stats = {
    total: episodes.length,
    avgImportance: episodes.length > 0
      ? (episodes.reduce((sum, e) => sum + e.importance, 0) / episodes.length).toFixed(2)
      : '0.00',
    highImportance: episodes.filter(e => e.importance >= 0.7).length,
    uniqueSessions: new Set(episodes.map(e => e.session_id)).size,
  };

  const timelineItems = episodes.slice(0, 20).map(episode => ({
    color: episode.importance >= 0.7 ? 'green' : 'blue' as const,
    dot: episode.importance >= 0.8 ? <ClockCircleOutlined style={{ fontSize: '16px' }} /> : undefined,
    children: (
      <div>
        <div style={{ fontWeight: 500 }}>{episode.title || episode.description?.substring(0, 60)}</div>
        <div style={{ fontSize: 12, color: '#888' }}>
          {dayjs.unix(episode.created_at).format('YYYY-MM-DD HH:mm')}
          <Tag color={typeColorMap[episode.event_type] || 'default'} style={{ marginLeft: 8 }}>
            {episode.event_type}
          </Tag>
          <span style={{ marginLeft: 8 }}>{(episode.importance * 100).toFixed(0)}%</span>
        </div>
      </div>
    ),
  }));

  const columns = [
    {
      title: 'Event Type',
      dataIndex: 'event_type',
      key: 'event_type',
      width: 120,
      render: (type: string) => (
        <Tag color={typeColorMap[type] || 'default'}>{type}</Tag>
      ),
    },
    {
      title: 'Title',
      dataIndex: 'title',
      key: 'title',
      ellipsis: true,
      render: (title: string, record: Episode) =>
        title || record.description?.substring(0, 80),
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
      title: 'Session',
      dataIndex: 'session_id',
      key: 'session_id',
      width: 140,
      ellipsis: true,
    },
    {
      title: 'Time',
      dataIndex: 'created_at',
      key: 'created_at',
      width: 160,
      render: (ts: number) => dayjs.unix(ts).format('YYYY-MM-DD HH:mm'),
    },
  ];

  return (
    <div>
      <Row gutter={16} style={{ marginBottom: 16 }}>
        <Col span={6}>
          <Card size="small">
            <Statistic title="Total Episodes" value={stats.total} />
          </Card>
        </Col>
        <Col span={6}>
          <Card size="small">
            <Statistic title="Avg Importance" value={stats.avgImportance} suffix="%" />
          </Card>
        </Col>
        <Col span={6}>
          <Card size="small">
            <Statistic title="High Importance" value={stats.highImportance} />
          </Card>
        </Col>
        <Col span={6}>
          <Card size="small">
            <Statistic title="Sessions" value={stats.uniqueSessions} />
          </Card>
        </Col>
      </Row>

      <Space style={{ marginBottom: 16, width: '100%' }} direction="vertical">
        <Space>
          <Input
            placeholder="Filter by session ID"
            value={sessionFilter}
            onChange={(e) => setSessionFilter(e.target.value)}
            style={{ width: 200 }}
            allowClear
          />
          <Space.Compact>
            <Input
              placeholder="Search episodes..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              onPressEnter={searchEpisodes}
              style={{ width: 300 }}
            />
            <Button type="primary" icon={<SearchOutlined />} onClick={searchEpisodes} loading={loading}>
              Search
            </Button>
          </Space.Compact>
          <Button icon={<ReloadOutlined />} onClick={loadEpisodes} loading={loading}>
            Refresh
          </Button>
          <Button type="primary" icon={<PlusOutlined />} onClick={() => setAddModalVisible(true)}>
            Add Episode
          </Button>
        </Space>
      </Space>

      <Row gutter={16}>
        <Col span={16}>
          <Table
            columns={columns}
            dataSource={episodes}
            rowKey="id"
            loading={loading}
            pagination={{ pageSize: 10 }}
            locale={{ emptyText: <Empty description="No episodes found" /> }}
          />
        </Col>
        <Col span={8}>
          <Card title="Timeline" size="small" style={{ maxHeight: 500, overflow: 'auto' }}>
            {timelineItems.length > 0 ? (
              <Timeline items={timelineItems} />
            ) : (
              <Empty description="No timeline data" />
            )}
          </Card>
        </Col>
      </Row>

      <Modal
        title="Add Episode"
        open={addModalVisible}
        onOk={handleAddEpisode}
        onCancel={() => {
          setAddModalVisible(false);
          addForm.resetFields();
        }}
        okText="Save"
      >
        <Form form={addForm} layout="vertical">
          <Form.Item name="session_id" label="Session ID">
            <Input placeholder="Enter session ID" />
          </Form.Item>
          <Form.Item name="type" label="Episode Type">
            <Select placeholder="Select type">
              <Select.Option value="conversation">Conversation</Select.Option>
              <Select.Option value="action">Action</Select.Option>
              <Select.Option value="decision">Decision</Select.Option>
              <Select.Option value="observation">Observation</Select.Option>
            </Select>
          </Form.Item>
          <Form.Item name="title" label="Title" rules={[{ required: true, message: 'Title is required' }]}>
            <Input placeholder="Episode title" />
          </Form.Item>
          <Form.Item name="description" label="Description">
            <Input.TextArea rows={3} placeholder="Describe what happened" />
          </Form.Item>
          <Form.Item name="importance" label="Importance" initialValue={0.5}>
            <Select>
              <Select.Option value={0.3}>Low (30%)</Select.Option>
              <Select.Option value={0.5}>Medium (50%)</Select.Option>
              <Select.Option value={0.7}>High (70%)</Select.Option>
              <Select.Option value={0.9}>Critical (90%)</Select.Option>
            </Select>
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
