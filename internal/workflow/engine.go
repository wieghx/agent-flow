package workflow

import (
	"fmt"
	"strings"

	agentflowiov1alpha1 "agent-flow/api/v1alpha1"
)

// ResolvedStep is a concrete executable step.
type ResolvedStep struct {
	ID         string
	Name       string
	Type       agentflowiov1alpha1.WorkflowStepType
	TaskSpec   agentflowiov1alpha1.WorkflowTaskSpec
	OutputPath string
	ChapterNum int
}

// ResolveSteps expands foreach steps using workflow status and outline.
func ResolveSteps(wf *agentflowiov1alpha1.Workflow) ([]ResolvedStep, error) {
	spec := wf.Spec
	if len(spec.Steps) == 0 && spec.Template != "" {
		built, err := BuildSpecFromTemplate(spec.Template, spec.Prompt, spec.Params)
		if err != nil {
			return nil, err
		}
		spec = built
	}

	var resolved []ResolvedStep
	for _, step := range spec.Steps {
		switch step.Type {
		case agentflowiov1alpha1.WorkflowStepTypeForeach:
			items, err := expandForeach(wf, step)
			if err != nil {
				// Defer foreach expansion until prerequisite artifacts (e.g. outline.json) exist.
				continue
			}
			resolved = append(resolved, items...)
		default:
			rs := ResolvedStep{
				ID:         step.ID,
				Name:       step.Name,
				Type:       step.Type,
				TaskSpec:   step.TaskSpec,
				OutputPath: step.Output.Path,
			}
			if instr := EnhanceStepInstruction(wf, rs); instr != "" {
				rs.TaskSpec.WorkerInstruction = instr
			}
			resolved = append(resolved, rs)
		}
	}
	return resolved, nil
}

func expandForeach(wf *agentflowiov1alpha1.Workflow, step agentflowiov1alpha1.WorkflowStep) ([]ResolvedStep, error) {
	if step.Foreach == nil {
		return nil, fmt.Errorf("foreach step %s missing config", step.ID)
	}
	raw, err := ReadArtifact(wf, step.Foreach.Source)
	if err != nil {
		return nil, fmt.Errorf("read foreach source %s: %w", step.Foreach.Source, err)
	}
	outline, err := ParseOutlineJSON(raw)
	if err != nil {
		return nil, err
	}
	if err := ValidateOutlineChapterCount(outline, IntParam(wf.Spec.Params, "chapterCount", 0)); err != nil {
		return nil, err
	}

	words := IntParam(wf.Spec.Params, "wordsPerChapter", 3000)
	contextWindow := IntParam(wf.Spec.Params, "contextChapters", 5)
	width := ChapterPaddingWidth(len(outline.Chapters))
	prefix := step.Foreach.StepIDPrefix
	if prefix == "" {
		prefix = "chapter"
	}

	arcInterval := DefaultArcInterval(wf.Spec.Params, len(outline.Chapters))
	maxChapter := outline.Chapters[len(outline.Chapters)-1].Num

	isPlot := prefix == "plot"
	var items []ResolvedStep
	for _, ch := range outline.Chapters {
		ctxBundle := BuildChapterContext(wf, ch.Num, width, contextWindow)
		bible, _ := LoadStyleBible(wf)
		var instruction string
		if isPlot {
			instruction = BuildPlotInstruction(wf, step.Foreach.Instruction, outline, ch, ctxBundle, wf.Spec.Params, bible)
		} else {
			instruction = BuildChapterInstruction(wf, step.Foreach.Instruction, outline, ch, ctxBundle, words, bible, width)
		}
		if block := ResearchContextBlock(wf); block != "" {
			instruction += block
		}
		if !isPlot && SegmentModeEnabled(wf.Spec.Params, words) {
			instruction = AppendSegmentDirectives(instruction, SegmentCount(wf.Spec.Params, words), SegmentWordsPerPart(wf.Spec.Params, words))
		}
		outputPath := strings.ReplaceAll(step.Foreach.OutputPath, "{{num}}", fmt.Sprintf("%0*d", width, ch.Num))
		taskSpec := step.TaskSpec
		taskSpec.WorkerInstruction = instruction
		name := fmt.Sprintf("第%d章 %s", ch.Num, ch.Title)
		if isPlot {
			name = fmt.Sprintf("第%d章剧情 %s", ch.Num, ch.Title)
		}
		items = append(items, ResolvedStep{
			ID:         ChapterStepID(prefix, ch.Num, width),
			Name:       name,
			Type:       agentflowiov1alpha1.WorkflowStepTypeAITask,
			TaskSpec:   taskSpec,
			OutputPath: outputPath,
			ChapterNum: ch.Num,
		})

		if isPlot {
			continue
		}
		if arcInterval > 0 && ch.Num%arcInterval == 0 && ch.Num < maxChapter {
			start, end := ArcRange(ch.Num, arcInterval)
			items = append(items, ResolvedStep{
				ID:   ArcStepID(ch.Num, width),
				Name: fmt.Sprintf("故事弧摘要 第%d-%d章", start, end),
				Type: agentflowiov1alpha1.WorkflowStepTypeAITask,
				TaskSpec: agentflowiov1alpha1.WorkflowTaskSpec{
					QualityThreshold: 65,
					MonitorTaskType:  "general",
				},
				OutputPath: ArcFileName(start, end, width),
			})
		}
	}
	return items, nil
}

