import { useCallback, useEffect, useMemo, useState } from 'react';
import { Link, useParams, useSearchParams } from 'react-router-dom';
import { Pagination } from '@/components/Pagination';
import { pageForIndex, usePagination } from '@/hooks/usePagination';
import {
  fetchNovelChapters,
  fetchNovels,
  fetchTextAsset,
  fetchWorkflowDetail,
  fetchWorkflows,
  regenerateChapter,
} from '@/api/client';
import { formatTokenCount } from '@/lib/tokens';
import { usePolling } from '@/hooks/usePolling';
import { chapterNumFromStepId, chapterUrlFromStep } from '@/lib/paths';
import { PhaseBadge } from '@/components/PhaseBadge';
import { Modal } from '@/components/Modal';
import { OutlineEditorModal } from '@/components/OutlineEditorModal';

const CHAPTER_PAGE_SIZE = 15;

export function NovelReaderPage() {
  const params = useParams();
  const [searchParams, setSearchParams] = useSearchParams();
  const namespace = params.namespace || 'default';
  const workflowName = params.name || searchParams.get('wf') || '';

  const { data: workflows = [] } = usePolling(fetchWorkflows, 15000);
  const { data: novels = [] } = usePolling(fetchNovels, 15000);
  const [detail, setDetail] = useState<Awaited<ReturnType<typeof fetchWorkflowDetail>> | null>(null);
  const [selectedChapter, setSelectedChapter] = useState(searchParams.get('ch') || '');
  const [chapterText, setChapterText] = useState('');
  const [loadingChapter, setLoadingChapter] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [showRewrite, setShowRewrite] = useState(false);
  const [rewriteLayer, setRewriteLayer] = useState<'chapter' | 'plot'>('chapter');
  const [rewriteInstruction, setRewriteInstruction] = useState('');
  const [selectedExcerpt, setSelectedExcerpt] = useState('');
  const [rewriting, setRewriting] = useState(false);
  const [rewriteJob, setRewriteJob] = useState<string | null>(null);
  const [rewriteStatus, setRewriteStatus] = useState<string | null>(null);
  const [showOutline, setShowOutline] = useState(false);
  const [chapterStats, setChapterStats] = useState<Record<number, { total_tokens?: number }>>({});

  const activeName = workflowName || workflows[0]?.name || '';
  const activeNovel = novels.find((n) => n.namespace === namespace && n.name === activeName);
  const displayTitle = activeNovel?.title?.trim() || activeName;

  const loadDetail = useCallback(async () => {
    if (!activeName) return;
    try {
      const wf = await fetchWorkflowDetail(activeName, namespace);
      setDetail(wf);
      setError(null);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }, [activeName, namespace]);

  useEffect(() => {
    loadDetail();
    const id = window.setInterval(loadDetail, 15000);
    return () => window.clearInterval(id);
  }, [loadDetail]);

  useEffect(() => {
    if (!activeName) return;
    fetchNovelChapters(namespace, activeName)
      .then((rows) => {
        const map: Record<number, { total_tokens?: number }> = {};
        for (const ch of rows) {
          map[ch.num] = { total_tokens: ch.total_tokens };
        }
        setChapterStats(map);
      })
      .catch(() => setChapterStats({}));
  }, [activeName, namespace, detail?.status?.completedSteps?.length]);

  const status = detail?.status;
  const workspace = status?.workspacePath;
  const completedChapters = useMemo(
    () => (status?.completedSteps || []).filter((s) => s.startsWith('chapter-')).sort(),
    [status?.completedSteps],
  );

  const {
    paginatedItems: pagedChapters,
    page: chapterPage,
    setPage: setChapterPage,
    totalPages: chapterTotalPages,
    totalItems: chapterTotalItems,
    rangeStart: chapterRangeStart,
    rangeEnd: chapterRangeEnd,
  } = usePagination(completedChapters, { pageSize: CHAPTER_PAGE_SIZE, resetKey: activeName });

  const reloadChapterText = useCallback(() => {
    if (!selectedChapter || !workspace) return;
    const url = chapterUrlFromStep(workspace, selectedChapter);
    if (!url) return;
    setLoadingChapter(true);
    fetchTextAsset(url)
      .then((text) => {
        const trimmed = text.trim();
        if (trimmed.startsWith('{') && trimmed.includes('"success"')) {
          throw new Error('章节路径错误，请刷新页面重试');
        }
        setChapterText(text);
      })
      .catch((e) => setChapterText(`加载失败: ${e.message}`))
      .finally(() => setLoadingChapter(false));
  }, [selectedChapter, workspace]);

  useEffect(() => {
    if (!selectedChapter && completedChapters.length > 0) {
      setSelectedChapter(completedChapters[0]);
    }
  }, [completedChapters, selectedChapter]);

  // Sync list page when selection changes (e.g. URL ?ch=). Do not depend on chapterPage —
  // otherwise manual pagination is immediately reset to the selected chapter's page.
  useEffect(() => {
    if (!selectedChapter) return;
    const idx = completedChapters.indexOf(selectedChapter);
    if (idx < 0) return;
    setChapterPage(pageForIndex(idx, CHAPTER_PAGE_SIZE));
  }, [selectedChapter, completedChapters, setChapterPage]);

  useEffect(() => {
    reloadChapterText();
  }, [reloadChapterText]);

  useEffect(() => {
    if (!rewriteJob) return;
    const poll = async () => {
      try {
        const wf = await fetchWorkflowDetail(rewriteJob, namespace);
        const phase = wf.status?.phase || 'Unknown';
        setRewriteStatus(phase);
        if (phase === 'Succeeded') {
          setRewriteJob(null);
          setRewriteStatus(null);
          await loadDetail();
          reloadChapterText();
        } else if (phase === 'Failed') {
          setRewriteJob(null);
          setError(`重写失败: ${wf.status?.message || 'unknown'}`);
        }
      } catch {
        /* ignore transient */
      }
    };
    poll();
    const id = window.setInterval(poll, 5000);
    return () => window.clearInterval(id);
  }, [rewriteJob, namespace, loadDetail, reloadChapterText]);

  const onSelectWorkflow = (name: string) => {
    setSearchParams({ wf: name });
    setSelectedChapter('');
    setChapterText('');
  };

  const onSelectChapter = (ch: string) => {
    const idx = completedChapters.indexOf(ch);
    if (idx >= 0) {
      setChapterPage(pageForIndex(idx, CHAPTER_PAGE_SIZE));
    }
    setSelectedChapter(ch);
    setSearchParams((prev) => {
      prev.set('ch', ch);
      if (activeName) prev.set('wf', activeName);
      return prev;
    });
  };

  const onTextSelection = () => {
    const sel = window.getSelection()?.toString().trim();
    if (sel && sel.length >= 8) {
      setSelectedExcerpt(sel);
    }
  };

  const openRewrite = () => {
    const base = rewriteInstruction.trim();
    if (selectedExcerpt && !base.includes(selectedExcerpt.slice(0, 40))) {
      setRewriteInstruction(
        base
          ? `${base}\n\n【选中片段】\n${selectedExcerpt}`
          : `请修改以下片段：\n${selectedExcerpt}`,
      );
    }
    setShowRewrite(true);
  };

  const submitRewrite = async () => {
    const num = chapterNumFromStepId(selectedChapter);
    if (!num || !rewriteInstruction.trim()) return;
    setRewriting(true);
    setError(null);
    try {
      const res = await regenerateChapter(namespace, activeName, num, {
        instruction: rewriteInstruction.trim(),
        layer: rewriteLayer,
      });
      setRewriteJob(res.rewrite_workflow);
      setRewriteStatus('Running');
      setShowRewrite(false);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setRewriting(false);
    }
  };

  if (!workflows.length && !activeName) {
    return <p className="p-10 text-center text-gray-500">暂无工作流，请先运行 novel-parallel-demo</p>;
  }

  return (
    <div className="h-full flex flex-col md:flex-row min-h-[calc(100vh-3.5rem)]">
      <aside className="w-full md:w-80 border-r border-dark-border bg-dark-card p-4 space-y-4 shrink-0 overflow-y-auto">
        <div>
          <label className="text-xs text-gray-400 block mb-1">工作流</label>
          <select
            className="w-full bg-dark-bg border border-dark-border rounded-lg px-3 py-2 text-sm"
            value={activeName}
            onChange={(e) => onSelectWorkflow(e.target.value)}
          >
            {workflows.map((wf) => {
              const novel = novels.find((n) => n.namespace === (wf.namespace || 'default') && n.name === wf.name);
              const label = novel?.title?.trim() || wf.name;
              return (
                <option key={wf.name} value={wf.name}>
                  {label} ({wf.progress}%)
                </option>
              );
            })}
          </select>
        </div>

        {status && (
          <div className="text-xs space-y-2">
            <p className="font-medium text-sm text-gray-200">{displayTitle}</p>
            <div className="flex items-center gap-2">
              <PhaseBadge phase={status.phase || 'Unknown'} />
              <span className="text-gray-400">{status.progress?.percent ?? 0}%</span>
            </div>
            <p className="text-gray-500">{status.message}</p>
            {activeNovel?.total_tokens ? (
              <p className="text-gray-500">
                全书 Token {formatTokenCount(activeNovel.total_tokens)}
                <span className="text-gray-600">
                  {' '}
                  (输入 {formatTokenCount(activeNovel.prompt_tokens)} / 输出{' '}
                  {formatTokenCount(activeNovel.completion_tokens)})
                </span>
              </p>
            ) : null}
            {rewriteStatus && (
              <p className="text-amber-400">重写进行中… ({rewriteStatus})</p>
            )}
            {workspace && (
              <button
                type="button"
                onClick={() => setShowOutline(true)}
                className="text-primary hover:underline block text-left"
              >
                查看 / 编辑大纲
              </button>
            )}
            <Link to="/workflows" className="text-gray-400 hover:text-white block">
              ← 返回工作流列表
            </Link>
          </div>
        )}

        <div>
          <h3 className="text-xs font-semibold text-gray-400 uppercase mb-2">
            已完成章节 ({completedChapters.length})
          </h3>
          {completedChapters.length > 0 && (
            <Pagination
              page={chapterPage}
              totalPages={chapterTotalPages}
              totalItems={chapterTotalItems}
              rangeStart={chapterRangeStart}
              rangeEnd={chapterRangeEnd}
              onPageChange={setChapterPage}
              className="mb-2"
            />
          )}
          <div className="space-y-1">
            {pagedChapters.map((ch) => {
              const num = chapterNumFromStepId(ch);
              const tokens = num ? chapterStats[num]?.total_tokens : undefined;
              return (
                <button
                  key={ch}
                  type="button"
                  onClick={() => onSelectChapter(ch)}
                  className={`w-full text-left px-3 py-2 rounded text-sm ${
                    selectedChapter === ch ? 'bg-primary/30 border border-primary/50' : 'hover:bg-dark-bg'
                  }`}
                >
                  <span className="flex items-center justify-between gap-2">
                    <span>{ch}</span>
                    {tokens ? (
                      <span className="text-[10px] text-gray-500 shrink-0">{formatTokenCount(tokens)}</span>
                    ) : null}
                  </span>
                </button>
              );
            })}
            {completedChapters.length === 0 && <p className="text-xs text-gray-600">尚无落盘章节</p>}
          </div>
        </div>

        {(status?.failedSteps?.length || 0) > 0 && (
          <p className="text-xs text-red-400">失败: {status!.failedSteps!.join(', ')}</p>
        )}
        {error && <p className="text-xs text-red-400">{error}</p>}
      </aside>

      <article className="flex-1 flex flex-col min-h-0">
        <div className="flex items-center justify-between gap-3 px-6 py-3 border-b border-dark-border bg-dark-card/50">
          <p className="text-sm text-gray-400 truncate">
            {selectedChapter ? `阅读 ${selectedChapter}` : '未选择章节'}
            {selectedChapter && chapterNumFromStepId(selectedChapter) ? (
              <span className="text-gray-600">
                {' '}
                · Token {formatTokenCount(chapterStats[chapterNumFromStepId(selectedChapter)!]?.total_tokens)}
              </span>
            ) : null}
          </p>
          <button
            type="button"
            disabled={!selectedChapter || !!rewriteJob}
            onClick={openRewrite}
            className="text-sm px-3 py-1.5 bg-primary rounded-lg disabled:opacity-40"
          >
            重写本章
          </button>
        </div>

        <div className="flex-1 p-6 overflow-y-auto" onMouseUp={onTextSelection}>
          {loadingChapter && <p className="text-gray-500">加载中...</p>}
          {!loadingChapter && !chapterText && (
            <p className="text-gray-500 text-center mt-20">选择左侧章节开始阅读</p>
          )}
          {!loadingChapter && chapterText && (
            <div className="max-w-3xl mx-auto">
              {selectedExcerpt && (
                <p className="text-xs text-gray-500 mb-3 border border-dark-border rounded p-2">
                  已选中片段（{selectedExcerpt.length} 字），点「重写本章」可带入修改意见
                </p>
              )}
              <pre className="whitespace-pre-wrap font-sans text-base leading-relaxed text-gray-100">
                {chapterText}
              </pre>
            </div>
          )}
        </div>
      </article>

      <OutlineEditorModal
        open={showOutline}
        namespace={namespace}
        name={activeName}
        displayTitle={displayTitle}
        onClose={() => setShowOutline(false)}
        onSaved={() => loadDetail()}
      />

      <Modal open={showRewrite} onClose={() => setShowRewrite(false)} title="重写本章">
        <div className="space-y-4">
          <p className="text-sm text-gray-400">
            将启动独立重写任务（RAG 参考 + 质检 + 同步梗概），完成后自动刷新本章正文。
          </p>
          <label className="block text-sm">
            <span className="text-gray-400">重写层级</span>
            <select
              className="mt-1 w-full bg-dark-bg border border-dark-border rounded-lg px-3 py-2"
              value={rewriteLayer}
              onChange={(e) => setRewriteLayer(e.target.value as 'chapter' | 'plot')}
            >
              <option value="chapter">正文</option>
              <option value="plot">剧情脚本</option>
            </select>
          </label>
          <label className="block text-sm">
            <span className="text-gray-400">修改意见 *</span>
            <textarea
              className="mt-1 w-full bg-dark-bg border border-dark-border rounded-lg px-3 py-2 min-h-[140px]"
              value={rewriteInstruction}
              onChange={(e) => setRewriteInstruction(e.target.value)}
              placeholder="例如：加强开篇悬念；选中片段语气更冷峻……"
            />
          </label>
          <div className="flex justify-end gap-2">
            <button
              type="button"
              onClick={() => setShowRewrite(false)}
              className="px-4 py-2 border border-dark-border rounded-lg text-sm"
            >
              取消
            </button>
            <button
              type="button"
              disabled={rewriting || !rewriteInstruction.trim()}
              onClick={submitRewrite}
              className="px-4 py-2 bg-primary rounded-lg text-sm font-medium disabled:opacity-50"
            >
              {rewriting ? '提交中…' : '开始重写'}
            </button>
          </div>
        </div>
      </Modal>
    </div>
  );
}