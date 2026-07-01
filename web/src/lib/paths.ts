import type { TaskSummary } from '@/types/api';

export function taskOutputFileUrl(task: Pick<TaskSummary, 'name' | 'namespace'>): string {
  const ns = task.namespace || 'default';
  return `/outputs/${ns}/${task.name}.txt`;
}

export function parseWorkflowChapterTask(name: string): { workflow: string; step: string; num: string } | null {
  const marker = '-chapter-';
  const idx = name.lastIndexOf(marker);
  if (!name.startsWith('wf-') || idx === -1) return null;
  const workflow = name.slice(3, idx);
  const step = name.slice(idx + 1);
  const num = step.replace('chapter-', '');
  if (!/^\d+$/.test(num)) return null;
  return { workflow, step, num: num.padStart(3, '0') };
}

export function chapterMarkdownUrl(task: Pick<TaskSummary, 'name' | 'namespace'>): string | null {
  const parsed = parseWorkflowChapterTask(task.name);
  if (!parsed) return null;
  const ns = task.namespace || 'default';
  return `/outputs/workflows/${ns}/${parsed.workflow}/chapters/chapter-${parsed.num}.md`;
}

export function workspaceRelativeUrl(workspacePath: string | undefined, relativePath: string): string | null {
  if (!workspacePath) return null;
  const prefix = '/data/outputs';
  const base = workspacePath.startsWith(prefix) ? workspacePath.slice(prefix.length) : workspacePath;
  // Must use /outputs/... so the API file handler serves chapter markdown, not /workflows JSON.
  return `/outputs${base}/${relativePath}`.replace(/\/+/g, '/');
}

export function chapterUrlFromStep(
  workspacePath: string | undefined,
  stepId: string,
): string | null {
  if (!stepId.startsWith('chapter-')) return null;
  return workspaceRelativeUrl(workspacePath, `chapters/${stepId}.md`);
}

export function outlineUrl(workspacePath: string | undefined): string | null {
  return workspaceRelativeUrl(workspacePath, 'outline.json');
}