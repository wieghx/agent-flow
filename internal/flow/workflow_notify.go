package flow

import (
	"fmt"

	agentflowiov1alpha1 "agent-flow/api/v1alpha1"
)

// WorkflowNotifier receives workflow lifecycle events for chat feedback.
type WorkflowNotifier interface {
	NotifyWorkflowEvent(event WorkflowEvent)
}

// WorkflowEvent is a chat-facing workflow progress update.
type WorkflowEvent struct {
	Namespace string
	Name      string
	Phase     string
	Percent   int32
	Message   string
}

// NotifyWorkflowEvent pushes a formatted system message into the active conversation.
func (r *ChatRouter) NotifyWorkflowEvent(event WorkflowEvent) {
	if r == nil {
		return
	}
	var content string
	switch event.Phase {
	case string(agentflowiov1alpha1.WorkflowPhaseSucceeded):
		content = fmt.Sprintf("✅ 工作流「%s」已完成！\n\n%s", event.Name, event.Message)
	case string(agentflowiov1alpha1.WorkflowPhaseFailed):
		content = fmt.Sprintf("❌ 工作流「%s」失败\n\n%s", event.Name, event.Message)
	default:
		if event.Percent > 0 {
			content = fmt.Sprintf("📚 工作流「%s」进度 %d%%\n%s", event.Name, event.Percent, event.Message)
		} else {
			content = fmt.Sprintf("📚 工作流「%s」: %s", event.Name, event.Message)
		}
	}
	r.AddSystemMessage(content)
}