const (
	ExecutionModeSequential = "sequential"
	ExecutionModeParallel   = "parallel"

	ChapterModeSequential = "sequential"
	ChapterModePipeline   = "pipeline"

	DefaultMaxParallel        = 4
	DefaultChapterPipeline    = 4
	DefaultStepMaxRetries     = 3
	DefaultStepRetryBaseSec   = 20
	DefaultStepRetryMaxSec    = 180
	DefaultTaskMaxRetries     = 5
)

// IsParallelMode reports whether the workflow should dispatch multiple ready steps.
func IsParallelMode(wf *agentflowiov1alpha1.Workflow) bool {
	return strings.EqualFold(wf.Spec.Execution.Mode, ExecutionModeParallel)
}

// MaxParallel returns the concurrent task limit for parallel execution.
func MaxParallel(wf *agentflowiov1alpha1.Workflow) int {
	if wf.Spec.Execution.MaxParallel > 0 {
		return wf.Spec.Execution.MaxParallel
	}
	return DefaultMaxParallel
}

// IsPipelineChapterMode reports whether chapters use sliding-window parallelism.
func IsPipelineChapterMode(wf *agentflowiov1alpha1.Workflow) bool {
	return strings.EqualFold(wf.Spec.Execution.ChapterMode, ChapterModePipeline)
}

// StepMaxRetries returns workflow-level retries after a task exhausts its own attempts.
// Zero disables auto-retry and preserves the legacy pause-on-first-failure behavior.
func StepMaxRetries(wf *agentflowiov1alpha1.Workflow) int {
	if wf.Spec.Execution.StepMaxRetries > 0 {
		return wf.Spec.Execution.StepMaxRetries
	}
	return DefaultStepMaxRetries
}

// StepAutoRetryEnabled reports whether failed steps should be retried by the workflow controller.
func StepAutoRetryEnabled(wf *agentflowiov1alpha1.Workflow) bool {
	return StepMaxRetries(wf) > 0
}

// StepRetryBaseDelaySec returns the base delay before re-dispatching a failed workflow step.
func StepRetryBaseDelaySec(wf *agentflowiov1alpha1.Workflow) int {
	if wf != nil && wf.Spec.Execution.StepRetryBaseDelaySec > 0 {
		return wf.Spec.Execution.StepRetryBaseDelaySec
	}
	return DefaultStepRetryBaseSec
}

// StepRetryMaxDelaySec returns the cap for workflow step retry backoff.
func StepRetryMaxDelaySec(wf *agentflowiov1alpha1.Workflow) int {
	if wf != nil && wf.Spec.Execution.StepRetryMaxDelaySec > 0 {
		return wf.Spec.Execution.StepRetryMaxDelaySec
	}
	return DefaultStepRetryMaxSec
}

