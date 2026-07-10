import { NavLink, Outlet, useLocation } from 'react-router-dom';
import { useEffect, useState } from 'react';
import {
  approveTask,
  approveWorkflow,
  fetchPendingTasks,
  fetchPendingWorkflows,
  rejectTask,
  rejectWorkflow,
} from '@/api/client';
import type { PendingTask, PendingWorkflow } from '@/types/api';

const NAV = [
  { to: '/', label: '对话', icon: '💬', title: '对话模式' },
  { to: '/tasks', label: '任务列表', icon: '📋', title: '任务列表' },
  { to: '/workflows', label: '工作流', icon: '📚', title: '工作流' },
  { to: '/library', label: '小说库', icon: '📕', title: '小说库' },
  { to: '/tokens', label: 'Token 报表', icon: '🪙', title: 'Token 用量汇总' },
  { to: '/novel', label: '小说阅读', icon: '📖', title: '小说阅读' },
  { to: '/monitor', label: '监控面板', icon: '📊', title: '监控面板' },
  { to: '/settings', label: '设置', icon: '⚙️', title: 'AI 配置' },
];

export function Layout() {
  const location = useLocation();
  const [pendingTasks, setPendingTasks] = useState<PendingTask[]>([]);
  const [pendingWorkflows, setPendingWorkflows] = useState<PendingWorkflow[]>([]);
  const pageTitle =
    NAV.find((n) => (n.to === '/' ? location.pathname === '/' : location.pathname.startsWith(n.to)))?.title ||
    'Agent Flow';

  const loadPending = async () => {
    try {
      const [tasks, workflows] = await Promise.all([fetchPendingTasks(), fetchPendingWorkflows()]);
      setPendingTasks(tasks);
      setPendingWorkflows(workflows);
    } catch {
      setPendingTasks([]);
      setPendingWorkflows([]);
    }
  };

  useEffect(() => {
    loadPending();
    const id = window.setInterval(loadPending, 15000);
    return () => window.clearInterval(id);
  }, []);

  return (
    <div className="min-h-screen flex">
      <aside className="w-72 bg-dark-card border-r border-dark-border flex flex-col shrink-0">
        <div className="p-5 border-b border-dark-border">
          <h1 className="text-xl font-bold bg-gradient-to-r from-primary to-purple-500 bg-clip-text text-transparent">
            Agent Flow
          </h1>
          <p className="text-xs text-gray-400 mt-1">React · 任务编排</p>
        </div>
        <nav className="flex-1 p-3 space-y-1">
          {NAV.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              end={item.to === '/'}
              className={({ isActive }) =>
                `w-full text-left px-4 py-3 rounded-lg flex items-center gap-3 transition-colors ${
                  isActive ? 'bg-dark-bg border border-primary/40' : 'hover:bg-dark-bg'
                }`
              }
            >
              <span>{item.icon}</span>
              <span>{item.label}</span>
            </NavLink>
          ))}
        </nav>
        <div className="p-3 border-t border-dark-border space-y-3 max-h-56 overflow-y-auto">
          <div>
            <h3 className="text-xs font-semibold text-gray-400 uppercase mb-2">待批准任务</h3>
            <div className="space-y-2">
              {pendingTasks.length === 0 && <p className="text-xs text-gray-500">无</p>}
              {pendingTasks.map((task) => {
                const id = task.id || task.ID || '';
                return (
                  <div key={id} className="bg-dark-bg border border-dark-border rounded-lg p-2">
                    <p className="text-xs truncate">{task.description || id}</p>
                    <div className="flex gap-1 mt-2">
                      <button
                        type="button"
                        className="text-xs px-2 py-0.5 bg-green-600 rounded"
                        onClick={() => approveTask(id).then(loadPending)}
                      >
                        批准
                      </button>
                      <button
                        type="button"
                        className="text-xs px-2 py-0.5 border border-dark-border rounded"
                        onClick={() => rejectTask(id, '暂不执行').then(loadPending)}
                      >
                        拒绝
                      </button>
                    </div>
                  </div>
                );
              })}
            </div>
          </div>
          <div>
            <h3 className="text-xs font-semibold text-gray-400 uppercase mb-2">待批准工作流</h3>
            <div className="space-y-2">
              {pendingWorkflows.length === 0 && <p className="text-xs text-gray-500">无</p>}
              {pendingWorkflows.map((wf) => {
                const id = wf.id || wf.ID || '';
                const chapters = wf.params?.chapterCount;
                return (
                  <div key={id} className="bg-dark-bg border border-emerald-800/50 rounded-lg p-2">
                    <p className="text-xs truncate">{wf.description || id}</p>
                    {chapters && <p className="text-[10px] text-gray-500 mt-0.5">{chapters} 章 · {wf.template}</p>}
                    <div className="flex gap-1 mt-2">
                      <button
                        type="button"
                        className="text-xs px-2 py-0.5 bg-emerald-600 rounded"
                        onClick={() => approveWorkflow(id).then(loadPending)}
                      >
                        批准
                      </button>
                      <button
                        type="button"
                        className="text-xs px-2 py-0.5 border border-dark-border rounded"
                        onClick={() => rejectWorkflow(id, '暂不执行').then(loadPending)}
                      >
                        拒绝
                      </button>
                    </div>
                  </div>
                );
              })}
            </div>
          </div>
        </div>
      </aside>
      <main className="flex-1 flex flex-col min-w-0">
        <header className="h-14 border-b border-dark-border flex items-center justify-between px-5 bg-dark-card shrink-0">
          <h2 className="text-lg font-semibold">{pageTitle}</h2>
          <div className="flex items-center gap-2 text-sm text-gray-400">
            <span className="w-2 h-2 bg-green-500 rounded-full animate-pulse" />
            系统在线
          </div>
        </header>
        <div className="flex-1 overflow-auto">
          <Outlet />
        </div>
      </main>
    </div>
  );
}