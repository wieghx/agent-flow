import type { NovelSummary } from '@/types/api';

const STAGE_LABELS: Record<string, string> = {
  deconstruct: '拆书',
  'rag-index': 'RAG 索引',
  outline: '大纲',
  plots: '剧情',
  chapters: '正文',
  merge: '合并',
  done: '完成',
};

type StageId = keyof typeof STAGE_LABELS;

function stagesForNovel(n: NovelSummary): StageId[] {
  const threeStage = n.params?.threeStage !== 'false';
  const isImport =
    n.template === 'novel-import-deconstruct' || n.params?.importedNovel === 'true';

  const stages: StageId[] = [];
  if (isImport) {
    stages.push('deconstruct', 'rag-index');
  }
  stages.push('outline');
  if (threeStage) {
    stages.push('plots');
  }
  stages.push('chapters', 'merge', 'done');
  return stages;
}

function stageIndex(stages: StageId[], current: string | undefined): number {
  if (!current) return 0;
  const idx = stages.indexOf(current as StageId);
  return idx >= 0 ? idx : 0;
}

export function PipelineStageBar({ novel }: { novel: NovelSummary }) {
  const stages = stagesForNovel(novel);
  const activeIdx = stageIndex(stages, novel.pipeline_stage);
  const showPlots = stages.includes('plots');
  const total = novel.chapter_count || 0;

  return (
    <div className="space-y-2">
      <div className="flex flex-wrap gap-1">
        {stages
          .filter((s) => s !== 'done')
          .map((stage, i) => {
            const isActive = stage === novel.pipeline_stage;
            const isPast = i < activeIdx;
            return (
              <span
                key={stage}
                className={`text-[10px] px-2 py-0.5 rounded-full border ${
                  isActive
                    ? 'border-primary bg-primary/20 text-primary'
                    : isPast
                      ? 'border-emerald-800/60 bg-emerald-900/20 text-emerald-400'
                      : 'border-dark-border text-gray-500'
                }`}
              >
                {STAGE_LABELS[stage]}
              </span>
            );
          })}
      </div>
      {(showPlots || total > 0) && (
        <div className="flex flex-wrap gap-x-3 gap-y-0.5 text-[10px] text-gray-500">
          {showPlots && total > 0 && (
            <span>
              剧情 {novel.plots_done ?? 0}/{total}
              {(novel.plots_writing ?? 0) > 0 && (
                <span className="text-amber-400"> · 进行中 {novel.plots_writing}</span>
              )}
            </span>
          )}
          {total > 0 && (
            <span>
              正文 {novel.chapters_done}/{total}
              {(novel.chapters_writing ?? 0) > 0 && (
                <span className="text-amber-400"> · 撰写中 {novel.chapters_writing}</span>
              )}
            </span>
          )}
        </div>
      )}
    </div>
  );
}