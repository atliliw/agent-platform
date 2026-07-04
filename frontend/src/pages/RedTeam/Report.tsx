import { useState, useEffect } from "react";
import {
  Card, Row, Col, Statistic, Alert, Tag, Table, Button, Space,
  Badge, Collapse, Spin, message
} from "antd";
import {
  SecurityScanOutlined, ArrowLeftOutlined, DownloadOutlined,
  WarningOutlined, CheckCircleOutlined
} from "@ant-design/icons";
import { useNavigate, useParams } from "react-router-dom";
import { useRef } from "react";
import * as echarts from "echarts";
import { redteamApi, type RedTeamReport, type RedTeamTest } from "../../api/redteam";

const severityConfig: Record<string, { color: string; label: string }> = {
  critical: { color: "#ff4d4f", label: "严重" },
  high: { color: "#faad14", label: "高" },
  medium: { color: "#fa8c16", label: "中" },
  low: { color: "#52c41a", label: "低" },
};

const levelConfig: Record<string, { color: string; label: string }> = {
  critical: { color: "#ff4d4f", label: "严重风险" },
  high: { color: "#faad14", label: "高风险" },
  medium: { color: "#fa8c16", label: "中等风险" },
  low: { color: "#ad6800", label: "低风险" },
  secure: { color: "#52c41a", label: "安全" },
};

function SeverityPieChart({ data }: { data: { critical: number; high: number; medium: number; low: number } }) {
  const chartRef = useRef<HTMLDivElement>(null);
  const chartInstance = useRef<echarts.ECharts | null>(null);

  useEffect(() => {
    if (chartRef.current) {
      chartInstance.current = echarts.init(chartRef.current);
      chartInstance.current.setOption({
        tooltip: { trigger: "item", formatter: "{b}: {c} ({d}%)" },
        legend: { orient: "vertical", right: 10, top: "center" },
        series: [
          {
            type: "pie",
            radius: ["40%", "70%"],
            avoidLabelOverlap: false,
            itemStyle: { borderRadius: 10, borderColor: "#fff", borderWidth: 2 },
            label: { show: false, position: "center" },
            emphasis: { label: { show: true, fontSize: 20, fontWeight: "bold" } },
            labelLine: { show: false },
            data: [
              { value: data.critical, name: "严重", itemStyle: { color: "#ff4d4f" } },
              { value: data.high, name: "高", itemStyle: { color: "#faad14" } },
              { value: data.medium, name: "中", itemStyle: { color: "#fa8c16" } },
              { value: data.low, name: "低", itemStyle: { color: "#52c41a" } },
            ],
          },
        ],
      });
    }
    return () => {
      chartInstance.current?.dispose();
    };
  }, [data]);

  return <div ref={chartRef} style={{ width: "100%", height: 250 }} />;
}

function RiskGauge({ score }: { score: number }) {
  const chartRef = useRef<HTMLDivElement>(null);
  const chartInstance = useRef<echarts.ECharts | null>(null);

  useEffect(() => {
    if (chartRef.current) {
      chartInstance.current = echarts.init(chartRef.current);
      const color = score >= 70 ? "#ff4d4f" : score >= 40 ? "#faad14" : score >= 20 ? "#fa8c16" : "#52c41a";
      chartInstance.current.setOption({
        series: [
          {
            type: "gauge",
            startAngle: 200,
            endAngle: -20,
            min: 0,
            max: 100,
            splitNumber: 10,
            itemStyle: { color },
            progress: { show: true, width: 20 },
            pointer: { show: false },
            axisLine: { lineStyle: { width: 20, color: [[1, "#e5e5e5"]] } },
            axisTick: { show: false },
            splitLine: { show: false },
            axisLabel: { show: false },
            title: { show: false },
            detail: { valueAnimation: true, fontSize: 40, fontWeight: "bold", formatter: "{value}", color },
            data: [{ value: score }],
          },
        ],
      });
    }
    return () => {
      chartInstance.current?.dispose();
    };
  }, [score]);

  return <div ref={chartRef} style={{ width: "100%", height: 200 }} />;
}

function safeParseJson(value: unknown): unknown[] {
  if (Array.isArray(value)) return value;
  if (typeof value === "string") {
    try {
      const parsed = JSON.parse(value);
      return Array.isArray(parsed) ? parsed : [];
    } catch {
      return [];
    }
  }
  return [];
}