// TaskMaxRetriesForStep resolves per-task retry budget from workflow params and step type.
func TaskMaxRetriesForStep(wf *agentflowiov1alpha1.Workflow, stepID string) int32 {
	if wf != nil {
		if n := IntParam(wf.Spec.Params, "taskMaxRetries", 0); n > 0 {
			return int32(n)
		}
	}
	if _, ok := ChapterNumFromStepID(stepID); ok {
		return int32(DefaultTaskMaxRetries)
	}
	return 3
}

// StepStatusRetries returns how many workflow-level retries have been attempted for a step.
func StepStatusRetries(wf *agentflowiov1alpha1.Workflow, stepID string) int32 {
	if st := findStepStatus(wf, stepID); st != nil {
		return st.Retries
	}
	return 0
}

// ChapterPipeline returns how many chapters can run concurrently in pipeline mode.
func ChapterPipeline(wf *agentflowiov1alpha1.Workflow) int {
	if wf.Spec.Execution.ChapterPipeline > 0 {
		return wf.Spec.Execution.ChapterPipeline
	}
	if n := MaxParallel(wf); n > 1 {
		return n
	}
	return DefaultChapterPipeline
}

// ReadySteps returns all executable steps whose dependencies are satisfied.
// Steps already running or pending are excluded. Missing chapters are prioritized for backfill.
// A failed step only blocks the workflow when it is the earliest missing chapter and retries are exhausted.
func ReadySteps(wf *agentflowiov1alpha1.Workflow, resolved []ResolvedStep) ([]ResolvedStep, error) {
	completed := completedStepSet(wf)
	deps := specDependencyMap(wf)
	missing := MissingChapterNumbers(wf)

	var ready []ResolvedStep
	var blockingFailed *ResolvedStep
	var blockingMessage string

	for i := range resolved {
		step := resolved[i]
		if completed[step.ID] {
			continue
		}
		if !dependenciesMet(wf, step.ID, deps, completed) {
			continue
		}
		if running := findStepStatus(wf, step.ID); running != nil {
			switch running.Phase {
			case agentflowiov1alpha1.TaskPhaseRunning, agentflowiov1alpha1.TaskPhasePending:
				continue
			case agentflowiov1alpha1.TaskPhaseFailed:
				if StepFailureExhausted(wf, step.ID) && failedStepBlocksProgress(wf, step, missing) {
					blockingFailed = &step
					blockingMessage = running.Message
				}
				continue
			}
		}
		ready = append(ready, step)
	}

	if len(ready) == 0 && blockingFailed != nil {
		return nil, fmt.Errorf("step %s failed: %s", blockingFailed.ID, blockingMessage)
	}
	return PrioritizeBackfillSteps(ready, missing), nil
}

// NextStep returns the first unresolved step in dependency order.
func NextStep(wf *agentflowiov1alpha1.Workflow, resolved []ResolvedStep) (*ResolvedStep, error) {
	ready, err := ReadySteps(wf, resolved)
	if err != nil || len(ready) == 0 {
		return nil, err
	}
	return &ready[0], nil
}

func completedStepSet(wf *agentflowiov1alpha1.Workflow) map[string]bool {
	completed := make(map[string]bool, len(wf.Status.CompletedSteps))
	for _, id := range wf.Status.CompletedSteps {
		completed[id] = true
	}
	return completed
}

func specDependencyMap(wf *agentflowiov1alpha1.Workflow) map[string][]string {
	specSteps := wf.Spec.Steps
	if len(specSteps) == 0 {
		built, err := BuildSpecFromTemplate(wf.Spec.Template, wf.Spec.Prompt, wf.Spec.Params)
		if err == nil {
			specSteps = built.Steps
		}
	}
	return dependencyMap(specSteps)
}

func dependencyMap(steps []agentflowiov1alpha1.WorkflowStep) map[string][]string {
	deps := make(map[string][]string)
	for _, step := range steps {
		if step.Type == agentflowiov1alpha1.WorkflowStepTypeForeach {
			prefix := "chapter"
			if step.Foreach != nil && step.Foreach.StepIDPrefix != "" {
				prefix = step.Foreach.StepIDPrefix
			}
			deps[prefix] = append([]string{}, step.DependsOn...)
			continue
		}
		deps[step.ID] = append([]string{}, step.DependsOn...)
	}
	return deps
}

