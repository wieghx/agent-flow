// Package metrics exposes Prometheus instrumentation for Agent Flow.
package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	crmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

const namespace = "agentflow"

var (
	aiRequestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "ai_requests_total",
		Help:      "Total AI chat completion requests by role, model, and status.",
	}, []string{"role", "model", "status"})

	aiRequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace,
		Name:      "ai_request_duration_seconds",
		Help:      "AI request latency in seconds.",
		Buckets:   prometheus.DefBuckets,
	}, []string{"role", "model"})

	aiTokensTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "ai_tokens_total",
		Help:      "Cumulative LLM tokens observed in API responses.",
	}, []string{"role", "kind"})

	taskCompletionsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "task_completions_total",
		Help:      "Task runs reaching a terminal phase.",
	}, []string{"phase"})

	qualityChecksTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "quality_checks_total",
		Help:      "Monitor quality evaluations.",
	}, []string{"passed", "method"})

	workflowReconcilesTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "workflow_reconciles_total",
		Help:      "Workflow controller reconcile results.",
	}, []string{"result"})

	workflowReconcileDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace,
		Name:      "workflow_reconcile_duration_seconds",
		Help:      "Workflow reconcile loop duration.",
		Buckets:   []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30},
	}, []string{"result"})
)

func init() {
	crmetrics.Registry.MustRegister(
		aiRequestsTotal,
		aiRequestDuration,
		aiTokensTotal,
		taskCompletionsTotal,
		qualityChecksTotal,
		workflowReconcilesTotal,
		workflowReconcileDuration,
	)
}

// RecordAIRequest records one AI chat call.
func RecordAIRequest(role, model string, err error, duration time.Duration, promptTokens, completionTokens int) {
	if model == "" {
		model = "unknown"
	}
	status := "ok"
	if err != nil {
		status = "error"
	}
	aiRequestsTotal.WithLabelValues(role, model, status).Inc()
	if err == nil {
		aiRequestDuration.WithLabelValues(role, model).Observe(duration.Seconds())
	}
	if promptTokens > 0 {
		aiTokensTotal.WithLabelValues(role, "prompt").Add(float64(promptTokens))
	}
	if completionTokens > 0 {
		aiTokensTotal.WithLabelValues(role, "completion").Add(float64(completionTokens))
	}
}

// RecordTaskCompletion records a task reaching a terminal phase.
func RecordTaskCompletion(phase string) {
	if phase == "" {
		phase = "unknown"
	}
	taskCompletionsTotal.WithLabelValues(phase).Inc()
}

// RecordQualityCheck records a monitor evaluation outcome.
func RecordQualityCheck(passed bool, method string) {
	if method == "" {
		method = "unknown"
	}
	label := "false"
	if passed {
		label = "true"
	}
	qualityChecksTotal.WithLabelValues(label, method).Inc()
}

// RecordWorkflowReconcile records one workflow controller reconcile.
func RecordWorkflowReconcile(result string, duration time.Duration) {
	if result == "" {
		result = "unknown"
	}
	workflowReconcilesTotal.WithLabelValues(result).Inc()
	workflowReconcileDuration.WithLabelValues(result).Observe(duration.Seconds())
}

// WorkflowReconcileResult maps reconcile outcome to a metric label.
func WorkflowReconcileResult(err error, requeue bool, requeueAfter time.Duration) string {
	if err != nil {
		return "error"
	}
	if requeue || requeueAfter > 0 {
		return "requeue"
	}
	return "ok"
}