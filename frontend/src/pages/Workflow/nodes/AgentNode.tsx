import { Handle, Position, type NodeProps } from '@xyflow/react';

export function AgentNode({ data, selected }: NodeProps) {
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
      }}
    >
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
