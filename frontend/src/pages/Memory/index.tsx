import { useState, useEffect } from 'react';
import {
  Card,
  Table,
  Tag,
  Input,
  Button,
  Space,
  message,
  Popconfirm,
  Tabs,
  Statistic,
  Row,
  Col,
  Empty,
  Descriptions,
  Drawer,
} from 'antd';
import {
  SaveOutlined,
  SearchOutlined,
  DeleteOutlined,
  EyeOutlined,
  ReloadOutlined,
} from '@ant-design/icons';
import dayjs from 'dayjs';
import client from '../../api/client';
import EpisodicMemoryPanel from './EpisodicMemoryPanel';
import SemanticGraphPanel from './SemanticGraphPanel';
import WorkingMemoryPanel from './WorkingMemoryPanel';
import MemoryManager from './MemoryManager';

interface Memory {
  id: string;
  session_id?: string;
  agent_id?: string;
  type: string;
  content: string;
  importance: number;
  created_at: number;
}

export default function MemoryPage() {
  const [memories, setMemories] = useState<Memory[]>([]);
  const [allMemories, setAllMemories] = useState<Memory[]>([]);
  const [searchQuery, setSearchQuery] = useState('');
  const [newMemory, setNewMemory] = useState('');
  const [loading, setLoading] = useState(false);
  const [detailVisible, setDetailVisible] = useState(false);
  const [selectedMemory, setSelectedMemory] = useState<Memory | null>(null);
  const [tenantId, setTenantId] = useState('default');

  // Load all memories
  const loadAllMemories = async () => {
    setLoading(true);
    try {
      const res = await client.get(`/api/v2/memory/all?tenant_id=${tenantId}`) as { memories: Memory[] };
      setAllMemories(res.memories || []);
      setMemories(res.memories || []);
    } catch (error) {
      console.error('Failed to load memories', error);
      message.error('Failed to load memories');
    } finally {
      setLoading(false);
    }
  };

  // Save memory
  const handleSaveMemory = async () => {
    if (!newMemory.trim()) {
      message.warning('Please enter memory content');
      return;
    }
    try {
      await client.post('/api/v2/memory', {
        content: newMemory,
        tenant_id: tenantId,
        type: 'MEMORY_TYPE_FACT',
        importance: 0.7,
      });
      message.success('Memory saved');
      setNewMemory('');
      loadAllMemories();
    } catch (error) {
      message.error('Save failed');
    }
  };

  // Recall memory
  const handleRecall = async () => {
    if (!searchQuery.trim()) {
      message.warning('Please enter search keywords');
      return;
    }
    setLoading(true);
    try {
      const res = await client.post('/api/v2/memory/recall', {
        query: searchQuery,
        tenant_id: tenantId,
        top_k: 20,
      }) as { memories: Memory[] };
      setMemories(res.memories || []);
    } catch (error) {
      message.error('Recall failed');
    } finally {
      setLoading(false);
    }
  };

  // Delete memory
  const handleDelete = async (id: string) => {
    try {
      await client.delete(`/api/v2/memory/${id}?tenant_id=${tenantId}`);
      message.success('Deleted');
      loadAllMemories();
    } catch (error) {
      message.error('Delete failed');
    }
  };

  // View detail
  const viewDetail = (memory: Memory) => {
    setSelectedMemory(memory);
    setDetailVisible(true);
  };

  // Clear search
  const clearSearch = () => {
    setSearchQuery('');
    setMemories(allMemories);
  };

  // Initial load
  useEffect(() => {
    loadAllMemories();
  }, [tenantId]);

  // Statistics
  const stats = {
    total: allMemories.length,
    facts: allMemories.filter(m => m.type === 'MEMORY_TYPE_FACT').length,
    summaries: allMemories.filter(m => m.type === 'MEMORY_TYPE_SUMMARY').length,
    important: allMemories.filter(m => m.type === 'MEMORY_TYPE_IMPORTANT').length,
    avgImportance: allMemories.length > 0
      ? (allMemories.reduce((sum, m) => sum + m.importance, 0) / allMemories.length).toFixed(2)
      : '0.00',
  };

  const columns = [
    {
      title: 'Type',
      dataIndex: 'type',
      key: 'type',
      width: 120,
      render: (type: string) => {
        const typeMap: Record<string, { color: string; text: string }> = {
          'MEMORY_TYPE_FACT': { color: 'green', text: 'Fact' },
          'MEMORY_TYPE_SUMMARY': { color: 'blue', text: 'Summary' },
          'MEMORY_TYPE_IMPORTANT': { color: 'orange', text: 'Important' },
          'MEMORY_TYPE_EPISODE': { color: 'purple', text: 'Episode' },
        };
        const item = typeMap[type] || { color: 'default', text: type };
        return <Tag color={item.color}>{item.text}</Tag>;
      },
    },
    {
      title: 'Content',
      dataIndex: 'content',
      key: 'content',
      ellipsis: true,
      render: (content: string) => (
        <span style={{ maxWidth: 400, display: 'inline-block' }}>
          {content.length > 80 ? `${content.substring(0, 80)}...` : content}
        </span>
      ),
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
      title: 'Created',
      dataIndex: 'created_at',
      key: 'created_at',
      width: 160,
      render: (timestamp: number) => dayjs.unix(timestamp).format('YYYY-MM-DD HH:mm'),
    },
    {
      title: 'Actions',
      key: 'action',
      width: 140,
      render: (_: unknown, record: Memory) => (
        <Space>
          <Button
            size="small"
            icon={<EyeOutlined />}
            onClick={() => viewDetail(record)}
          >
            View
          </Button>
          <Popconfirm
            title="Delete this memory?"
            onConfirm={() => handleDelete(record.id)}
            okText="Delete"
            cancelText="Cancel"
          >
            <Button size="small" danger icon={<DeleteOutlined />}>
              Delete
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  const tabItems = [
    {
      key: 'basic',
      label: `Basic Memory (${allMemories.length})`,
      children: (
        <div>
          {/* Stats cards */}
          <Row gutter={16} style={{ marginBottom: 24 }}>
            <Col span={6}>
              <Card>
                <Statistic title="Total Memories" value={stats.total} />
              </Card>
            </Col>
            <Col span={6}>
              <Card>
                <Statistic title="Facts" value={stats.facts} />
              </Card>
            </Col>
            <Col span={6}>
              <Card>
                <Statistic title="Summaries" value={stats.summaries} />
              </Card>
            </Col>
            <Col span={6}>
              <Card>
                <Statistic title="Avg Importance" value={stats.avgImportance} suffix="%" />
              </Card>
            </Col>
          </Row>

          {/* Actions */}
          <Card title="Memory Actions" style={{ marginBottom: 24 }}>
            <Space direction="vertical" style={{ width: '100%' }} size="middle">
              <Space>
                <span>User ID:</span>
                <Input
                  style={{ width: 200 }}
                  value={tenantId}
                  onChange={(e) => setTenantId(e.target.value)}
                  placeholder="Enter user ID"
                />
              </Space>

              <Space.Compact style={{ width: '100%' }}>
                <Input
                  placeholder="Enter memory content to save"
                  value={newMemory}
                  onChange={(e) => setNewMemory(e.target.value)}
                  onPressEnter={handleSaveMemory}
                  style={{ width: 'calc(100% - 100px)' }}
                />
                <Button type="primary" icon={<SaveOutlined />} onClick={handleSaveMemory}>
                  Save
                </Button>
              </Space.Compact>

              <Space.Compact style={{ width: '100%' }}>
                <Input
                  placeholder="Enter keywords to search (semantic search)"
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  onPressEnter={handleRecall}
                  style={{ width: 'calc(100% - 180px)' }}
                />
                <Button type="primary" icon={<SearchOutlined />} onClick={handleRecall} loading={loading}>
                  Search
                </Button>
                <Button icon={<ReloadOutlined />} onClick={clearSearch}>
                  Reset
                </Button>
              </Space.Compact>

              <Space>
                <Button icon={<ReloadOutlined />} onClick={loadAllMemories} loading={loading}>
                  Refresh
                </Button>
                <Popconfirm
                  title="Clear all memories for this user? This cannot be undone!"
                  onConfirm={async () => {
                    try {
                      await client.delete(`/api/v2/memory/session/clear?tenant_id=${tenantId}`);
                      message.success('Cleared');
                      loadAllMemories();
                    } catch {
                      message.error('Clear failed');
                    }
                  }}
                  okText="Clear"
                  cancelText="Cancel"
                  okButtonProps={{ danger: true }}
                >
                  <Button danger>Clear All</Button>
                </Popconfirm>
              </Space>
            </Space>
          </Card>

          {/* Memory table */}
          <Card>
            <Table
              columns={columns}
              dataSource={memories}
              rowKey="id"
              loading={loading}
              pagination={{ pageSize: 10, showSizeChanger: true }}
              locale={{ emptyText: <Empty description="No memories found" /> }}
            />
          </Card>
        </div>
      ),
    },
    {
      key: 'episodic',
      label: 'Episodic Memory',
      children: <EpisodicMemoryPanel />,
    },
    {
      key: 'semantic',
      label: 'Semantic Graph',
      children: <SemanticGraphPanel />,
    },
    {
      key: 'working',
      label: 'Working Memory',
      children: <WorkingMemoryPanel />,
    },
    {
      key: 'manager',
      label: '记忆管理',
      children: <MemoryManager />,
    },
  ];

  return (
    <div>
      <h2 style={{ marginBottom: 24 }}>Memory Management</h2>

      <Card>
        <Tabs items={tabItems} defaultActiveKey="basic" />
      </Card>

      {/* Detail drawer */}
      <Drawer
        title="Memory Detail"
        placement="right"
        width={500}
        open={detailVisible}
        onClose={() => setDetailVisible(false)}
      >
        {selectedMemory && (
          <Descriptions bordered column={1}>
            <Descriptions.Item label="ID">{selectedMemory.id}</Descriptions.Item>
            <Descriptions.Item label="Type">
              <Tag color={selectedMemory.type === 'MEMORY_TYPE_FACT' ? 'green' : 'blue'}>
                {selectedMemory.type === 'MEMORY_TYPE_FACT' ? 'Fact' : 'Summary'}
              </Tag>
            </Descriptions.Item>
            <Descriptions.Item label="Content">
              <div style={{ whiteSpace: 'pre-wrap' }}>{selectedMemory.content}</div>
            </Descriptions.Item>
            <Descriptions.Item label="Importance">
              {(selectedMemory.importance * 100).toFixed(0)}%
            </Descriptions.Item>
            <Descriptions.Item label="Session ID">
              {selectedMemory.session_id || '-'}
            </Descriptions.Item>
            <Descriptions.Item label="Agent ID">
              {selectedMemory.agent_id || '-'}
            </Descriptions.Item>
            <Descriptions.Item label="Created">
              {dayjs.unix(selectedMemory.created_at).format('YYYY-MM-DD HH:mm:ss')}
            </Descriptions.Item>
          </Descriptions>
        )}
      </Drawer>
    </div>
  );
}
