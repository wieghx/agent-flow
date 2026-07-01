const STEP_LABELS: Record<string, string> = {
  'import-deconstruct': '拆书中',
  'import-rag-index': '构建 RAG 索引',
  'historical-research': '历史调研',
  'outline-skeleton': '分卷骨架',
  'outline-merge': '合并大纲',
  'outline-refine': '大纲精修',
  'outline': '生成大纲',
  'style-bible': '设定圣经',
  plots: '剧情扩写',
  chapters: '正文撰写',
  merge: '合并书稿',
};

export function formatWorkflowStep(step: string | undefined): string {
  if (!step) return '-';
  if (STEP_LABELS[step]) return STEP_LABELS[step];
  if (step.startsWith('plot-')) return `剧情 ${step.replace('plot-', '第')}章`;
  if (step.startsWith('chapter-')) return `正文 ${step.replace('chapter-', '第')}章`;
  if (step.startsWith('outline-vol-')) return `分卷大纲 ${step}`;
  return step;
}