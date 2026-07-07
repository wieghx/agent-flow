import { useCallback, useEffect, useMemo, useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import {
  fetchWorkflowDetail,
  fetchWorkflowTasks,
  resumeNovel,
} from '@/api/client';
import { PhaseBadge } from '@/components/PhaseBadge';
import { formatWorkflowStep } from '@/lib/pipeline';
import type { TaskSummary, WorkflowDetail, WorkflowStepStatus } from '@/types/api';

function formatTime(iso?: string) {
  if (!iso) return '—';
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? iso : d.toLocaleString('zh-CN');
}

function stepPhaseClass(phase?: string) {
  switch (phase) {
    case 'Succeeded':
      return 'border-emerald-700/50 bg-emerald-950/30';
    case 'Running':
      return 'border-amber-600/50 bg-amber-950/20';
    case 'Failed':
      return 'border-red-700/50 bg-red-950/20';
    default:
      return 'border-dark-border bg-dark-bg/40';
  }
}

export function WorkflowDetailPage() {
  const { namespace = 'default', name = '' } = useParams();
  const [detail, setDetail] = useState<WorkflowDetail | null>(null);
  const [tasks, setTasks] = useState<TaskSummary[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [resuming, setResuming] = useState(false);

  const load = useCallback(async () => {
    if (!name) return;
    try {
      const [wf, wfTasks] = await Promise.all([
        fetchWorkflowDetail(name, namespace),
        fetchWorkflowTasks(namespace, name),
      ]);
      setDetail(wf);
      setTasks(wfTasks);
      setError(null);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }, [name, namespace]);

  useEffect(() => {
    load();
    const id = window.setInterval(load, 8000);
    return () => window.clearInterval(id);
  }, [load]);

  const status = detail?.status;
  const phase = status?.phase || 'Unknown';
  const isRunning = ['Running', 'Pending'].includes(phase);
  const canResume =
    ['Failed', 'Paused'].includes(phase) ||
    (phase === 'Succeeded' && (status?.progress?.completed ?? 0) < (status?.progress?.total ?? 0));

  const stepRows = useMemo(() => {
    const rows: WorkflowStepStatus[] = status?.stepStatuses ? [...status.stepStatuses] : [];
    const seen = new Set(rows.map((s) => s.id));
    for (const id of status?.completedSteps || []) {
      if (!seen.has(id)) {
        rows.push({ id, phase: 'Succeeded' });
        seen.add(id);
      }
    }
    for (const id of status?.failedSteps || []) {
      if (!seen.has(id)) {
        rows.push({ id, phase: 'Failed' });
        seen.add(id);
      }
    }
    if (status?.currentStep && !seen.has(status.currentStep)) {
      rows.unshift({ id: status.currentStep, phase: 'Running', message: status.message });
    }
    return rows;
  }, [status]);

  const onResume = async () => {
    setResuming(true);
    setActionError(null);
    try {
      await resumeNovel(namespace, name);
      await load();
    } catch (e) {
      setActionError(e instanceof Error ? e.message : String(e));
    } finally {
      setResuming(false);
    }
  };

  if (!name) {
    return <p className="p-5 text-gray-500">缺少工作流名称</p>;
  }

  return (
    <div className="p-5 max-w-6xl mx-auto space-y-5">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <Link to="/workflows" className="text-xs text-gray-500 hover:text-white">
            ← 返回工作流列表
          </Link>
          <h2 className="text-lg font-semibold mt-1">{name}</h2>
          <p className="text-xs text-gray-500">
            {namespace} · {detail?.spec?.template || '—'}
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <PhaseBadge phase={phase} />
          <button
            type="button"
            onClick={load}
            className="text-sm px-3 py-1.5 border border-dark-border rounded-lg hover:bg-dark-bg"
          >
            刷新
          </button>
          {canResume && (
            <button
              type="button"
              disabled={resuming}
              onClick={onResume}
              className="text-sm px-3 py-1.5 bg-amber-700/80 rounded-lg text-white disabled:opacity-50"
            >
              {resuming ? '恢复中…' : '恢复执行'}
            </button>
          )}
          <Link
            to={`/novel/${namespace}/${name}`}
            className="text-sm px-3 py-1.5 bg-emerald-700/80 rounded-lg text-white"
          >
            阅读章节
          </Link>
        </div>
      </div>

      {(error || actionError) && (
        <p className="text-sm text-red-400">{error || actionError}</p>
      )}

      {!detail && !error && <p className="text-gray-500 text-center py-16">加载中…</p>}

      {detail && (
        <>
          <div className="bg-dark-card border border-dark-border rounded-xl p-4 space-y-3">
            <p className="text-sm text-gray-300">{status?.message || '—'}</p>
            <div className="w-full bg-dark-bg rounded-full h-2">
              <div
                className="bg-primary h-2 rounded-full transition-all"
                style={{ width: `${Math.min(status?.progress?.percent ?? 0, 100)}%` }}
              />
            </div>
            <div className="flex flex-wrap gap-4 text-xs text-gray-500">
              <span>进度 {status?.progress?.percent ?? 0}%</span>
              <span>
                步骤 {status?.progress?.completed ?? 0}/{status?.progress?.total ?? '?'}
              </span>
              <span>当前 {formatWorkflowStep(status?.currentStep)}</span>
              {status?.workspacePath && <span className="truncate max-w-md">工作区 {status.workspacePath}</span>}
              {isRunning && <span className="text-amber-400">自动刷新中</span>}
            </div>
          </div>

          {detail.spec?.params && Object.keys(detail.spec.params).length > 0 && (
            <div className="bg-dark-card border border-dark-border rounded-xl p-4">
              <h3 className="text-sm font-medium mb-3">参数</h3>
              <div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-3 text-xs">
                {Object.entries(detail.spec.params).map(([k, v]) => (
                  <div key={k} className="flex gap-2">
                    <span className="text-gray-500 shrink-0">{k}</span>
                    <span className="text-gray-300 break-all">{v}</span>
                  </div>
                ))}
              </div>
            </div>
          )}

          <div className="bg-dark-card border border-dark-border rounded-xl p-4">
            <h3 className="text-sm font-medium mb-3">步骤时间线</h3>
            {stepRows.length === 0 ? (
              <p className="text-sm text-gray-500">暂无步骤状态</p>
            ) : (
              <div className="space-y-2">
                {stepRows.map((step) => (
                  <div
                    key={step.id}
                    className={`flex flex-wrap items-center justify-between gap-2 rounded-lg border px-3 py-2 text-sm ${stepPhaseClass(step.phase)}`}
                  >
                    <div className="min-w-0">
                      <span className="font-medium">{formatWorkflowStep(step.id)}</span>
                      <span className="text-xs text-gray-500 ml-2">{step.id}</span>
                      {step.message && <p className="text-xs text-gray-400 mt-0.5 truncate">{step.message}</p>}
                    </div>
                    <div className="flex items-center gap-3 text-xs text-gray-400 shrink-0">
                      <PhaseBadge phase={step.phase || 'Pending'} />
                      {step.score != null && step.score > 0 && <span>评分 {step.score}</span>}
                      {step.retries != null && step.retries > 0 && <span>重试 {step.retries}</span>}
                      {step.taskName && <span>{step.taskName}</span>}
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>

          <div className="bg-dark-card border border-dark-border rounded-xl p-4 overflow-x-auto">
            <h3 className="text-sm font-medium mb-3">关联 Task ({tasks.length})</h3>
            {tasks.length === 0 ? (
              <p className="text-sm text-gray-500">暂无 Task</p>
            ) : (
              <table className="w-full text-xs">
                <thead>
                  <tr className="text-gray-500 text-left border-b border-dark-border">
                    <th className="py-2 pr-3">步骤</th>
                    <th className="py-2 pr-3">Task</th>
                    <th className="py-2 pr-3">状态</th>
                    <th className="py-2 pr-3">评分</th>
                    <th className="py-2 pr-3">重试</th>
                    <th className="py-2">完成时间</th>
                  </tr>
                </thead>
                <tbody>
                  {tasks.map((t) => (
                    <tr key={`${t.namespace}/${t.name}`} className="border-b border-dark-border/50">
                      <td className="py-2 pr-3">{formatWorkflowStep(t.step_id)}</td>
                      <td className="py-2 pr-3 font-mono">{t.name}</td>
                      <td className="py-2 pr-3">
                        <PhaseBadge phase={t.phase} />
                      </td>
                      <td className="py-2 pr-3 tabular-nums">{t.score ? t.score : '—'}</td>
                      <td className="py-2 pr-3 tabular-nums">{t.retries ?? 0}</td>
                      <td className="py-2 text-gray-500">{formatTime(t.completion_at)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>

          {detail.spec?.prompt && (
            <details className="bg-dark-card border border-dark-border rounded-xl p-4">
              <summary className="text-sm font-medium cursor-pointer">原始 Prompt</summary>
              <pre className="mt-3 text-xs text-gray-400 whitespace-pre-wrap">{detail.spec.prompt}</pre>
            </details>
          )}
        </>
      )}
    </div>
  );
}