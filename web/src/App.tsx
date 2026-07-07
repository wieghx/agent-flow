import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom';
import { Layout } from '@/components/Layout';
import { ChatPage } from '@/pages/ChatPage';
import { TasksPage } from '@/pages/TasksPage';
import { WorkflowsPage } from '@/pages/WorkflowsPage';
import { WorkflowDetailPage } from '@/pages/WorkflowDetailPage';
import { MonitorPage } from '@/pages/MonitorPage';
import { NovelReaderPage } from '@/pages/NovelReaderPage';
import { NovelLibraryPage } from '@/pages/NovelLibraryPage';
import { TokenReportPage } from '@/pages/TokenReportPage';

export function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route element={<Layout />}>
          <Route path="/" element={<ChatPage />} />
          <Route path="/tasks" element={<TasksPage />} />
          <Route path="/workflows" element={<WorkflowsPage />} />
          <Route path="/workflows/:namespace/:name" element={<WorkflowDetailPage />} />
          <Route path="/library" element={<NovelLibraryPage />} />
          <Route path="/tokens" element={<TokenReportPage />} />
          <Route path="/monitor" element={<MonitorPage />} />
          <Route path="/novel" element={<NovelReaderPage />} />
          <Route path="/novel/:namespace/:name" element={<NovelReaderPage />} />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Route>
      </Routes>
    </BrowserRouter>
  );
}