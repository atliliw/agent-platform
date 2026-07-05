import { useState, useEffect, useCallback } from "react";
import {
  Card, Row, Col, Statistic, Table, Tag, Button, Empty, Spin,
  Typography, Space, Tooltip, Progress, message
} from "antd";
import {
  DashboardOutlined, PlayCircleOutlined, CheckCircleOutlined,
  CloseCircleOutlined, ClockCircleOutlined, BarChartOutlined,
  HistoryOutlined, ThunderboltOutlined
} from "@ant-design/icons";
import { useNavigate } from "react-router-dom";
import { ragApi, type RAGMetrics } from "../../api/rag";

const { Title } = Typography;

function getScoreColor(score: number): string {
  if (score >= 0.8) return "#52c41a";
  if (score >= 0.6) return "#1677ff";
  if (score >= 0.4) return "#faad14";
  return "#ff4d4f";
}

function getScoreTag(score: number): React.ReactNode {
  const color = score >= 0.7 ? "green" : score >= 0.5 ? "blue" : score >= 0.3 ? "orange" : "red";
  return <Tag color={color}>{(score * 100).toFixed(1)}%</Tag>;
}

function formatTimestamp(ts: number): string {
  if (!ts) return "-";
  return new Date(ts * 1000).toLocaleString("zh-CN");
}

const METRIC_COLUMNS = [
  {
    title: "Query",
    dataIndex: "query",
    key: "query",
    ellipsis: true,
    width: 180,
    render: (q: string) => <Tooltip title={q}><span>{q || "-"}</span></Tooltip>,
  },
  {
    title: "RAGAS",
    dataIndex: "ragas_score",
    key: "ragas_score",
    width: 85,
    sorter: (a: RAGMetrics, b: RAGMetrics) => (a.ragas_score || 0) - (b.ragas_score || 0),
    render: (s: number) => getScoreTag(s),
  },
  {
    title: "Ctx Precision",
    dataIndex: "context_precision",
    key: "context_precision",
    width: 100,
    render: (s: number) => getScoreTag(s),
  },
  {
    title: "Ctx Recall",
    dataIndex: "context_recall",
    key: "context_recall",
    width: 90,
    render: (s: number) => getScoreTag(s),
  },
  {
    title: "Faithfulness",
    dataIndex: "faithfulness",
    key: "faithfulness",
    width: 95,
    render: (s: number) => getScoreTag(s),
  },
  {
    title: "Hallucination",
    dataIndex: "hallucination",
    key: "hallucination",
    width: 95,
    render: (s: number) => {
      if (s === undefined || s === null) return <Tag>-</Tag>;
      const color = s <= 0.2 ? "green" : s <= 0.5 ? "orange" : "red";
      return <Tag color={color}>{(s * 100).toFixed(1)}%</Tag>;
    },
  },
  {
    title: "Coherence",
    dataIndex: "coherence",
    key: "coherence",
    width: 85,
    render: (s: number) => getScoreTag(s),
  },
  {
    title: "Time",
    dataIndex: "timestamp",
    key: "timestamp",
    width: 150,
    sorter: (a: RAGMetrics, b: RAGMetrics) => (a.timestamp || 0) - (b.timestamp || 0),
    render: (ts: number) => formatTimestamp(ts),
  },
];

const EVAL_COLUMNS = [
  {
    title: "Name",
    dataIndex: "name",
    key: "name",
    ellipsis: true,
    width: 180,
  },
  {
    title: "Status",
    dataIndex: "status",
    key: "status",
    width: 100,
    render: (s: string) => {
      const map: Record<string, { color: string; icon: React.ReactNode }> = {
        pending: { color: "default", icon: <ClockCircleOutlined /> },
        running: { color: "processing", icon: <ThunderboltOutlined /> },
        completed: { color: "success", icon: <CheckCircleOutlined /> },
        failed: { color: "error", icon: <CloseCircleOutlined /> },
      };
      const cfg = map[s] || { color: "default", icon: null };
      return <Tag color={cfg.color} icon={cfg.icon}>{s}</Tag>;
    },
  },
  {
    title: "Queries",
    dataIndex: "queries",
    key: "queries",
    width: 80,
    render: (q: unknown[]) => (Array.isArray(q) ? q.length : 0),
  },
  {
    title: "Created",
    dataIndex: "created_at",
    key: "created_at",
    width: 160,
    render: (ts: number) => formatTimestamp(ts),
  },
];

