import { useEffect, useMemo } from 'react';
import { useLocation } from 'react-router-dom';
import { fetchTasks } from '@/api/client';
import { usePolling } from '@/hooks/usePolling';
import { useTaskSSE } from '@/hooks/useTaskSSE';

const STEP_COLORS: Record<string, string> = {
  worker: 'text-blue-300',
  monitor: 'text-purple-300',
  sandbox: 'text-yellow-300',
  succeeded: 'text-green-400',
  failed: 'text-red-400',
  error: 'text-red-400',
};

export function MonitorPage() {
  const location = useLocation();
  const presetTask = (location.state as { taskName?: string } | null)?.taskName;
  const { data: tasks = [] } = usePolling(fetchTasks, 10000);
  const { connectedTask, status, events, connect, disconnect, clearEvents } = useTaskSSE();

  const running = useMemo(() => tasks.filter((t) => t.phase === 'Running'), [tasks]);
  const stats = useMemo(() => {
    const scores = tasks.filter((t) => (t.score || 0) > 0).map((t) => t.score || 0);
    const avg = scores.length ? Math.round(scores.reduce((a, b) => a + b, 0) / scores.length) : 0;
    return {
      total: tasks.length,
      succeeded: tasks.filter((t) => t.phase === 'Succeeded').length,
      failed: tasks.filter((t) => t.phase === 'Failed').length,
      running: running.length,
      avgScore: avg,
    };
  }, [tasks, running.length]);

  useEffect(() => {
    if (presetTask) connect(presetTask);
  }, [presetTask, connect]);

  return (
    <div className="p-5 max-w-6xl mx-auto space-y-6">
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        {[
          ['总任务', stats.total, 'text-primary'],
          ['成功', stats.succeeded, 'text-green-500'],
          ['失败', stats.failed, 'text-red-500'],
          ['运行中', stats.running, 'text-yellow-500'],
        ].map(([label, val, color]) => (
          <div key={label as string} className="bg-dark-card border border-dark-border rounded-xl p-4">
            <div className={`text-3xl font-bold ${color}`}>{val}</div>
            <div className="text-sm text-gray-400">{label}</div>
          </div>
        ))}
      </div>

      <div className="grid md:grid-cols-2 gap-4">
        <div className="bg-dark-card border border-dark-border rounded-xl p-4">
          <div className="text-sm text-gray-400 mb-2">平均质量分</div>
          <div className="text-2xl font-bold text-primary">{stats.avgScore}</div>
        </div>
        <div className="bg-dark-card border border-dark-border rounded-xl p-4">
          <div className="text-sm text-gray-400 mb-2">最近任务</div>
          <div className="space-y-1 text-xs">
            {tasks.slice(0, 6).map((t) => (
              <div key={t.name} className="flex justify-between border-b border-dark-border py-1">
                <span className="truncate mr-2">{t.name}</span>
                <span className="text-gray-500">{t.phase}</span>
              </div>
            ))}
          </div>
        </div>
      </div>

      <div className="bg-dark-card border border-dark-border rounded-xl p-4">
        <div className="flex flex-wrap items-center gap-2 mb-3">
          <span className="text-sm text-gray-400">SSE 实时事件</span>
          <select
            className="bg-dark-bg border border-dark-border rounded px-2 py-1 text-sm max-w-xs"
            value={connectedTask || ''}
            onChange={(e) => (e.target.value ? connect(e.target.value) : disconnect())}
          >
            <option value="">选择运行中任务...</option>
            {running.map((t) => (
              <option key={t.name} value={t.name}>
                {t.name}
              </option>
            ))}
          </select>
          <button type="button" onClick={disconnect} className="text-xs px-2 py-1 border border-dark-border rounded">
            断开
          </button>
          <button type="button" onClick={clearEvents} className="text-xs px-2 py-1 border border-dark-border rounded">
            清空
          </button>
          <span className="text-xs text-gray-500 ml-auto">{status}</span>
        </div>
        <div className="bg-dark-bg border border-dark-border rounded-lg p-3 h-64 overflow-y-auto font-mono text-xs space-y-1">
          {events.length === 0 && <p className="text-gray-600">等待事件...</p>}
          {events.map((ev) => (
            <div key={ev.id} className={STEP_COLORS[ev.step] || 'text-gray-300'}>
              <span className="text-gray-600">{ev.at}</span> [{ev.step}] {ev.message}
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}