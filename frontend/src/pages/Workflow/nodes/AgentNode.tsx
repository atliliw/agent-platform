import { Handle, Position, type NodeProps } from '@xyflow/react';

const STATUS_COLORS: Record<string, string> = {
  running: '#1677ff',
  completed: '#52c41a',
  failed: '#ff4d4f',
  timed_out: '#fa8c16',
};

export function AgentNode({ data, selected }: NodeProps) {
  const status = data.status as string | undefined;
  return (
    <div
      style={{
        padding: '12px 20px',
        borderRadius: 8,
        border: `2px solid ${selected ? '#1677ff' : '#69b1ff'}`,
        background: '#e6f4ff',
        minWidth: 160,
        textAlign: 'center',
        fontSize: 13,
        boxShadow: selected ? '0 0 0 2px rgba(22,119,255,0.2)' : 'none',
        position: 'relative',
      }}
    >
      {status && status !== 'idle' && (
        <div style={{
          height: 3,
          borderRadius: '6px 6px 0 0',
          background: STATUS_COLORS[status] ?? '#d9d9d9',
          margin: '-12px -20px 8px',
          animation: status === 'running' ? 'pulse 1.5s infinite' : undefined,
        }} />
      )}
      <div style={{ fontWeight: 600, marginBottom: 4, color: '#1677ff' }}>
        Agent
      </div>
      <div style={{ color: '#333' }}>{(data.label as string) || 'Agent Node'}</div>
      {data.agent_id && (
        <div style={{ fontSize: 11, color: '#999', marginTop: 2 }}>
          ID: {data.agent_id as string}
        </div>
      )}
      <Handle type="target" position={Position.Top} />
      <Handle type="source" position={Position.Bottom} />
    </div>
  );
}
