import { useCallback, useEffect, useMemo, useState } from 'react';
import { fetchNovelOutline, saveNovelOutline } from '@/api/client';
import { Modal } from '@/components/Modal';
import type { NovelOutline } from '@/types/api';

type ViewMode = 'form' | 'json';

function emptyOutline(): NovelOutline {
  return { title: '', synopsis: '', characters: [], chapters: [] };
}

function parseOutlineJson(raw: string): NovelOutline {
  const parsed = JSON.parse(raw) as NovelOutline;
  if (!parsed || typeof parsed !== 'object' || !Array.isArray(parsed.chapters)) {
    throw new Error('JSON 须包含 chapters 数组');
  }
  return {
    title: String(parsed.title ?? ''),
    synopsis: String(parsed.synopsis ?? ''),
    characters: Array.isArray(parsed.characters) ? parsed.characters : [],
    chapters: parsed.chapters.map((ch, i) => ({
      num: Number(ch.num) || i + 1,
      title: String(ch.title ?? ''),
      summary: String(ch.summary ?? ''),
    })),
  };
}

export function OutlineEditorModal({
  open,
  namespace,
  name,
  displayTitle,
  onClose,
  onSaved,
}: {
  open: boolean;
  namespace: string;
  name: string;
  displayTitle?: string;
  onClose: () => void;
  onSaved?: (outline: NovelOutline) => void;
}) {
  const [mode, setMode] = useState<ViewMode>('form');
  const [outline, setOutline] = useState<NovelOutline>(emptyOutline);
  const [jsonText, setJsonText] = useState('');
  const [charactersText, setCharactersText] = useState('[]');
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const modalTitle = useMemo(
    () => `编辑大纲${displayTitle ? ` · ${displayTitle}` : ''}`,
    [displayTitle],
  );

  const applyOutline = useCallback((next: NovelOutline) => {
    setOutline(next);
    setJsonText(JSON.stringify(next, null, 2));
    setCharactersText(JSON.stringify(next.characters ?? [], null, 2));
  }, []);

  useEffect(() => {
    if (!open || !name) return;
    setLoading(true);
    setError(null);
    setMode('form');
    fetchNovelOutline(namespace, name)
      .then(applyOutline)
      .catch((e) => {
        setError(e instanceof Error ? e.message : String(e));
        applyOutline(emptyOutline());
      })
      .finally(() => setLoading(false));
  }, [open, namespace, name, applyOutline]);

  const switchMode = (next: ViewMode) => {
    if (next === mode) return;
    try {
      if (next === 'json') {
        const chars = JSON.parse(charactersText || '[]');
        const merged = { ...outline, characters: Array.isArray(chars) ? chars : [] };
        setJsonText(JSON.stringify(merged, null, 2));
      } else {
        const parsed = parseOutlineJson(jsonText);
        applyOutline(parsed);
      }
      setError(null);
      setMode(next);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  };

  const updateChapter = (index: number, field: 'title' | 'summary', value: string) => {
    setOutline((prev) => {
      const chapters = [...prev.chapters];
      chapters[index] = { ...chapters[index], [field]: value };
      return { ...prev, chapters };
    });
  };

  const handleSave = async () => {
    setSaving(true);
    setError(null);
    try {
      let payload: NovelOutline;
      if (mode === 'json') {
        payload = parseOutlineJson(jsonText);
      } else {
        const chars = JSON.parse(charactersText || '[]');
        if (!Array.isArray(chars)) {
          throw new Error('characters 须为 JSON 数组');
        }
        payload = { ...outline, characters: chars };
      }
      const saved = await saveNovelOutline(namespace, name, payload);
      applyOutline(saved);
      onSaved?.(saved);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setSaving(false);
    }
  };

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={modalTitle}
      wide
      footer={
        <>
          <button
            type="button"
            onClick={onClose}
            className="px-4 py-2 bg-dark-bg border border-dark-border rounded-lg hover:border-primary"
          >
            取消
          </button>
          <button
            type="button"
            disabled={loading || saving}
            onClick={handleSave}
            className="px-4 py-2 bg-primary rounded-lg font-medium disabled:opacity-50"
          >
            {saving ? '保存中…' : '保存大纲'}
          </button>
        </>
      }
    >
      <div className="space-y-4">
        <div className="flex items-center gap-2 text-sm">
          <button
            type="button"
            onClick={() => switchMode('form')}
            className={`px-3 py-1.5 rounded-lg border ${
              mode === 'form' ? 'border-primary bg-primary/20 text-white' : 'border-dark-border text-gray-400'
            }`}
          >
            结构化编辑
          </button>
          <button
            type="button"
            onClick={() => switchMode('json')}
            className={`px-3 py-1.5 rounded-lg border ${
              mode === 'json' ? 'border-primary bg-primary/20 text-white' : 'border-dark-border text-gray-400'
            }`}
          >
            JSON 源码
          </button>
          <span className="text-gray-500 text-xs ml-auto">
            共 {outline.chapters.length} 章 · 保存后写入工作区 outline.json
          </span>
        </div>

        {loading && <p className="text-gray-500 text-sm">加载大纲…</p>}
        {error && <p className="text-red-400 text-sm">{error}</p>}

        {!loading && mode === 'form' && (
          <div className="space-y-4">
            <label className="block text-sm">
              <span className="text-gray-400">书名</span>
              <input
                className="mt-1 w-full bg-dark-bg border border-dark-border rounded-lg px-3 py-2"
                value={outline.title}
                onChange={(e) => setOutline((prev) => ({ ...prev, title: e.target.value }))}
              />
            </label>
            <label className="block text-sm">
              <span className="text-gray-400">简介</span>
              <textarea
                className="mt-1 w-full bg-dark-bg border border-dark-border rounded-lg px-3 py-2 min-h-[88px]"
                value={outline.synopsis}
                onChange={(e) => setOutline((prev) => ({ ...prev, synopsis: e.target.value }))}
              />
            </label>
            <label className="block text-sm">
              <span className="text-gray-400">人物设定（JSON 数组）</span>
              <textarea
                className="mt-1 w-full bg-dark-bg border border-dark-border rounded-lg px-3 py-2 min-h-[100px] font-mono text-xs"
                value={charactersText}
                onChange={(e) => setCharactersText(e.target.value)}
                spellCheck={false}
              />
            </label>
            <div>
              <p className="text-gray-400 text-sm mb-2">章节梗概</p>
              <div className="space-y-3 max-h-[42vh] overflow-y-auto pr-1">
                {outline.chapters.map((ch, index) => (
                  <div key={`${ch.num}-${index}`} className="border border-dark-border rounded-lg p-3 bg-dark-bg/40">
                    <p className="text-xs text-gray-500 mb-2">第 {ch.num} 章</p>
                    <input
                      className="w-full bg-dark-bg border border-dark-border rounded px-2 py-1.5 text-sm mb-2"
                      value={ch.title}
                      placeholder="章节标题"
                      onChange={(e) => updateChapter(index, 'title', e.target.value)}
                    />
                    <textarea
                      className="w-full bg-dark-bg border border-dark-border rounded px-2 py-1.5 text-sm min-h-[72px]"
                      value={ch.summary}
                      placeholder="章节梗概"
                      onChange={(e) => updateChapter(index, 'summary', e.target.value)}
                    />
                  </div>
                ))}
                {outline.chapters.length === 0 && (
                  <p className="text-gray-600 text-sm">暂无章节，请切换 JSON 模式编辑或等待大纲生成完成。</p>
                )}
              </div>
            </div>
          </div>
        )}

        {!loading && mode === 'json' && (
          <textarea
            className="w-full bg-dark-bg border border-dark-border rounded-lg px-3 py-2 min-h-[52vh] font-mono text-xs leading-relaxed"
            value={jsonText}
            onChange={(e) => setJsonText(e.target.value)}
            spellCheck={false}
          />
        )}
      </div>
    </Modal>
  );
}