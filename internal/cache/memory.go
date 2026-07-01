package cache

import (
	"context"
	"fmt"
	"sync"
)

// MemoryStore is an in-process fallback when Redis is unavailable.
type MemoryStore struct {
	mu            sync.RWMutex
	currentConvID string
	conversations map[string]*StoredConversation
	pendingTasks  []StoredTaskRequest
	checkpoints   map[string]*Checkpoint
	eventSubs     map[string][]chan TaskEvent
	evalHistory   map[string][]EvalRecord
}

// NewMemoryStore creates an in-memory state store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		conversations: make(map[string]*StoredConversation),
		checkpoints:   make(map[string]*Checkpoint),
		eventSubs:     make(map[string][]chan TaskEvent),
		evalHistory:   make(map[string][]EvalRecord),
	}
}

func (m *MemoryStore) Ping(_ context.Context) error { return nil }

func (m *MemoryStore) GetCurrentConvID(_ context.Context) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentConvID, nil
}

func (m *MemoryStore) SetCurrentConvID(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.currentConvID = id
	return nil
}

func (m *MemoryStore) SaveConversation(_ context.Context, conv *StoredConversation) error {
	if conv == nil {
		return fmt.Errorf("conversation is nil")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	copied := *conv
	copied.Messages = append([]StoredMessage(nil), conv.Messages...)
	m.conversations[conv.ID] = &copied
	return nil
}

func (m *MemoryStore) GetConversation(_ context.Context, id string) (*StoredConversation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	conv, ok := m.conversations[id]
	if !ok {
		return nil, nil
	}
	copied := *conv
	copied.Messages = append([]StoredMessage(nil), conv.Messages...)
	return &copied, nil
}

func (m *MemoryStore) ListPendingTasks(_ context.Context) ([]StoredTaskRequest, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]StoredTaskRequest, len(m.pendingTasks))
	copy(out, m.pendingTasks)
	return out, nil
}

func (m *MemoryStore) SetPendingTasks(_ context.Context, tasks []StoredTaskRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pendingTasks = append([]StoredTaskRequest(nil), tasks...)
	return nil
}

func (m *MemoryStore) SaveCheckpoint(_ context.Context, namespace, name string, cp *Checkpoint) error {
	if cp == nil {
		return fmt.Errorf("checkpoint is nil")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	copied := *cp
	m.checkpoints[checkpointKey(namespace, name)] = &copied
	return nil
}

func (m *MemoryStore) GetCheckpoint(_ context.Context, namespace, name string) (*Checkpoint, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cp, ok := m.checkpoints[checkpointKey(namespace, name)]
	if !ok {
		return nil, nil
	}
	copied := *cp
	return &copied, nil
}

func (m *MemoryStore) DeleteCheckpoint(_ context.Context, namespace, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.checkpoints, checkpointKey(namespace, name))
	return nil
}

func (m *MemoryStore) PublishEvent(_ context.Context, namespace, name string, event *TaskEvent) error {
	if event == nil {
		return fmt.Errorf("event is nil")
	}
	m.mu.RLock()
	subs := append([]chan TaskEvent(nil), m.eventSubs[eventsChannel(namespace, name)]...)
	m.mu.RUnlock()

	for _, ch := range subs {
		select {
		case ch <- *event:
		default:
		}
	}
	return nil
}

func (m *MemoryStore) SubscribeEvents(_ context.Context, namespace, name string) (<-chan TaskEvent, func(), error) {
	ch := make(chan TaskEvent, 16)
	channel := eventsChannel(namespace, name)

	m.mu.Lock()
	m.eventSubs[channel] = append(m.eventSubs[channel], ch)
	m.mu.Unlock()

	cancel := func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		subs := m.eventSubs[channel]
		for i, sub := range subs {
			if sub == ch {
				m.eventSubs[channel] = append(subs[:i], subs[i+1:]...)
				break
			}
		}
		close(ch)
	}
	return ch, cancel, nil
}

func (m *MemoryStore) AppendEvalHistory(_ context.Context, namespace, name string, record EvalRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := evalHistoryKey(namespace, name)
	m.evalHistory[key] = append(m.evalHistory[key], record)
	return nil
}

func (m *MemoryStore) ListEvalHistory(_ context.Context, namespace, name string) ([]EvalRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	key := evalHistoryKey(namespace, name)
	records := m.evalHistory[key]
	out := make([]EvalRecord, len(records))
	copy(out, records)
	return out, nil
}
