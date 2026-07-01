import { Link } from 'react-router-dom';
import { fetchWorkflows } from '@/api/client';
import { usePolling } from '@/hooks/usePolling';
import { PhaseBadge } from '@/components/PhaseBadge';

export function WorkflowsPage() {
  const { data: workflows = [], refresh, error } = usePolling(fetchWorkflows, 10000);

  return (
    <div className="p-5 max-w-6xl mx-auto space-y-4">
      <div className="flex justify-between items-center">
        <p className="text-sm text-gray-400">编排中的多步骤任务（如 100 章小说）</p>
        <button type="button" onClick={refresh} className="text-sm text-gray-400 hover:text-white">
          刷新
        </button>
      </div>
      {error && <p className="text-red-400 text-sm">{error}</p>}
      {workflows.length === 0 && <p className="text-center text-gray-500 py-16">暂无工作流</p>}
      <div className="space-y-3">
        {workflows.map((wf) => (
          <div key={`${wf.namespace}/${wf.name}`} className="bg-dark-card border border-dark-border rounded-xl p-4">
            <div className="flex items-center justify-between mb-2">
              <span className="font-medium">{wf.name}</span>
              <PhaseBadge phase={wf.phase} />
            </div>
            <p className="text-xs text-gray-400 mb-3">{wf.message}</p>
            <div className="w-full bg-dark-bg rounded-full h-2 mb-2">
              <div className="bg-primary h-2 rounded-full transition-all" style={{ width: `${Math.min(wf.progress, 100)}%` }} />
            </div>
            <div className="flex items-center justify-between text-xs text-gray-500">
              <span>
                {wf.progress}% · 当前 {wf.currentStep || '-'}
              </span>
              <div className="flex gap-2">
                <Link
                  to={`/novel/${wf.namespace || 'default'}/${wf.name}`}
                  className="px-3 py-1.5 bg-emerald-700/80 rounded text-white"
                >
                  阅读章节
                </Link>
              </div>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}