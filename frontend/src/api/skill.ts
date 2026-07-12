// Skill API - 技能库 CRUD（能力模块，Agent 通过 ID 挂载）
import client from './client';

// 技能状态
export type SkillStatus = 'draft' | 'active';

// Skill 类型定义
export interface Skill {
  id: string;
  name: string;
  description: string;
  instructions: string;
  tools?: string[];
  tags?: string[];
  status: SkillStatus;
  version: number;
  created_at: number;
  updated_at: number;
}

// 创建/更新技能的请求体
export interface SkillInput {
  id?: string;
  name: string;
  description: string;
  instructions: string;
  tools?: string[];
  tags?: string[];
  status?: SkillStatus;
}

// Skill API
// Note: client response interceptor already unwraps ApiResponse envelope,
// so return types reflect the inner data.
export const skillApi = {
  // 获取所有技能
  listSkills: (): Promise<{ skills: Skill[]; pagination: { total: number } }> =>
    client.get('/api/v2/skills'),

  // 获取单个技能
  getSkill: (id: string): Promise<{ skill: Skill }> =>
    client.get(`/api/v2/skills/${id}`),

  // 创建技能
  createSkill: (skill: SkillInput): Promise<{ skill: Skill }> =>
    client.post('/api/v2/skills', skill),

  // 更新技能
  updateSkill: (id: string, skill: SkillInput): Promise<{ skill: Skill }> =>
    client.put(`/api/v2/skills/${id}`, skill),

  // 删除技能
  deleteSkill: (id: string): Promise<null> =>
    client.delete(`/api/v2/skills/${id}`),

  // 导出技能为可移植 YAML（返回 yaml 字符串 + 建议文件名）
  exportSkill: (id: string): Promise<{ yaml: string; filename: string }> =>
    client.get(`/api/v2/skills/${id}/export`),

  // 导入技能 YAML（带 ID 则 upsert 更新，不带 ID 则新建）
  importSkill: (yaml: string): Promise<{ skill: Skill; imported: boolean }> =>
    client.post('/api/v2/skills/import', { yaml }),
};

export default skillApi;
