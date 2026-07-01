package cache

import "time"

// StoredMessage is a JSON-serializable chat message.
type StoredMessage struct {
	Role      string `json:"role"`
	Content   string `json:"content"`
	Timestamp string `json:"timestamp"`
}

// StoredConversation is a JSON-serializable conversation.
type StoredConversation struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Rules     string          `json:"rules"`
	Messages  []StoredMessage `json:"messages"`
	CreatedAt string          `json:"created_at"`
	UpdatedAt string          `json:"updated_at"`
}

// StoredTaskRequest is a JSON-serializable pending task.
type StoredTaskRequest struct {
	ID                string `json:"id"`
	Description       string `json:"description"`
	Rationale         string `json:"rationale"`
	Priority          int    `json:"priority"`
	CPU               string `json:"cpu"`
	Memory            string `json:"memory"`
	Duration          int    `json:"duration"`
	NeedsQualityCheck bool   `json:"needs_quality_check"`
	CreatedAt         string `json:"created_at"`
	Approved          bool   `json:"approved"`
	ApprovedAt        string `json:"approved_at,omitempty"`
	ApprovedBy        string `json:"approved_by,omitempty"`
	RejectionReason   string `json:"rejection_reason,omitempty"`
}

// Checkpoint captures Worker-Monitor loop progress for crash recovery.
type Checkpoint struct {
	Retry             int    `json:"retry"`
	WorkerOutput      string `json:"worker_output,omitempty"`
	MonitorFeedback   string `json:"monitor_feedback,omitempty"`
	WorkerInstruction string `json:"worker_instruction"`
	QualityThreshold  int    `json:"quality_threshold"`
	Phase             string `json:"phase"`
	UpdatedAt         string `json:"updated_at"`
}

// TaskEvent is a real-time progress notification.
type TaskEvent struct {
	TaskName  string `json:"task_name"`
	Namespace string `json:"namespace"`
	Step      string `json:"step"`
	Message   string `json:"message"`
	Retry     int    `json:"retry,omitempty"`
	Score     int    `json:"score,omitempty"`
	Timestamp string `json:"timestamp"`
}

const (
	CheckpointPhaseExecuting = "executing"
	CheckpointPhaseCompleted = "completed"
)

const (
	EventStepSandbox   = "sandbox"
	EventStepWorker    = "worker"
	EventStepMonitor   = "monitor"
	EventStepSucceeded = "succeeded"
	EventStepFailed    = "failed"
)

// EvalRecord stores one monitor evaluation attempt.
type EvalRecord struct {
	Attempt     int      `json:"attempt"`
	Score       int      `json:"score"`
	Passed      bool     `json:"passed"`
	Feedback    string   `json:"feedback"`
	Issues      []string `json:"issues,omitempty"`
	TaskType    string   `json:"task_type,omitempty"`
	CheckMethod string   `json:"check_method,omitempty"`
	Timestamp   string   `json:"timestamp"`
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}
