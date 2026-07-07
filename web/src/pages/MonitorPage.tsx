import { useEffect, useMemo } from 'react';
import { Link, useLocation } from 'react-router-dom';
import { fetchObservability, fetchTasks } from '@/api/client';
import { formatCostUSD } from '@/lib/cost';
import { formatTokenCount } from '@/lib/tokens';
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

function StatCard({ label, value, hint }: { label: string; value: string | number; hint?: string }) {
  return (
    <div className="bg-dark-card border border-dark-border rounded-xl p-4">
      <div className="text-3xl font-bold text-primary tabular-nums">{value}</div>
      <div className="text-sm text-gray-400">{label}</div>
      {hint ? <div className="text-xs text-gray-600 mt-1">{hint}</div> : null}
    </div>
  );
}

export function MonitorPage() {
  const location = useLocation();
  const presetTask = (location.state as { taskName?: string } | null)?.taskName;
  const { data: tasks = [] } = usePolling(fetchTasks, 10000);
  const { data: obs, refresh: refreshObs } = usePolling(fetchObservability, 12000);
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

  const metrics = obs?.metrics;
  const cluster = obs?.cluster;
  const estCost =
    metrics && metrics.ai_tokens_prompt + metrics.ai_tokens_completion > 0
      ? formatCostUSD(
          (metrics.ai_tokens_prompt / 1_000_000) * 0.27 + (metrics.ai_tokens_completion / 1_000_000) * 1.1,
        )
      : '$0.00';

  useEffect(() => {
    if (presetTask) connect(presetTask);
  }, [presetTask, connect]);

  return (
    <div className="p-5 max-w-6xl mx-auto space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <p className="text-sm text-gray-400">进程指标 + 集群实时状态（Prometheus 见 manager :8080/metrics）</p>
        <button
          type="button"
          onClick={refreshObs}
          className="text-sm px-3 py-1.5 border border-dark-border rounded-lg hover:bg-dark-bg"
        >
          刷新指标
        </button>
      </div>

      <div>
        <h3 className="text-sm font-medium text-gray-300 mb-3">集群状态</h3>
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
          <StatCard label="Task 总数" value={cluster?.tasks_total ?? stats.total} />
          <StatCard label="Workflow 总数" value={cluster?.workflows_total ?? '—'} />
          <StatCard label="Task 运行中" value={cluster?.tasks_by_phase?.Running ?? stats.running} />
          <StatCard
            label="Workflow 运行中"
            value={cluster?.workflows_by_phase?.Running ?? 0}
          />
        </div>
      </div>

      <div>
        <h3 className="text-sm font-medium text-gray-300 mb-3">进程 AI 指标（自启动累计）</h3>
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
          <StatCard
            label="AI 请求"
            value={Math.round((metrics?.ai_requests_ok ?? 0) + (metrics?.ai_requests_error ?? 0))}
            hint={`成功 ${Math.round(metrics?.ai_requests_ok ?? 0)} / 失败 ${Math.round(metrics?.ai_requests_error ?? 0)}`}
          />
          <StatCard
            label="进程 Token"
            value={formatTokenCount(
              Math.round((metrics?.ai_tokens_prompt ?? 0) + (metrics?.ai_tokens_completion ?? 0)),
            )}
            hint={`输入 ${formatTokenCount(Math.round(metrics?.ai_tokens_prompt ?? 0))} / 输出 ${formatTokenCount(Math.round(metrics?.ai_tokens_completion ?? 0))}`}
          />
          <StatCard label="预估费用（进程）" value={estCost} hint="按 deepseek-chat 单价粗算" />
          <StatCard
            label="质检通过 / 失败"
            value={`${Math.round(metrics?.quality_checks_passed ?? 0)} / ${Math.round(metrics?.quality_checks_failed ?? 0)}`}
          />
        </div>
      </div>

      {metrics?.ai_by_role && metrics.ai_by_role.length > 0 && (
        <div className="bg-dark-card border border-dark-border rounded-xl p-4 overflow-x-auto">
          <h3 className="text-sm font-medium mb-3">按角色 AI 用量</h3>
          <table className="w-full text-xs">
            <thead>
              <tr className="text-gray-500 text-left border-b border-dark-border">
                <th className="py-2 pr-3">角色</th>
                <th className="py-2 pr-3">模型</th>
                <th className="py-2 pr-3 text-right">成功</th>
                <th className="py-2 pr-3 text-right">失败</th>
                <th className="py-2 pr-3 text-right">Token</th>
                <th className="py-2 text-right">均延迟(s)</th>
              </tr>
            </thead>
            <tbody>
              {metrics.ai_by_role.map((row) => (
                <tr key={`${row.role}-${row.model}`} className="border-b border-dark-border/40">
                  <td className="py-2 pr-3">{row.role}</td>
                  <td className="py-2 pr-3 text-gray-400">{row.model || '—'}</td>
                  <td className="py-2 pr-3 text-right tabular-nums">{Math.round(row.requests_ok)}</td>
                  <td className="py-2 pr-3 text-right tabular-nums text-red-400">{Math.round(row.requests_error)}</td>
                  <td className="py-2 pr-3 text-right tabular-nums">
                    {formatTokenCount(Math.round(row.prompt_tokens + row.completion_tokens))}
                  </td>
                  <td className="py-2 text-right tabular-nums">
                    {row.avg_duration_seconds ? row.avg_duration_seconds.toFixed(2) : '—'}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        {[
          ['任务成功', stats.succeeded, 'text-green-500'],
          ['任务失败', stats.failed, 'text-red-500'],
          ['平均质量分', stats.avgScore, 'text-primary'],
          ['Workflow Reconcile', Math.round(metrics?.workflow_reconciles?.ok ?? 0), 'text-gray-300'],
        ].map(([label, val, color]) => (
          <div key={label as string} className="bg-dark-card border border-dark-border rounded-xl p-4">
            <div className={`text-2xl font-bold tabular-nums ${color}`}>{val}</div>
            <div className="text-sm text-gray-400">{label}</div>
          </div>
        ))}
      </div>

      <div className="flex flex-wrap gap-3 text-xs text-gray-500">
        <Link to="/tokens" className="text-primary hover:underline">
          Token 报表（SQLite 累计）
        </Link>
        <span>·</span>
        <span>Prometheus 抓取: agentflow-manager:8080{metrics?.prometheus_metrics_path || '/metrics'}</span>
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