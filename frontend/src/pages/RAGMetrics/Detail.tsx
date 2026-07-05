import { useState, useEffect } from "react";
import { Card, Row, Col, Progress, Button, Typography, Descriptions, Spin, Empty, Alert } from "antd";
import { ArrowLeftOutlined } from "@ant-design/icons";
import { useParams, useNavigate } from "react-router-dom";
import { ragApi } from "../../api/rag";

const { Title, Paragraph } = Typography;

function getScoreColor(score: number) {
  if (score >= 0.8) return "#52c41a";
  if (score >= 0.6) return "#1677ff";
  if (score >= 0.4) return "#faad14";
  return "#ff4d4f";
}

export default function RAGDetailPage() {
  const { id } = useParams();
  const navigate = useNavigate();
  const [loading, setLoading] = useState(false);
  const [evaluation, setEvaluation] = useState<any>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!id) return;
    setLoading(true);
    setError(null);
    ragApi.getMetrics(id)
      .then((res: any) => setEvaluation(res))
      .catch((e: any) => {
        setError(e.message || "加载失败");
        setEvaluation(null);
      })
      .finally(() => setLoading(false));
  }, [id]);

  if (loading) return <Spin tip="加载中..." />;
  if (error) return <Alert type="error" message="加载失败" description={error} showIcon />;
  if (!evaluation) return <Empty description="未找到评估数据" />;

  const metrics = [
    { key: "ragas_score", value: evaluation.ragas_score || 0, label: "RAGAS Score" },
    { key: "context_precision", value: evaluation.context_precision || 0, label: "Context Precision" },
    { key: "faithfulness", value: evaluation.faithfulness || 0, label: "Faithfulness" },
    { key: "answer_relevancy", value: evaluation.answer_relevancy || 0, label: "Answer Relevancy" },
  ];

  return (
    <div>
      <Button icon={<ArrowLeftOutlined />} onClick={() => navigate("/rag-metrics")}>
        返回列表
      </Button>
      <Title level={3}>RAG Metrics Detail: {evaluation.id || evaluation.query_id}</Title>
      <Row gutter={24}>
        <Col span={16}>
          <Card title="核心指标">
            <Row gutter={16}>
              {metrics.map(m => (
                <Col span={6} key={m.key}>
                  <Progress
                    type="dashboard"
                    percent={Math.round(m.value * 100)}
                    strokeColor={getScoreColor(m.value)}
                    format={() => m.label}
                  />
                  <div style={{ textAlign: "center", marginTop: 8, color: getScoreColor(m.value) }}>
                    {(m.value * 100).toFixed(1)}%
                  </div>
                </Col>
              ))}
            </Row>
          </Card>
          <Card title="Query" style={{ marginTop: 16 }}>
            <Paragraph>{evaluation.query || "-"}</Paragraph>
          </Card>
          {evaluation.generated_answer && (
            <Card title="Generated Answer" style={{ marginTop: 16 }}>
              <Paragraph>{evaluation.generated_answer}</Paragraph>
            </Card>
          )}
          {evaluation.ground_truth && (
            <Card title="Ground Truth" style={{ marginTop: 16 }}>
              <Paragraph>{evaluation.ground_truth}</Paragraph>
            </Card>
          )}
        </Col>
        <Col span={8}>
          <Card title="详细指标">
            <Descriptions bordered column={1} size="small">
              <Descriptions.Item label="ID">{evaluation.id}</Descriptions.Item>
              <Descriptions.Item label="Context Precision">
                <span style={{ color: getScoreColor(evaluation.context_precision) }}>
                  {((evaluation.context_precision || 0) * 100).toFixed(1)}%
                </span>
              </Descriptions.Item>
              <Descriptions.Item label="Context Recall">
                <span style={{ color: getScoreColor(evaluation.context_recall) }}>
                  {((evaluation.context_recall || 0) * 100).toFixed(1)}%
                </span>
              </Descriptions.Item>
              <Descriptions.Item label="Context Relevancy">
                <span style={{ color: getScoreColor(evaluation.context_relevancy) }}>
                  {((evaluation.context_relevancy || 0) * 100).toFixed(1)}%
                </span>
              </Descriptions.Item>
              <Descriptions.Item label="Faithfulness">
                <span style={{ color: getScoreColor(evaluation.faithfulness) }}>
                  {((evaluation.faithfulness || 0) * 100).toFixed(1)}%
                </span>
              </Descriptions.Item>
              <Descriptions.Item label="Answer Relevancy">
                <span style={{ color: getScoreColor(evaluation.answer_relevancy) }}>
                  {((evaluation.answer_relevancy || 0) * 100).toFixed(1)}%
                </span>
              </Descriptions.Item>
              <Descriptions.Item label="Answer Correctness">
                <span style={{ color: getScoreColor(evaluation.answer_correctness) }}>
                  {((evaluation.answer_correctness || 0) * 100).toFixed(1)}%
                </span>
              </Descriptions.Item>
              <Descriptions.Item label="MRR">
                {((evaluation.mrr || 0) * 100).toFixed(1)}%
              </Descriptions.Item>
              <Descriptions.Item label="NDCG">
                {((evaluation.ndcg || 0) * 100).toFixed(1)}%
              </Descriptions.Item>
              <Descriptions.Item label="RAGAS Score">
                <span style={{ color: getScoreColor(evaluation.ragas_score), fontWeight: "bold" }}>
                  {((evaluation.ragas_score || 0) * 100).toFixed(1)}%
                </span>
              </Descriptions.Item>
              {evaluation.timestamp && (
                <Descriptions.Item label="Time">
                  {new Date(evaluation.timestamp * 1000).toLocaleString("zh-CN")}
                </Descriptions.Item>
              )}
            </Descriptions>
          </Card>
        </Col>
      </Row>
    </div>
  );
}
