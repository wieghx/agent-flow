import { useCallback, useEffect, useMemo, useState } from 'react';
import { Link, useParams, useSearchParams } from 'react-router-dom';
import { fetchNovels, fetchTextAsset, fetchWorkflowDetail, fetchWorkflows } from '@/api/client';
import { usePolling } from '@/hooks/usePolling';
import { chapterUrlFromStep, outlineUrl } from '@/lib/paths';
import { PhaseBadge } from '@/components/PhaseBadge';

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

  const status = detail?.status;
  const workspace = status?.workspacePath;
  const completedChapters = useMemo(
    () => (status?.completedSteps || []).filter((s) => s.startsWith('chapter-')).sort(),
    [status?.completedSteps],
  );

  useEffect(() => {
    if (!selectedChapter && completedChapters.length > 0) {
      setSelectedChapter(completedChapters[0]);
    }
  }, [completedChapters, selectedChapter]);

  useEffect(() => {
    if (!selectedChapter || !workspace) {
      setChapterText('');
      return;
    }
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

  const onSelectWorkflow = (name: string) => {
    setSearchParams({ wf: name });
    setSelectedChapter('');
    setChapterText('');
  };

  const onSelectChapter = (ch: string) => {
    setSelectedChapter(ch);
    setSearchParams((prev) => {
      prev.set('ch', ch);
      if (activeName) prev.set('wf', activeName);
      return prev;
    });
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
            {outlineUrl(workspace) && (
              <a href={outlineUrl(workspace)!} target="_blank" rel="noreferrer" className="text-primary hover:underline block">
                查看 outline.json
              </a>
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
          <div className="space-y-1 max-h-96 overflow-y-auto">
            {completedChapters.map((ch) => (
              <button
                key={ch}
                type="button"
                onClick={() => onSelectChapter(ch)}
                className={`w-full text-left px-3 py-2 rounded text-sm ${
                  selectedChapter === ch ? 'bg-primary/30 border border-primary/50' : 'hover:bg-dark-bg'
                }`}
              >
                {ch}
              </button>
            ))}
            {completedChapters.length === 0 && <p className="text-xs text-gray-600">尚无落盘章节</p>}
          </div>
        </div>

        {(status?.failedSteps?.length || 0) > 0 && (
          <p className="text-xs text-red-400">失败: {status!.failedSteps!.join(', ')}</p>
        )}
        {error && <p className="text-xs text-red-400">{error}</p>}
      </aside>

      <article className="flex-1 p-6 overflow-y-auto">
        {loadingChapter && <p className="text-gray-500">加载中...</p>}
        {!loadingChapter && !chapterText && (
          <p className="text-gray-500 text-center mt-20">选择左侧章节开始阅读</p>
        )}
        {!loadingChapter && chapterText && (
          <div className="max-w-3xl mx-auto prose prose-invert prose-sm">
            <pre className="whitespace-pre-wrap font-sans text-base leading-relaxed text-gray-100">{chapterText}</pre>
          </div>
        )}
      </article>
    </div>
  );
}