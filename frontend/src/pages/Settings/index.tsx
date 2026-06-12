import { Tabs } from 'antd';
import { SettingOutlined, TrophyOutlined } from '@ant-design/icons';
import SettingsForm from './SettingsForm';
import CookiePage from './Cookie';

export default function SettingsPage() {
  const items = [
    {
      key: 'settings',
      label: '系统设置',
      icon: <SettingOutlined />,
      children: <SettingsForm />,
    },
    {
      key: 'cookies',
      label: 'Cookie 管理',
      icon: <TrophyOutlined />,
      children: <CookiePage />,
    },
  ];

  return (
    <div>
      <h2 style={{ marginBottom: 24 }}>设置</h2>
      <Tabs items={items} />
    </div>
  );
}