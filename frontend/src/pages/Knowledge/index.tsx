import { useState, useEffect } from 'react';
import { Card, Upload, Button, Table, Input, Select, Space, Tag, Progress, message } from 'antd';
import { InboxOutlined, DeleteOutlined, SearchOutlined, FileTextOutlined } from '@ant-design/icons';
import { knowledgeApi } from '../../api';
import { EmptyState } from '../../components/Common';
import type { Document, SearchResult, UploadConfig } from '../../types';

const { Dragger } = Upload;

export default function KnowledgePage() {
  const [documents, setDocuments] = useState<Document[]>([]);
  const [loading, setLoading] = useState(false);
  const [uploadProgress, setUploadProgress] = useState<number | null>(null);
  const [searchResults, setSearchResults] = useState<SearchResult[]>([]);
  const [searchQuery, setSearchQuery] = useState('');

  const [uploadConfig] = useState<UploadConfig>({
    chunk_strategy: 'token',
    chunk_size: 512,
    chunk_overlap: 50,
  });

  useEffect(() => {
    loadDocuments();
  }, []);

  const loadDocuments = async () => {
    setLoading(true);
    try {
      const res = await knowledgeApi.listDocuments();
      setDocuments(res.data.documents || []);
    } catch (error) {
      console.error('Load documents failed:', error);
    } finally {
      setLoading(false);
    }
  };

  const handleUpload = async (file: File) => {
    setUploadProgress(0);
    try {
      await knowledgeApi.uploadFile(file, uploadConfig, setUploadProgress);
      message.success('上传成功');
      loadDocuments();
    } catch (error) {
      message.error('上传失败');
    } finally {
      setUploadProgress(null);
    }
  };

  const handleDelete = async (id: string) => {
    try {
      await knowledgeApi.deleteDocument(id);
      message.success('删除成功');
      loadDocuments();
    } catch (error) {
      message.error('删除失败');
    }
  };

  const handleSearch = async () => {
    if (!searchQuery.trim()) return;
    try {
      const res = await knowledgeApi.search({
        query: searchQuery,
        top_k: 10,
        search_type: 'hybrid',
      });
      setSearchResults(res.data.results || []);
    } catch (error) {
      message.error('搜索失败');
    }
  };

  const columns = [
    {
      title: '文件名',
      dataIndex: 'filename',
      key: 'filename',
      render: (name: string) => (
        <Space>
          <FileTextOutlined />
          {name}
        </Space>
      ),
    },
    {
      title: '分块数',
      dataIndex: 'chunk_count',
      key: 'chunk_count',
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      render: (status: string) => (
        <Tag color={status === 'ready' ? 'green' : 'blue'}>{status}</Tag>
      ),
    },
    {
      title: '创建时间',
      dataIndex: 'created_at',
      key: 'created_at',
    },
    {
      title: '操作',
      key: 'action',
      render: (_: unknown, record: Document) => (
        <Button
          type="text"
          danger
          icon={<DeleteOutlined />}
          onClick={() => handleDelete(record.id)}
        >
          删除
        </Button>
      ),
    },
  ];

  return (
    <div>
      <h2 style={{ marginBottom: 24 }}>知识库管理</h2>

      <Card title="上传文档" style={{ marginBottom: 24 }}>
        <Dragger
          accept=".pdf,.doc,.docx,.md,.txt,.json,.csv"
          showUploadList={false}
          beforeUpload={(file) => {
            handleUpload(file);
            return false;
          }}
        >
          <p className="ant-upload-drag-icon">
            <InboxOutlined />
          </p>
          <p className="ant-upload-text">点击或拖拽文件到此区域上传</p>
          <p className="ant-upload-hint">
            支持 PDF、Word、Markdown、TXT、JSON、CSV 格式
          </p>
        </Dragger>
        {uploadProgress !== null && (
          <Progress percent={uploadProgress} style={{ marginTop: 16 }} />
        )}
      </Card>

      <Card title="文档列表" style={{ marginBottom: 24 }}>
        <Table
          columns={columns}
          dataSource={documents}
          rowKey="id"
          loading={loading}
          pagination={{ pageSize: 10 }}
        />
      </Card>

      <Card title="检索测试">
        <Space.Compact style={{ width: '100%', marginBottom: 16 }}>
          <Select defaultValue="hybrid" style={{ width: 120 }}>
            <Select.Option value="vector">向量检索</Select.Option>
            <Select.Option value="bm25">BM25</Select.Option>
            <Select.Option value="hybrid">混合检索</Select.Option>
          </Select>
          <Input
            placeholder="输入搜索内容"
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            onPressEnter={handleSearch}
          />
          <Button type="primary" icon={<SearchOutlined />} onClick={handleSearch}>
            搜索
          </Button>
        </Space.Compact>

        {searchResults.length > 0 ? (
          <div>
            {searchResults.map((result, index) => (
              <Card
                key={result.chunk_id}
                size="small"
                style={{ marginBottom: 8 }}
                title={
                  <span>
                    结果 {index + 1} - 相似度: {result.score.toFixed(3)}
                  </span>
                }
              >
                {result.content}
              </Card>
            ))}
          </div>
        ) : (
          <EmptyState description="输入关键词进行检索测试" />
        )}
      </Card>
    </div>
  );
}