import { useState } from 'react';
import { Card, Table, Tag, Input, Button, Space, message } from 'antd';
import { SaveOutlined, SearchOutlined } from '@ant-design/icons';
import client from '../../api/client';
import { EmptyState } from '../../components/Common';

interface Memory {
  id: string;
  session_id?: string;
  agent_id?: string;
  type: string;
  content: string;
  importance: number;
  created_at: string;
}

export default function MemoryPage() {
  const [memories, setMemories] = useState<Memory[]>([]);
  const [searchQuery, setSearchQuery] = useState('');
  const [newMemory, setNewMemory] = useState('');

  const handleSaveMemory = async () => {
    if (!newMemory.trim()) return;
    try {
      await client.post('/api/v2/memory', {
        content: newMemory,
        user_id: 'default',
      });
      message.success('记忆保存成功');
      setNewMemory('');
    } catch (error) {
      message.error('保存失败');
    }
  };

  const handleRecall = async () => {
    if (!searchQuery.trim()) return;
    try {
      const res = await client.post('/api/v2/memory/recall', {
        query: searchQuery,
        user_id: 'default',
      });
      setMemories(res.data?.memories || []);
    } catch (error) {
      message.error('召回失败');
    }
  };

  const columns = [
    {
      title: '类型',
      dataIndex: 'type',
      key: 'type',
      render: (type: string) => {
        const colors: Record<string, string> = {
          important: 'gold',
          summary: 'blue',
          fact: 'green',
        };
        return <Tag color={colors[type] || 'default'}>{type}</Tag>;
      },
    },
    {
      title: '内容',
      dataIndex: 'content',
      key: 'content',
      ellipsis: true,
    },
    {
      title: '重要性',
      dataIndex: 'importance',
      key: 'importance',
      render: (val: number) => `${(val * 100).toFixed(0)}%`,
    },
    {
      title: '创建时间',
      dataIndex: 'created_at',
      key: 'created_at',
    },
  ];

  return (
    <div>
      <h2 style={{ marginBottom: 24 }}>记忆管理</h2>

      <Card title="保存记忆" style={{ marginBottom: 24 }}>
        <Space.Compact style={{ width: '100%' }}>
          <Input
            placeholder="输入要保存的记忆内容"
            value={newMemory}
            onChange={(e) => setNewMemory(e.target.value)}
          />
          <Button type="primary" icon={<SaveOutlined />} onClick={handleSaveMemory}>
            保存
          </Button>
        </Space.Compact>
      </Card>

      <Card title="召回记忆" style={{ marginBottom: 24 }}>
        <Space.Compact style={{ width: '100%', marginBottom: 16 }}>
          <Input
            placeholder="输入关键词召回相关记忆"
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            onPressEnter={handleRecall}
          />
          <Button type="primary" icon={<SearchOutlined />} onClick={handleRecall}>
            召回
          </Button>
        </Space.Compact>
      </Card>

      <Card title="记忆列表">
        {memories.length > 0 ? (
          <Table
            columns={columns}
            dataSource={memories}
            rowKey="id"
            pagination={{ pageSize: 10 }}
          />
        ) : (
          <EmptyState description="暂无记忆数据" />
        )}
      </Card>
    </div>
  );
}