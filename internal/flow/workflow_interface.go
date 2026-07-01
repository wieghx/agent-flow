package flow

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"

	agentflowiov1alpha1 "agent-flow/api/v1alpha1"
	wfengine "agent-flow/internal/workflow"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var workflowNameSanitizer = regexp.MustCompile(`[^a-z0-9-]+`)

// WorkflowRequest is a pending workflow awaiting approval.
type WorkflowRequest struct {
	ID          string            `json:"id"`
	Description string            `json:"description"`
	Template    string            `json:"template"`
	Params      map[string]string `json:"params,omitempty"`
	Prompt      string            `json:"prompt"`
	ProposedName string           `json:"proposed_name,omitempty"`
	CreatedName string            `json:"created_name,omitempty"`
	CreatedAt   metav1.Time       `json:"created_at"`
	Approved    bool              `json:"approved"`
	ApprovedAt  *metav1.Time      `json:"approved_at,omitempty"`
	ApprovedBy  string            `json:"approved_by,omitempty"`
}

// NewWorkflowCRD builds a Workflow CRD from template parameters.
func NewWorkflowCRD(template, prompt string, params map[string]string, name, namespace string) (*agentflowiov1alpha1.Workflow, error) {
	spec, err := wfengine.BuildSpecFromTemplate(template, prompt, params)
	if err != nil {
		return nil, err
	}
	return &agentflowiov1alpha1.Workflow{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"agentflow.io/template": template,
				"agentflow.io/source":   "chat",
			},
		},
		Spec: spec,
	}, nil
}

// CreateWorkflowFromRequest builds a Workflow CRD.
func (r *ChatRouter) CreateWorkflowFromRequest(req *WorkflowRequest) (*agentflowiov1alpha1.Workflow, error) {
	if !req.Approved {
		return nil, fmt.Errorf("workflow not approved")
	}
	name := req.ProposedName
	if name == "" {
		name = ProposeWorkflowName(req.Description)
	}
	return NewWorkflowCRD(req.Template, req.Prompt, req.Params, name, "default")
}

// ApproveWorkflow approves and creates Workflow CRD.
func (r *ChatRouter) ApproveWorkflow(workflowID, approver string) (*WorkflowRequest, error) {
	r.taskQueueMu.Lock()
	defer r.taskQueueMu.Unlock()

	for i, wf := range r.pendingWorkflows {
		if wf.ID == workflowID {
			wf.Approved = true
			now := metav1.Now()
			wf.ApprovedAt = &now
			wf.ApprovedBy = approver
			r.pendingWorkflows = append(r.pendingWorkflows[:i], r.pendingWorkflows[i+1:]...)

			crd, err := r.CreateWorkflowFromRequest(wf)
			if err != nil {
				return wf, err
			}
			if r.client != nil {
				ctx := context.Background()
				if err := r.client.Create(ctx, crd); err != nil {
					return wf, fmt.Errorf("create workflow failed: %w", err)
				}
				wf.CreatedName = crd.Name
				r.NotifyWorkflowEvent(WorkflowEvent{
					Namespace: crd.Namespace,
					Name:      crd.Name,
					Phase:     string(agentflowiov1alpha1.WorkflowPhaseRunning),
					Message:   fmt.Sprintf("工作流已创建并开始执行（模板 %s）", wf.Template),
				})
			}
			return wf, nil
		}
	}
	return nil, fmt.Errorf("workflow request not found: %s", workflowID)
}

// ListPendingWorkflows returns pending workflow requests.
func (r *ChatRouter) ListPendingWorkflows() []*WorkflowRequest {
	r.taskQueueMu.RLock()
	defer r.taskQueueMu.RUnlock()
	return r.pendingWorkflows
}

func (r *ChatRouter) addPendingWorkflow(wf *WorkflowRequest) {
	r.taskQueueMu.Lock()
	defer r.taskQueueMu.Unlock()
	r.pendingWorkflows = append(r.pendingWorkflows, wf)
}

