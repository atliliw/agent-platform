import { useState, useEffect } from 'react';
import {
  Card,
  Select,
  Button,
  Progress,
  Table,
  Tag,
  Descriptions,
  Alert,
  Drawer,
  Input,
  Switch,
  Row,
  Col,
  Statistic,
  Space,
  message,
  Modal,
  Form,
} from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { CaretRightOutlined, DownloadOutlined } from '@ant-design/icons';
import { evaluationApi } from '../../api';
import client from '../../api/client';
import type { EvalSuite, EvalReport, EvalResult, RunEvalConfig, Regression } from '../../api/evaluation';

// 获取得分颜色
const getScoreColor = (score: number): string => {
  if (score >= 0.8) return '#52c41a';
  if (score >= 0.6) return '#faad14';
  return '#ff4d4f';
};

// 获取得分状态类型
const getScoreType = (score: number): 'success' | 'warning' | 'error' => {
  if (score >= 0.8) return 'success';
  if (score >= 0.6) return 'warning';
  return 'error';
};

export default function EvalReport() {
  // 套件列表
  const [suites, setSuites] = useState<EvalSuite[]>([]);
  const [selectedSuite, setSelectedSuite] = useState<string>('');
  const [report, setReport] = useState<EvalReport | null>(null);
  const [loading, setLoading] = useState(false);

  // 详情抽屉
  const [detailVisible, setDetailVisible] = useState(false);
  const [selectedResult, setSelectedResult] = useState<EvalResult | null>(null);

  // 运行评测对话框
  const [runDialogVisible, setRunDialogVisible] = useState(false);
  const [runForm] = Form.useForm();

  // 加载套件列表
  const loadSuites = async () => {
    try {
      const data = await client.get('/api/v2/harness/eval/suites').catch(async () => {
        return await evaluationApi.getEvalSuites();
      });
      const suitesList = Array.isArray(data) ? data : [];
      setSuites(suitesList);
      if (suitesList.length > 0) {
        setSelectedSuite(suitesList[0].id);
        loadReport(suitesList[0].id);
      }
    } catch (error) {
      message.error('加载套件列表失败');
      console.error(error);
      setSuites([]);
    }
  };

  // 加载报告
  const loadReport = async (suiteId: string) => {
    if (!suiteId) return;
    setLoading(true);
    try {
      const data = await client.get('/api/v2/harness/eval/results', { params: { suite_id: suiteId } }).catch(async () => {
        return await evaluationApi.getEvalResults(suiteId);
      });
      setReport(data as unknown as EvalReport);
    } catch (error) {
      message.error('加载报告失败');
      console.error(error);
      setReport(null);
    } finally {
      setLoading(false);
    }
  };

  // 执行评测
  const executeRun = async () => {
    try {
      const values = await runForm.validateFields();
      const config: RunEvalConfig = {
        model: values.model,
        parallel: values.parallel,
        evaluate_trajectory: values.evaluateTrajectory,
        evaluate_react: values.evaluateReAct,
        compare_to: values.compareTo,
        save_baseline: values.saveBaseline,
      };

      setRunDialogVisible(false);
      message.info('评测正在运行，请稍候...');

      const data = await client.post('/api/v2/harness/eval/run', { suite_id: selectedSuite, ...config }).catch(async () => {
        return await evaluationApi.runEvaluation(selectedSuite, config);
      });
      setReport(data as unknown as EvalReport);
      message.success('评测完成');
    } catch (error) {
      message.error('运行评测失败');
      console.error(error);
    }
  };

  // 显示结果详情
  const showResultDetail = (result: EvalResult) => {
    setSelectedResult(result);
    setDetailVisible(true);
  };

  // 导出报告
  const exportReport = () => {
    if (!report) return;
    const data = JSON.stringify(report, null, 2);
    const blob = new Blob([data], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `eval-report-${selectedSuite}-${new Date().toISOString().split('T')[0]}.json`;
    a.click();
    URL.revokeObjectURL(url);
  };

  // 表格列定义
  const columns: ColumnsType<EvalResult> = [
    {
      title: '测试名称',
      dataIndex: 'name',
      key: 'name',
      width: 200,
    },
    {
      title: '得分',
      dataIndex: 'score',
      key: 'score',
      width: 100,
      sorter: true,
      render: (score: number) => (
        <Tag color={getScoreType(score)}>{(score * 100).toFixed(1)}%</Tag>
      ),
    },
    {
      title: '状态',
      dataIndex: 'passed',
      key: 'passed',
      width: 80,
      render: (passed: boolean) => (
        <Tag color={passed ? 'success' : 'error'}>{passed ? '通过' : '失败'}</Tag>
      ),
    },
    {
      title: '详细得分',
      key: 'scoreDetails',
      width: 400,
      render: (_, record) => (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <span style={{ width: 80, fontSize: 13 }}>忠实度</span>
            <Progress percent={record.score_details.faithfulness * 100} strokeWidth={8} showInfo={false} />
          </div>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <span style={{ width: 80, fontSize: 13 }}>相关性</span>
            <Progress percent={record.score_details.relevancy * 100} strokeWidth={8} showInfo={false} />
          </div>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <span style={{ width: 80, fontSize: 13 }}>精确度</span>
            <Progress percent={record.score_details.precision * 100} strokeWidth={8} showInfo={false} />
          </div>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <span style={{ width: 80, fontSize: 13 }}>推理质量</span>
            <Progress percent={record.score_details.reasoning * 100} strokeWidth={8} showInfo={false} />
          </div>
        </div>
      ),
    },
    {
      title: '耗时(ms)',
      dataIndex: 'duration_ms',
      key: 'duration_ms',
      width: 100,
      sorter: true,
    },
    {
      title: '步骤数',
      dataIndex: 'steps',
      key: 'steps',
      width: 80,
      sorter: true,
    },
    {
      title: '操作',
      key: 'action',
      fixed: 'right',
      width: 100,
      render: (_, record) => (
        <Button size="small" onClick={() => showResultDetail(record)}>
          详情
        </Button>
      ),
    },
  ];

  // 初始化加载
  useEffect(() => {
    loadSuites();
  }, []);

  // 初始化运行表单
  useEffect(() => {
    runForm.setFieldsValue({
      parallel: true,
      evaluateTrajectory: true,
      evaluateReAct: true,
      compareTo: false,
      saveBaseline: false,
    });
  }, []);

  return (
    <div className="eval-report">
      {/* 头部 */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 24 }}>
        <h2 style={{ margin: 0 }}>评测报告</h2>
        <Space>
          <Select
            style={{ width: 200 }}
            value={selectedSuite}
            onChange={(value) => {
              setSelectedSuite(value);
              loadReport(value);
            }}
            options={suites.map((s) => ({ value: s.id, label: s.name }))}
          />
          <Button type="primary" icon={<CaretRightOutlined />} onClick={() => setRunDialogVisible(true)}>
            运行评测
          </Button>
          <Button icon={<DownloadOutlined />} onClick={exportReport}>
            导出报告
          </Button>
        </Space>
      </div>

      {report && (
        <>
          {/* 报告概览 */}
          <Card style={{ marginBottom: 24 }}>
            <Row gutter={24} align="middle">
              <Col span={4}>
                <div style={{ textAlign: 'center' }}>
                  <Progress
                    type="dashboard"
                    percent={report.avg_score * 100}
                    strokeColor={getScoreColor(report.avg_score)}
                    format={(percent) => (
                      <div>
                        <span style={{ fontSize: 24, fontWeight: 'bold' }}>{percent?.toFixed(1)}</span>
                        <span style={{ fontSize: 14, color: '#999' }}>平均得分</span>
                      </div>
                    )}
                  />
                </div>
              </Col>
              <Col span={20}>
                <Row gutter={[24, 16]}>
                  <Col span={4}>
                    <Statistic title="总测试数" value={report.total_tests} />
                  </Col>
                  <Col span={4}>
                    <Statistic title="通过" value={report.passed_tests} valueStyle={{ color: '#3f8600' }} />
                  </Col>
                  <Col span={4}>
                    <Statistic title="失败" value={report.failed_tests} valueStyle={{ color: '#cf1322' }} />
                  </Col>
                  <Col span={4}>
                    <Statistic title="执行时间" value={report.duration_ms} suffix="ms" />
                  </Col>
                  <Col span={4}>
                    <Statistic title="Token消耗" value={report.tokens_used} />
                  </Col>
                  <Col span={4}>
                    <Statistic title="成本" value={report.cost} prefix="$" precision={4} />
                  </Col>
                </Row>
              </Col>
            </Row>
          </Card>

          {/* 分类得分 */}
          <Card title="分类得分" style={{ marginBottom: 24 }}>
            <Row gutter={16}>
              {Object.entries(report.score_by_category).map(([category, score]) => (
                <Col span={4} key={category}>
                  <Card>
                    <div style={{ fontWeight: 500, marginBottom: 12 }}>{category}</div>
                    <Progress percent={score * 100} strokeColor={getScoreColor(score)} />
                    <div style={{ fontSize: 24, fontWeight: 'bold', marginTop: 8 }}>
                      {(score * 100).toFixed(1)}%
                    </div>
                  </Card>
                </Col>
              ))}
            </Row>
          </Card>

          {/* 回归警告 */}
          {report.regressions && report.regressions.length > 0 && (
            <Card title="回归警告" style={{ marginBottom: 24 }}>
              {report.regressions.map((reg: Regression, idx: number) => (
                <Alert
                  key={idx}
                  message={reg.case_name}
                  description={`得分下降 ${reg.delta.toFixed(2)} (从 ${reg.before_score.toFixed(2)} 到 ${reg.after_score.toFixed(2)})`}
                  type={reg.severity === 'high' ? 'error' : reg.severity === 'medium' ? 'warning' : 'info'}
                  showIcon
                  style={{ marginBottom: 12 }}
                />
              ))}
            </Card>
          )}

          {/* 性能指标 */}
          {report.metrics_summary && (
            <Card title="性能指标" style={{ marginBottom: 24 }}>
              <Row gutter={16}>
                <Col span={4}>
                  <Card>
                    <Statistic
                      title="平均步骤数"
                      value={report.metrics_summary.avg_steps.toFixed(1)}
                    />
                  </Card>
                </Col>
                <Col span={4}>
                  <Card>
                    <Statistic
                      title="平均延迟"
                      value={report.metrics_summary.avg_latency_ms.toFixed(0)}
                      suffix="ms"
                    />
                  </Card>
                </Col>
                <Col span={4}>
                  <Card>
                    <Statistic
                      title="工具调用总数"
                      value={report.metrics_summary.total_tool_calls}
                    />
                  </Card>
                </Col>
                <Col span={4}>
                  <Card>
                    <Statistic
                      title="工具成功率"
                      value={report.metrics_summary.tool_success_rate * 100}
                      precision={1}
                      suffix="%"
                    />
                  </Card>
                </Col>
              </Row>
            </Card>
          )}

          {/* 详细结果表格 */}
          <Card title="详细结果">
            <Table
              columns={columns}
              dataSource={report.results}
              rowKey="id"
              loading={loading}
              scroll={{ x: 1200 }}
            />
          </Card>
        </>
      )}

      {/* 结果详情抽屉 */}
      <Drawer
        title="测试详情"
        placement="right"
        width="70%"
        open={detailVisible}
        onClose={() => setDetailVisible(false)}
      >
        {selectedResult && (
          <div>
            <Descriptions bordered column={2}>
              <Descriptions.Item label="测试名称">{selectedResult.name}</Descriptions.Item>
              <Descriptions.Item label="得分">
                <Tag color={getScoreType(selectedResult.score)}>
                  {(selectedResult.score * 100).toFixed(1)}%
                </Tag>
              </Descriptions.Item>
              <Descriptions.Item label="执行时间">{selectedResult.duration_ms}ms</Descriptions.Item>
              <Descriptions.Item label="步骤数">{selectedResult.steps}</Descriptions.Item>
            </Descriptions>

            {selectedResult.output && (
              <Card title="输出内容" style={{ marginTop: 24 }}>
                <Input.TextArea value={selectedResult.output} rows={6} readOnly />
              </Card>
            )}

            {selectedResult.tool_calls && selectedResult.tool_calls.length > 0 && (
              <Card title="工具调用记录" style={{ marginTop: 24 }}>
                <Table
                  dataSource={selectedResult.tool_calls}
                  rowKey="name"
                  columns={[
                    { title: '工具名称', dataIndex: 'name', key: 'name', width: 150 },
                    {
                      title: '成功',
                      dataIndex: 'success',
                      key: 'success',
                      width: 80,
                      render: (s: boolean) => <Tag color={s ? 'success' : 'error'}>{s ? '成功' : '失败'}</Tag>,
                    },
                    { title: '耗时(ms)', dataIndex: 'duration_ms', key: 'duration_ms', width: 100 },
                    { title: '重试次数', dataIndex: 'retry_count', key: 'retry_count', width: 80 },
                  ]}
                  size="small"
                />
              </Card>
            )}

            {selectedResult.trajectory && (
              <Card title="执行路径分析" style={{ marginTop: 24 }}>
                <Descriptions bordered column={1}>
                  <Descriptions.Item label="路径效率">
                    {(selectedResult.trajectory.efficiency * 100).toFixed(1)}%
                  </Descriptions.Item>
                  <Descriptions.Item label="冗余步骤">
                    {selectedResult.trajectory.redundant_steps}
                  </Descriptions.Item>
                  <Descriptions.Item label="缺失步骤">
                    {selectedResult.trajectory.missing_steps?.join(', ') || '无'}
                  </Descriptions.Item>
                </Descriptions>
              </Card>
            )}

            {selectedResult.react_score && (
              <Card title="ReAct 评估" style={{ marginTop: 24 }}>
                <Descriptions bordered column={2}>
                  <Descriptions.Item label="推理质量">
                    {(selectedResult.react_score.reasoning_quality * 100).toFixed(1)}%
                  </Descriptions.Item>
                  <Descriptions.Item label="行动相关性">
                    {(selectedResult.react_score.action_relevance * 100).toFixed(1)}%
                  </Descriptions.Item>
                  <Descriptions.Item label="思考-行动一致性">
                    {(selectedResult.react_score.thought_action_coherence * 100).toFixed(1)}%
                  </Descriptions.Item>
                  <Descriptions.Item label="错误处理">
                    {(selectedResult.react_score.error_handling * 100).toFixed(1)}%
                  </Descriptions.Item>
                </Descriptions>
              </Card>
            )}
          </div>
        )}
      </Drawer>

      {/* 运行评测对话框 */}
      <Modal
        title="运行评测"
        open={runDialogVisible}
        onCancel={() => setRunDialogVisible(false)}
        onOk={executeRun}
        width={500}
      >
        <Form form={runForm} layout="vertical">
          <Form.Item name="model" label="评测模型">
            <Input placeholder="例如：qwen-plus" />
          </Form.Item>
          <Form.Item name="parallel" label="并行执行" valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item name="evaluateTrajectory" label="轨迹评估" valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item name="evaluateReAct" label="ReAct评估" valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item name="compareTo" label="对比基线" valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item name="saveBaseline" label="保存为基线" valuePropName="checked">
            <Switch />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}