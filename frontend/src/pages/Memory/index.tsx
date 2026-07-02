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

  // 加载所有记忆
  const loadAllMemories = async () => {
    setLoading(true);
    try {
      const res = await client.get(`/api/v2/memory/all?tenant_id=${tenantId}`) as { memories: Memory[] };
      setAllMemories(res.memories || []);
      setMemories(res.memories || []);
    } catch (error) {
      console.error('加载记忆失败', error);
      message.error('加载记忆失败');
    } finally {
      setLoading(false);
    }
  };

  // 保存记忆
  const handleSaveMemory = async () => {
    if (!newMemory.trim()) {
      message.warning('请输入记忆内容');
      return;
    }
    try {
      await client.post('/api/v2/memory', {
        content: newMemory,
        tenant_id: tenantId,
        type: 'MEMORY_TYPE_FACT',
        importance: 0.7,
      });
      message.success('记忆保存成功');
      setNewMemory('');
      loadAllMemories();
    } catch (error) {
      message.error('保存失败');
    }
  };

  // 召回记忆
  const handleRecall = async () => {
    if (!searchQuery.trim()) {
      message.warning('请输入搜索关键词');
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
      message.error('召回失败');
    } finally {
      setLoading(false);
    }
  };

  // 删除记忆
  const handleDelete = async (id: string) => {
    try {
      await client.delete(`/api/v2/memory/${id}?tenant_id=${tenantId}`);
      message.success('删除成功');
      loadAllMemories();
    } catch (error) {
      message.error('删除失败');
    }
  };

  // 查看详情
  const viewDetail = (memory: Memory) => {
    setSelectedMemory(memory);
    setDetailVisible(true);
  };

  // 清空搜索
  const clearSearch = () => {
    setSearchQuery('');
    setMemories(allMemories);
  };

  // 初始加载
  useEffect(() => {
    loadAllMemories();
  }, [tenantId]);

  // 统计
  const stats = {
    total: allMemories.length,
    facts: allMemories.filter(m => m.type === 'MEMORY_TYPE_FACT').length,
    summaries: allMemories.filter(m => m.type === 'MEMORY_TYPE_SUMMARY').length,
    avgImportance: allMemories.length > 0
      ? (allMemories.reduce((sum, m) => sum + m.importance, 0) / allMemories.length).toFixed(2)
      : '0.00',
  };

  const columns = [
    {
      title: '类型',
      dataIndex: 'type',
      key: 'type',
      width: 120,
      render: (type: string) => {
        const typeMap: Record<string, { color: string; text: string }> = {
          'MEMORY_TYPE_FACT': { color: 'green', text: '事实' },
          'MEMORY_TYPE_SUMMARY': { color: 'blue', text: '摘要' },
          'MEMORY_TYPE_EPISODE': { color: 'orange', text: '事件' },
        };
        const item = typeMap[type] || { color: 'default', text: type };
        return <Tag color={item.color}>{item.text}</Tag>;
      },
    },
    {
      title: '内容',
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
      title: '重要性',
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
      title: '创建时间',
      dataIndex: 'created_at',
      key: 'created_at',
      width: 160,
      render: (timestamp: number) => dayjs.unix(timestamp).format('YYYY-MM-DD HH:mm'),
    },
    {
      title: '操作',
      key: 'action',
      width: 140,
      render: (_: unknown, record: Memory) => (
        <Space>
          <Button
            size="small"
            icon={<EyeOutlined />}
            onClick={() => viewDetail(record)}
          >
            查看
          </Button>
          <Popconfirm
            title="确定删除这条记忆吗？"
            onConfirm={() => handleDelete(record.id)}
            okText="删除"
            cancelText="取消"
          >
            <Button size="small" danger icon={<DeleteOutlined />}>
              删除
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  const tabItems = [
    {
      key: 'all',
      label: `全部记忆 (${allMemories.length})`,
      children: (
        <Table
          columns={columns}
          dataSource={memories}
          rowKey="id"
          loading={loading}
          pagination={{ pageSize: 10, showSizeChanger: true }}
          locale={{ emptyText: <Empty description="暂无记忆数据" /> }}
        />
      ),
    },
    {
      key: 'facts',
      label: `事实记忆 (${stats.facts})`,
      children: (
        <Table
          columns={columns}
          dataSource={allMemories.filter(m => m.type === 'MEMORY_TYPE_FACT')}
          rowKey="id"
          loading={loading}
          pagination={{ pageSize: 10 }}
          locale={{ emptyText: <Empty description="暂无事实记忆" /> }}
        />
      ),
    },
    {
      key: 'summaries',
      label: `摘要记忆 (${stats.summaries})`,
      children: (
        <Table
          columns={columns}
          dataSource={allMemories.filter(m => m.type === 'MEMORY_TYPE_SUMMARY')}
          rowKey="id"
          loading={loading}
          pagination={{ pageSize: 10 }}
          locale={{ emptyText: <Empty description="暂无摘要记忆" /> }}
        />
      ),
    },
  ];

  return (
    <div>
      <h2 style={{ marginBottom: 24 }}>记忆管理</h2>

      {/* 统计卡片 */}
      <Row gutter={16} style={{ marginBottom: 24 }}>
        <Col span={6}>
          <Card>
            <Statistic title="总记忆数" value={stats.total} />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic title="事实记忆" value={stats.facts} />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic title="摘要记忆" value={stats.summaries} />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic title="平均重要性" value={stats.avgImportance} suffix="%" />
          </Card>
        </Col>
      </Row>

      {/* 操作区 */}
      <Card title="记忆操作" style={{ marginBottom: 24 }}>
        <Space direction="vertical" style={{ width: '100%' }} size="middle">
          {/* 租户选择 */}
          <Space>
            <span>用户ID:</span>
            <Input
              style={{ width: 200 }}
              value={tenantId}
              onChange={(e) => setTenantId(e.target.value)}
              placeholder="输入用户ID"
            />
          </Space>

          {/* 保存记忆 */}
          <Space.Compact style={{ width: '100%' }}>
            <Input
              placeholder="输入要保存的记忆内容（如：用户的名字是张三）"
              value={newMemory}
              onChange={(e) => setNewMemory(e.target.value)}
              onPressEnter={handleSaveMemory}
              style={{ width: 'calc(100% - 100px)' }}
            />
            <Button type="primary" icon={<SaveOutlined />} onClick={handleSaveMemory}>
              保存
            </Button>
          </Space.Compact>

          {/* 搜索记忆 */}
          <Space.Compact style={{ width: '100%' }}>
            <Input
              placeholder="输入关键词搜索记忆（语义搜索）"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              onPressEnter={handleRecall}
              style={{ width: 'calc(100% - 180px)' }}
            />
            <Button type="primary" icon={<SearchOutlined />} onClick={handleRecall} loading={loading}>
              搜索
            </Button>
            <Button icon={<ReloadOutlined />} onClick={clearSearch}>
              重置
            </Button>
          </Space.Compact>

          {/* 快捷操作 */}
          <Space>
            <Button icon={<ReloadOutlined />} onClick={loadAllMemories} loading={loading}>
              刷新记忆
            </Button>
            <Popconfirm
              title="确定清空该用户的所有记忆吗？此操作不可恢复！"
              onConfirm={async () => {
                try {
                  await client.delete(`/api/v2/memory/session/clear?tenant_id=${tenantId}`);
                  message.success('清空成功');
                  loadAllMemories();
                } catch {
                  message.error('清空失败');
                }
              }}
              okText="清空"
              cancelText="取消"
              okButtonProps={{ danger: true }}
            >
              <Button danger>清空所有记忆</Button>
            </Popconfirm>
          </Space>
        </Space>
      </Card>

      {/* 记忆列表 */}
      <Card>
        <Tabs items={tabItems} />
      </Card>

      {/* 详情抽屉 */}
      <Drawer
        title="记忆详情"
        placement="right"
        width={500}
        open={detailVisible}
        onClose={() => setDetailVisible(false)}
      >
        {selectedMemory && (
          <Descriptions bordered column={1}>
            <Descriptions.Item label="ID">{selectedMemory.id}</Descriptions.Item>
            <Descriptions.Item label="类型">
              <Tag color={selectedMemory.type === 'MEMORY_TYPE_FACT' ? 'green' : 'blue'}>
                {selectedMemory.type === 'MEMORY_TYPE_FACT' ? '事实' : '摘要'}
              </Tag>
            </Descriptions.Item>
            <Descriptions.Item label="内容">
              <div style={{ whiteSpace: 'pre-wrap' }}>{selectedMemory.content}</div>
            </Descriptions.Item>
            <Descriptions.Item label="重要性">
              {(selectedMemory.importance * 100).toFixed(0)}%
            </Descriptions.Item>
            <Descriptions.Item label="Session ID">
              {selectedMemory.session_id || '-'}
            </Descriptions.Item>
            <Descriptions.Item label="Agent ID">
              {selectedMemory.agent_id || '-'}
            </Descriptions.Item>
            <Descriptions.Item label="创建时间">
              {dayjs.unix(selectedMemory.created_at).format('YYYY-MM-DD HH:mm:ss')}
            </Descriptions.Item>
          </Descriptions>
        )}
      </Drawer>
    </div>
  );
}
