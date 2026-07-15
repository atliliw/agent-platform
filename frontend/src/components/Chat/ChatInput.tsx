import { useState, useEffect } from 'react';
import { Input, Button, Tooltip } from 'antd';
import { SendOutlined, LoadingOutlined } from '@ant-design/icons';

interface ChatInputProps {
  onSend: (message: string) => void;
  disabled?: boolean;
  loading?: boolean;
  placeholder?: string;
  maxLength?: number;
}

export default function ChatInput({
  onSend,
  disabled = false,
  loading = false,
  placeholder = '输入消息，按 Enter 发送，Shift+Enter 换行',
  maxLength = 4000,
}: ChatInputProps) {
  const [value, setValue] = useState('');

  useEffect(() => {
    // Focus on mount
  }, []);

  const handleSend = () => {
    if (value.trim() && !loading && !disabled) {
      onSend(value.trim());
      setValue('');
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };

  return (
    <div className="chat-input-wrapper">
      <Input.TextArea
        value={value}
        onChange={(e) => setValue(e.target.value)}
        onKeyDown={handleKeyDown}
        placeholder={placeholder}
        maxLength={maxLength}
        autoSize={{ minRows: 1, maxRows: 6 }}
        disabled={disabled || loading}
        className="chat-input-textarea"
      />
      <div className="chat-input-actions">
        <span className="char-count">
          {value.length}/{maxLength}
        </span>
        <Tooltip title="发送消息">
          <Button
            type="primary"
            icon={loading ? <LoadingOutlined /> : <SendOutlined />}
            onClick={handleSend}
            disabled={!value.trim() || disabled || loading}
            loading={loading}
          />
        </Tooltip>
      </div>
    </div>
  );
}