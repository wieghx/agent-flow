// +kubebuilder:object:generate=true
// +groupName=agentflow.io
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen=true
// MonitorSpec defines the desired state of Monitor
type MonitorSpec struct {
	// Namespace is the namespace to monitor (empty means all namespaces)
	Namespace string `json:"namespace,omitempty"`

	// LabelSelector filters resources by labels
	LabelSelector map[string]string `json:"labelSelector,omitempty"`

	// CheckIntervalSeconds is how often to check task status
	CheckIntervalSeconds int32 `json:"checkIntervalSeconds,omitempty"`

	// FailedTaskThreshold is how many failures before alerting
	FailedTaskThreshold int32 `json:"failedTaskThreshold,omitempty"`

	// StaleTaskThreshold is how long before a task is considered stale
	StaleTaskThreshold int32 `json:"staleTaskThreshold,omitempty"`

	// Actions defines what to do when conditions are met
	Actions []MonitorAction `json:"actions,omitempty"`
}

// +k8s:deepcopy-gen=true
// MonitorAction defines an action to take on certain conditions
type MonitorAction struct {
	// Condition is the condition that triggers this action
	Condition MonitorCondition `json:"condition"`

	// ActionType is the type of action to take
	ActionType ActionType `json:"actionType"`

	// Severity is the alert severity level
	Severity string `json:"severity,omitempty"`
}

// MonitorCondition defines what condition to monitor
type MonitorCondition string

const (
	MonitorConditionOnFailure MonitorCondition = "OnFailure"
	MonitorConditionOnTimeout MonitorCondition = "OnTimeout"
	MonitorConditionOnStale   MonitorCondition = "OnStale"
	MonitorConditionOnRetry   MonitorCondition = "OnRetry"
)

// ActionType defines what action to take
type ActionType string

const (
	ActionTypeAlert    ActionType = "Alert"
	ActionTypeRetry    ActionType = "Retry"
	ActionTypeCancel   ActionType = "Cancel"
	ActionTypeNotify   ActionType = "Notify"
	ActionTypeEscalate ActionType = "Escalate"
)

// +k8s:deepcopy-gen=true
// MonitorStatus defines the observed state of Monitor
type MonitorStatus struct {
	// Phase is the current state of the monitor
	Phase MonitorPhase `json:"phase"`

	// Message provides additional status information
	Message string `json:"message,omitempty"`

	// LastCheckTime is when the last check was performed
	LastCheckTime *metav1.Time `json:"lastCheckTime,omitempty"`

	// LastCheckTimeUnix is when the last check was performed (Unix timestamp)
	LastCheckTimeUnix int64 `json:"lastCheckTimeUnix,omitempty"`

	// MonitoredCount is the number of tasks being monitored
	MonitoredCount int32 `json:"monitoredCount,omitempty"`

	// PhaseDistribution tracks tasks by phase
	PhaseDistribution TaskPhaseCounts `json:"phaseDistribution,omitempty"`

	// Alerts is the list of recent alerts
	Alerts []Alert `json:"alerts,omitempty"`

	// StartTime is when the monitor started
	StartTime *metav1.Time `json:"startTime,omitempty"`
}

// MonitorPhase represents the phase of a monitor
type MonitorPhase string

const (
	MonitorPhaseRunning MonitorPhase = "Running"
	MonitorPhasePaused  MonitorPhase = "Paused"
	MonitorPhaseError   MonitorPhase = "Error"
)

// TaskPhaseCounts tracks the count of tasks by phase
type TaskPhaseCounts struct {
	Pending   int32 `json:"pending,omitempty"`
	Running   int32 `json:"running,omitempty"`
	Succeeded int32 `json:"succeeded,omitempty"`
	Failed    int32 `json:"failed,omitempty"`
	Cancelled int32 `json:"cancelled,omitempty"`
}

// +k8s:deepcopy-gen=true
// Alert represents a generated alert
type Alert struct {
	// Time is when the alert was generated
	Time metav1.Time `json:"time"`

	// TaskName is the name of the task that triggered the alert
	TaskName string `json:"taskName"`

	// Phase is the phase of the task
	Phase string `json:"phase"`

	// Message is the alert message
	Message string `json:"message"`

	// ActionType is the action taken
	ActionType string `json:"actionType"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.phase",description="Current status of the monitor"
// +kubebuilder:printcolumn:name="Tasks",type="integer",JSONPath=".status.tasksMonitored",description="Number of tasks monitored"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time since creation"
// +kubebuilder:storageversion

// Monitor is the Schema for the monitors API
type Monitor struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MonitorSpec   `json:"spec,omitempty"`
	Status MonitorStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MonitorList contains a list of Monitor
type MonitorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Monitor `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Monitor{}, &MonitorList{})
}
