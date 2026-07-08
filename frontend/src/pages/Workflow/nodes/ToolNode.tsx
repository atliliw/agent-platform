import { Handle, Position, type NodeProps } from '@xyflow/react';

export function ToolNode({ data, selected }: NodeProps) {
  return (
    <div
      style={{
        padding: '12px 20px',
        borderRadius: 8,
        border: `2px solid ${selected ? '#13c2c2' : '#5cdbd3'}`,
        background: '#e6fffb',
        minWidth: 160,
        textAlign: 'center',
        fontSize: 13,
        boxShadow: selected ? '0 0 0 2px rgba(19,194,194,0.2)' : 'none',
      }}
    >
      <div style={{ fontWeight: 600, marginBottom: 4, color: '#13c2c2' }}>
        Tool
      </div>
      <div style={{ color: '#333' }}>{(data.label as string) || 'Tool Node'}</div>
      {data.tool_name && (
        <div style={{ fontSize: 11, color: '#999', marginTop: 2 }}>
          {(data.tool_name as string)}
        </div>
      )}
      <Handle type="target" position={Position.Top} />
      <Handle type="source" position={Position.Bottom} />
    </div>
  );
}
