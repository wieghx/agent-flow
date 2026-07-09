package architecture

import (
	"context"
	"fmt"

	agentflowiov1alpha1 "agent-flow/api/v1alpha1"
	"agent-flow/internal/store"
	wfengine "agent-flow/internal/workflow"
	ctrl "sigs.k8s.io/controller-runtime/pkg/log"
)

func (c *WorkflowController) missingChapters(ctx context.Context, wf *agentflowiov1alpha1.Workflow) []int {
	if c.Store != nil && c.Store.Enabled() {
		hasOutline, err := c.Store.HasOutline(ctx, wf)
		if err == nil && hasOutline {
			if missing, err := c.Store.MissingChapterNumbers(ctx, wf); err == nil {
				return missing
			}
		}
	}
	return wfengine.MissingChapterNumbers(wf)
}

func (c *WorkflowController) ensureNovelInStore(ctx context.Context, wf *agentflowiov1alpha1.Workflow) {
	if c.Store == nil || !c.Store.Enabled() {
		return
	}
	if err := c.Store.EnsureNovel(ctx, wf); err != nil {
		ctrl.FromContext(ctx).Error(err, "ensure novel in store", "workflow", wf.Name)
	}
}

func (c *WorkflowController) syncOutlineToStore(ctx context.Context, wf *agentflowiov1alpha1.Workflow, raw string) {
	if c.Store == nil || !c.Store.Enabled() || raw == "" {
		return
	}
	outline, err := wfengine.ParseOutlineJSON(raw)
	if err != nil {
		ctrl.FromContext(ctx).Error(err, "parse outline for store", "workflow", wf.Name)
		return
	}
	if err := c.Store.SyncOutline(ctx, wf, outline); err != nil {
		ctrl.FromContext(ctx).Error(err, "sync outline to store", "workflow", wf.Name)
	}
}

func (c *WorkflowController) markChapterWriting(ctx context.Context, wf *agentflowiov1alpha1.Workflow, stepID string) {
	if c.Store == nil || !c.Store.Enabled() {
		return
	}
	num, ok := wfengine.ChapterNumFromStepID(stepID)
	if !ok {
		return
	}
	if err := c.Store.MarkChapterWriting(ctx, wf, num, stepID); err != nil {
		ctrl.FromContext(ctx).Error(err, "mark chapter writing", "workflow", wf.Name, "chapter", num)
	}
}

func (c *WorkflowController) markChapterDone(ctx context.Context, wf *agentflowiov1alpha1.Workflow, stepID, bodyPath, summaryPath, content string, qc *agentflowiov1alpha1.QualityCheck) {
	if c.Store == nil || !c.Store.Enabled() {
		return
	}
	num, ok := wfengine.ChapterNumFromStepID(stepID)
	if !ok {
		return
	}
	var qcScore *int
	if qc != nil {
		s := int(qc.Score)
		qcScore = &s
	}
	if err := c.Store.MarkChapterDone(ctx, wf, num, bodyPath, summaryPath, store.CountRunes(content), qcScore); err != nil {
		ctrl.FromContext(ctx).Error(err, "mark chapter done", "workflow", wf.Name, "chapter", num)
	}
}

func (c *WorkflowController) backfillStoreFromWorkspace(ctx context.Context, wf *agentflowiov1alpha1.Workflow) {
	if c.Store == nil || !c.Store.Enabled() {
		return
	}
	c.ensureNovelInStore(ctx, wf)
	hasOutline, err := c.Store.HasOutline(ctx, wf)
	if err != nil || hasOutline {
		return
	}
	if !wfengine.OutlineReady(wf) {
		return
	}
	if err := store.SyncOutlineFromWorkflow(ctx, c.Store, wf); err != nil {
		ctrl.FromContext(ctx).Error(err, "backfill outline to store", "workflow", wf.Name)
		return
	}
	width := wfengine.ChapterPaddingWidth(wfengine.OutlineChapterCount(wf))
	for _, stepID := range wf.Status.CompletedSteps {
		num, ok := wfengine.ChapterNumFromStepID(stepID)
		if !ok {
			continue
		}
		bodyPath := fmt.Sprintf("chapters/%s", wfengine.ChapterFileName(num, width))
		summaryPath := fmt.Sprintf("chapters/%s", wfengine.ChapterSummaryFileName(num, width))
		content, _ := wfengine.ReadArtifact(wf, bodyPath)
		c.markChapterDone(ctx, wf, stepID, bodyPath, summaryPath, content, nil)
	}
	for _, stepID := range wf.Status.FailedSteps {
		c.markChapterFailed(ctx, wf, stepID)
	}
}

func (c *WorkflowController) markChapterFailed(ctx context.Context, wf *agentflowiov1alpha1.Workflow, stepID string) {
	if c.Store == nil || !c.Store.Enabled() {
		return
	}
	num, ok := wfengine.ChapterNumFromStepID(stepID)
	if !ok {
		return
	}
	if err := c.Store.MarkChapterFailed(ctx, wf, num, stepID); err != nil {
		ctrl.FromContext(ctx).Error(err, "mark chapter failed", "workflow", wf.Name, "chapter", num)
	}
}
