import { useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { fetchTokenReport } from '@/api/client';
import { usePolling } from '@/hooks/usePolling';
import { usePagination } from '@/hooks/usePagination';
import { Pagination } from '@/components/Pagination';
import { formatTokenCount } from '@/lib/tokens';
import type { TokenReportNovel } from '@/types/api';

type SortKey = 'total' | 'prompt' | 'completion' | 'avg' | 'title';

function displayTitle(n: TokenReportNovel) {
  return n.title?.trim() || n.name;
}

function exportCsv(novels: TokenReportNovel[]) {
  const header = ['namespace', 'name', 'title', 'chapters_done', 'chapter_count', 'prompt_tokens', 'completion_tokens', 'total_tokens', 'avg_chapter_tokens'];
  const rows = novels.map((n) => [
    n.namespace,
    n.name,
    displayTitle(n),
    String(n.chapters_done),
    String(n.chapter_count),
    String(n.prompt_tokens),
    String(n.completion_tokens),
    String(n.total_tokens),
    String(n.avg_chapter_tokens),
  ]);
  const csv = [header, ...rows].map((r) => r.map((c) => `"${c.replace(/"/g, '""')}"`).join(',')).join('\n');
  const blob = new Blob([csv], { type: 'text/csv;charset=utf-8' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = `token-report-${new Date().toISOString().slice(0, 10)}.csv`;
  a.click();
  URL.revokeObjectURL(url);
}

function StatCard({ label, value, hint }: { label: string; value: string; hint?: string }) {
  return (
    <div className="bg-dark-card border border-dark-border rounded-xl p-4">
      <p className="text-xs text-gray-500 uppercase tracking-wide">{label}</p>
      <p className="text-2xl font-semibold mt-1 tabular-nums">{value}</p>
      {hint ? <p className="text-xs text-gray-600 mt-1">{hint}</p> : null}
    </div>
  );
}

export function TokenReportPage() {
  const { data: report, refresh, error } = usePolling(fetchTokenReport, 15000);
  const [query, setQuery] = useState('');
  const [onlyWithUsage, setOnlyWithUsage] = useState(false);
  const [sortKey, setSortKey] = useState<SortKey>('total');
  const [expanded, setExpanded] = useState<string | null>(null);

  const novels = useMemo(() => {
    let list = report?.novels || [];
    const q = query.trim().toLowerCase();
    if (q) {
      list = list.filter(
        (n) =>
          n.name.toLowerCase().includes(q) ||
          n.title?.toLowerCase().includes(q) ||
          n.namespace.toLowerCase().includes(q),
      );
    }
    if (onlyWithUsage) {
      list = list.filter((n) => n.total_tokens > 0);
    }
    return [...list].sort((a, b) => {
      switch (sortKey) {
        case 'title':
          return displayTitle(a).localeCompare(displayTitle(b), 'zh');
        case 'prompt':
          return b.prompt_tokens - a.prompt_tokens;
        case 'completion':
          return b.completion_tokens - a.completion_tokens;
        case 'avg':
          return b.avg_chapter_tokens - a.avg_chapter_tokens;
        default:
          return b.total_tokens - a.total_tokens;
      }
    });
  }, [report?.novels, query, onlyWithUsage, sortKey]);

  const {
    paginatedItems: pagedNovels,
    page,
    setPage,
    totalPages,
    totalItems,
    rangeStart,
    rangeEnd,
  } = usePagination(novels, { pageSize: 8, resetKey: `${query}|${onlyWithUsage}|${sortKey}` });

  const r = report;

  return (
    <div className="p-5 max-w-6xl mx-auto space-y-5">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h2 className="text-lg font-semibold">Token 用量汇总</h2>
          <p className="text-sm text-gray-400 mt-1">
            统计所有小说的 AI 调用 token，含大纲、剧情、正文、质检与重试。
          </p>
        </div>
        <div className="flex flex-wrap gap-2">
          <button
            type="button"
            onClick={refresh}
            className="text-sm px-3 py-1.5 border border-dark-border rounded-lg hover:bg-dark-bg"
          >
            刷新
          </button>
          <button
            type="button"
            disabled={!novels.length}
            onClick={() => exportCsv(novels)}
            className="text-sm px-3 py-1.5 bg-primary rounded-lg disabled:opacity-40"
          >
            导出 CSV
          </button>
        </div>
      </div>

      {error && <p className="text-red-400 text-sm">{error}</p>}

      {!r && !error && <p className="text-gray-500 text-center py-16">加载中…</p>}

      {r && (
        <>
          <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
            <StatCard label="总 Token" value={formatTokenCount(r.total_tokens)} hint={`${r.novel_count} 部小说`} />
            <StatCard
              label="输入 / 输出"
              value={`${formatTokenCount(r.prompt_tokens)} / ${formatTokenCount(r.completion_tokens)}`}
              hint="Prompt / Completion"
            />
            <StatCard
              label="平均每部小说"
              value={formatTokenCount(r.avg_novel_tokens)}
              hint={`${r.chapters_with_tokens} 章有数据`}
            />
            <StatCard
              label="平均每章"
              value={formatTokenCount(r.avg_chapter_tokens)}
              hint={`已完成 ${r.chapters_done} / ${r.chapter_count} 章`}
            />
          </div>

          <div className="flex flex-wrap gap-3 items-center">
            <input
              type="search"
              placeholder="搜索书名 / 工作流名…"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              className="flex-1 min-w-[200px] bg-dark-bg border border-dark-border rounded-lg px-3 py-2 text-sm"
            />
            <label className="flex items-center gap-2 text-sm text-gray-400">
              <input
                type="checkbox"
                checked={onlyWithUsage}
                onChange={(e) => setOnlyWithUsage(e.target.checked)}
                className="rounded"
              />
              仅显示有 Token 记录
            </label>
            <select
              value={sortKey}
              onChange={(e) => setSortKey(e.target.value as SortKey)}
              className="bg-dark-bg border border-dark-border rounded-lg px-3 py-2 text-sm"
            >
              <option value="total">按总量排序</option>
              <option value="prompt">按输入排序</option>
              <option value="completion">按输出排序</option>
              <option value="avg">按章均排序</option>
              <option value="title">按书名排序</option>
            </select>
          </div>

          {novels.length === 0 && (
            <p className="text-center text-gray-500 py-12">
              {r.novel_count === 0 ? '暂无小说数据，请先在小说库创建项目。' : '没有匹配的小说。'}
            </p>
          )}

          {novels.length > 0 && (
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
            {pagedNovels.map((n) => {
              const key = `${n.namespace}/${n.name}`;
              const isOpen = expanded === key;
              const chaptersWithTokens = n.chapters.filter((ch) => ch.total_tokens > 0);
              return (
                <div key={key} className="bg-dark-card border border-dark-border rounded-xl overflow-hidden">
                  <button
                    type="button"
                    onClick={() => setExpanded(isOpen ? null : key)}
                    className="w-full text-left px-4 py-3 flex flex-wrap items-center justify-between gap-3 hover:bg-dark-bg/50"
                  >
                    <div className="min-w-0">
                      <p className="font-medium truncate">{displayTitle(n)}</p>
                      <p className="text-xs text-gray-500 truncate">
                        {n.namespace}/{n.name} · 章节 {n.chapters_done}/{n.chapter_count}
                      </p>
                    </div>
                    <div className="flex flex-wrap gap-x-4 gap-y-1 text-sm tabular-nums shrink-0">
                      <span className="text-primary font-semibold">{formatTokenCount(n.total_tokens)}</span>
                      <span className="text-gray-500">
                        输入 {formatTokenCount(n.prompt_tokens)}
                      </span>
                      <span className="text-gray-500">
                        输出 {formatTokenCount(n.completion_tokens)}
                      </span>
                      {n.avg_chapter_tokens > 0 && (
                        <span className="text-gray-500">章均 {formatTokenCount(n.avg_chapter_tokens)}</span>
                      )}
                    </div>
                  </button>

                  {isOpen && (
                    <div className="border-t border-dark-border px-4 py-3 space-y-3">
                      <div className="flex flex-wrap gap-2">
                        <Link
                          to={`/novel/${n.namespace}/${n.name}`}
                          className="text-xs px-3 py-1.5 bg-emerald-700/80 rounded text-white"
                        >
                          阅读
                        </Link>
                        <Link to="/library" className="text-xs px-3 py-1.5 border border-dark-border rounded">
                          小说库
                        </Link>
                      </div>

                      {chaptersWithTokens.length === 0 ? (
                        <p className="text-xs text-gray-600">本章尚无 Token 记录（可能为历史任务或未启用统计）。</p>
                      ) : (
                        <div className="overflow-x-auto">
                          <table className="w-full text-xs">
                            <thead>
                              <tr className="text-gray-500 text-left border-b border-dark-border">
                                <th className="py-2 pr-3 font-medium">章</th>
                                <th className="py-2 pr-3 font-medium">标题</th>
                                <th className="py-2 pr-3 font-medium">状态</th>
                                <th className="py-2 pr-3 font-medium text-right">字数</th>
                                <th className="py-2 pr-3 font-medium text-right">输入</th>
                                <th className="py-2 pr-3 font-medium text-right">输出</th>
                                <th className="py-2 font-medium text-right">合计</th>
                              </tr>
                            </thead>
                            <tbody>
                              {chaptersWithTokens.map((ch) => (
                                <tr key={ch.num} className="border-b border-dark-border/50 last:border-0">
                                  <td className="py-2 pr-3 tabular-nums">第{ch.num}章</td>
                                  <td className="py-2 pr-3 max-w-[12rem] truncate">{ch.title || '—'}</td>
                                  <td className="py-2 pr-3 text-gray-500">{ch.status}</td>
                                  <td className="py-2 pr-3 text-right tabular-nums">{ch.word_count || '—'}</td>
                                  <td className="py-2 pr-3 text-right tabular-nums">{formatTokenCount(ch.prompt_tokens)}</td>
                                  <td className="py-2 pr-3 text-right tabular-nums">{formatTokenCount(ch.completion_tokens)}</td>
                                  <td className="py-2 text-right tabular-nums text-primary">{formatTokenCount(ch.total_tokens)}</td>
                                </tr>
                              ))}
                            </tbody>
                          </table>
                        </div>
                      )}
                    </div>
                  )}
                </div>
              );
            })}
          </div>

          {novels.length > 0 && totalPages > 1 && (
            <Pagination
              page={page}
              totalPages={totalPages}
              totalItems={totalItems}
              rangeStart={rangeStart}
              rangeEnd={rangeEnd}
              onPageChange={setPage}
            />
          )}
        </>
      )}
    </div>
  );
}