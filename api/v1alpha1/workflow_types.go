package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// WorkflowPhase represents workflow lifecycle phase.
type WorkflowPhase string

const (
	WorkflowPhasePending   WorkflowPhase = "Pending"
	WorkflowPhaseRunning   WorkflowPhase = "Running"
	WorkflowPhasePaused    WorkflowPhase = "Paused"
	WorkflowPhaseSucceeded WorkflowPhase = "Succeeded"
	WorkflowPhaseFailed    WorkflowPhase = "Failed"
)

// WorkflowStepType identifies how a step executes.
type WorkflowStepType string

const (
	WorkflowStepTypeAITask  WorkflowStepType = "ai-task"
	WorkflowStepTypeForeach WorkflowStepType = "foreach"
	WorkflowStepTypeMerge   WorkflowStepType = "merge"
)

// WorkflowSpec defines desired workflow state.
type WorkflowSpec struct {
	Prompt    string            `json:"prompt,omitempty"`
	Template  string            `json:"template,omitempty"`
	Params    map[string]string `json:"params,omitempty"`
	Steps     []WorkflowStep    `json:"steps,omitempty"`
	Execution WorkflowExecution `json:"execution,omitempty"`
	Workspace WorkflowWorkspace `json:"workspace,omitempty"`
}

// WorkflowExecution controls orchestration behavior.
type WorkflowExecution struct {
	Mode                  string `json:"mode,omitempty"`
	MaxParallel           int    `json:"maxParallel,omitempty"`
	ChapterMode           string `json:"chapterMode,omitempty"`
	ChapterPipeline       int    `json:"chapterPipeline,omitempty"`
	StepMaxRetries        int    `json:"stepMaxRetries,omitempty"`
	StepRetryBaseDelaySec int    `json:"stepRetryBaseDelaySec,omitempty"`
	StepRetryMaxDelaySec  int    `json:"stepRetryMaxDelaySec,omitempty"`
	PauseOnStepFail       bool   `json:"pauseOnStepFail,omitempty"`
	AutoApprove           bool   `json:"autoApprove,omitempty"`
}

// WorkflowWorkspace defines artifact storage.
type WorkflowWorkspace struct {
	PVC      string `json:"pvc,omitempty"`
	BasePath string `json:"basePath,omitempty"`
}

// WorkflowStep is a single stage in the pipeline.
type WorkflowStep struct {
	ID        string             `json:"id"`
	Name      string             `json:"name,omitempty"`
	Type      WorkflowStepType   `json:"type"`
	DependsOn []string           `json:"dependsOn,omitempty"`
	Optional  bool               `json:"optional,omitempty"`
	TaskSpec  WorkflowTaskSpec   `json:"taskSpec,omitempty"`
	Foreach   *WorkflowForeach   `json:"foreach,omitempty"`
	Output    WorkflowStepOutput `json:"output,omitempty"`
}

// WorkflowTaskSpec describes AI task execution for a step.
type WorkflowTaskSpec struct {
	WorkerInstruction string `json:"workerInstruction,omitempty"`
	MCPMode           bool   `json:"mcpMode,omitempty"`
	QualityThreshold  int32  `json:"qualityThreshold,omitempty"`
	MonitorTaskType   string `json:"monitorTaskType,omitempty"`
}

// WorkflowForeach expands a step into repeated child executions.
type WorkflowForeach struct {
	Source       string `json:"source,omitempty"`
	JSONPath     string `json:"jsonPath,omitempty"`
	StepIDPrefix string `json:"stepIdPrefix,omitempty"`
	Instruction  string `json:"instruction,omitempty"`
	OutputPath   string `json:"outputPath,omitempty"`
}

// WorkflowStepOutput defines where step output is stored in workspace.
type WorkflowStepOutput struct {
	Path   string `json:"path,omitempty"`
	Format string `json:"format,omitempty"`
}

// WorkflowProgress tracks overall completion.
type WorkflowProgress struct {
	Total     int32 `json:"total,omitempty"`
	Completed int32 `json:"completed,omitempty"`
	Percent   int32 `json:"percent,omitempty"`
}

// WorkflowStepStatus records runtime state for a step.
type WorkflowStepStatus struct {
	ID         string       `json:"id"`
	Phase      TaskPhase    `json:"phase"`
	Retries    int32        `json:"retries,omitempty"`
	TaskName   string       `json:"taskName,omitempty"`
	Message    string       `json:"message,omitempty"`
	Output     string       `json:"outputPath,omitempty"`
	Score      int32        `json:"score,omitempty"`
	StartedAt  *metav1.Time `json:"startedAt,omitempty"`
	FinishedAt *metav1.Time `json:"finishedAt,omitempty"`
}

// WorkflowStatus defines observed workflow state.
type WorkflowStatus struct {
	Phase          WorkflowPhase        `json:"phase"`
	Message        string               `json:"message,omitempty"`
	CurrentStep    string               `json:"currentStep,omitempty"`
	CompletedSteps []string             `json:"completedSteps,omitempty"`
	FailedSteps    []string             `json:"failedSteps,omitempty"`
	StepStatuses   []WorkflowStepStatus `json:"stepStatuses,omitempty"`
	Progress       WorkflowProgress     `json:"progress,omitempty"`
	WorkspacePath  string               `json:"workspacePath,omitempty"`
	StartTime      *metav1.Time         `json:"startTime,omitempty"`
	CompletionTime *metav1.Time         `json:"completionTime,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Step",type="string",JSONPath=".status.currentStep"
// +kubebuilder:printcolumn:name="Progress",type="string",JSONPath=".status.progress.percent"
// +kubebuilder:storageversion

// Workflow is the Schema for the workflows API.
type Workflow struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WorkflowSpec   `json:"spec,omitempty"`
	Status WorkflowStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// WorkflowList contains a list of Workflow.
type WorkflowList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Workflow `json:"items"`
}
