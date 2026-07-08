import { Handle, Position, type NodeProps } from '@xyflow/react';

export function ConditionNode({ data, selected }: NodeProps) {
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
