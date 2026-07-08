import { useState, useEffect } from 'react';
import {
  Card, Row, Col, Input, Tag, Rate, Statistic, Empty, Spin, Space, Alert
} from 'antd';
import { SearchOutlined } from '@ant-design/icons';
import { harnessApi } from '../../api/harness';

interface CatalogItem {
  id: string;
  name: string;
  description: string;
  usage_count: number;
  rating: number;
  category: string;
  tags: string[];
  version: string;
}

export default function CatalogPanel() {
  const [catalog, setCatalog] = useState<CatalogItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [searchText, setSearchText] = useState('');
  const [selectedItem, setSelectedItem] = useState<CatalogItem | null>(null);

  useEffect(() => {
    loadCatalog();
  }, []);

  const loadCatalog = async () => {
    setLoading(true);
    try {
      const res = await harnessApi.listCatalog() as any;
      setCatalog(res?.agents || res?.catalog || []);
    } catch {
      setCatalog([]);
    } finally {
      setLoading(false);
    }
  };

  const filteredCatalog = catalog.filter((item) => {
    if (!searchText) return true;
    const lower = searchText.toLowerCase();
    return (
      item.name.toLowerCase().includes(lower) ||
      item.description.toLowerCase().includes(lower) ||
      item.category?.toLowerCase().includes(lower) ||
      item.tags?.some((t) => t.toLowerCase().includes(lower))
    );
  });

  const getCategoryColor = (cat: string) => {
    const map: Record<string, string> = {
      chat: 'blue',
      rag: 'green',
      tool: 'orange',
      workflow: 'purple',
      monitoring: 'cyan',
    };
    return map[cat] || 'default';
  };

  if (loading) {
    return <div style={{ textAlign: 'center', padding: 60 }}><Spin size="large" /></div>;
  }

  return (
    <div>
      <Alert message="Agent 目录 — 浏览和发现可用的 Agent" type="info" showIcon style={{ marginBottom: 16 }} />

      <Input
        placeholder="搜索 Agent 名称、描述、分类..."
        prefix={<SearchOutlined />}
        value={searchText}
        onChange={(e) => setSearchText(e.target.value)}
        style={{ marginBottom: 16, width: 400 }}
        allowClear
      />

      {selectedItem ? (
        <Card
          title={<span>{selectedItem.name} <Tag color={getCategoryColor(selectedItem.category)}>{selectedItem.category}</Tag></span>}
          extra={<a onClick={() => setSelectedItem(null)}>返回列表</a>}
        >
          <Row gutter={16}>
            <Col span={16}>
              <p style={{ color: 'rgba(0,0,0,0.65)', marginBottom: 16 }}>{selectedItem.description}</p>
              <div style={{ marginBottom: 12 }}>
                <strong>标签：</strong>
                {selectedItem.tags?.map((t) => <Tag key={t}>{t}</Tag>) || '无'}
              </div>
              <div style={{ marginBottom: 12 }}>
                <strong>版本：</strong> {selectedItem.version || '-'}
              </div>
            </Col>
            <Col span={8}>
              <Card size="small">
                <Statistic title="使用次数" value={selectedItem.usage_count} />
              </Card>
              <Card size="small" style={{ marginTop: 8 }}>
                <div style={{ textAlign: 'center' }}>
                  <div style={{ color: 'rgba(0,0,0,0.45)', fontSize: 14, marginBottom: 4 }}>评分</div>
                  <Rate disabled value={selectedItem.rating} allowHalf />
                </div>
              </Card>
            </Col>
          </Row>
        </Card>
      ) : (
        <Row gutter={[16, 16]}>
          {filteredCatalog.length === 0 ? (
            <Col span={24}>
              <Empty description="暂无 Agent 数据" />
            </Col>
          ) : (
            filteredCatalog.map((item) => (
              <Col span={8} key={item.id}>
                <Card
                  hoverable
                  onClick={() => setSelectedItem(item)}
                  size="small"
                >
                  <Card.Meta
                    title={
                      <Space>
                        {item.name}
                        <Tag color={getCategoryColor(item.category)}>{item.category}</Tag>
                      </Space>
                    }
                    description={item.description.length > 80 ? item.description.slice(0, 80) + '...' : item.description}
                  />
                  <div style={{ marginTop: 12, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                    <Statistic title="使用次数" value={item.usage_count} valueStyle={{ fontSize: 14 }} />
                    <Rate disabled value={item.rating} allowHalf style={{ fontSize: 12 }} />
                  </div>
                </Card>
              </Col>
            ))
          )}
        </Row>
      )}
    </div>
  );
}
