import { Handle, Position, type NodeProps } from '@xyflow/react';

const STATUS_COLORS: Record<string, string> = {
  running: '#1677ff',
  completed: '#52c41a',
  failed: '#ff4d4f',
  timed_out: '#fa8c16',
};

export function ConditionNode({ data, selected }: NodeProps) {
  const status = data.status as string | undefined;
  return (
    <div
      style={{
        padding: '12px 20px',
        borderRadius: 8,
        border: `2px solid ${selected ? '#fa8c16' : '#ffc069'}`,
        background: '#fff7e6',
        minWidth: 160,
        textAlign: 'center',
        fontSize: 13,
        boxShadow: selected ? '0 0 0 2px rgba(250,140,22,0.2)' : 'none',
      }}
    >
      {status && status !== 'idle' && (
        <div style={{
          height: 3,
          borderRadius: '6px 6px 0 0',
          background: STATUS_COLORS[status] ?? '#d9d9d9',
          margin: '-12px -20px 8px',
        }} />
      )}
      <div style={{ fontWeight: 600, marginBottom: 4, color: '#fa8c16' }}>
        Condition
      </div>
      <div style={{ color: '#333' }}>{(data.label as string) || 'Condition Node'}</div>
      {data.condition && (
        <div style={{ fontSize: 11, color: '#999', marginTop: 2, wordBreak: 'break-all' }}>
          {(data.condition as string)}
        </div>
      )}
      <Handle type="target" position={Position.Top} />
      <Handle
        type="source"
        position={Position.Bottom}
        id="true"
        style={{ left: '30%' }}
      />
      <Handle
        type="source"
        position={Position.Bottom}
        id="false"
        style={{ left: '70%' }}
      />
    </div>
  );
}