export default function ReportPage() {
  const navigate = useNavigate();
  const { test_id: testId } = useParams<{ test_id: string }>();
  const [loading, setLoading] = useState(true);
  const [report, setReport] = useState<RedTeamReport | null>(null);
  const [testData, setTestData] = useState<RedTeamTest | null>(null);
  const [attackResults, setAttackResults] = useState<any[]>([]);

  useEffect(() => {
    if (testId) {
      loadReport(testId);
      loadAttackResults(testId);
      loadTestData(testId);
    }
  }, [testId]);

  const loadTestData = async (id: string) => {
    try {
      const res = await redteamApi.getTest(id) as any;
      setTestData(res?.test || res?.data?.test || null);
    } catch {
      setTestData(null);
    }
  };

  const loadReport = async (id: string) => {
    setLoading(true);
    try {
      const res = await redteamApi.getReport(id) as any;
      setReport(res?.report || null);
    } catch {
      setReport(null);
    } finally {
      setLoading(false);
    }
  };

  const loadAttackResults = async (id: string) => {
    try {
      const res = await redteamApi.getAttacks(id) as any;
      setAttackResults(res?.attacks || []);
    } catch {
      setAttackResults([]);
    }
  };

  const downloadReport = () => {
    if (report) {
      const blob = new Blob([JSON.stringify(report, null, 2)], { type: "application/json" });
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `redteam-report-${report.test_id}.json`;
      a.click();
      URL.revokeObjectURL(url);
      message.success("报告下载成功");
    }
  };

  if (loading) {
    return <div style={{ textAlign: "center", padding: 100 }}><Spin size="large" /><div style={{ marginTop: 16 }}>加载报告数据...</div></div>;
  }

  if (!report) {
    return <div><Button icon={<ArrowLeftOutlined />} onClick={() => navigate("/redteam")}>返回列表</Button><Alert message="未找到测试报告" type="error" style={{ marginTop: 24 }} /></div>;
  }

  // Parse JSON string fields from protobuf
  const vulnerabilities = safeParseJson(report.vulnerabilities);
  const recommendations = safeParseJson(report.recommendations);

  // Construct severity distribution from separate count fields
  const severityDistribution = {
    critical: report.critical_count || 0,
    high: report.high_count || 0,
    medium: report.medium_count || 0,
    low: report.low_count || 0,
  };

  // In red team context: failed_attacks = attacks that penetrated (vulnerabilities found)
  const vulnerabilitiesFound = report.failed_attacks || 0;
  const blockedAttacks = report.blocked_attacks || 0;
  const totalAttacks = report.total_attacks || 0;

  // Get test metadata from separate fetch
  const testName = testData?.name || testId;
  const agentId = testData?.agent_id || testData?.agent_id || "-";
  const model = testData?.model || "-";
  const category = testData?.category || testData?.category || "-";

  const level = levelConfig[report.security_level] || levelConfig[report.security_level] || levelConfig.medium;

  const vulColumns = [
    { title: "攻击名称", dataIndex: "name", key: "name", render: (val: string, _: unknown, idx: number) => val || `漏洞 #${idx + 1}` },
    { title: "类别", dataIndex: "type", key: "type", render: (c: string) => <Tag>{c || "-"}</Tag> },
    { title: "严重性", dataIndex: "severity", key: "severity", render: (s: string) => <Tag color={(severityConfig[s] || {}).color || "default"}>{(severityConfig[s] || {}).label || s}</Tag> },
    { title: "输入提示", dataIndex: "payload", key: "payload", ellipsis: true, width: 200 },
    { title: "检测原因", dataIndex: "description", key: "description", ellipsis: true, render: (val: string, rec: Record<string, unknown>) => val || rec.response || "-" },
  ];

  const resultColumns = [
    { title: "攻击类型", dataIndex: "attack_type", key: "attack_type", render: (val: string, rec: Record<string, unknown>) => val || rec.attack_type || "-" },
    { title: "输入提示", dataIndex: "payload", key: "payload", ellipsis: true, width: 200, render: (val: string, rec: Record<string, unknown>) => val || rec.input_prompt || "-" },
    { title: "预期行为", dataIndex: "expected", key: "expected", ellipsis: true, width: 150, render: (val: string, rec: Record<string, unknown>) => val || rec.expected_behavior || "-" },
    { title: "实际响应", dataIndex: "actual", key: "actual", ellipsis: true, width: 200, render: (val: string, rec: Record<string, unknown>) => val || rec.agent_response || "-" },
    { title: "结果", dataIndex: "passed", key: "passed", render: (p: boolean) => <Badge status={p ? "success" : "error"} text={p ? "通过" : "失败"} /> },
    { title: "耗时", dataIndex: "duration", key: "duration", render: (val: number, rec: Record<string, unknown>) => val != null ? `${val}ms` : rec.duration_ms != null ? `${rec.duration_ms}ms` : "-" },
  ];

  return (
    <div>
      <h2 style={{ marginBottom: 24 }}>
        <Button icon={<ArrowLeftOutlined />} onClick={() => navigate("/redteam")} style={{ marginRight: 8 }} />
        <SecurityScanOutlined style={{ marginRight: 8 }} />
        {testName} - 安全报告
      </h2>

      <Row gutter={16} style={{ marginBottom: 24 }}>
        <Col span={6}><Card><Statistic title="测试 Agent" value={agentId} /></Card></Col>
        <Col span={6}><Card><Statistic title="使用模型" value={model} /></Card></Col>
        <Col span={6}><Card><Statistic title="攻击总数" value={totalAttacks} /></Card></Col>
        <Col span={6}><Card><Statistic title="发现漏洞" value={vulnerabilitiesFound} valueStyle={{ color: vulnerabilitiesFound > 0 ? "#ff4d4f" : "#52c41a" }} /></Card></Col>
      </Row>

      <Row gutter={16} style={{ marginBottom: 24 }}>
        <Col span={8}>
          <Card title="风险评分">
            <RiskGauge score={report.risk_score || report.risk_score || 0} />
            <div style={{ textAlign: "center", marginTop: 16 }}><Tag color={level.color} style={{ fontSize: 16, padding: "4px 12px" }}>{level.label}</Tag></div>
          </Card>
        </Col>
        <Col span={8}>
          <Card title="严重性分布"><SeverityPieChart data={severityDistribution} /></Card>
        </Col>
        <Col span={8}>
          <Card title="安全状态">
            {vulnerabilitiesFound === 0 ? (
              <div style={{ textAlign: "center", padding: 40 }}><CheckCircleOutlined style={{ fontSize: 60, color: "#52c41a" }} /><div style={{ marginTop: 16, fontSize: 18, color: "#52c41a" }}>测试通过</div><div style={{ marginTop: 8, color: "#666" }}>所有攻击均被成功防御</div></div>
            ) : (
              <div style={{ textAlign: "center", padding: 40 }}><WarningOutlined style={{ fontSize: 60, color: "#ff4d4f" }} /><div style={{ marginTop: 16, fontSize: 18, color: "#ff4d4f" }}>发现漏洞</div><div style={{ marginTop: 8, color: "#666" }}>需要关注并修复安全问题</div></div>
            )}
          </Card>
        </Col>
      </Row>

      <Card title="漏洞列表" style={{ marginBottom: 24 }} extra={<Badge count={vulnerabilities.length} />}>
        {vulnerabilities.length > 0 ? <Table columns={vulColumns} dataSource={vulnerabilities} rowKey={(_, idx) => `vul-${idx}`} /> : <Alert message="未发现安全漏洞" type="success" showIcon />}
      </Card>

      <Card title="改进建议" style={{ marginBottom: 24 }} extra={<Badge count={recommendations.length} style={{ backgroundColor: "#1677ff" }} />}>
        <Collapse accordion>
          {recommendations.map((rec, idx) => {
            const r = rec as Record<string, unknown>;
            const priority = (r.priority as string) || "medium";
            const title = (r.title as string) || `建议 #${idx + 1}`;
            const description = (r.description as string) || "-";
            const actions = (r.actions as string) || (r.remediation as string) || "-";
            return (
              <Collapse.Panel key={`rec-${idx}`} header={<Space><Tag color={(severityConfig[priority] || {}).color || "default"}>{(severityConfig[priority] || {}).label || priority}</Tag><span>{title}</span></Space>}>
                <p><strong>问题描述:</strong> {description}</p>
                <p><strong>修复建议:</strong> {actions}</p>
              </Collapse.Panel>
            );
          })}
        </Collapse>
      </Card>

      <Card title="详细攻击结果" extra={<Button icon={<DownloadOutlined />} onClick={downloadReport}>下载报告</Button>}>
        <Table columns={resultColumns} dataSource={attackResults} rowKey={(_, idx) => `attack-${idx}`} locale={{ emptyText: "暂无详细攻击结果数据" }} />
      </Card>
    </div>
  );
}
