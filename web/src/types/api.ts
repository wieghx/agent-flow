export interface ApiResponse<T = unknown> {
  success: boolean;
  message?: string;
  data?: T;
  error?: string;
}

export interface QualityCheck {
  score?: number;
  passed?: boolean;
  feedback?: string;
  taskType?: string;
  checkMethod?: string;
  attempt?: number;
  issues?: string[];
  dimensions?: {
    completeness?: number;
    accuracy?: number;
    quality?: number;
  };
}

export interface TaskSummary {
  name: string;
  namespace: string;
  workflow?: string;
  step_id?: string;
  phase: string;
  message?: string;
  output?: string;
  score?: number;
  passed?: boolean;
  retries?: number;
  qualityCheck?: QualityCheck;
  created_at?: string;
  completion_at?: string;
}

export interface NovelSummary {
  namespace: string;
  name: string;
  title?: string;
  synopsis?: string;
  phase: string;
  progress: number;
  currentStep?: string;
  message?: string;
  chapter_count: number;
  chapters_done: number;
  chapters_writing: number;
  chapters_failed: number;
  plots_done?: number;
  plots_writing?: number;
  plots_failed?: number;
  pipeline_stage?: string;
  prompt?: string;
  template?: string;
  params?: Record<string, string>;
  workspace_path?: string;
  book_url?: string;
  outline_url?: string;
  created_at?: string;
  updated_at?: string;
  completion_at?: string;
  prompt_tokens?: number;
  completion_tokens?: number;
  total_tokens?: number;
}

export interface ChapterSummary {
  num: number;
  title: string;
  summary?: string;
  status: string;
  word_count: number;
  prompt_tokens: number;
  completion_tokens: number;
  total_tokens: number;
}

export interface TokenReportNovel {
  namespace: string;
  name: string;
  title: string;
  chapter_count: number;
  chapters_done: number;
  prompt_tokens: number;
  completion_tokens: number;
  total_tokens: number;
  avg_chapter_tokens: number;
  estimated_cost_usd?: number;
  chapters: ChapterSummary[];
}

export interface TokenReport {
  novel_count: number;
  chapter_count: number;
  chapters_done: number;
  chapters_with_tokens: number;
  prompt_tokens: number;
  completion_tokens: number;
  total_tokens: number;
  avg_novel_tokens: number;
  avg_chapter_tokens: number;
  estimated_cost_usd?: number;
  cost_model?: string;
  novels: TokenReportNovel[];
}

export interface ChapterOutline {
  num: number;
  title: string;
  summary: string;
}

export interface NovelOutline {
  title: string;
  synopsis: string;
  characters?: Record<string, unknown>[];
  chapters: ChapterOutline[];
}

export type NovelTemplate = 'novel-team-chapters' | 'novel-team-historical';

export interface CreateNovelPayload {
  title?: string;
  prompt?: string;
  template?: NovelTemplate;
  historical_era?: string;
  chapter_count?: number;
  words_per_chapter?: number;
  quality_threshold?: number;
  params?: Record<string, string>;
  namespace?: string;
  name?: string;
}

export interface ImportNovelPayload {
  title?: string;
  text: string;
  prompt?: string;
  continue_writing?: boolean;
  namespace?: string;
  name?: string;
}

export interface RAGChunk {
  id: string;
  chapter?: number;
  title?: string;
  text: string;
  source: string;
}

export interface RAGSearchResult {
  query: string;
  count: number;
  chunks: RAGChunk[];
}

export interface RegenerateChapterPayload {
  instruction: string;
  layer?: 'plot' | 'chapter';
}

export interface RegenerateChapterResult {
  parent_workflow: string;
  rewrite_workflow: string;
  namespace: string;
  chapter_num: number;
  layer: string;
  workspace_path?: string;
}

export interface WorkflowSummary {
  name: string;
  namespace: string;
  phase: string;
  currentStep?: string;
  progress: number;
  message?: string;
  template?: string;
  workspacePath?: string;
  created_at?: string;
  completion_at?: string;
}

export interface WorkflowProgress {
  completed?: number;
  total?: number;
  percent?: number;
}

export interface WorkflowStepStatus {
  id: string;
  phase?: string;
  retries?: number;
  taskName?: string;
  message?: string;
  score?: number;
}

export interface WorkflowStatus {
  phase?: string;
  message?: string;
  currentStep?: string;
  workspacePath?: string;
  completedSteps?: string[];
  failedSteps?: string[];
  stepStatuses?: WorkflowStepStatus[];
  progress?: WorkflowProgress;
}

export interface WorkflowDetail {
  metadata?: { name?: string; namespace?: string };
  spec?: { template?: string; params?: Record<string, string>; prompt?: string };
  status?: WorkflowStatus;
}

export interface PendingTask {
  id?: string;
  ID?: string;
  description?: string;
}

export interface PendingWorkflow {
  id?: string;
  ID?: string;
  description?: string;
  template?: string;
  proposed_name?: string;
  params?: Record<string, string>;
}

export interface TaskEvent {
  task_name?: string;
  namespace?: string;
  step?: string;
  message?: string;
  retry?: number;
  score?: number;
  timestamp?: string;
}

export interface ConversationMessage {
  role: string;
  content: string;
}

export interface ConversationData {
  id: string;
  messages: ConversationMessage[];
  updated_at?: string;
}

export interface ChatResponseData {
  assistant_reply: string;
  task?: PendingTask;
  workflow_suggested?: boolean;
  workflow?: PendingWorkflow;
}

export type PhaseFilter = 'all' | 'Pending' | 'Running' | 'Succeeded' | 'Failed';