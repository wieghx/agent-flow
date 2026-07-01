package flow

import (
	"context"
	"testing"

	"agent-flow/internal/cache"
)

func TestChatRouterPersistsConversationAndPendingTasks(t *testing.T) {
	store := cache.NewMemoryStore()
	router1 := NewChatRouter(nil, nil, nil, store)
	router1.ensureDefaultConversation()
	router1.currentConv.Messages = append(router1.currentConv.Messages, Message{Role: "user", Content: "写一首诗"})
	if err := router1.persistConversation(context.Background()); err != nil {
		t.Fatalf("persist conversation: %v", err)
	}

	task := &TaskRequest{
		ID:          "task-1",
		Description: "写一首七言绝句",
		Resources:   ResourceEstimate{CPU: "500m", Memory: "256Mi", Duration: 60},
	}
	router1.addPendingTask(task)
	if err := router1.persistPendingTasks(context.Background()); err != nil {
		t.Fatalf("persist pending tasks: %v", err)
	}

	router2 := NewChatRouter(nil, nil, nil, store)
	conv := router2.GetCurrentConversation()
	if conv == nil || len(conv.Messages) != 1 {
		t.Fatalf("expected restored conversation, got %+v", conv)
	}
	pending := router2.ListPendingTasks()
	if len(pending) != 1 || pending[0].ID != "task-1" {
		t.Fatalf("expected restored pending task, got %+v", pending)
	}
}
