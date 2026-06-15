import { Routes, Route } from 'react-router-dom';
import { MainLayout } from './components/Layout';
import HomePage from './pages/Home';
import ChatPage from './pages/Chat';
import KnowledgePage from './pages/Knowledge';
import MemoryPage from './pages/Memory';
import AgentsPage from './pages/Agents';
import HarnessPage from './pages/Harness';
import SettingsPage from './pages/Settings';
import ObservabilityPage from './pages/Observability';

function App() {
  return (
    <Routes>
      <Route path="/" element={<MainLayout />}>
        <Route index element={<HomePage />} />
        <Route path="chat" element={<ChatPage />} />
        <Route path="knowledge" element={<KnowledgePage />} />
        <Route path="memory" element={<MemoryPage />} />
        <Route path="agents" element={<AgentsPage />} />
        <Route path="harness" element={<HarnessPage />} />
        <Route path="observability/*" element={<ObservabilityPage />} />
        <Route path="settings" element={<SettingsPage />} />
      </Route>
    </Routes>
  );
}

export default App;