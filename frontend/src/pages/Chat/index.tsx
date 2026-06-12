import { useEffect, useRef } from 'react';
import { Row, Col, Card } from 'antd';
import { ChatMessage, ChatInput, ChatHistory } from '../../components/Chat';
import { EmptyState } from '../../components/Common';
import { useChatStore } from '../../stores';
import './ChatPage.css';

export default function ChatPage() {
  const {
    sessions,
    currentSessionId,
    messages,
    isLoading,
    loadSessions,
    selectSession,
    deleteSession,
    createSession,
    sendMessage,
  } = useChatStore();

  const messagesEndRef = useRef<HTMLDivElement>(null);

  // 加载会话列表
  useEffect(() => {
    loadSessions();
  }, []);

  // 滚动到底部
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages]);

  return (
    <div className="chat-page">
      <Row gutter={16} style={{ height: '100%' }}>
        <Col span={6}>
          <Card className="chat-history-card" title="会话列表">
            <ChatHistory
              sessions={sessions}
              currentSessionId={currentSessionId}
              onSelect={selectSession}
              onDelete={deleteSession}
              onNew={createSession}
            />
          </Card>
        </Col>

        <Col span={18}>
          <Card className="chat-main-card">
            <div className="chat-messages">
              {messages.length === 0 ? (
                <EmptyState
                  description="开始一段新的对话"
                  action={{
                    text: '发送消息',
                    onClick: () => {},
                  }}
                />
              ) : (
                <>
                  {messages.map((msg) => (
                    <ChatMessage key={msg.id} message={msg} />
                  ))}
                  <div ref={messagesEndRef} />
                </>
              )}
            </div>

            <div className="chat-input-area">
              <ChatInput
                onSend={sendMessage}
                loading={isLoading}
              />
            </div>
          </Card>
        </Col>
      </Row>
    </div>
  );
}