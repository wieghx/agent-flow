package cache

import (
	"context"
	"fmt"
	"os"
	"time"

	applog "agent-flow/internal/log"
)

const (
	keyPrefix       = "agentflow:"
	keyCurrentConv  = keyPrefix + "conv:current"
	keyConv         = keyPrefix + "conv:"
	keyPendingTasks = keyPrefix + "pending:tasks"
	keyCheckpoint   = keyPrefix + "task:%s/%s:checkpoint"
	keyTaskEvents   = keyPrefix + "task:%s/%s:events"
	keyEvalHistory  = keyPrefix + "task:%s/%s:evals"
	evalHistoryTTL  = 7 * 24 * time.Hour
	checkpointTTL   = 24 * time.Hour
	conversationTTL = 7 * 24 * time.Hour
	pendingTasksTTL = 7 * 24 * time.Hour
)

// StateStore persists hot intermediate state shared across controller restarts.
type StateStore interface {
	Ping(ctx context.Context) error

	GetCurrentConvID(ctx context.Context) (string, error)
	SetCurrentConvID(ctx context.Context, id string) error

	SaveConversation(ctx context.Context, conv *StoredConversation) error
	GetConversation(ctx context.Context, id string) (*StoredConversation, error)

	ListPendingTasks(ctx context.Context) ([]StoredTaskRequest, error)
	SetPendingTasks(ctx context.Context, tasks []StoredTaskRequest) error

	SaveCheckpoint(ctx context.Context, namespace, name string, cp *Checkpoint) error
	GetCheckpoint(ctx context.Context, namespace, name string) (*Checkpoint, error)
	DeleteCheckpoint(ctx context.Context, namespace, name string) error

	PublishEvent(ctx context.Context, namespace, name string, event *TaskEvent) error
	SubscribeEvents(ctx context.Context, namespace, name string) (<-chan TaskEvent, func(), error)

	AppendEvalHistory(ctx context.Context, namespace, name string, record EvalRecord) error
	ListEvalHistory(ctx context.Context, namespace, name string) ([]EvalRecord, error)
}

// NewFromEnv returns Redis when REDIS_URL is set, otherwise an in-memory store.
func NewFromEnv() StateStore {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		return NewMemoryStore()
	}
	log := applog.Component("cache")
	store, err := NewRedisStore(redisURL)
	if err != nil {
		log.Warn("redis unavailable, using in-memory store", "error", err)
		return NewMemoryStore()
	}
	log.Info("using redis store", "url", redisURL)
	return store
}

func checkpointKey(namespace, name string) string {
	return fmt.Sprintf(keyCheckpoint, namespace, name)
}

func eventsChannel(namespace, name string) string {
	return fmt.Sprintf(keyTaskEvents, namespace, name)
}

func evalHistoryKey(namespace, name string) string {
	return fmt.Sprintf(keyEvalHistory, namespace, name)
}
