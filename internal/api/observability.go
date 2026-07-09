package api

import (
	"context"
	"net/http"

	agentflowiov1alpha1 "agent-flow/api/v1alpha1"
	"agent-flow/internal/metrics"
)

// ClusterStats is live resource counts from the Kubernetes API.
type ClusterStats struct {
	TasksByPhase     map[string]int `json:"tasks_by_phase"`
	WorkflowsByPhase map[string]int `json:"workflows_by_phase"`
	TasksTotal       int            `json:"tasks_total"`
	WorkflowsTotal   int            `json:"workflows_total"`
}

// ObservabilityReport combines process metrics and cluster state.
type ObservabilityReport struct {
	Metrics *metrics.Summary `json:"metrics"`
	Cluster ClusterStats     `json:"cluster"`
}

func (a *API) handleObservability(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	summary, err := metrics.GatherSummary()
	if err != nil {
		writeJSON(w, Response{Success: false, Error: err.Error()})
		return
	}

	cluster, err := a.gatherClusterStats(r.Context())
	if err != nil {
		writeJSON(w, Response{Success: false, Error: err.Error()})
		return
	}

	writeJSON(w, Response{
		Success: true,
		Data: ObservabilityReport{
			Metrics: summary,
			Cluster: cluster,
		},
	})
}

func (a *API) gatherClusterStats(ctx context.Context) (ClusterStats, error) {
	out := ClusterStats{
		TasksByPhase:     map[string]int{},
		WorkflowsByPhase: map[string]int{},
	}

	taskList := &agentflowiov1alpha1.TaskList{}
	if err := a.client.List(ctx, taskList); err != nil {
		return out, err
	}
	out.TasksTotal = len(taskList.Items)
	for _, t := range taskList.Items {
		phase := string(t.Status.Phase)
		if phase == "" {
			phase = "Pending"
		}
		out.TasksByPhase[phase]++
	}

	wfList := &agentflowiov1alpha1.WorkflowList{}
	if err := a.client.List(ctx, wfList); err != nil {
		return out, err
	}
	out.WorkflowsTotal = len(wfList.Items)
	for _, wf := range wfList.Items {
		phase := string(wf.Status.Phase)
		if phase == "" {
			phase = "Pending"
		}
		out.WorkflowsByPhase[phase]++
	}
	return out, nil
}
