import { useState, useEffect, useCallback } from 'react';
import { Card, Table, Tag, Button, Space, Input, Select, message, Popconfirm, Badge, Empty } from 'antd';
import { PlusOutlined, SearchOutlined, EditOutlined, HistoryOutlined, DeleteOutlined, ReloadOutlined } from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';
import { promptApi } from '../../api/prompt';

const categories = ['system', 'user', 'template', 'rag', 'agent'];

export default function PromptListPage() {
  const navigate = useNavigate();
  const [prompts, setPrompts] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [categoryFilter, setCategoryFilter] = useState<string>();
  const [searchText, setSearchText] = useState('');

  const loadPrompts = useCallback(async () => {
    setLoading(true);
    try {
      const res = await promptApi.listPrompts({ category: categoryFilter }) as any;
      setPrompts(res?.prompts || []);
    } catch {
      setPrompts([]);
    } finally { setLoading(false); }
  }, [categoryFilter]);

  useEffect(() => { loadPrompts(); }, [loadPrompts]);

  const deletePrompt = async (key: string) => {
    try {
      await promptApi.deletePrompt(key);
      message.success('Deleted');
      loadPrompts();
    } catch { message.error('Delete failed'); }
  };

  const filtered = prompts.filter(p =>
    !searchText || p.name?.includes(searchText) || p.key?.includes(searchText)
  );

  const columns = [
    { title: 'Key', dataIndex: 'key', key: 'key', render: (k: string) => <code>{k}</code> },
    { title: 'Name', dataIndex: 'name', key: 'name' },
    { title: 'Category', dataIndex: 'category', key: 'category', render: (c: string) => <Tag color="blue">{c}</Tag> },
    { title: 'Status', key: 'status', render: () => <Badge status="success" text="Active" /> },
    { title: 'Created', dataIndex: 'created_at', key: 'created_at', render: (t: number) => t ? new Date(t * 1000).toLocaleDateString() : '-' },
    { title: 'Actions', key: 'actions', render: (_: unknown, record: any) => (
      <Space>
        <Button size="small" icon={<EditOutlined />} onClick={() => navigate(`/prompt/editor/${record.key}`)}>Edit</Button>
        <Button size="small" icon={<HistoryOutlined />} onClick={() => navigate(`/prompt/history/${record.key}`)}>History</Button>
        <Popconfirm title="Delete?" onConfirm={() => deletePrompt(record.key)}><Button size="small" danger icon={<DeleteOutlined />}>Delete</Button></Popconfirm>
      </Space>
    ) },
  ];

  return (
    <div>
      <h2 style={{ marginBottom: 24 }}>Prompt Management</h2>
      <Card extra={
        <Space>
          <Input prefix={<SearchOutlined />} placeholder="Search prompts..." value={searchText} onChange={e => setSearchText(e.target.value)} style={{ width: 200 }} />
          <Select placeholder="Category" allowClear style={{ width: 120 }} value={categoryFilter} onChange={setCategoryFilter}>
            {categories.map(c => <Select.Option key={c} value={c}>{c}</Select.Option>)}
          </Select>
          <Button icon={<ReloadOutlined />} onClick={loadPrompts}>Refresh</Button>
          <Button type="primary" icon={<PlusOutlined />} onClick={() => navigate('/prompt/editor/new')}>Create Prompt</Button>
        </Space>
      }>
        <Table columns={columns} dataSource={filtered} rowKey="id" loading={loading} locale={{ emptyText: <Empty description="No prompts yet" /> }} />
      </Card>
    </div>
  );
}
