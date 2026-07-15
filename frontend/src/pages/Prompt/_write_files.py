#!/usr/bin/env python3
import os

files = {
    'index.tsx': '''import { useState, useEffect, useCallback } from 'react';
import {
  Card,
  Table,
  Tag,
  Button,
  Space,
  Input,
  Select,
  message,
  Popconfirm,
  Badge,
  Row,
  Col,
  Statistic,
  Empty,
  Tooltip,
} from 'antd';
import {
  PlusOutlined,
  SearchOutlined,
  EditOutlined,
  HistoryOutlined,
  ExperimentOutlined,
  DeleteOutlined,
  TagsOutlined,
  AppstoreOutlined,
  ReloadOutlined,
} from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';
import dayjs from 'dayjs';
import { promptApi, type Prompt } from '../../api/prompt';

const mockPrompts: Prompt[] = [
  {
    id: '1',
    key: 'chat-default',
    name: '默认聊天 Prompt',
    description: '用于通用聊天场景的默认 Prompt 模板',
    category: 'chat',
    tags: ['chat', 'default', 'general'],
    active_version: 'v1.2.0',
    created_at: Date.now() - 86400000 * 30,
    updated_at: Date.now() - 86400000 * 2,
  },
  {
    id: '2',
    key: 'browser-agent-main',
    name: 'Browser Agent 主 Prompt',
    description: '控制 Browser Agent 行为的核心 Prompt',
    category: 'agent',
    tags: ['browser', 'agent', 'automation'],
    active_version: 'v2.0.1',
    created_at: Date.now() - 86400000 * 60,
    updated_at: Date.now() - 86400000 * 5,
  },
  {
    id: '3',
    key: 'summarization',
    name: '文本摘要 Prompt',
    description: '用于长文本摘要生成的 Prompt',
    category: 'nlp',
    tags: ['summarization', 'nlp', 'text'],
    active_version: 'v1.0.3',
    created_at: Date.now() - 86400000 * 15,
    updated_at: Date.now() - 86400000 * 1,
  },
];

const categoryColors: Record<string, string> = {
  chat: 'blue',
  agent: 'green',
  nlp: 'purple',
  code: 'orange',
};

export default function PromptListPage() {
  const navigate = useNavigate();
  const [prompts, setPrompts] = useState<Prompt[]>([]);
  const [loading, setLoading] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');
  const [categoryFilter, setCategoryFilter] = useState<string>();
  const [tagFilter, setTagFilter] = useState<string>();
  const [categories, setCategories] = useState<string[]>([]);
  const [allTags, setAllTags] = useState<string[]>([]);

  const loadPrompts = useCallback(async () => {
    setLoading(true);
    try {
      const data = await promptApi.list({
        category: categoryFilter,
        tags: tagFilter ? [tagFilter] : undefined,
        search: searchQuery || undefined,
      });
      setPrompts(data);
    } catch (error) {
      let filtered = [...mockPrompts];
      if (categoryFilter) filtered = filtered.filter(p => p.category === categoryFilter);
      if (tagFilter) filtered = filtered.filter(p => p.tags.includes(tagFilter));
      if (searchQuery) {
        const q = searchQuery.toLowerCase();
        filtered = filtered.filter(p => p.name.toLowerCase().includes(q) || p.key.toLowerCase().includes(q));
      }
      setPrompts(filtered);
    } finally {
      setLoading(false);
    }
  }, [categoryFilter, tagFilter, searchQuery]);

  const loadFilters = async () => {
    try {
      const [cats, tags] = await Promise.all([promptApi.listCategories(), promptApi.listTags()]);
      setCategories(cats);
      setAllTags(tags);
    } catch {
      setCategories([...new Set(mockPrompts.map(p => p.category))]);
      setAllTags([...new Set(mockPrompts.flatMap(p => p.tags))]);
    }
  };

  const handleDelete = async (key: string) => {
    try {
      await promptApi.delete(key);
      message.success('删除成功');
      loadPrompts();
    } catch {
      message.error('删除失败');
    }
  };

  useEffect(() => {
    loadPrompts();
    loadFilters();
  }, [loadPrompts]);

  const stats = {
    total: prompts.length,
    categories: categories.length,
    byCategory: categories.map(c => ({ category: c, count: prompts.filter(p => p.category === c).length })),
  };

  const columns = [
    { title: 'Key', dataIndex: 'key', key: 'key', width: 180, render: (k: string) => <code style={{ color: '#1677ff' }}>{k}</code> },
    { title: '名称', dataIndex: 'name', key: 'name', width: 200, render: (n: string, r: Prompt) => <Tooltip title={r.description}><span>{n}</span></Tooltip> },
    { title: '分类', dataIndex: 'category', key: 'category', width: 100, render: (c: string) => <Tag color={categoryColors[c] || 'default'}>{c}</Tag> },
    { title: '标签', dataIndex: 'tags', key: 'tags', width: 200, render: (t: string[]) => <Space size={4} wrap>{t.slice(0, 3).map(x => <Tag key={x}>{x}</Tag>)}</Space> },
    { title: '活跃版本', dataIndex: 'active_version', key: 'active_version', width: 120, render: (v: string) => <Badge status="success" text={v} /> },
    { title: '更新时间', dataIndex: 'updated_at', key: 'updated_at', width: 160, render: (ts: number) => dayjs(ts).format('YYYY-MM-DD HH:mm') },
    { title: '操作', key: 'action', width: 240, render: (_: unknown, r: Prompt) => (
      <Space>
        <Button size="small" type="primary" icon={<EditOutlined />} onClick={() => navigate('/prompt/editor/' + r.key)}>编辑</Button>
        <Button size="small" icon={<HistoryOutlined />} onClick={() => navigate('/prompt/history/' + r.key)}>版本</Button>
        <Button size="small" icon={<ExperimentOutlined />} onClick={() => navigate('/prompt/compare/' + r.key)}>对比</Button>
        <Popconfirm title="确定删除?" onConfirm={() => handleDelete(r.key)} okText="删除" cancelText="取消"><Button size="small" danger icon={<DeleteOutlined />} /></Popconfirm>
      </Space>
    ) },
  ];

  return (
    <div>
      <h2 style={{ marginBottom: 24 }}>Prompt 版本管理</h2>
      <Row gutter={16} style={{ marginBottom: 24 }}>
        <Col span={6}><Card><Statistic title="Prompt 总数" value={stats.total} /></Card></Col>
        <Col span={6}><Card><Statistic title="分类数量" value={stats.categories} /></Card></Col>
        <Col span={6}><Card><Statistic title="最活跃分类" value={stats.byCategory.sort((a, b) => b.count - a.count)[0]?.category || '-'} /></Card></Col>
        <Col span={6}><Card><Statistic title="标签总数" value={allTags.length} /></Card></Col>
      </Row>
      <Card style={{ marginBottom: 24 }}>
        <Space wrap size="middle">
          <Input placeholder="搜索 Prompt" prefix={<SearchOutlined />} value={searchQuery} onChange={e => setSearchQuery(e.target.value)} style={{ width: 300 }} onPressEnter={loadPrompts} />
          <Select placeholder="选择分类" value={categoryFilter} onChange={setCategoryFilter} allowClear style={{ width: 150 }} options={categories.map(c => ({ value: c, label: c }))} />
          <Select placeholder="选择标签" value={tagFilter} onChange={setTagFilter} allowClear style={{ width: 150 }} options={allTags.map(t => ({ value: t, label: t }))} />
          <Button icon={<SearchOutlined />} type="primary" onClick={loadPrompts}>搜索</Button>
          <Button icon={<ReloadOutlined />} onClick={() => { setSearchQuery(''); setCategoryFilter(undefined); setTagFilter(undefined); }}>重置</Button>
          <Button type="primary" icon={<PlusOutlined />} onClick={() => navigate('/prompt/editor/new')}>新建 Prompt</Button>
        </Space>
      </Card>
      <Card><Table columns={columns} dataSource={prompts} rowKey="id" loading={loading} pagination={{ pageSize: 10 }} locale={{ emptyText: <Empty description="暂无 Prompt 数据" /> }} /></Card>
    </div>
  );
}
''
