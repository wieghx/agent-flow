import { useState } from 'react';
import { Link } from 'react-router-dom';
import { Pagination } from '@/components/Pagination';
import { usePagination } from '@/hooks/usePagination';
import {
  createNovel,
  deleteNovel,
  fetchNovels,
  importNovel,
  resumeNovel,
  searchNovelRAG,
} from '@/api/client';
import { usePolling } from '@/hooks/usePolling';
import { PhaseBadge } from '@/components/PhaseBadge';
import { PipelineStageBar } from '@/components/PipelineStageBar';
import { Modal } from '@/components/Modal';
import { OutlineEditorModal } from '@/components/OutlineEditorModal';
import type { NovelSummary, RAGChunk } from '@/types/api';

const EMPTY_FORM = {
  title: '',
  prompt: '',
  chapter_count: 20,
  words_per_chapter: 2500,
  quality_threshold: 72,
};

const EMPTY_IMPORT = {
  title: '',
  prompt: '',
  text: '',
  continue_writing: true,
};

export function NovelLibraryPage() {
  const { data: novels = [], refresh, error } = usePolling(fetchNovels, 10000);
  const [showCreate, setShowCreate] = useState(false);
  const [showImport, setShowImport] = useState(false);
  const [form, setForm] = useState(EMPTY_FORM);
  const [importForm, setImportForm] = useState(EMPTY_IMPORT);
  const [creating, setCreating] = useState(false);
  const [importing, setImporting] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);
  const [expanded, setExpanded] = useState<string | null>(null);
  const [ragQuery, setRagQuery] = useState('');
  const [ragChunks, setRagChunks] = useState<RAGChunk[]>([]);
  const [ragLoading, setRagLoading] = useState(false);
  const [outlineEditor, setOutlineEditor] = useState<NovelSummary | null>(null);

  const {
    paginatedItems: pagedNovels,
    page,
    setPage,
    totalPages,
    totalItems,
    rangeStart,
    rangeEnd,
  } = usePagination(novels, { pageSize: 6 });

  const displayTitle = (n: NovelSummary) =>
    n.title?.trim() || n.name;

  const submitImport = async () => {
    if (!importForm.text.trim()) return;
    setImporting(true);
    setActionError(null);
    try {
      await importNovel({
        title: importForm.title.trim(),
        prompt: importForm.prompt.trim(),
        text: importForm.text.trim(),
        continue_writing: importForm.continue_writing,
      });
      setShowImport(false);
      setImportForm(EMPTY_IMPORT);
      await refresh();
    } catch (e) {
      setActionError(e instanceof Error ? e.message : String(e));
    } finally {
      setImporting(false);
    }
  };

  const onRagSearch = async (n: NovelSummary) => {
    const q = ragQuery.trim();
    if (!q) return;
    setRagLoading(true);
    setActionError(null);
    try {
      const res = await searchNovelRAG(n.namespace, n.name, q);
      setRagChunks(res.chunks || []);
    } catch (e) {
      setActionError(e instanceof Error ? e.message : String(e));
      setRagChunks([]);
    } finally {
      setRagLoading(false);
    }
  };

  const submitCreate = async () => {
    if (!form.prompt.trim() && !form.title.trim()) return;
    setCreating(true);
    setActionError(null);
    try {
      await createNovel({
        title: form.title.trim(),
        prompt: form.prompt.trim(),
        chapter_count: form.chapter_count,
        words_per_chapter: form.words_per_chapter,
        quality_threshold: form.quality_threshold,
      });
      setShowCreate(false);
      setForm(EMPTY_FORM);
      await refresh();
    } catch (e) {
      setActionError(e instanceof Error ? e.message : String(e));
    } finally {
      setCreating(false);
    }
  };

  const onResume = async (n: NovelSummary) => {
    setActionError(null);
    try {
      await resumeNovel(n.namespace, n.name);
      await refresh();
    } catch (e) {
      setActionError(e instanceof Error ? e.message : String(e));
    }
  };

  const onDelete = async (n: NovelSummary) => {
    if (!window.confirm(`确定删除「${displayTitle(n)}」？Workflow 与元数据将移除，PVC 章节文件保留。`)) {
      return;
    }
    setActionError(null);
    try {
      await deleteNovel(n.namespace, n.name);
      await refresh();
    } catch (e) {
      setActionError(e instanceof Error ? e.message : String(e));
    }
  };

  return (
    <div className="p-5 max-w-6xl mx-auto space-y-5">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 className="text-lg font-semibold">小说库</h2>
          <p className="text-sm text-gray-400">管理多部小说的生成、进度与产出</p>
        </div>
        <div className="flex gap-2">
          <button
            type="button"
            onClick={refresh}
            className="text-sm px-3 py-2 border border-dark-border rounded-lg hover:bg-dark-bg"
          >
            刷新
          </button>
          <button
            type="button"
            onClick={() => setShowImport(true)}
            className="text-sm px-4 py-2 border border-dark-border rounded-lg hover:bg-dark-bg"
          >
            导入拆书
          </button>
          <button
            type="button"
            onClick={() => setShowCreate(true)}
            className="text-sm px-4 py-2 bg-primary rounded-lg font-medium"
          >
            + 新建小说
          </button>
        </div>
      </div>

      {(error || actionError) && (
        <p className="text-sm text-red-400">{error || actionError}</p>
      )}

      {novels.length === 0 && (
        <div className="text-center py-20 text-gray-500 border border-dashed border-dark-border rounded-xl">
          <p className="mb-4">还没有小说项目</p>
          <button type="button" onClick={() => setShowCreate(true)} className="text-primary hover:underline">
            创建第一部小说
          </button>
        </div>
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

      <div className="grid gap-4 md:grid-cols-2">
        {pagedNovels.map((n) => {
          const key = `${n.namespace}/${n.name}`;
          const isOpen = expanded === key;
          const canResume =
            ['Failed', 'Paused'].includes(n.phase) ||
            (n.phase === 'Succeeded' && n.chapters_done < n.chapter_count);
          return (
            <div
              key={key}
              className="bg-dark-card border border-dark-border rounded-xl p-4 flex flex-col gap-3"
            >
              <div className="flex items-start justify-between gap-2">
                <div className="min-w-0">
                  <h3 className="font-semibold truncate">{displayTitle(n)}</h3>
                  <p className="text-xs text-gray-500 truncate">{n.name} · {n.namespace}</p>
                </div>
                <PhaseBadge phase={n.phase} />
              </div>

              {n.synopsis && (
                <p className="text-sm text-gray-400 line-clamp-2">{n.synopsis}</p>
              )}

              <PipelineStageBar novel={n} />

              <div className="w-full bg-dark-bg rounded-full h-2">
                <div
                  className="bg-primary h-2 rounded-full transition-all"
                  style={{ width: `${Math.min(n.progress, 100)}%` }}
                />
              </div>

              <div className="flex flex-wrap gap-x-4 gap-y-1 text-xs text-gray-500">
                <span>{n.progress}% 总进度</span>
                <span>
                  章节 {n.chapters_done}/{n.chapter_count || '?'}
                </span>
                {n.chapters_writing > 0 && <span className="text-amber-400">撰写中 {n.chapters_writing}</span>}
                {n.chapters_failed > 0 && <span className="text-red-400">失败 {n.chapters_failed}</span>}
              </div>

              <div className="flex flex-wrap gap-2 pt-1">
                <Link
                  to={`/novel/${n.namespace}/${n.name}`}
                  className="text-xs px-3 py-1.5 bg-emerald-700/80 rounded text-white"
                >
                  阅读
                </Link>
                {canResume && n.phase !== 'Running' && (
                  <button
                    type="button"
                    onClick={() => onResume(n)}
                    className="text-xs px-3 py-1.5 bg-amber-700/80 rounded text-white"
                  >
                    恢复生成
                  </button>
                )}
                {n.book_url && (
                  <a
                    href={n.book_url}
                    target="_blank"
                    rel="noreferrer"
                    className="text-xs px-3 py-1.5 border border-dark-border rounded"
                  >
                    导出书稿
                  </a>
                )}
                <button
                  type="button"
                  onClick={() => {
                    if (isOpen) {
                      setExpanded(null);
                      setRagChunks([]);
                      setRagQuery('');
                    } else {
                      setExpanded(key);
                      setRagChunks([]);
                      setRagQuery('');
                    }
                  }}
                  className="text-xs px-3 py-1.5 border border-dark-border rounded"
                >
                  {isOpen ? '收起' : '详情'}
                </button>
                <button
                  type="button"
                  onClick={() => onDelete(n)}
                  className="text-xs px-3 py-1.5 border border-red-900/50 text-red-400 rounded ml-auto"
                >
                  删除
                </button>
              </div>

              {isOpen && (
                <div className="text-xs space-y-2 border-t border-dark-border pt-3 text-gray-400">
                  {n.message && <p>状态: {n.message}</p>}
                  {n.currentStep && <p>当前步骤: {n.currentStep}</p>}
                  {n.prompt && (
                    <div>
                      <p className="text-gray-500 mb-1">创作约束 / Prompt:</p>
                      <pre className="whitespace-pre-wrap bg-dark-bg p-2 rounded max-h-40 overflow-y-auto text-gray-300">
                        {n.prompt}
                      </pre>
                    </div>
                  )}
                  {n.params && Object.keys(n.params).length > 0 && (
                    <p>参数: {JSON.stringify(n.params)}</p>
                  )}
                  {n.outline_url && (
                    <button
                      type="button"
                      onClick={() => setOutlineEditor(n)}
                      className="text-primary hover:underline block text-left"
                    >
                      查看 / 编辑大纲
                    </button>
                  )}
                  {n.params?.ragEnabled !== 'false' && (
                    <div className="space-y-2 pt-1">
                      <p className="text-gray-500">RAG 剧情检索</p>
                      <div className="flex gap-2">
                        <input
                          className="flex-1 bg-dark-bg border border-dark-border rounded px-2 py-1 text-gray-300"
                          placeholder="关键词，如：朝堂 宰相"
                          value={expanded === key ? ragQuery : ''}
                          onChange={(e) => setRagQuery(e.target.value)}
                          onKeyDown={(e) => e.key === 'Enter' && onRagSearch(n)}
                        />
                        <button
                          type="button"
                          disabled={ragLoading}
                          onClick={() => onRagSearch(n)}
                          className="px-2 py-1 border border-dark-border rounded text-gray-300 disabled:opacity-50"
                        >
                          {ragLoading ? '…' : '检索'}
                        </button>
                      </div>
                      {ragChunks.length > 0 && (
                        <ul className="space-y-1 max-h-32 overflow-y-auto">
                          {ragChunks.map((ch) => (
                            <li key={ch.id} className="bg-dark-bg p-2 rounded text-gray-400">
                              <span className="text-gray-500">[{ch.source}] </span>
                              {ch.text.length > 120 ? `${ch.text.slice(0, 120)}…` : ch.text}
                            </li>
                          ))}
                        </ul>
                      )}
                    </div>
                  )}
                  <p className="text-gray-600">创建: {n.created_at}</p>
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

      <Modal open={showCreate} onClose={() => setShowCreate(false)} title="新建小说">
        <div className="space-y-4">
          <label className="block text-sm">
            <span className="text-gray-400">书名（可选）</span>
            <input
              className="mt-1 w-full bg-dark-bg border border-dark-border rounded-lg px-3 py-2"
              value={form.title}
              onChange={(e) => setForm((f) => ({ ...f, title: e.target.value }))}
              placeholder="荒岛求生"
            />
          </label>
          <label className="block text-sm">
            <span className="text-gray-400">创作约束 / 题材设定 *</span>
            <textarea
              className="mt-1 w-full bg-dark-bg border border-dark-border rounded-lg px-3 py-2 min-h-[120px]"
              value={form.prompt}
              onChange={(e) => setForm((f) => ({ ...f, prompt: e.target.value }))}
              placeholder="第三人称，荒岛生存题材，禁止现代网络用语……"
            />
          </label>
          <div className="grid grid-cols-3 gap-3 text-sm">
            <label>
              <span className="text-gray-400 block mb-1">章节数</span>
              <input
                type="number"
                min={1}
                className="w-full bg-dark-bg border border-dark-border rounded-lg px-3 py-2"
                value={form.chapter_count}
                onChange={(e) => setForm((f) => ({ ...f, chapter_count: Number(e.target.value) }))}
              />
            </label>
            <label>
              <span className="text-gray-400 block mb-1">每章字数</span>
              <input
                type="number"
                min={500}
                className="w-full bg-dark-bg border border-dark-border rounded-lg px-3 py-2"
                value={form.words_per_chapter}
                onChange={(e) => setForm((f) => ({ ...f, words_per_chapter: Number(e.target.value) }))}
              />
            </label>
            <label>
              <span className="text-gray-400 block mb-1">质量分阈值</span>
              <input
                type="number"
                min={50}
                max={100}
                className="w-full bg-dark-bg border border-dark-border rounded-lg px-3 py-2"
                value={form.quality_threshold}
                onChange={(e) => setForm((f) => ({ ...f, quality_threshold: Number(e.target.value) }))}
              />
            </label>
          </div>
          <div className="flex justify-end gap-2 pt-2">
            <button
              type="button"
              onClick={() => setShowCreate(false)}
              className="px-4 py-2 border border-dark-border rounded-lg text-sm"
            >
              取消
            </button>
            <button
              type="button"
              disabled={creating || (!form.prompt.trim() && !form.title.trim())}
              onClick={submitCreate}
              className="px-4 py-2 bg-primary rounded-lg text-sm font-medium disabled:opacity-50"
            >
              {creating ? '创建中…' : '开始生成'}
            </button>
          </div>
        </div>
      </Modal>

      <Modal open={showImport} onClose={() => setShowImport(false)} title="导入小说并拆书">
        <div className="space-y-4">
          <p className="text-sm text-gray-400">
            粘贴已有小说全文，系统将自动拆章、AI 拆书提取人物与梗概，构建 RAG 索引，并可续写后续章节。
          </p>
          <label className="block text-sm">
            <span className="text-gray-400">书名（可选）</span>
            <input
              className="mt-1 w-full bg-dark-bg border border-dark-border rounded-lg px-3 py-2"
              value={importForm.title}
              onChange={(e) => setImportForm((f) => ({ ...f, title: e.target.value }))}
              placeholder="导入小说"
            />
          </label>
          <label className="block text-sm">
            <span className="text-gray-400">小说全文 *</span>
            <textarea
              className="mt-1 w-full bg-dark-bg border border-dark-border rounded-lg px-3 py-2 min-h-[200px] font-mono text-xs"
              value={importForm.text}
              onChange={(e) => setImportForm((f) => ({ ...f, text: e.target.value }))}
              placeholder={'第1章 标题\n正文…\n\n第2章 标题\n正文…'}
            />
          </label>
          <label className="block text-sm">
            <span className="text-gray-400">续写说明（可选）</span>
            <textarea
              className="mt-1 w-full bg-dark-bg border border-dark-border rounded-lg px-3 py-2 min-h-[80px]"
              value={importForm.prompt}
              onChange={(e) => setImportForm((f) => ({ ...f, prompt: e.target.value }))}
              placeholder="拆书后希望如何续写、改写方向……"
            />
          </label>
          <label className="flex items-center gap-2 text-sm text-gray-400">
            <input
              type="checkbox"
              checked={importForm.continue_writing}
              onChange={(e) => setImportForm((f) => ({ ...f, continue_writing: e.target.checked }))}
            />
            拆书完成后自动续写（三阶段：剧情 → 正文）
          </label>
          <div className="flex justify-end gap-2 pt-2">
            <button
              type="button"
              onClick={() => setShowImport(false)}
              className="px-4 py-2 border border-dark-border rounded-lg text-sm"
            >
              取消
            </button>
            <button
              type="button"
              disabled={importing || !importForm.text.trim()}
              onClick={submitImport}
              className="px-4 py-2 bg-primary rounded-lg text-sm font-medium disabled:opacity-50"
            >
              {importing ? '导入中…' : '开始拆书'}
            </button>
          </div>
        </div>
      </Modal>

      <OutlineEditorModal
        open={!!outlineEditor}
        namespace={outlineEditor?.namespace || 'default'}
        name={outlineEditor?.name || ''}
        displayTitle={outlineEditor ? displayTitle(outlineEditor) : undefined}
        onClose={() => setOutlineEditor(null)}
        onSaved={() => refresh()}
      />
    </div>
  );
}