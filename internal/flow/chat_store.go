package flow

import (
	"context"
	"fmt"
	"time"

	"agent-flow/internal/cache"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (r *ChatRouter) persistConversation(ctx context.Context) error {
	if r.store == nil || r.currentConv == nil {
		return nil
	}
	if err := r.store.SaveConversation(ctx, toStoredConversation(r.currentConv)); err != nil {
		return err
	}
	return r.store.SetCurrentConvID(ctx, r.currentConv.ID)
}

func (r *ChatRouter) persistPendingTasks(ctx context.Context) error {
	if r.store == nil {
		return nil
	}
	tasks := make([]cache.StoredTaskRequest, 0, len(r.pendingTasks))
	for _, task := range r.pendingTasks {
		tasks = append(tasks, toStoredTaskRequest(task))
	}
	return r.store.SetPendingTasks(ctx, tasks)
}

func (r *ChatRouter) loadStateFromStore(ctx context.Context) {
	if r.store == nil {
		return
	}

	if convID, err := r.store.GetCurrentConvID(ctx); err == nil && convID != "" {
		if stored, err := r.store.GetConversation(ctx, convID); err == nil && stored != nil {
			r.currentConv = fromStoredConversation(stored)
			r.conversations[convID] = r.currentConv
		}
	}

	if storedTasks, err := r.store.ListPendingTasks(ctx); err == nil {
		r.pendingTasks = make([]*TaskRequest, 0, len(storedTasks))
		for i := range storedTasks {
			task := fromStoredTaskRequest(&storedTasks[i])
			r.pendingTasks = append(r.pendingTasks, task)
		}
	}
}

func (r *ChatRouter) ensureDefaultConversation() {
	if r.currentConv != nil {
		return
	}
	r.currentConv = &Conversation{
		ID:        "default",
		Name:      "默认对话",
		Rules:     r.systemRole,
		Messages:  []Message{},
		CreatedAt: metav1.Now(),
		UpdatedAt: metav1.Now(),
	}
	r.conversations["default"] = r.currentConv
}

func toStoredConversation(conv *Conversation) *cache.StoredConversation {
	if conv == nil {
		return nil
	}
	stored := &cache.StoredConversation{
		ID:        conv.ID,
		Name:      conv.Name,
		Rules:     conv.Rules,
		CreatedAt: formatMetaTime(conv.CreatedAt),
		UpdatedAt: formatMetaTime(conv.UpdatedAt),
		Messages:  make([]cache.StoredMessage, 0, len(conv.Messages)),
	}
	for _, msg := range conv.Messages {
		stored.Messages = append(stored.Messages, cache.StoredMessage{
			Role:      msg.Role,
			Content:   msg.Content,
			Timestamp: formatMetaTime(msg.Timestamp),
		})
	}
	return stored
}

func fromStoredConversation(stored *cache.StoredConversation) *Conversation {
	if stored == nil {
		return nil
	}
	conv := &Conversation{
		ID:        stored.ID,
		Name:      stored.Name,
		Rules:     stored.Rules,
		CreatedAt: parseMetaTime(stored.CreatedAt),
		UpdatedAt: parseMetaTime(stored.UpdatedAt),
		Messages:  make([]Message, 0, len(stored.Messages)),
	}
	for _, msg := range stored.Messages {
		conv.Messages = append(conv.Messages, Message{
			Role:      msg.Role,
			Content:   msg.Content,
			Timestamp: parseMetaTime(msg.Timestamp),
		})
	}
	return conv
}

func toStoredTaskRequest(task *TaskRequest) cache.StoredTaskRequest {
	stored := cache.StoredTaskRequest{
		ID:                task.ID,
		Description:       task.Description,
		Rationale:         task.Rationale,
		Priority:          task.Priority,
		CPU:               task.Resources.CPU,
		Memory:            task.Resources.Memory,
		Duration:          task.Resources.Duration,
		NeedsQualityCheck: task.NeedsQualityCheck,
		CreatedAt:         formatMetaTime(task.CreatedAt),
		Approved:          task.Approved,
		ApprovedBy:        task.ApprovedBy,
		RejectionReason:   task.RejectionReason,
	}
	if task.ApprovedAt != nil {
		stored.ApprovedAt = formatMetaTime(*task.ApprovedAt)
	}
	return stored
}

func fromStoredTaskRequest(stored *cache.StoredTaskRequest) *TaskRequest {
	task := &TaskRequest{
		ID:          stored.ID,
		Description: stored.Description,
		Rationale:   stored.Rationale,
		Priority:    stored.Priority,
		Resources: ResourceEstimate{
			CPU:      stored.CPU,
			Memory:   stored.Memory,
			Duration: stored.Duration,
		},
		NeedsQualityCheck: stored.NeedsQualityCheck,
		CreatedAt:         parseMetaTime(stored.CreatedAt),
		Approved:          stored.Approved,
		ApprovedBy:        stored.ApprovedBy,
		RejectionReason:   stored.RejectionReason,
	}
	if stored.ApprovedAt != "" {
		t := parseMetaTime(stored.ApprovedAt)
		task.ApprovedAt = &t
	}
	return task
}

func formatMetaTime(t metav1.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func parseMetaTime(s string) metav1.Time {
	if s == "" {
		return metav1.Now()
	}
	parsed, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return metav1.Now()
	}
	return metav1.NewTime(parsed)
}

func nextTaskID(pending []*TaskRequest) string {
	max := 0
	for _, task := range pending {
		var n int
		if _, err := fmt.Sscanf(task.ID, "task-%d", &n); err == nil && n > max {
			max = n
		}
	}
	return fmt.Sprintf("task-%d", max+1)
}