func parseWorkflowMarker(resp string) (cleanResp string, wf *WorkflowRequest) {
	marker := "[CREATE_WORKFLOW:"
	idx := strings.Index(resp, marker)
	if idx == -1 {
		return resp, nil
	}
	start := idx + len(marker)
	end := strings.Index(resp[start:], "]")
	if end == -1 {
		return resp, nil
	}
	payload := strings.TrimSpace(resp[start : start+end])
	cleanResp = strings.TrimSpace(strings.Replace(resp, resp[idx:start+end+1], "", 1))

	template := "novel-outline-chapters"
	params := map[string]string{}
	desc := payload

	if strings.Contains(payload, ":") {
		parts := strings.SplitN(payload, ":", 2)
		template = strings.TrimSpace(parts[0])
		if len(parts) == 2 {
			_ = json.Unmarshal([]byte(parts[1]), &params)
			desc = fmt.Sprintf("工作流模板 %s", template)
		}
	}

	if template == "novel-outline-chapters" || template == "novel-team-chapters" || template == "novel-team-historical" || template == "" {
		chapters := wfengine.ExtractChapterCountFromText(desc)
		if chapters == 0 {
			chapters = wfengine.IntParam(params, "chapterCount", 0)
		}
		params = wfengine.MergeParams(wfengine.DefaultNovelParams(chapters), params)
		template = wfengine.DefaultNovelTemplate(params, desc)
		if template == "novel-team-historical" {
			desc = fmt.Sprintf("历史小说团队模式（%s 章，含联网调研）", params["chapterCount"])
		} else {
			desc = fmt.Sprintf("长篇小说团队模式（%s 章）", params["chapterCount"])
		}
	}

	req := &WorkflowRequest{
		ID:           fmt.Sprintf("workflow-%d", time.Now().UnixNano()),
		Description:  desc,
		Template:     template,
		Params:       params,
		ProposedName: ProposeWorkflowName(desc),
		CreatedAt:    metav1.Now(),
	}
	return cleanResp, req
}

// InferNovelWorkflowRequest builds a workflow proposal from user text when the model omits the marker.
func InferNovelWorkflowRequest(userMessage string) *WorkflowRequest {
	if !wfengine.LooksLikeNovelIntent(userMessage) {
		return nil
	}
	chapters := wfengine.ExtractChapterCountFromText(userMessage)
	params := wfengine.DefaultNovelParams(chapters)
	return &WorkflowRequest{
		ID:           fmt.Sprintf("workflow-%d", time.Now().UnixNano()),
		Description:  fmt.Sprintf("长篇小说（%s 章）", params["chapterCount"]),
		Template:     wfengine.DefaultNovelTemplate(params, userMessage),
		Params:       params,
		Prompt:       strings.TrimSpace(userMessage),
		ProposedName: ProposeWorkflowName(userMessage),
		CreatedAt:    metav1.Now(),
	}
}

// ProposeWorkflowName generates a DNS-safe workflow name from user text.
func ProposeWorkflowName(seed string) string {
	seed = strings.ToLower(strings.TrimSpace(seed))
	if seed == "" {
		seed = "novel"
	}
	var b strings.Builder
	prevDash := false
	for _, r := range seed {
		if unicode.Is(unicode.Han, r) {
			continue
		}
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			prevDash = false
			continue
		}
		if !prevDash && b.Len() > 0 {
			b.WriteByte('-')
			prevDash = true
		}
	}
	slug := workflowNameSanitizer.ReplaceAllString(b.String(), "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		slug = "novel"
	}
	if len(slug) > 32 {
		slug = slug[:32]
	}
	return fmt.Sprintf("%s-%d", slug, time.Now().Unix()%100000)
}

// RejectWorkflow removes a pending workflow request.
func (r *ChatRouter) RejectWorkflow(workflowID, reason string) error {
	r.taskQueueMu.Lock()
	defer r.taskQueueMu.Unlock()

	for i, wf := range r.pendingWorkflows {
		if wf.ID == workflowID {
			_ = reason
			r.pendingWorkflows = append(r.pendingWorkflows[:i], r.pendingWorkflows[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("workflow request not found: %s", workflowID)
}