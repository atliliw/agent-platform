import { useState } from "react";
import { Card, Form, Input, Button, Space, message, Typography, Spin, Progress, Row, Col, Alert, Descriptions } from "antd";
import { PlayCircleOutlined, PlusOutlined, DeleteOutlined } from "@ant-design/icons";
import { useNavigate } from "react-router-dom";
import { ragApi } from "../../api/rag";

const { TextArea } = Input;
const { Title } = Typography;

export default function RAGEvaluatePage() {
  const navigate = useNavigate();
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<any>(null);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = async (values: any) => {
    setLoading(true);
    setError(null);
    setResult(null);
    try {
      const evaluation = await ragApi.evaluate(values);
      message.success("评估完成");
      setResult(evaluation);
    } catch (e: any) {
      setError(e.message || "评估失败，请检查后端服务是否正常");
    } finally {
      setLoading(false);
    }
  };

  const ragasScore = result?.ragas_score || 0;

  return (
    <div>
      <Title level={3}><PlayCircleOutlined /> RAG Evaluation</Title>
      <Row gutter={24}>
        <Col span={12}>
          <Card title="输入">
            <Form form={form} layout="vertical" onFinish={handleSubmit}>
              <Form.Item name="query" label="Query" rules={[{ required: true }]}>
                <TextArea rows={3} placeholder="输入查询问题" />
              </Form.Item>
              <Form.Item label="Contexts（检索到的上下文）">
                <Form.List name="contexts">
                  {(fields, { add, remove }) => (
                    <>
                      {fields.map(({ key, name }) => (
                        <Space key={key} style={{ display: "flex", marginBottom: 8 }}>
                          <Form.Item name={name} rules={[{ required: true }]} style={{ flex: 1 }}>
                            <TextArea rows={2} style={{ width: 400 }} placeholder={`Context ${name + 1}`} />
                          </Form.Item>
                          <Button danger icon={<DeleteOutlined />} onClick={() => remove(name)} />
                        </Space>
                      ))}
                      <Button type="dashed" icon={<PlusOutlined />} onClick={() => add()}>
                        添加上下文
                      </Button>
                    </>
                  )}
                </Form.List>
              </Form.Item>
              <Form.Item name="answer" label="Answer（生成的回答）" rules={[{ required: true }]}>
                <TextArea rows={4} placeholder="输入模型生成的回答" />
              </Form.Item>
              <Form.Item name="ground_truth" label="Ground Truth（参考答案，可选）">
                <TextArea rows={2} placeholder="输入参考答案，用于计算 correctness" />
              </Form.Item>
              <Button type="primary" htmlType="submit" loading={loading}>
                开始评估
              </Button>
            </Form>
          </Card>
        </Col>
        <Col span={12}>
          <Card title="评估结果">
            {loading && <Spin tip="正在使用 LLM 评估中，请稍候..." />}
            {error && (
              <Alert type="error" message="评估失败" description={error} showIcon style={{ marginBottom: 16 }} />
            )}
            {result && (
              <Space direction="vertical" style={{ width: "100%" }} size="large">
                <div>
                  <Title level={5}>RAGAS Score</Title>
                  <Progress
                    type="dashboard"
                    percent={Math.round(ragasScore * 100)}
                    strokeColor={ragasScore >= 0.7 ? "#52c41a" : ragasScore >= 0.5 ? "#1677ff" : "#ff4d4f"}
                    format={(p) => `${p}%`}
                  />
                </div>
                <Descriptions bordered column={1} size="small">
                  <Descriptions.Item label="Context Precision">{((result.context_precision || 0) * 100).toFixed(1)}%</Descriptions.Item>
                  <Descriptions.Item label="Context Recall">{((result.context_recall || 0) * 100).toFixed(1)}%</Descriptions.Item>
                  <Descriptions.Item label="Context Relevancy">{((result.context_relevancy || 0) * 100).toFixed(1)}%</Descriptions.Item>
                  {result.context_entity_recall > 0 && (
                    <Descriptions.Item label="Context Entity Recall">{(result.context_entity_recall * 100).toFixed(1)}%</Descriptions.Item>
                  )}
                  <Descriptions.Item label="Faithfulness">{((result.faithfulness || 0) * 100).toFixed(1)}%</Descriptions.Item>
                  <Descriptions.Item label="Hallucination">
                    <span style={{ color: result.hallucination <= 0.2 ? "#52c41a" : result.hallucination <= 0.5 ? "#faad14" : "#ff4d4f" }}>
                      {((result.hallucination || 0) * 100).toFixed(1)}%
                    </span>
                    <span style={{ fontSize: 11, color: "#999" }}> (越低越好)</span>
                  </Descriptions.Item>
                  <Descriptions.Item label="Answer Relevancy">{((result.answer_relevancy || 0) * 100).toFixed(1)}%</Descriptions.Item>
                  {result.answer_correctness > 0 && (
                    <Descriptions.Item label="Answer Correctness">{(result.answer_correctness * 100).toFixed(1)}%</Descriptions.Item>
                  )}
                  {result.comprehensiveness > 0 && (
                    <Descriptions.Item label="Comprehensiveness">{(result.comprehensiveness * 100).toFixed(1)}%</Descriptions.Item>
                  )}
                  <Descriptions.Item label="Coherence">{((result.coherence || 0) * 100).toFixed(1)}%</Descriptions.Item>
                  {result.noise_sensitivity > 0 && (
                    <Descriptions.Item label="Noise Sensitivity">
                      <span style={{ color: result.noise_sensitivity <= 0.2 ? "#52c41a" : result.noise_sensitivity <= 0.5 ? "#faad14" : "#ff4d4f" }}>
                        {(result.noise_sensitivity * 100).toFixed(1)}%
                      </span>
                      <span style={{ fontSize: 11, color: "#999" }}> (越低越好)</span>
                    </Descriptions.Item>
                  )}
                  <Descriptions.Item label="MRR">{((result.mrr || 0) * 100).toFixed(1)}%</Descriptions.Item>
                  <Descriptions.Item label="NDCG">{((result.ndcg || 0) * 100).toFixed(1)}%</Descriptions.Item>
                </Descriptions>
                {result.id && (
                  <Button onClick={() => navigate("/rag-metrics/" + result.id)}>
                    查看详情
                  </Button>
                )}
              </Space>
            )}
            {!loading && !error && !result && (
              <Alert type="info" message="请填写左侧表单并点击「开始评估」" />
            )}
          </Card>
        </Col>
      </Row>
    </div>
  );
}
