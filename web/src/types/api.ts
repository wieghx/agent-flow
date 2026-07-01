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
  prompt?: string;
  template?: string;
  params?: Record<string, string>;
  workspace_path?: string;
  book_url?: string;
  outline_url?: string;
  created_at?: string;
  updated_at?: string;
  completion_at?: string;
}

export interface CreateNovelPayload {
  title?: string;
  prompt?: string;
  chapter_count?: number;
  words_per_chapter?: number;
  quality_threshold?: number;
  namespace?: string;
  name?: string;
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

export interface WorkflowStatus {
  phase?: string;
  message?: string;
  currentStep?: string;
  workspacePath?: string;
  completedSteps?: string[];
  failedSteps?: string[];
  progress?: WorkflowProgress;
}

export interface WorkflowDetail {
  metadata?: { name?: string; namespace?: string };
  spec?: { template?: string; params?: Record<string, string> };
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