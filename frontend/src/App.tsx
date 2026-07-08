import { Routes, Route } from "react-router-dom";
import { MainLayout } from "./components/Layout";
import HomePage from "./pages/Home";
import ChatPage from "./pages/Chat";
import KnowledgePage from "./pages/Knowledge";
import MemoryPage from "./pages/Memory";
import AgentsPage from "./pages/Agents";
import HarnessPage from "./pages/Harness";
import GatewayPage from "./pages/Gateway";
import SettingsPage from "./pages/Settings";
import PromptListPage from "./pages/Prompt";
import PromptEditor from "./pages/Prompt/Editor";
import PromptVersionHistory from "./pages/Prompt/VersionHistory";
import PromptCompare from "./pages/Prompt/Compare";
import ObservabilityPage from "./pages/Observability";
import RAGMetricsPage from "./pages/RAGMetrics";
import RAGEvaluatePage from "./pages/RAGMetrics/Evaluate";
import RAGDetailPage from "./pages/RAGMetrics/Detail";
import SessionListPage from "./pages/Session";
import SessionReplayPage from "./pages/Session/Replay";
import PlaygroundPage from "./pages/Playground";
import WorkflowEditor from "./pages/Workflow";

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
        <Route path="gateway" element={<GatewayPage />} />
        <Route path="session" element={<SessionListPage />} />
        <Route path="session/replay/:sessionId" element={<SessionReplayPage />} />
        <Route path="prompt" element={<PromptListPage />} />
        <Route path="prompt/editor/:key" element={<PromptEditor />} />
        <Route path="prompt/history/:key" element={<PromptVersionHistory />} />
        <Route path="prompt/compare/:key" element={<PromptCompare />} />
        <Route path="playground" element={<PlaygroundPage />} />
        <Route path="observability/*" element={<ObservabilityPage />} />
        <Route path="rag-metrics" element={<RAGMetricsPage />} />
        <Route path="rag-metrics/evaluate" element={<RAGEvaluatePage />} />
        <Route path="rag-metrics/:id" element={<RAGDetailPage />} />
        <Route path="workflow" element={<WorkflowEditor />} />
        <Route path="workflow/:id" element={<WorkflowEditor />} />
        <Route path="settings" element={<SettingsPage />} />
      </Route>
    </Routes>
  );
}

export default App;

