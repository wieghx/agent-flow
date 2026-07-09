package metrics

import (
	"sort"
	"time"

	dto "github.com/prometheus/client_model/go"
	crmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

// AIRoleStats aggregates AI usage for one role.
type AIRoleStats struct {
	Role               string  `json:"role"`
	Model              string  `json:"model,omitempty"`
	RequestsOK         float64 `json:"requests_ok"`
	RequestsError      float64 `json:"requests_error"`
	PromptTokens       float64 `json:"prompt_tokens"`
	CompletionTokens   float64 `json:"completion_tokens"`
	AvgDurationSeconds float64 `json:"avg_duration_seconds,omitempty"`
}

// Summary is a JSON-friendly observability snapshot.
type Summary struct {
	GeneratedAt           string             `json:"generated_at"`
	AIByRole              []AIRoleStats      `json:"ai_by_role"`
	AIRequestsOK          float64            `json:"ai_requests_ok"`
	AIRequestsError       float64            `json:"ai_requests_error"`
	AITokensPrompt        float64            `json:"ai_tokens_prompt"`
	AITokensCompletion    float64            `json:"ai_tokens_completion"`
	TaskCompletions       map[string]float64 `json:"task_completions"`
	QualityChecksPassed   float64            `json:"quality_checks_passed"`
	QualityChecksFailed   float64            `json:"quality_checks_failed"`
	WorkflowReconciles    map[string]float64 `json:"workflow_reconciles"`
	PrometheusMetricsPath string             `json:"prometheus_metrics_path"`
}

// GatherSummary builds a snapshot from the process Prometheus registry.
func GatherSummary() (*Summary, error) {
	mfs, err := crmetrics.Registry.Gather()
	if err != nil {
		return nil, err
	}
	sum := &Summary{
		GeneratedAt:           time.Now().UTC().Format(time.RFC3339),
		TaskCompletions:       map[string]float64{},
		WorkflowReconciles:    map[string]float64{},
		PrometheusMetricsPath: "/metrics",
	}
	roleMap := map[string]*AIRoleStats{}

	for _, mf := range mfs {
		name := mf.GetName()
		for _, m := range mf.GetMetric() {
			val := metricValue(m)
			labels := labelMap(m)
			switch name {
			case namespace + "_ai_requests_total":
				role := labels["role"]
				model := labels["model"]
				key := role + "\x00" + model
				st, ok := roleMap[key]
				if !ok {
					st = &AIRoleStats{Role: role, Model: model}
					roleMap[key] = st
				}
				switch labels["status"] {
				case "ok":
					st.RequestsOK += val
					sum.AIRequestsOK += val
				case "error":
					st.RequestsError += val
					sum.AIRequestsError += val
				}
			case namespace + "_ai_tokens_total":
				role := labels["role"]
				key := role + "\x00"
				st, ok := roleMap[key]
				if !ok {
					st = &AIRoleStats{Role: role}
					roleMap[key] = st
				}
				switch labels["kind"] {
				case "prompt":
					st.PromptTokens += val
					sum.AITokensPrompt += val
				case "completion":
					st.CompletionTokens += val
					sum.AITokensCompletion += val
				}
			case namespace + "_ai_request_duration_seconds":
				role := labels["role"]
				model := labels["model"]
				key := role + "\x00" + model
				st, ok := roleMap[key]
				if !ok {
					st = &AIRoleStats{Role: role, Model: model}
					roleMap[key] = st
				}
				if h := m.GetHistogram(); h != nil && h.GetSampleCount() > 0 {
					st.AvgDurationSeconds = h.GetSampleSum() / float64(h.GetSampleCount())
				}
			case namespace + "_task_completions_total":
				sum.TaskCompletions[labels["phase"]] += val
			case namespace + "_quality_checks_total":
				if labels["passed"] == "true" {
					sum.QualityChecksPassed += val
				} else {
					sum.QualityChecksFailed += val
				}
			case namespace + "_workflow_reconciles_total":
				sum.WorkflowReconciles[labels["result"]] += val
			}
		}
	}

	sum.AIByRole = make([]AIRoleStats, 0, len(roleMap))
	for _, st := range roleMap {
		sum.AIByRole = append(sum.AIByRole, *st)
	}
	sort.Slice(sum.AIByRole, func(i, j int) bool {
		if sum.AIByRole[i].Role == sum.AIByRole[j].Role {
			return sum.AIByRole[i].Model < sum.AIByRole[j].Model
		}
		return sum.AIByRole[i].Role < sum.AIByRole[j].Role
	})
	return sum, nil
}

func labelMap(m *dto.Metric) map[string]string {
	out := map[string]string{}
	for _, lp := range m.GetLabel() {
		out[lp.GetName()] = lp.GetValue()
	}
	return out
}

func metricValue(m *dto.Metric) float64 {
	if c := m.GetCounter(); c != nil {
		return c.GetValue()
	}
	if h := m.GetHistogram(); h != nil {
		return float64(h.GetSampleCount())
	}
	return 0
}
