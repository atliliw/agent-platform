import { useState, useEffect, useRef } from 'react';
import {
  Card,
  Radio,
  DatePicker,
  Button,
  Progress,
  Table,
  Tag,
  Modal,
  Form,
  Input,
  InputNumber,
  Select,
  Slider,
  Row,
  Col,
  Statistic,
  Space,
  message,
  Switch,
} from 'antd';
import {
  DollarOutlined,
  MessageOutlined,
  FileTextOutlined,
  ClockCircleOutlined,
  DownloadOutlined,
  PlusOutlined,
} from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import dayjs from 'dayjs';
import * as echarts from 'echarts';
import { costApi } from '../../api';
import type { CostSummary, CostTrend, Budget, CreateBudgetRequest, CostDetailQueryParams, CostDetail } from '../../api/cost';

const { RangePicker } = DatePicker;

export default function CostDashboard() {
  // 时间范围
  const [timeRange, setTimeRange] = useState<'today' | 'week' | 'month' | 'custom'>('today');
  const [customRange, setCustomRange] = useState<[dayjs.Dayjs, dayjs.Dayjs] | null>(null);

  // 数据状态
  const [summary, setSummary] = useState<CostSummary>({
    total_cost: 0,
    total_calls: 0,
    total_tokens: 0,
    input_tokens: 0,
    output_tokens: 0,
    avg_latency_ms: 0,
    cost_trend: 0,
    by_model: [],
    by_agent: [],
    by_tool: [],
    by_date: [],
  });
  const [trendData, setTrendData] = useState<CostTrend>({
    timestamps: [],
    costs: [],
    tokens: [],
    calls: [],
  });
  const [budgets, setBudgets] = useState<Budget[]>([]);
  const [costDetails, setCostDetails] = useState<CostDetail[]>([]);
  const [totalDetails, setTotalDetails] = useState(0);
  const [currentPage, setCurrentPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [loading, setLoading] = useState(false);

  // 图表引用
  const trendChartRef = useRef<HTMLDivElement>(null);
  const modelPieRef = useRef<HTMLDivElement>(null);
  const agentBarRef = useRef<HTMLDivElement>(null);
  const toolBarRef = useRef<HTMLDivElement>(null);
  const trendChart = useRef<echarts.ECharts | null>(null);
  const modelPie = useRef<echarts.ECharts | null>(null);
  const agentBar = useRef<echarts.ECharts | null>(null);
  const toolBar = useRef<echarts.ECharts | null>(null);

  // 显示 Token 还是成本
  const [showTokens, setShowTokens] = useState(false);

  // 预算对话框
  const [budgetDialogVisible, setBudgetDialogVisible] = useState(false);
  const [budgetForm] = Form.useForm();

  // 加载所有数据
  const loadData = async () => {
    setLoading(true);
    try {
      const rangeParams: CostDetailQueryParams | undefined = timeRange === 'custom' && customRange
        ? {
            range: 'custom',
            start_time: customRange[0].toISOString(),
            end_time: customRange[1].toISOString(),
          }
        : { range: timeRange };

      const [summaryRes, trendRes, budgetsRes] = await Promise.all([
        costApi.getCostSummary(rangeParams),
        costApi.getCostTrend(rangeParams),
        costApi.getBudgets(),
      ]);

      setSummary(summaryRes);
      setTrendData(trendRes);
      setBudgets(budgetsRes);

      // 渲染图表
      setTimeout(() => {
        renderCharts();
      }, 100);
    } catch (error) {
      message.error('加载数据失败');
      console.error(error);
    } finally {
      setLoading(false);
    }
  };

  // 加载成本明细
  const loadDetails = async () => {
    try {
      const rangeParams: CostDetailQueryParams | undefined = timeRange === 'custom' && customRange
        ? {
            range: 'custom',
            start_time: customRange[0].toISOString(),
            end_time: customRange[1].toISOString(),
          }
        : { range: timeRange };

      const response = await costApi.getCostDetails({
        page: currentPage,
        size: pageSize,
        ...rangeParams,
      });
      setCostDetails(response.details || []);
      setTotalDetails(response.total || 0);
    } catch (error) {
      message.error('加载明细失败');
      console.error(error);
    }
  };

  // 渲染图表
  const renderCharts = () => {
    renderTrendChart();
    renderModelPie();
    renderAgentBar();
    renderToolBar();
  };

  // 渲染趋势图
  const renderTrendChart = () => {
    if (!trendChartRef.current) return;

    if (!trendChart.current) {
      trendChart.current = echarts.init(trendChartRef.current);
    }

    const option = {
      tooltip: {
        trigger: 'axis',
      },
      xAxis: {
        type: 'category',
        data: trendData.timestamps || [],
      },
      yAxis: {
        type: 'value',
        name: showTokens ? 'Token' : '成本 ($)',
      },
      series: [
        {
          data: showTokens ? trendData.tokens : trendData.costs,
          type: 'line',
          smooth: true,
          areaStyle: {
            opacity: 0.3,
          },
        },
      ],
    };

    trendChart.current.setOption(option);
  };

  // 渲染模型饼图
  const renderModelPie = () => {
    if (!modelPieRef.current) return;

    if (!modelPie.current) {
      modelPie.current = echarts.init(modelPieRef.current);
    }

    const option = {
      tooltip: {
        trigger: 'item',
        formatter: '{b}: ${c} ({d}%)',
      },
      legend: {
        orient: 'vertical',
        left: 'left',
      },
      series: [
        {
          type: 'pie',
          radius: '50%',
          data: summary.by_model?.map((m) => ({
            name: m.model,
            value: m.cost,
          })) || [],
        },
      ],
    };

    modelPie.current.setOption(option);
  };

  // 渲染 Agent 柱状图
  const renderAgentBar = () => {
    if (!agentBarRef.current) return;

    if (!agentBar.current) {
      agentBar.current = echarts.init(agentBarRef.current);
    }

    const option = {
      tooltip: {
        trigger: 'axis',
      },
      xAxis: {
        type: 'category',
        data: summary.by_agent?.map((a) => a.agent_id) || [],
      },
      yAxis: {
        type: 'value',
        name: '成本 ($)',
      },
      series: [
        {
          data: summary.by_agent?.map((a) => a.cost) || [],
          type: 'bar',
        },
      ],
    };

    agentBar.current.setOption(option);
  };

  // 渲染工具柱状图
  const renderToolBar = () => {
    if (!toolBarRef.current) return;

    if (!toolBar.current) {
      toolBar.current = echarts.init(toolBarRef.current);
    }

    const option = {
      tooltip: {
        trigger: 'axis',
      },
      xAxis: {
        type: 'category',
        data: summary.by_tool?.map((t) => t.tool) || [],
      },
      yAxis: {
        type: 'value',
        name: '成本 ($)',
      },
      series: [
        {
          data: summary.by_tool?.map((t) => t.cost) || [],
          type: 'bar',
        },
      ],
    };

    toolBar.current.setOption(option);
  };

  // 创建预算
  const handleCreateBudget = async () => {
    try {
      const values = await budgetForm.validateFields();
      const request: CreateBudgetRequest = {
        name: values.name,
        limit: values.limit,
        period: values.period,
        alert_threshold: values.alertThreshold,
      };
      await costApi.createBudget(request);
      message.success('预算创建成功');
      setBudgetDialogVisible(false);
      budgetForm.resetFields();
      loadData();
    } catch (error) {
      message.error('创建预算失败');
      console.error(error);
    }
  };

  // 导出数据
  const handleExport = () => {
    const url = costApi.exportCostData('csv', timeRange);
    window.open(url);
  };

  // 表格列定义
  const columns: ColumnsType<CostDetail> = [
    {
      title: '时间',
      dataIndex: 'timestamp',
      key: 'timestamp',
      width: 180,
      render: (time: string) => dayjs(time).format('YYYY-MM-DD HH:mm:ss'),
    },
    {
      title: '会话ID',
      dataIndex: 'session_id',
      key: 'session_id',
      width: 200,
      render: (id: string) => <Tag>{id?.substring(0, 12)}...</Tag>,
    },
    {
      title: 'Agent',
      dataIndex: 'agent_id',
      key: 'agent_id',
      width: 120,
    },
    {
      title: '模型',
      dataIndex: 'model',
      key: 'model',
      width: 150,
    },
    {
      title: '输入Token',
      dataIndex: 'input_tokens',
      key: 'input_tokens',
      width: 100,
      sorter: true,
    },
    {
      title: '输出Token',
      dataIndex: 'output_tokens',
      key: 'output_tokens',
      width: 100,
      sorter: true,
    },
    {
      title: '成本',
      dataIndex: 'cost',
      key: 'cost',
      width: 100,
      sorter: true,
      render: (cost: number) => `$${cost.toFixed(6)}`,
    },
    {
      title: '状态',
      dataIndex: 'success',
      key: 'success',
      width: 80,
      render: (success: boolean) => (
        <Tag color={success ? 'success' : 'error'}>{success ? '成功' : '失败'}</Tag>
      ),
    },
  ];

  // 初始化加载
  useEffect(() => {
    loadData();
    loadDetails();
  }, [timeRange, customRange, currentPage, pageSize]);

  // 窗口大小变化时重绘图表
  useEffect(() => {
    const handleResize = () => {
      trendChart.current?.resize();
      modelPie.current?.resize();
      agentBar.current?.resize();
      toolBar.current?.resize();
    };

    window.addEventListener('resize', handleResize);
    return () => {
      window.removeEventListener('resize', handleResize);
      trendChart.current?.dispose();
      modelPie.current?.dispose();
      agentBar.current?.dispose();
      toolBar.current?.dispose();
    };
  }, []);

  return (
    <div className="cost-dashboard">
      {/* 头部 */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 24 }}>
        <h2 style={{ margin: 0 }}>成本监控面板</h2>
        <Space>
          <Radio.Group value={timeRange} onChange={(e) => setTimeRange(e.target.value)}>
            <Radio.Button value="today">今日</Radio.Button>
            <Radio.Button value="week">本周</Radio.Button>
            <Radio.Button value="month">本月</Radio.Button>
            <Radio.Button value="custom">自定义</Radio.Button>
          </Radio.Group>
          {timeRange === 'custom' && (
            <RangePicker
              showTime
              value={customRange}
              onChange={(dates) => setCustomRange(dates as [dayjs.Dayjs, dayjs.Dayjs] | null)}
            />
          )}
        </Space>
      </div>

      {/* 总览卡片 */}
      <Row gutter={16} style={{ marginBottom: 24 }}>
        <Col span={4}>
          <Card>
            <Statistic
              title="总成本"
              value={summary.total_cost}
              prefix={<DollarOutlined />}
              precision={4}
              valueStyle={{ color: summary.cost_trend >= 0 ? '#ff4d4f' : '#3f8600' }}
              suffix={
                <span style={{ fontSize: 12, marginLeft: 8 }}>
                  {summary.cost_trend >= 0 ? '↑' : '↓'} {Math.abs(summary.cost_trend).toFixed(1)}%
                </span>
              }
            />
          </Card>
        </Col>
        <Col span={4}>
          <Card>
            <Statistic
              title="总调用次数"
              value={summary.total_calls}
              prefix={<MessageOutlined />}
            />
          </Card>
        </Col>
        <Col span={4}>
          <Card>
            <Statistic
              title="总Token数"
              value={summary.total_tokens}
              prefix={<FileTextOutlined />}
            />
          </Card>
        </Col>
        <Col span={4}>
          <Card>
            <Statistic
              title="平均延迟"
              value={summary.avg_latency_ms}
              prefix={<ClockCircleOutlined />}
              suffix="ms"
            />
          </Card>
        </Col>
      </Row>

      {/* 图表区域 */}
      <Row gutter={16} style={{ marginBottom: 24 }}>
        <Col span={12}>
          <Card
            title="成本趋势"
            extra={
              <Space>
                <span>显示:</span>
                <Switch
                  checkedChildren="Token"
                  unCheckedChildren="成本"
                  checked={showTokens}
                  onChange={(checked) => {
                    setShowTokens(checked);
                    setTimeout(renderTrendChart, 100);
                  }}
                />
              </Space>
            }
          >
            <div ref={trendChartRef} style={{ height: 300 }} />
          </Card>
        </Col>
        <Col span={12}>
          <Card title="模型成本分布">
            <div ref={modelPieRef} style={{ height: 300 }} />
          </Card>
        </Col>
      </Row>

      <Row gutter={16} style={{ marginBottom: 24 }}>
        <Col span={12}>
          <Card title="Agent 成本排行">
            <div ref={agentBarRef} style={{ height: 300 }} />
          </Card>
        </Col>
        <Col span={12}>
          <Card title="工具成本分布">
            <div ref={toolBarRef} style={{ height: 300 }} />
          </Card>
        </Col>
      </Row>

      {/* 预算管理 */}
      <Card
        title="预算管理"
        extra={
          <Button type="primary" icon={<PlusOutlined />} onClick={() => setBudgetDialogVisible(true)}>
            创建预算
          </Button>
        }
        style={{ marginBottom: 24 }}
      >
        <Row gutter={16}>
          {budgets.map((budget) => (
            <Col span={6} key={budget.id}>
              <Card
                style={{
                  borderLeft: `4px solid ${budget.exceeded ? '#ff4d4f' : budget.percent_used > 80 ? '#faad14' : '#52c41a'}`,
                }}
              >
                <div style={{ fontWeight: 500, marginBottom: 12 }}>{budget.name}</div>
                <Progress
                  percent={Math.min(budget.percent_used, 100)}
                  status={budget.exceeded ? 'exception' : undefined}
                />
                <div style={{ display: 'flex', justifyContent: 'space-between', marginTop: 12, fontSize: 13, color: '#666' }}>
                  <span>已用: ${budget.used.toFixed(4)}</span>
                  <span>预算: ${budget.limit.toFixed(4)}</span>
                </div>
                <div style={{ fontSize: 12, color: '#999', marginTop: 8 }}>{budget.period}</div>
              </Card>
            </Col>
          ))}
        </Row>
      </Card>

      {/* 成本明细 */}
      <Card title="成本明细" extra={<Button icon={<DownloadOutlined />} onClick={handleExport}>导出数据</Button>}>
        <Table
          columns={columns}
          dataSource={costDetails}
          rowKey="id"
          loading={loading}
          pagination={{
            current: currentPage,
            pageSize: pageSize,
            total: totalDetails,
            showSizeChanger: true,
            showQuickJumper: true,
            showTotal: (total) => `共 ${total} 条`,
            onChange: (page, size) => {
              setCurrentPage(page);
              setPageSize(size);
            },
          }}
        />
      </Card>

      {/* 预算创建对话框 */}
      <Modal
        title="创建预算"
        open={budgetDialogVisible}
        onCancel={() => setBudgetDialogVisible(false)}
        onOk={handleCreateBudget}
        width={500}
      >
        <Form form={budgetForm} layout="vertical" initialValues={{ period: 'daily', alertThreshold: 80 }}>
          <Form.Item name="name" label="预算名称" rules={[{ required: true }]}>
            <Input placeholder="例如：日预算" />
          </Form.Item>
          <Form.Item name="limit" label="预算金额" rules={[{ required: true }]}>
            <InputNumber min={0} precision={4} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="period" label="周期">
            <Select
              options={[
                { value: 'daily', label: '每日' },
                { value: 'weekly', label: '每周' },
                { value: 'monthly', label: '每月' },
              ]}
            />
          </Form.Item>
          <Form.Item name="alertThreshold" label="预警阈值">
            <Slider marks={{ 50: '50%', 80: '80%', 95: '95%' }} min={0} max={100} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}