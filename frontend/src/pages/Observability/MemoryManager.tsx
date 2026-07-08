import { useState, useEffect, useRef } from 'react';
import {
  Card,
  Tabs,
  Table,
  Button,
  Tag,
  Descriptions,
  Input,
  Space,
  Row,
  Col,
  Statistic,
  Timeline,
  Drawer,
  Form,
  Slider,
  message,
  Modal,
  List,
  Empty,
} from 'antd';
import type { ColumnsType } from 'antd/es/table';
import {
  PlusOutlined,
  SearchOutlined,
  ReloadOutlined,
  DeleteOutlined,
  EditOutlined,
  CompressOutlined,
  PlayCircleOutlined,
} from '@ant-design/icons';
import dayjs from 'dayjs';
import * as echarts from 'echarts';
import { memoryApi } from '../../api';
import type {
  EpisodicMemory,
  SemanticMemory,
  ProceduralMemory,
  TimelineEvent,
  KnowledgeGraph,
  MemoryStats,
  MemorySearchRequest,
  MemorySearchResult,
} from '../../api/memory';

export default function MemoryManager() {
  // 统计数据
  const [stats, setStats] = useState<MemoryStats>({
    working_memory_count: 0,
    episodic_memory_count: 0,
    semantic_memory_count: 0,
    procedural_memory_count: 0,
    total_size_bytes: 0,
    avg_importance: 0,
    oldest_memory: '',
    newest_memory: '',
  });

  // 情节记忆
  const [episodicMemories, setEpisodicMemories] = useState<EpisodicMemory[]>([]);
  const [episodicLoading, setEpisodicLoading] = useState(false);

  // 语义记忆
  const [semanticMemories, setSemanticMemories] = useState<SemanticMemory[]>([]);
  const [semanticLoading, setSemanticLoading] = useState(false);
  const [knowledgeGraph, setKnowledgeGraph] = useState<KnowledgeGraph | null>(null);
  const graphRef = useRef<HTMLDivElement>(null);
  const graphChart = useRef<echarts.ECharts | null>(null);

  // 程序记忆
  const [proceduralMemories, setProceduralMemories] = useState<ProceduralMemory[]>([]);
  const [proceduralLoading, setProceduralLoading] = useState(false);

  // 时间线
  const [timelineEvents, setTimelineEvents] = useState<TimelineEvent[]>([]);

  // 搜索
  const [searchQuery, setSearchQuery] = useState('');
  const [searchResults, setSearchResults] = useState<MemorySearchResult[]>([]);
  const [searchLoading, setSearchLoading] = useState(false);

  // 详情抽屉
  const [detailVisible, setDetailVisible] = useState(false);
  const [selectedMemory, setSelectedMemory] = useState<EpisodicMemory | SemanticMemory | ProceduralMemory | null>(null);
  const [memoryType, setMemoryType] = useState<'episodic' | 'semantic' | 'procedural'>('episodic');

  // 创建记忆对话框
  const [createModalVisible, setCreateModalVisible] = useState(false);
  const [createForm] = Form.useForm();
  const [createType, setCreateType] = useState<'episodic' | 'semantic' | 'procedural'>('episodic');

  // 加载统计
  const loadStats = async () => {
    try {
      const data = await memoryApi.getMemoryStats();
      setStats(data);
    } catch (error) {
      console.error('加载统计失败', error);
    }
  };

  // 加载情节记忆
  const loadEpisodicMemories = async () => {
    setEpisodicLoading(true);
    try {
      const data = await memoryApi.listEpisodicMemories({ limit: 50 });
      setEpisodicMemories(data?.memories || []);
    } catch (error) {
      console.error('加载情节记忆失败', error);
      setEpisodicMemories([]);
    } finally {
      setEpisodicLoading(false);
    }
  };

  // 加载语义记忆
  const loadSemanticMemories = async () => {
    setSemanticLoading(true);
    try {
      const data = await memoryApi.listSemanticMemories({ limit: 50 });
      setSemanticMemories(data?.memories || []);
    } catch (error) {
      console.error('加载语义记忆失败', error);
      setSemanticMemories([]);
    } finally {
      setSemanticLoading(false);
    }
  };

  // 加载程序记忆
  const loadProceduralMemories = async () => {
    setProceduralLoading(true);
    try {
      const data = await memoryApi.listProceduralMemories({ limit: 50 });
      setProceduralMemories(data?.memories || []);
    } catch (error) {
      console.error('加载程序记忆失败', error);
      setProceduralMemories([]);
    } finally {
      setProceduralLoading(false);
    }
  };

  // 加载时间线
  const loadTimeline = async () => {
    try {
      const data = await memoryApi.getTimeline({ limit: 20 });
      setTimelineEvents(data);
    } catch (error) {
      console.error('加载时间线失败', error);
    }
  };

  // 搜索记忆
  const handleSearch = async () => {
    if (!searchQuery.trim()) return;
    setSearchLoading(true);
    try {
      const request: MemorySearchRequest = {
        query: searchQuery,
        limit: 20,
      };
      const results = await memoryApi.searchMemory(request);
      setSearchResults(results);
    } catch (error) {
      message.error('搜索失败');
      console.error(error);
    } finally {
      setSearchLoading(false);
    }
  };

  // 加载知识图谱
  const loadKnowledgeGraph = async () => {
    try {
      const data = await memoryApi.getKnowledgeGraph({ depth: 2 });
      setKnowledgeGraph(data);
      setTimeout(renderKnowledgeGraph, 100);
    } catch (error) {
      console.error('加载知识图谱失败', error);
    }
  };

  // 渲染知识图谱
  const renderKnowledgeGraph = () => {
    if (!graphRef.current || !knowledgeGraph) return;

    if (!graphChart.current) {
      graphChart.current = echarts.init(graphRef.current);
    }

    const nodes = knowledgeGraph.nodes.map((node) => ({
      id: node.id,
      name: node.name,
      symbolSize: 30,
      category: node.type,
    }));

    const edges = knowledgeGraph.edges.map((edge) => ({
      source: edge.source_id,
      target: edge.target_id,
      value: edge.relation,
    }));

    const option = {
      tooltip: {},
      legend: {
        data: ['concept', 'entity', 'event'],
      },
      series: [
        {
          type: 'graph',
          layout: 'force',
          data: nodes,
          links: edges,
          roam: true,
          label: {
            show: true,
            position: 'right',
          },
          force: {
            repulsion: 100,
          },
        },
      ],
    };

    graphChart.current.setOption(option);
  };

  // 删除情节记忆
  const handleDeleteEpisodic = async (id: string) => {
    try {
      await memoryApi.deleteEpisodicMemory(id);
      message.success('删除成功');
      loadEpisodicMemories();
    } catch (error) {
      message.error('删除失败');
      console.error(error);
    }
  };

  // 删除语义记忆
  const handleDeleteSemantic = async (id: string) => {
    try {
      await memoryApi.deleteSemanticMemory(id);
      message.success('删除成功');
      loadSemanticMemories();
    } catch (error) {
      message.error('删除失败');
      console.error(error);
    }
  };

  // 删除程序记忆
  const handleDeleteProcedural = async (id: string) => {
    try {
      await memoryApi.deleteProceduralMemory(id);
      message.success('删除成功');
      loadProceduralMemories();
    } catch (error) {
      message.error('删除失败');
      console.error(error);
    }
  };

  // 整合记忆
  const handleConsolidate = async () => {
    try {
      const result = await memoryApi.consolidateMemory({ session_id: 'all' });
      message.success(`整合完成: 处理 ${result.processed_count} 条记忆`);
      loadStats();
      loadTimeline();
    } catch (error) {
      message.error('整合失败');
      console.error(error);
    }
  };

  // 创建记忆
  const handleCreate = async () => {
    let values;
    try {
      values = await createForm.validateFields();
    } catch {
      return;
    }
    try {
      if (createType === 'episodic') {
        await memoryApi.createEpisodicMemory({
          session_id: 'manual',
          event_type: values.event_type,
          title: values.title,
          description: values.description,
          importance: values.importance ?? 0.5,
          participants: [],
          actions: [],
          outcome: '',
          started_at: new Date().toISOString(),
        });
      } else if (createType === 'semantic') {
        await memoryApi.createSemanticMemory({
          concept: values.concept,
          category: values.category,
          description: values.description,
          attributes: {},
          relations: [],
          source_count: 1,
          confidence: values.confidence ?? 0.5,
        });
      } else {
        await memoryApi.createProceduralMemory({
          name: values.name,
          category: values.category,
          description: values.description,
          steps: [],
          preconditions: values.preconditions ? values.preconditions.split('\n') : [],
          postconditions: [],
        });
      }
      message.success('创建成功');
      setCreateModalVisible(false);
      createForm.resetFields();
      if (createType === 'episodic') loadEpisodicMemories();
      else if (createType === 'semantic') loadSemanticMemories();
      else loadProceduralMemories();
    } catch (error) {
      message.error('创建失败');
      console.error(error);
    }
  };

  // 查看详情
  const viewDetail = (memory: EpisodicMemory | SemanticMemory | ProceduralMemory, type: 'episodic' | 'semantic' | 'procedural') => {
    setSelectedMemory(memory);
    setMemoryType(type);
    setDetailVisible(true);
  };

  // 情节记忆表格列
  const episodicColumns: ColumnsType<EpisodicMemory> = [
    {
      title: '标题',
      dataIndex: 'title',
      key: 'title',
      width: 200,
    },
    {
      title: '事件类型',
      dataIndex: 'event_type',
      key: 'event_type',
      width: 120,
      render: (type: string) => <Tag color="blue">{type}</Tag>,
    },
    {
      title: '重要性',
      dataIndex: 'importance',
      key: 'importance',
      width: 120,
      render: (val: number) => (
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <Slider value={val} min={0} max={1} step={0.1} style={{ width: 60 }} disabled />
          <span>{val.toFixed(2)}</span>
        </div>
      ),
    },
    {
      title: '开始时间',
      dataIndex: 'started_at',
      key: 'started_at',
      width: 180,
      render: (time: string) => dayjs(time).format('YYYY-MM-DD HH:mm:ss'),
    },
    {
      title: '操作',
      key: 'action',
      width: 150,
      render: (_, record) => (
        <Space>
          <Button size="small" icon={<EditOutlined />} onClick={() => viewDetail(record, 'episodic')}>
            查看
          </Button>
          <Button size="small" danger icon={<DeleteOutlined />} onClick={() => handleDeleteEpisodic(record.id)}>
            删除
          </Button>
        </Space>
      ),
    },
  ];

  // 语义记忆表格列
  const semanticColumns: ColumnsType<SemanticMemory> = [
    {
      title: '概念',
      dataIndex: 'concept',
      key: 'concept',
      width: 180,
    },
    {
      title: '分类',
      dataIndex: 'category',
      key: 'category',
      width: 120,
      render: (cat: string) => <Tag color="green">{cat}</Tag>,
    },
    {
      title: '描述',
      dataIndex: 'description',
      key: 'description',
      ellipsis: true,
    },
    {
      title: '置信度',
      dataIndex: 'confidence',
      key: 'confidence',
      width: 100,
      render: (val: number) => `${(val * 100).toFixed(0)}%`,
    },
    {
      title: '访问次数',
      dataIndex: 'access_count',
      key: 'access_count',
      width: 100,
    },
    {
      title: '操作',
      key: 'action',
      width: 150,
      render: (_, record) => (
        <Space>
          <Button size="small" icon={<EditOutlined />} onClick={() => viewDetail(record, 'semantic')}>
            查看
          </Button>
          <Button size="small" danger icon={<DeleteOutlined />} onClick={() => handleDeleteSemantic(record.id)}>
            删除
          </Button>
        </Space>
      ),
    },
  ];

  // 程序记忆表格列
  const proceduralColumns: ColumnsType<ProceduralMemory> = [
    {
      title: '技能名称',
      dataIndex: 'name',
      key: 'name',
      width: 180,
    },
    {
      title: '分类',
      dataIndex: 'category',
      key: 'category',
      width: 120,
      render: (cat: string) => <Tag color="orange">{cat}</Tag>,
    },
    {
      title: '描述',
      dataIndex: 'description',
      key: 'description',
      ellipsis: true,
    },
    {
      title: '成功率',
      dataIndex: 'success_rate',
      key: 'success_rate',
      width: 100,
      render: (val: number) => `${(val * 100).toFixed(0)}%`,
    },
    {
      title: '使用次数',
      dataIndex: 'usage_count',
      key: 'usage_count',
      width: 100,
    },
    {
      title: '操作',
      key: 'action',
      width: 200,
      render: (_, record) => (
        <Space>
          <Button size="small" icon={<EditOutlined />} onClick={() => viewDetail(record, 'procedural')}>
            查看
          </Button>
          <Button size="small" type="primary" ghost icon={<PlayCircleOutlined />}>
            执行
          </Button>
          <Button size="small" danger icon={<DeleteOutlined />} onClick={() => handleDeleteProcedural(record.id)}>
            删除
          </Button>
        </Space>
      ),
    },
  ];

  // 初始化
  useEffect(() => {
    loadStats();
    loadTimeline();
    loadKnowledgeGraph();
    loadEpisodicMemories();
    loadSemanticMemories();
    loadProceduralMemories();
  }, []);

  // 窗口大小变化时重绘图表
  useEffect(() => {
    const handleResize = () => {
      graphChart.current?.resize();
    };
    window.addEventListener('resize', handleResize);
    return () => {
      window.removeEventListener('resize', handleResize);
      graphChart.current?.dispose();
    };
  }, []);

  // Tab 项
  const tabItems = [
    {
      key: 'episodic',
      label: `情节记忆 (${stats.episodic_memory_count})`,
      children: (
        <Card>
          <div style={{ marginBottom: 16 }}>
            <Space>
              <Input placeholder="搜索情节记忆..." prefix={<SearchOutlined />} style={{ width: 240 }} />
              <Button type="primary" icon={<PlusOutlined />} onClick={() => { setCreateType('episodic'); setCreateModalVisible(true); }}>
                创建记忆
              </Button>
              <Button icon={<ReloadOutlined />} onClick={loadEpisodicMemories}>刷新</Button>
            </Space>
          </div>
          <Table
            columns={episodicColumns}
            dataSource={episodicMemories}
            rowKey="id"
            loading={episodicLoading}
          />
        </Card>
      ),
    },
    {
      key: 'semantic',
      label: `语义记忆 (${stats.semantic_memory_count})`,
      children: (
        <Card>
          <div style={{ marginBottom: 16 }}>
            <Space>
              <Input placeholder="搜索语义记忆..." prefix={<SearchOutlined />} style={{ width: 240 }} />
              <Button type="primary" icon={<PlusOutlined />} onClick={() => { setCreateType('semantic'); setCreateModalVisible(true); }}>
                创建记忆
              </Button>
              <Button icon={<ReloadOutlined />} onClick={loadSemanticMemories}>刷新</Button>
            </Space>
          </div>
          <Table
            columns={semanticColumns}
            dataSource={semanticMemories}
            rowKey="id"
            loading={semanticLoading}
          />
        </Card>
      ),
    },
    {
      key: 'procedural',
      label: `程序记忆 (${stats.procedural_memory_count})`,
      children: (
        <Card>
          <div style={{ marginBottom: 16 }}>
            <Space>
              <Input placeholder="搜索程序记忆..." prefix={<SearchOutlined />} style={{ width: 240 }} />
              <Button type="primary" icon={<PlusOutlined />} onClick={() => { setCreateType('procedural'); setCreateModalVisible(true); }}>
                创建技能
              </Button>
              <Button icon={<ReloadOutlined />} onClick={loadProceduralMemories}>刷新</Button>
            </Space>
          </div>
          <Table
            columns={proceduralColumns}
            dataSource={proceduralMemories}
            rowKey="id"
            loading={proceduralLoading}
          />
        </Card>
      ),
    },
    {
      key: 'knowledge',
      label: '知识图谱',
      children: (
        <Card>
          <div ref={graphRef} style={{ height: 500 }} />
        </Card>
      ),
    },
    {
      key: 'search',
      label: '记忆搜索',
      children: (
        <Card>
          <div style={{ marginBottom: 24 }}>
            <Space.Compact style={{ width: '100%' }}>
              <Input
                placeholder="搜索所有记忆..."
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                onPressEnter={handleSearch}
                style={{ width: 'calc(100% - 100px)' }}
              />
              <Button type="primary" onClick={handleSearch} loading={searchLoading}>
                搜索
              </Button>
            </Space.Compact>
          </div>
          {searchResults.length > 0 ? (
            <List
              dataSource={searchResults}
              renderItem={(item) => (
                <List.Item
                  actions={[<Button size="small">详情</Button>]}
                >
                  <List.Item.Meta
                    title={
                      <Space>
                        <Tag color={item.type === 'episodic' ? 'blue' : item.type === 'semantic' ? 'green' : 'orange'}>
                          {item.type}
                        </Tag>
                        <span>{item.content.substring(0, 100)}...</span>
                      </Space>
                    }
                    description={`重要性: ${item.importance.toFixed(2)} | 相关度: ${item.relevance_score.toFixed(2)} | ${dayjs(item.created_at).format('YYYY-MM-DD HH:mm')}`}
                  />
                </List.Item>
              )}
            />
          ) : (
            <Empty description="输入关键词搜索记忆" />
          )}
        </Card>
      ),
    },
  ];

  return (
    <div className="memory-manager">
      <h2 style={{ marginBottom: 24 }}>记忆管理</h2>

      {/* 统计卡片 */}
      <Row gutter={16} style={{ marginBottom: 24 }}>
        <Col span={4}>
          <Card>
            <Statistic title="工作记忆" value={stats.working_memory_count} />
          </Card>
        </Col>
        <Col span={4}>
          <Card>
            <Statistic title="情节记忆" value={stats.episodic_memory_count} />
          </Card>
        </Col>
        <Col span={4}>
          <Card>
            <Statistic title="语义记忆" value={stats.semantic_memory_count} />
          </Card>
        </Col>
        <Col span={4}>
          <Card>
            <Statistic title="程序记忆" value={stats.procedural_memory_count} />
          </Card>
        </Col>
        <Col span={4}>
          <Card>
            <Statistic title="平均重要性" value={stats.avg_importance} precision={2} />
          </Card>
        </Col>
        <Col span={4}>
          <Card>
            <Statistic title="总大小" value={(stats.total_size_bytes / 1024).toFixed(2)} suffix="KB" />
          </Card>
        </Col>
      </Row>

      {/* 时间线 */}
      <Card title="最近时间线" style={{ marginBottom: 24 }} extra={<Button icon={<CompressOutlined />} onClick={handleConsolidate}>整合记忆</Button>}>
        <Timeline
          items={timelineEvents.slice(0, 10).map((event) => ({
            color: event.importance > 0.7 ? 'green' : event.importance > 0.4 ? 'blue' : 'gray',
            children: (
              <div>
                <div style={{ fontWeight: 500 }}>{event.title}</div>
                <div style={{ color: '#999', fontSize: 12 }}>
                  {dayjs(event.timestamp).format('YYYY-MM-DD HH:mm')} | 类型: {event.event_type}
                </div>
                <div style={{ fontSize: 13, marginTop: 4 }}>{event.description}</div>
              </div>
            ),
          }))}
        />
      </Card>

      {/* 记忆管理 Tabs */}
      <Card>
        <Tabs items={tabItems} />
      </Card>

      {/* 详情抽屉 */}
      <Drawer
        title="记忆详情"
        placement="right"
        width="50%"
        open={detailVisible}
        onClose={() => setDetailVisible(false)}
      >
        {selectedMemory && (
          <div>
            {memoryType === 'episodic' && (
              <>
                <Descriptions bordered column={1}>
                  <Descriptions.Item label="标题">{(selectedMemory as EpisodicMemory).title}</Descriptions.Item>
                  <Descriptions.Item label="事件类型">{(selectedMemory as EpisodicMemory).event_type}</Descriptions.Item>
                  <Descriptions.Item label="描述">{(selectedMemory as EpisodicMemory).description}</Descriptions.Item>
                  <Descriptions.Item label="重要性">{(selectedMemory as EpisodicMemory).importance}</Descriptions.Item>
                  <Descriptions.Item label="参与者">{(selectedMemory as EpisodicMemory).participants?.join(', ')}</Descriptions.Item>
                  <Descriptions.Item label="结果">{(selectedMemory as EpisodicMemory).outcome}</Descriptions.Item>
                </Descriptions>
              </>
            )}
            {memoryType === 'semantic' && (
              <>
                <Descriptions bordered column={1}>
                  <Descriptions.Item label="概念">{(selectedMemory as SemanticMemory).concept}</Descriptions.Item>
                  <Descriptions.Item label="分类">{(selectedMemory as SemanticMemory).category}</Descriptions.Item>
                  <Descriptions.Item label="描述">{(selectedMemory as SemanticMemory).description}</Descriptions.Item>
                  <Descriptions.Item label="置信度">{(selectedMemory as SemanticMemory).confidence}</Descriptions.Item>
                  <Descriptions.Item label="访问次数">{(selectedMemory as SemanticMemory).access_count}</Descriptions.Item>
                </Descriptions>
              </>
            )}
            {memoryType === 'procedural' && (
              <>
                <Descriptions bordered column={1}>
                  <Descriptions.Item label="技能名称">{(selectedMemory as ProceduralMemory).name}</Descriptions.Item>
                  <Descriptions.Item label="分类">{(selectedMemory as ProceduralMemory).category}</Descriptions.Item>
                  <Descriptions.Item label="描述">{(selectedMemory as ProceduralMemory).description}</Descriptions.Item>
                  <Descriptions.Item label="成功率">{(selectedMemory as ProceduralMemory).success_rate}</Descriptions.Item>
                  <Descriptions.Item label="使用次数">{(selectedMemory as ProceduralMemory).usage_count}</Descriptions.Item>
                </Descriptions>
              </>
            )}
          </div>
        )}
      </Drawer>

      {/* 创建记忆对话框 */}
      <Modal
        title={`创建${createType === 'episodic' ? '情节记忆' : createType === 'semantic' ? '语义记忆' : '程序记忆'}`}
        open={createModalVisible}
        onCancel={() => setCreateModalVisible(false)}
        onOk={handleCreate}
        width={600}
      >
        <Form form={createForm} layout="vertical">
          {createType === 'episodic' && (
            <>
              <Form.Item name="title" label="标题" rules={[{ required: true }]}>
                <Input placeholder="事件标题" />
              </Form.Item>
              <Form.Item name="event_type" label="事件类型" rules={[{ required: true }]}>
                <Input placeholder="例如：对话、任务、决策" />
              </Form.Item>
              <Form.Item name="description" label="描述" rules={[{ required: true }]}>
                <Input.TextArea rows={4} placeholder="事件详细描述" />
              </Form.Item>
              <Form.Item name="importance" label="重要性">
                <Slider min={0} max={1} step={0.1} marks={{ 0: '低', 0.5: '中', 1: '高' }} />
              </Form.Item>
            </>
          )}
          {createType === 'semantic' && (
            <>
              <Form.Item name="concept" label="概念" rules={[{ required: true }]}>
                <Input placeholder="概念名称" />
              </Form.Item>
              <Form.Item name="category" label="分类" rules={[{ required: true }]}>
                <Input placeholder="例如：实体、概念、关系" />
              </Form.Item>
              <Form.Item name="description" label="描述" rules={[{ required: true }]}>
                <Input.TextArea rows={4} placeholder="概念详细描述" />
              </Form.Item>
              <Form.Item name="confidence" label="置信度">
                <Slider min={0} max={1} step={0.1} marks={{ 0: '不确定', 0.5: '可能', 1: '确定' }} />
              </Form.Item>
            </>
          )}
          {createType === 'procedural' && (
            <>
              <Form.Item name="name" label="技能名称" rules={[{ required: true }]}>
                <Input placeholder="技能名称" />
              </Form.Item>
              <Form.Item name="category" label="分类" rules={[{ required: true }]}>
                <Input placeholder="例如：工具、流程、策略" />
              </Form.Item>
              <Form.Item name="description" label="描述" rules={[{ required: true }]}>
                <Input.TextArea rows={4} placeholder="技能详细描述" />
              </Form.Item>
              <Form.Item name="preconditions" label="前置条件">
                <Input.TextArea rows={2} placeholder="每行一个前置条件" />
              </Form.Item>
            </>
          )}
        </Form>
      </Modal>
    </div>
  );
}