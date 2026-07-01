import type {
  ApiResponse,
  ChatResponseData,
  ConversationData,
  CreateNovelPayload,
  NovelSummary,
  PendingTask,
  PendingWorkflow,
  TaskSummary,
  WorkflowDetail,
  WorkflowSummary,
} from '@/types/api';

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, init);
  const data = (await res.json()) as ApiResponse<T>;
  if (!data.success) {
    throw new Error(data.error || data.message || `Request failed: ${path}`);
  }
  return data.data as T;
}

export async function fetchTasks(): Promise<TaskSummary[]> {
  const data = await request<{ tasks: TaskSummary[] }>('/tasks');
  return data.tasks || [];
}

export async function fetchWorkflows(): Promise<WorkflowSummary[]> {
  const data = await request<{ workflows: WorkflowSummary[] }>('/workflows');
  return data.workflows || [];
}

export async function fetchNovels(): Promise<NovelSummary[]> {
  const data = await request<{ novels: NovelSummary[] }>('/novels');
  return data.novels || [];
}

export async function fetchNovelDetail(namespace: string, name: string): Promise<NovelSummary> {
  return request<NovelSummary>(`/novels/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}`);
}

export async function createNovel(payload: CreateNovelPayload): Promise<NovelSummary> {
  return request<NovelSummary>('/novels/create', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
}

export async function resumeNovel(namespace: string, name: string): Promise<void> {
  await request(`/novels/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}/resume`, {
    method: 'POST',
  });
}

export async function deleteNovel(namespace: string, name: string): Promise<void> {
  await request(`/novels/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}`, {
    method: 'DELETE',
  });
}

export async function fetchWorkflowDetail(name: string, namespace = 'default'): Promise<WorkflowDetail> {
  return request<WorkflowDetail>(`/workflows/${encodeURIComponent(name)}?namespace=${encodeURIComponent(namespace)}`);
}

export async function fetchPendingTasks(): Promise<PendingTask[]> {
  const data = await request<{ tasks: PendingTask[] }>('/tasks/pending');
  return data.tasks || [];
}

export async function fetchPendingWorkflows(): Promise<PendingWorkflow[]> {
  const data = await request<{ workflows: PendingWorkflow[] }>('/workflows/pending');
  return data.workflows || [];
}

export async function fetchConversation(): Promise<ConversationData | null> {
  try {
    return await request<ConversationData>('/conversation');
  } catch {
    return null;
  }
}

export async function sendChat(message: string): Promise<ChatResponseData> {
  return request<ChatResponseData>('/chat', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ message, role: 'user' }),
  });
}

export async function approveTask(taskId: string): Promise<void> {
  await request('/tasks/approve', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ task_id: taskId, approver: 'user' }),
  });
}

export async function rejectTask(taskId: string, reason: string): Promise<void> {
  await request('/tasks/reject', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ task_id: taskId, reason }),
  });
}

export async function approveWorkflow(workflowId: string): Promise<void> {
  await request('/workflows/approve', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ workflow_id: workflowId, approver: 'user' }),
  });
}

export async function rejectWorkflow(workflowId: string, reason: string): Promise<void> {
  await request('/workflows/reject', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ workflow_id: workflowId, reason }),
  });
}

export async function deleteTask(name: string): Promise<void> {
  const res = await fetch(`/tasks/${encodeURIComponent(name)}`, { method: 'DELETE' });
  const data = (await res.json()) as ApiResponse;
  if (!data.success) throw new Error(data.error || 'Delete failed');
}

export async function fetchTextAsset(url: string): Promise<string> {
  const res = await fetch(url);
  if (!res.ok) {
    const maybeJson = await res.json().catch(() => null);
    if (maybeJson && typeof maybeJson === 'object' && 'error' in maybeJson) {
      throw new Error(String((maybeJson as { error: string }).error));
    }
    throw new Error(`Failed to load ${url}`);
  }
  return res.text();
}