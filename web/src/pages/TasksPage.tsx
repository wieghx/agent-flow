import { useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { deleteTask, fetchTasks } from '@/api/client';
import { usePolling } from '@/hooks/usePolling';
import { usePagination } from '@/hooks/usePagination';
import { PhaseBadge } from '@/components/PhaseBadge';
import { Pagination } from '@/components/Pagination';
import { Modal } from '@/components/Modal';
import { chapterMarkdownUrl, taskOutputFileUrl } from '@/lib/paths';
import type { PhaseFilter, TaskSummary } from '@/types/api';

export function TasksPage() {
  const { data: tasks = [], refresh } = usePolling(fetchTasks, 10000);
  const [filter, setFilter] = useState<PhaseFilter>('all');
  const [detail, setDetail] = useState<TaskSummary | null>(null);

  const filtered = useMemo(() => {
    if (filter === 'all') return tasks;
    return tasks.filter((t) => t.phase === filter);
  }, [tasks, filter]);

  const {
    paginatedItems: pagedTasks,
    page,
    setPage,
    totalPages,
    totalItems,
    rangeStart,
    rangeEnd,
  } = usePagination(filtered, { pageSize: 10, resetKey: filter });

  return (
    <div className="p-5 max-w-6xl mx-auto space-y-4">
      <div className="flex flex-wrap gap-2">
        {(['all', 'Pending', 'Running', 'Succeeded', 'Failed'] as PhaseFilter[]).map((f) => (
          <button
            key={f}
            type="button"
            onClick={() => setFilter(f)}
            className={`px-4 py-2 rounded-lg text-sm ${
              filter === f ? 'bg-primary text-white' : 'bg-dark-card border border-dark-border'
            }`}
          >
            {f === 'all' ? '全部' : f}
          </button>
        ))}
        <button type="button" onClick={refresh} className="ml-auto text-sm text-gray-400 hover:text-white">
          刷新
        </button>
      </div>

      {filtered.length === 0 && <p className="text-center text-gray-500 py-16">暂无任务</p>}

      {filtered.length > 0 && (
        <Pagination
          page={page}
          totalPages={totalPages}
          totalItems={totalItems}
          rangeStart={rangeStart}
          rangeEnd={rangeEnd}
          onPageChange={setPage}
        />
      )}

      <div className="space-y-3">
        {pagedTasks.map((task) => {
          const chapterUrl = chapterMarkdownUrl(task);
          const outputUrl = taskOutputFileUrl(task);
          return (
            <div key={task.name} className="bg-dark-card border border-dark-border rounded-xl p-4">
              <div className="flex items-center justify-between gap-2 mb-2">
                <span className="font-medium truncate">{task.name}</span>
                <div className="flex items-center gap-2 shrink-0">
                  {task.score ? (
                    <span className="text-xs px-2 py-0.5 rounded bg-purple-500/20 text-purple-300">
                      {task.score}分
                    </span>
                  ) : null}
                  <PhaseBadge phase={task.phase} />
                </div>
              </div>
              {task.message && <p className="text-xs text-gray-400 mb-2">{task.message}</p>}
              {task.output && (
                <pre className="text-xs bg-dark-bg border border-dark-border rounded p-2 max-h-24 overflow-y-auto mb-2 whitespace-pre-wrap">
                  {task.output.slice(0, 300)}
                  {task.output.length > 300 ? '...' : ''}
                </pre>
              )}
              <div className="flex flex-wrap justify-end gap-2">
                <a href={outputUrl} target="_blank" rel="noreferrer" className="text-xs px-3 py-1.5 bg-green-700 rounded">
                  任务产出
                </a>
                {chapterUrl && task.phase === 'Succeeded' && (
                  <a href={chapterUrl} target="_blank" rel="noreferrer" className="text-xs px-3 py-1.5 bg-emerald-700 rounded">
                    章节正文
                  </a>
                )}
                {task.phase === 'Running' && (
                  <Link to="/monitor" state={{ taskName: task.name }} className="text-xs px-3 py-1.5 bg-primary/30 text-primary border border-primary/40 rounded">
                    实时 SSE
                  </Link>
                )}
                <button type="button" onClick={() => setDetail(task)} className="text-xs px-3 py-1.5 border border-dark-border rounded">
                  详情
                </button>
                <button
                  type="button"
                  onClick={() => deleteTask(task.name).then(refresh)}
                  className="text-xs px-3 py-1.5 text-red-400 border border-red-500/40 rounded"
                >
                  删除
                </button>
              </div>
            </div>
          );
        })}
      </div>

      {filtered.length > 0 && totalPages > 1 && (
        <Pagination
          page={page}
          totalPages={totalPages}
          totalItems={totalItems}
          rangeStart={rangeStart}
          rangeEnd={rangeEnd}
          onPageChange={setPage}
        />
      )}

      <Modal title={`任务 - ${detail?.name || ''}`} open={!!detail} onClose={() => setDetail(null)}>
        {detail && (
          <div className="space-y-3 text-sm">
            <p>
              <span className="text-gray-400">状态: </span>
              {detail.phase}
            </p>
            <p>
              <span className="text-gray-400">消息: </span>
              {detail.message || '无'}
            </p>
            {detail.output && (
              <pre className="bg-dark-bg border border-dark-border rounded p-3 text-xs whitespace-pre-wrap max-h-80 overflow-y-auto">
                {detail.output}
              </pre>
            )}
          </div>
        )}
      </Modal>
    </div>
  );
}