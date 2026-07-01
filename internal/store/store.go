package store

import (
	"context"

	agentflowiov1alpha1 "agent-flow/api/v1alpha1"
	wfengine "agent-flow/internal/workflow"
)

const (
	ChapterStatusPending = "pending"
	ChapterStatusWriting = "writing"
	ChapterStatusDone    = "done"
	ChapterStatusFailed  = "failed"
)

// NovelRef identifies a workflow-backed novel.
type NovelRef struct {
	Namespace     string
	Name          string
	WorkspacePath string
	ChapterCount  int
}

// ChapterRecord is one chapter row.
type ChapterRecord struct {
	Num         int
	Title       string
	Summary     string
	Status      string
	WordCount   int
	QCScore     *int
	BodyPath    string
	SummaryPath string
	StepID      string
}

// Store persists novel domain metadata. Chapter bodies remain on PVC.
type Store interface {
	Enabled() bool
	Ping(ctx context.Context) error
	Close() error

	EnsureNovel(ctx context.Context, wf *agentflowiov1alpha1.Workflow) error
	HasNovel(ctx context.Context, namespace, name string) (bool, error)
	HasOutline(ctx context.Context, wf *agentflowiov1alpha1.Workflow) (bool, error)
	SyncOutline(ctx context.Context, wf *agentflowiov1alpha1.Workflow, outline *wfengine.NovelOutline) error
	MarkChapterWriting(ctx context.Context, wf *agentflowiov1alpha1.Workflow, num int, stepID string) error
	MarkChapterDone(ctx context.Context, wf *agentflowiov1alpha1.Workflow, num int, bodyPath, summaryPath string, wordCount int, qcScore *int) error
	MarkChapterFailed(ctx context.Context, wf *agentflowiov1alpha1.Workflow, num int, stepID string) error
	MissingChapterNumbers(ctx context.Context, wf *agentflowiov1alpha1.Workflow) ([]int, error)
}