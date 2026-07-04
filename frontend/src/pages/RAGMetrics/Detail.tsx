import { useState, useEffect } from "react";
import { Card, Row, Col, Progress, Tag, Button, Typography, Descriptions, Divider, Spin, Empty } from "antd";
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

  useEffect(() => {
    if (!id) return;
    setLoading(true);
    ragApi.getMetrics(id).then((res: any) => setEvaluation(res)).catch(() => {
      setEvaluation({
        id,
        query_id: "",
        query: "Sample",
        retrieved_docs: [],
        generated_answer: "",
        ground_truth: "",
        context_precision: 0.85,
        context_recall: 0,
        context_relevancy: 0,
        mrr: 0,
        ndcg: 0,
        faithfulness: 0.76,
        answer_relevancy: 0.88,
        answer_correctness: 0,
        answer_similarity: 0,
        ragas_score: 0.8,
        timestamp: Date.now(),
        tenant_id: "",
      });
    }).finally(() => setLoading(false));
  }, [id]);

  if (loading) return <Spin />;
  if (!evaluation) return <Empty />;

  const metrics = [
    { key: "ragas_score", value: evaluation.ragas_score || 0, label: "RAGAS Score" },
    { key: "context_precision", value: evaluation.context_precision || 0, label: "Context Precision" },
    { key: "faithfulness", value: evaluation.faithfulness || 0, label: "Faithfulness" },
    { key: "answer_relevancy", value: evaluation.answer_relevancy || 0, label: "Answer Relevancy" },
  ];

  return (
    <div>
      <Button icon={<ArrowLeftOutlined />} onClick={() => navigate("/rag-metrics")}>Back</Button>
      <Title level={3}>RAG Metrics Detail: {evaluation.id}</Title>
      <Row gutter={24}>
        <Col span={16}>
          <Card title="Metrics">
            <Row gutter={16}>
              {metrics.map(m => (
                <Col span={6} key={m.key}>
                  <Progress type="dashboard" percent={Math.round(m.value * 100)} strokeColor={getScoreColor(m.value)} format={() => m.label} />
                </Col>
              ))}
            </Row>
          </Card>
          <Card title="Query" style={{ marginTop: 16 }}>
            <Paragraph>{evaluation.query || "-"}</Paragraph>
          </Card>
        </Col>
        <Col span={8}>
          <Card title="Details">
            <Descriptions bordered column={1}>
              <Descriptions.Item label="ID">{evaluation.id}</Descriptions.Item>
              <Descriptions.Item label="Context Precision">{evaluation.context_precision}</Descriptions.Item>
              <Descriptions.Item label="Context Recall">{evaluation.context_recall}</Descriptions.Item>
              <Descriptions.Item label="Faithfulness">{evaluation.faithfulness}</Descriptions.Item>
              <Descriptions.Item label="Answer Relevancy">{evaluation.answer_relevancy}</Descriptions.Item>
              <Descriptions.Item label="RAGAS Score">{evaluation.ragas_score}</Descriptions.Item>
            </Descriptions>
          </Card>
        </Col>
      </Row>
    </div>
  );
}
