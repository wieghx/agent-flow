package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisStore persists state in Redis.
type RedisStore struct {
	client *redis.Client
}

// NewRedisStore connects to Redis using a URL such as redis://localhost:6379/0.
func NewRedisStore(redisURL string) (*RedisStore, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}
	client := redis.NewClient(opts)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("redis ping: %w", err)
	}
	return &RedisStore{client: client}, nil
}

func (r *RedisStore) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

func (r *RedisStore) GetCurrentConvID(ctx context.Context) (string, error) {
	id, err := r.client.Get(ctx, keyCurrentConv).Result()
	if err == redis.Nil {
		return "", nil
	}
	return id, err
}

func (r *RedisStore) SetCurrentConvID(ctx context.Context, id string) error {
	return r.client.Set(ctx, keyCurrentConv, id, 0).Err()
}

func (r *RedisStore) SaveConversation(ctx context.Context, conv *StoredConversation) error {
	if conv == nil {
		return fmt.Errorf("conversation is nil")
	}
	data, err := json.Marshal(conv)
	if err != nil {
		return err
	}
	return r.client.Set(ctx, keyConv+conv.ID, data, conversationTTL).Err()
}

func (r *RedisStore) GetConversation(ctx context.Context, id string) (*StoredConversation, error) {
	data, err := r.client.Get(ctx, keyConv+id).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var conv StoredConversation
	if err := json.Unmarshal([]byte(data), &conv); err != nil {
		return nil, err
	}
	return &conv, nil
}

func (r *RedisStore) ListPendingTasks(ctx context.Context) ([]StoredTaskRequest, error) {
	data, err := r.client.Get(ctx, keyPendingTasks).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var tasks []StoredTaskRequest
	if err := json.Unmarshal([]byte(data), &tasks); err != nil {
		return nil, err
	}
	return tasks, nil
}

func (r *RedisStore) SetPendingTasks(ctx context.Context, tasks []StoredTaskRequest) error {
	if len(tasks) == 0 {
		return r.client.Del(ctx, keyPendingTasks).Err()
	}
	data, err := json.Marshal(tasks)
	if err != nil {
		return err
	}
	return r.client.Set(ctx, keyPendingTasks, data, pendingTasksTTL).Err()
}

func (r *RedisStore) SaveCheckpoint(ctx context.Context, namespace, name string, cp *Checkpoint) error {
	if cp == nil {
		return fmt.Errorf("checkpoint is nil")
	}
	data, err := json.Marshal(cp)
	if err != nil {
		return err
	}
	return r.client.Set(ctx, checkpointKey(namespace, name), data, checkpointTTL).Err()
}

func (r *RedisStore) GetCheckpoint(ctx context.Context, namespace, name string) (*Checkpoint, error) {
	data, err := r.client.Get(ctx, checkpointKey(namespace, name)).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var cp Checkpoint
	if err := json.Unmarshal([]byte(data), &cp); err != nil {
		return nil, err
	}
	return &cp, nil
}

func (r *RedisStore) DeleteCheckpoint(ctx context.Context, namespace, name string) error {
	return r.client.Del(ctx, checkpointKey(namespace, name)).Err()
}

func (r *RedisStore) PublishEvent(ctx context.Context, namespace, name string, event *TaskEvent) error {
	if event == nil {
		return fmt.Errorf("event is nil")
	}
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return r.client.Publish(ctx, eventsChannel(namespace, name), data).Err()
}

func (r *RedisStore) SubscribeEvents(ctx context.Context, namespace, name string) (<-chan TaskEvent, func(), error) {
	pubsub := r.client.Subscribe(ctx, eventsChannel(namespace, name))
	ch := make(chan TaskEvent, 16)

	go func() {
		defer close(ch)
		for msg := range pubsub.Channel() {
			var event TaskEvent
			if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
				continue
			}
			select {
			case ch <- event:
			case <-ctx.Done():
				return
			}
		}
	}()

	cancel := func() {
		_ = pubsub.Close()
	}
	return ch, cancel, nil
}

func (r *RedisStore) AppendEvalHistory(ctx context.Context, namespace, name string, record EvalRecord) error {
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	key := evalHistoryKey(namespace, name)
	pipe := r.client.Pipeline()
	pipe.RPush(ctx, key, data)
	pipe.Expire(ctx, key, evalHistoryTTL)
	_, err = pipe.Exec(ctx)
	return err
}

func (r *RedisStore) ListEvalHistory(ctx context.Context, namespace, name string) ([]EvalRecord, error) {
	items, err := r.client.LRange(ctx, evalHistoryKey(namespace, name), 0, -1).Result()
	if err != nil {
		return nil, err
	}
	records := make([]EvalRecord, 0, len(items))
	for _, item := range items {
		var record EvalRecord
		if err := json.Unmarshal([]byte(item), &record); err != nil {
			continue
		}
		records = append(records, record)
	}
	return records, nil
}
