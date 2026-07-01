package architecture

import (
	"fmt"

	agentflowiov1alpha1 "agent-flow/api/v1alpha1"
	wfengine "agent-flow/internal/workflow"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func taskNameForWorkflowStep(workflowName, stepID string) string {
	return fmt.Sprintf("wf-%s-%s", workflowName, stepID)
}

func buildWorkflowTask(wf *agentflowiov1alpha1.Workflow, step wfengine.ResolvedStep) *agentflowiov1alpha1.Task {
	instruction := wfengine.PrepareWorkerInstruction(wf, step)
	if instruction == "" {
		instruction = wf.Spec.Prompt
	}

	threshold := step.TaskSpec.QualityThreshold
	if threshold <= 0 {
		threshold = 70
	}

	task := &agentflowiov1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name:      taskNameForWorkflowStep(wf.Name, step.ID),
			Namespace: wf.Namespace,
			Labels: map[string]string{
				"agentflow.io/workflow":            wf.Name,
				"agentflow.io/workflow-step":       step.ID,
				"agentflow.io/needs-quality-check": "true",
			},
			Annotations: map[string]string{
				"agentflow.io/workflow-uid":      string(wf.UID),
				"agentflow.io/workflow-output":   step.OutputPath,
				"agentflow.io/quality-threshold": fmt.Sprintf("%d", threshold),
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: agentflowiov1alpha1.GroupVersion.String(),
					Kind:       "Workflow",
					Name:       wf.Name,
					UID:        wf.UID,
				},
			},
		},
		Spec: agentflowiov1alpha1.TaskSpec{
			Command: []string{"/bin/sh", "-c"},
			Args:    []string{fmt.Sprintf("echo 'Executing: %s'", instruction)},
			Image:   "docker.io/library/alpine:latest",
			RetryPolicy: agentflowiov1alpha1.RetryPolicy{
				MaxRetries:        wfengine.TaskMaxRetriesForStep(wf, step.ID),
				RetryDelaySeconds: 15,
				RetryOn:           []agentflowiov1alpha1.TaskCondition{agentflowiov1alpha1.TaskConditionOnFailure},
			},
			Resources: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					"cpu":    resource.MustParse("500m"),
					"memory": resource.MustParse("512Mi"),
				},
				Requests: corev1.ResourceList{
					"cpu":    resource.MustParse("250m"),
					"memory": resource.MustParse("128Mi"),
				},
			},
			TimeoutSeconds: func() *int32 { v := int32(600); return &v }(),
		},
	}

	if step.TaskSpec.MCPMode {
		task.Annotations["agentflow.io/mcp-mode"] = "true"
		task.Annotations["agentflow.io/network-policy"] = "permissive"
	}
	if step.ID == "historical-research" {
		task.Labels["agentflow.io/needs-quality-check"] = "false"
		task.Spec.TimeoutSeconds = func() *int32 { v := int32(1200); return &v }()
	}
	if step.TaskSpec.MonitorTaskType != "" {
		task.Annotations["agentflow.io/monitor-task-type"] = step.TaskSpec.MonitorTaskType
	}
	if TeamModeFromWorkflow(wf) || step.TaskSpec.MonitorTaskType == "novel-chapter-team" {
		task.Annotations["agentflow.io/team-mode"] = "true"
	}
	return task
}

func TeamModeFromWorkflow(wf *agentflowiov1alpha1.Workflow) bool {
	if wf == nil {
		return false
	}
	return wfengine.TeamModeEnabled(wf.Spec.Params)
}