export default function RAGMetricsPage() {
  const navigate = useNavigate();
  const [metrics, setMetrics] = useState<RAGMetrics[]>([]);
  const [evaluations, setEvaluations] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [evalLoading, setEvalLoading] = useState(false);

  const loadMetrics = useCallback(async () => {
    setLoading(true);
    try {
      const res = await ragApi.listMetrics({ limit: 50 }) as any;
      setMetrics(res?.metrics || []);
    } catch {
      setMetrics([]);
    } finally {
      setLoading(false);
    }
  }, []);

  const loadEvaluations = useCallback(async () => {
    setEvalLoading(true);
    try {
      const res = await ragApi.listEvaluations() as any;
      setEvaluations(res?.evaluations || []);
    } catch {
      setEvaluations([]);
    } finally {
      setEvalLoading(false);
    }
  }, []);

  useEffect(() => {
    loadMetrics();
    loadEvaluations();
  }, [loadMetrics, loadEvaluations]);

  // Compute overview stats from metrics
  const totalCount = metrics.length;
  const avgRagas = totalCount > 0
    ? metrics.reduce((sum, m) => sum + (m.ragas_score || 0), 0) / totalCount
    : 0;
  const passCount = metrics.filter(m => (m.ragas_score || 0) >= 0.7).length;
  const passRate = totalCount > 0 ? Math.round(passCount / totalCount * 100) : 0;
  const latestTime = totalCount > 0
    ? Math.max(...metrics.map(m => m.timestamp || 0))
    : 0;

  // Average per-metric scores
  const avgContextPrecision = totalCount > 0
    ? metrics.reduce((s, m) => s + (m.context_precision || 0), 0) / totalCount : 0;
  const avgFaithfulness = totalCount > 0
    ? metrics.reduce((s, m) => s + (m.faithfulness || 0), 0) / totalCount : 0;
  const avgHallucination = totalCount > 0
    ? metrics.reduce((s, m) => s + (m.hallucination || 0), 0) / totalCount : 0;
  const avgCoherence = totalCount > 0
    ? metrics.reduce((s, m) => s + (m.coherence || 0), 0) / totalCount : 0;

  const handleRunEvaluation = async (id: string) => {
    try {
      await ragApi.runEvaluation(id);
      message.success("评估任务已启动");
      loadEvaluations();
      loadMetrics();
    } catch (e: any) {
      message.error(e.message || "启动评估失败");
    }
  };

  return (
    <div>
      <Title level={3}>
        <DashboardOutlined /> RAG Metrics Dashboard
      </Title>

      {/* Overview Cards */}
      <Row gutter={16} style={{ marginBottom: 24 }}>
        <Col span={6}>
          <Card>
            <Statistic
              title="总评估次数"
              value={totalCount}
              prefix={<BarChartOutlined />}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic
              title="平均 RAGAS Score"
              value={(avgRagas * 100).toFixed(1)}
              suffix="%"
              valueStyle={{ color: getScoreColor(avgRagas) }}
            />
            <Progress
              percent={Math.round(avgRagas * 100)}
              strokeColor={getScoreColor(avgRagas)}
              showInfo={false}
              size="small"
              style={{ marginTop: 8 }}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic
              title="通过率 (RAGAS≥0.7)"
              value={passRate}
              suffix="%"
              prefix={passRate >= 70 ? <CheckCircleOutlined /> : <CloseCircleOutlined />}
              valueStyle={{ color: passRate >= 70 ? "#52c41a" : "#ff4d4f" }}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic
              title="最近评估"
              value={latestTime > 0 ? formatTimestamp(latestTime) : "-"}
              prefix={<ClockCircleOutlined />}
              valueStyle={{ fontSize: 14 }}
            />
          </Card>
        </Col>
      </Row>

      {/* Metric Averages */}
      <Row gutter={16} style={{ marginBottom: 24 }}>
        <Col span={6}>
          <Card>
            <Statistic
              title="Context Precision 均值"
              value={(avgContextPrecision * 100).toFixed(1)}
              suffix="%"
              valueStyle={{ color: getScoreColor(avgContextPrecision) }}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic
              title="Faithfulness 均值"
              value={(avgFaithfulness * 100).toFixed(1)}
              suffix="%"
              valueStyle={{ color: getScoreColor(avgFaithfulness) }}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic
              title="平均幻觉率"
              value={(avgHallucination * 100).toFixed(1)}
              suffix="%"
              valueStyle={{ color: avgHallucination <= 0.2 ? "#52c41a" : avgHallucination <= 0.5 ? "#faad14" : "#ff4d4f" }}
            />
            <div style={{ fontSize: 12, color: "#999", marginTop: 4 }}>越低越好</div>
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic
              title="Coherence 均值"
              value={(avgCoherence * 100).toFixed(1)}
              suffix="%"
              valueStyle={{ color: getScoreColor(avgCoherence) }}
            />
          </Card>
        </Col>
      </Row>

      {/* Action Bar */}
      <Row justify="space-between" align="middle" style={{ marginBottom: 16 }}>
        <Col>
          <Space>
            <Button
              type="primary"
              icon={<PlayCircleOutlined />}
              onClick={() => navigate("/rag-metrics/evaluate")}
            >
              新建评估
            </Button>
            <Button
              icon={<HistoryOutlined />}
              onClick={() => { loadMetrics(); loadEvaluations(); }}
            >
              刷新数据
            </Button>
          </Space>
        </Col>
      </Row>

      {/* Metrics Table */}
      <Card
        title={<Space><BarChartOutlined /> 评估指标记录</Space>}
        style={{ marginBottom: 24 }}
      >
        {loading ? (
          <Spin />
        ) : totalCount === 0 ? (
          <Empty description="暂无评估数据，请先进行一次评估" />
        ) : (
          <Table
            columns={METRIC_COLUMNS}
            dataSource={metrics}
            rowKey="id"
            pagination={{ pageSize: 10, showSizeChanger: true }}
            onRow={(record) => ({
              onClick: () => navigate(`/rag-metrics/${record.id}`),
              style: { cursor: "pointer" },
            })}
          />
        )}
      </Card>

      {/* Evaluation Tasks */}
      <Card
        title={<Space><HistoryOutlined /> 批量评估任务</Space>}
      >
        {evalLoading ? (
          <Spin />
        ) : evaluations.length === 0 ? (
          <Empty description="暂无批量评估任务" />
        ) : (
          <Table
            columns={[
              ...EVAL_COLUMNS,
              {
                title: "操作",
                key: "action",
                width: 140,
                render: (_: unknown, record: any) => (
                  <Space>
                    {record.status === "pending" && (
                      <Button
                        type="primary"
                        size="small"
                        icon={<PlayCircleOutlined />}
                        onClick={() => handleRunEvaluation(record.id)}
                      >
                        执行
                      </Button>
                    )}
                    {record.status === "completed" && (
                      <Button
                        size="small"
                        onClick={() => navigate(`/rag-metrics/${record.id}`)}
                      >
                        查看结果
                      </Button>
                    )}
                  </Space>
                ),
              },
            ]}
            dataSource={evaluations}
            rowKey="id"
            pagination={{ pageSize: 5 }}
          />
        )}
      </Card>
    </div>
  );
}
