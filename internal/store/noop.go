package store

import (
	"context"
	"errors"

	agentflowiov1alpha1 "agent-flow/api/v1alpha1"
	wfengine "agent-flow/internal/workflow"
)

// NoopStore disables DB persistence; workflow falls back to PVC files.
type NoopStore struct{}

func Noop() Store { return NoopStore{} }

func (NoopStore) Enabled() bool { return false }

func (NoopStore) Ping(context.Context) error { return nil }

func (NoopStore) Close() error { return nil }

func (NoopStore) EnsureNovel(context.Context, *agentflowiov1alpha1.Workflow) error { return nil }

func (NoopStore) HasNovel(context.Context, string, string) (bool, error) { return false, nil }

func (NoopStore) HasOutline(context.Context, *agentflowiov1alpha1.Workflow) (bool, error) {
	return false, nil
}

func (NoopStore) SyncOutline(context.Context, *agentflowiov1alpha1.Workflow, *wfengine.NovelOutline) error {
	return nil
}

func (NoopStore) MarkChapterWriting(context.Context, *agentflowiov1alpha1.Workflow, int, string) error {
	return nil
}

func (NoopStore) MarkChapterDone(context.Context, *agentflowiov1alpha1.Workflow, int, string, string, int, *int) error {
	return nil
}

func (NoopStore) MarkChapterFailed(context.Context, *agentflowiov1alpha1.Workflow, int, string) error {
	return nil
}

func (NoopStore) MissingChapterNumbers(context.Context, *agentflowiov1alpha1.Workflow) ([]int, error) {
	return nil, errors.New("store disabled")
}