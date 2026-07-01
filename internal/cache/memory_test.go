package cache

import (
	"context"
	"testing"
	"time"
)

func TestMemoryStoreConversationAndPendingTasks(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()

	conv := &StoredConversation{
		ID:        "default",
		Name:      "默认对话",
		Rules:     "test",
		Messages:  []StoredMessage{{Role: "user", Content: "hello", Timestamp: formatTime(time.Now())}},
		CreatedAt: formatTime(time.Now()),
		UpdatedAt: formatTime(time.Now()),
	}
	if err := store.SaveConversation(ctx, conv); err != nil {
		t.Fatalf("save conversation: %v", err)
	}
	if err := store.SetCurrentConvID(ctx, "default"); err != nil {
		t.Fatalf("set current conv: %v", err)
	}

	gotID, err := store.GetCurrentConvID(ctx)
	if err != nil || gotID != "default" {
		t.Fatalf("current conv id = %q, err = %v", gotID, err)
	}

	gotConv, err := store.GetConversation(ctx, "default")
	if err != nil || gotConv == nil || len(gotConv.Messages) != 1 {
		t.Fatalf("get conversation: %+v, err = %v", gotConv, err)
	}

	tasks := []StoredTaskRequest{{ID: "task-1", Description: "写诗", CPU: "500m", Memory: "256Mi", Duration: 60}}
	if err := store.SetPendingTasks(ctx, tasks); err != nil {
		t.Fatalf("set pending tasks: %v", err)
	}
	gotTasks, err := store.ListPendingTasks(ctx)
	if err != nil || len(gotTasks) != 1 || gotTasks[0].ID != "task-1" {
		t.Fatalf("list pending tasks: %+v, err = %v", gotTasks, err)
	}
}

func TestMemoryStoreCheckpointAndEvents(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()

	cp := &Checkpoint{Retry: 1, Phase: CheckpointPhaseExecuting, WorkerInstruction: "写诗"}
	if err := store.SaveCheckpoint(ctx, "default", "task-a", cp); err != nil {
		t.Fatalf("save checkpoint: %v", err)
	}
	got, err := store.GetCheckpoint(ctx, "default", "task-a")
	if err != nil || got == nil || got.Retry != 1 {
		t.Fatalf("get checkpoint: %+v, err = %v", got, err)
	}

	events, cancel, err := store.SubscribeEvents(ctx, "default", "task-a")
	if err != nil {
		t.Fatalf("subscribe events: %v", err)
	}
	defer cancel()

	event := &TaskEvent{TaskName: "task-a", Namespace: "default", Step: EventStepWorker, Message: "running"}
	if err := store.PublishEvent(ctx, "default", "task-a", event); err != nil {
		t.Fatalf("publish event: %v", err)
	}

	select {
	case gotEvent := <-events:
		if gotEvent.Step != EventStepWorker {
			t.Fatalf("unexpected event: %+v", gotEvent)
		}
	default:
		t.Fatal("expected event on subscriber channel")
	}

	if err := store.DeleteCheckpoint(ctx, "default", "task-a"); err != nil {
		t.Fatalf("delete checkpoint: %v", err)
	}
	got, err = store.GetCheckpoint(ctx, "default", "task-a")
	if err != nil || got != nil {
		t.Fatalf("checkpoint should be deleted, got=%+v err=%v", got, err)
	}
}