func dependenciesMet(wf *agentflowiov1alpha1.Workflow, stepID string, deps map[string][]string, completed map[string]bool) bool {
	if arcEnd, ok := ArcEndFromStepID(stepID); ok {
		width := chapterDependencyWidth(wf)
		return completed[ChapterStepID("chapter", arcEnd, width)]
	}

	if num, ok := PlotNumFromStepID(stepID); ok {
		width := chapterDependencyWidth(wf)
		if num > 1 {
			gate := chapterGateNum(wf, num)
			if gate > 0 {
				return completed[ChapterStepID("plot", gate, width)]
			}
		}
		for _, dep := range deps["plot"] {
			if !completed[dep] {
				return false
			}
		}
		return true
	}

	if num, ok := ChapterNumFromStepID(stepID); ok {
		width := chapterDependencyWidth(wf)
		if PlotsStageActive(wf) && !completed[ChapterStepID("plot", num, width)] {
			return false
		}
		if num > 1 {
			prev := num - 1
			interval := DefaultArcInterval(wf.Spec.Params, OutlineChapterCount(wf))
			if interval > 0 && prev%interval == 0 {
				return completed[ArcStepID(prev, width)]
			}
			gate := chapterGateNum(wf, num)
			if gate > 0 {
				return completed[ChapterStepID("chapter", gate, width)]
			}
		}
		for _, dep := range deps["chapter"] {
			if dep == "plots" {
				continue
			}
			if !completed[dep] {
				return false
			}
		}
		return true
	}

	stepDeps := deps[stepID]
	for _, dep := range stepDeps {
		if dep == "chapters" {
			if !completed["chapters"] {
				return false
			}
			continue
		}
		if !completed[dep] {
			return false
		}
	}
	return true
}

// chapterGateNum returns the chapter that must complete before num can start.
// In pipeline mode, chapter N only waits for chapter N-pipeline instead of N-1.
func chapterGateNum(wf *agentflowiov1alpha1.Workflow, num int) int {
	if num <= 1 {
		return 0
	}
	if !IsPipelineChapterMode(wf) {
		return num - 1
	}
	pipeline := ChapterPipeline(wf)
	gate := num - pipeline
	if gate < 1 {
		return 0
	}
	return gate
}

func chapterDependencyWidth(wf *agentflowiov1alpha1.Workflow) int {
	if count := OutlineChapterCount(wf); count > 0 {
		return ChapterPaddingWidth(count)
	}
	return 2
}

func findStepStatus(wf *agentflowiov1alpha1.Workflow, id string) *agentflowiov1alpha1.WorkflowStepStatus {
	for i := range wf.Status.StepStatuses {
		if wf.Status.StepStatuses[i].ID == id {
			return &wf.Status.StepStatuses[i]
		}
	}
	return nil
}

// TotalSteps estimates progress total count.
func TotalSteps(wf *agentflowiov1alpha1.Workflow, resolved []ResolvedStep) int32 {
	if len(resolved) > 0 {
		return int32(len(resolved))
	}

	spec := wf.Spec
	if len(spec.Steps) == 0 && spec.Template != "" {
		if built, err := BuildSpecFromTemplate(spec.Template, spec.Prompt, spec.Params); err == nil {
			spec = built
		}
	}

	chapterCount := IntParam(wf.Spec.Params, "chapterCount", 10)
	arcInterval := DefaultArcInterval(wf.Spec.Params, chapterCount)
	count := int32(0)
	for _, step := range spec.Steps {
		if step.Type == agentflowiov1alpha1.WorkflowStepTypeForeach {
			if chapterCount > 0 {
				count += int32(chapterCount)
				if arcInterval > 0 {
					count += int32((chapterCount - 1) / arcInterval)
				}
			}
			continue
		}
		count++
	}
	return count
}
