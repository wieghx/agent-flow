package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	agentflowiov1alpha1 "agent-flow/api/v1alpha1"
	"agent-flow/internal/flow"
	wfengine "agent-flow/internal/workflow"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// RegenerateChapterRequest triggers a single chapter/plot rewrite workflow.
type RegenerateChapterRequest struct {
	Instruction string `json:"instruction"`
	Layer         string `json:"layer"`
}

// RegenerateChapterResponse describes the spawned rewrite workflow.
type RegenerateChapterResponse struct {
	ParentWorkflow   string `json:"parent_workflow"`
	RewriteWorkflow  string `json:"rewrite_workflow"`
	Namespace        string `json:"namespace"`
	ChapterNum       int    `json:"chapter_num"`
	Layer            string `json:"layer"`
	WorkspacePath    string `json:"workspace_path,omitempty"`
}

func (a *API) handleNovelRegenerateChapter(w http.ResponseWriter, r *http.Request, namespace, name string, chapterNum int) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req RegenerateChapterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, Response{Success: false, Error: err.Error()})
		return
	}
	instruction := strings.TrimSpace(req.Instruction)
	if instruction == "" {
		writeJSON(w, Response{Success: false, Error: "instruction is required"})
		return
	}
	layer := strings.ToLower(strings.TrimSpace(req.Layer))
	if layer == "" {
		layer = wfengine.RewriteLayerChapter
	}
	if layer != wfengine.RewriteLayerPlot && layer != wfengine.RewriteLayerChapter {
		writeJSON(w, Response{Success: false, Error: "layer must be plot or chapter"})
		return
	}

	parent := &agentflowiov1alpha1.Workflow{}
	if err := a.client.Get(r.Context(), client.ObjectKey{Namespace: namespace, Name: name}, parent); err != nil {
		writeJSON(w, Response{Success: false, Error: err.Error()})
		return
	}
	workspace := parent.Status.WorkspacePath
	if workspace == "" {
		workspace = wfengine.WorkspacePath(parent)
	}
	if err := wfengine.EnsureWorkspace(parent); err != nil {
		writeJSON(w, Response{Success: false, Error: err.Error()})
		return
	}

	outline, err := wfengine.LoadOutline(parent)
	if err != nil {
		writeJSON(w, Response{Success: false, Error: fmt.Sprintf("outline not ready: %v", err)})
		return
	}
	found := false
	for _, ch := range outline.Chapters {
		if ch.Num == chapterNum {
			found = true
			break
		}
	}
	if !found {
		writeJSON(w, Response{Success: false, Error: fmt.Sprintf("chapter %d not in outline", chapterNum)})
		return
	}

	params := rewriteParamsForParent(parent, chapterNum, layer, instruction, workspace)
	outputPath := wfengine.RewriteOutputPath(params)
	if outputPath == "" {
		writeJSON(w, Response{Success: false, Error: "invalid rewrite params"})
		return
	}
	if _, err := wfengine.ReadArtifact(parent, outputPath); err != nil {
		if layer == wfengine.RewriteLayerChapter {
			writeJSON(w, Response{Success: false, Error: fmt.Sprintf("chapter %d has no saved content yet", chapterNum)})
			return
		}
	}

	rewriteName := proposeRewriteWorkflowName(name, chapterNum, layer)
	existing := &agentflowiov1alpha1.Workflow{}
	if err := a.client.Get(r.Context(), client.ObjectKey{Namespace: namespace, Name: rewriteName}, existing); err == nil {
		if existing.Status.Phase == agentflowiov1alpha1.WorkflowPhaseRunning ||
			existing.Status.Phase == agentflowiov1alpha1.WorkflowPhasePending {
			writeJSON(w, Response{Success: false, Error: fmt.Sprintf("rewrite workflow %s already running", rewriteName)})
			return
		}
		_ = a.client.Delete(r.Context(), existing)
	} else if !k8serrors.IsNotFound(err) {
		writeJSON(w, Response{Success: false, Error: err.Error()})
		return
	}

	prompt := fmt.Sprintf("重写第%d章%s：%s", chapterNum, layerLabel(layer), instruction)
	wf, err := flow.NewWorkflowCRD("novel-chapter-rewrite", prompt, params, rewriteName, namespace)
	if err != nil {
		writeJSON(w, Response{Success: false, Error: err.Error()})
		return
	}
	if wf.Labels == nil {
		wf.Labels = map[string]string{}
	}
	wf.Labels["agentflow.io/rewrite-parent"] = name
	wf.Labels["agentflow.io/source"] = "novel-rewrite"

	if err := a.client.Create(r.Context(), wf); err != nil {
		writeJSON(w, Response{Success: false, Error: err.Error()})
		return
	}

	writeJSON(w, Response{
		Success: true,
		Message: "Chapter rewrite workflow created",
		Data: RegenerateChapterResponse{
			ParentWorkflow:  name,
			RewriteWorkflow: rewriteName,
			Namespace:       namespace,
			ChapterNum:      chapterNum,
			Layer:           layer,
			WorkspacePath:   workspace,
		},
	})
}

func rewriteParamsForParent(parent *agentflowiov1alpha1.Workflow, chapterNum int, layer, instruction, workspace string) map[string]string {
	params := wfengine.MergeParams(parent.Spec.Params, map[string]string{
		wfengine.ParamSharedWorkspace: workspace,
		wfengine.ParamRewriteChapter:  strconv.Itoa(chapterNum),
		wfengine.ParamRewriteLayer:    layer,
		wfengine.ParamRewriteNote:     instruction,
		wfengine.ParamParentWorkflow:  parent.Name,
		"chapterCount":                strconv.Itoa(wfengine.OutlineChapterCount(parent)),
	})
	if params["chapterCount"] == "0" {
		params["chapterCount"] = strconv.Itoa(chapterNum)
	}
	return params
}

func proposeRewriteWorkflowName(parent string, chapterNum int, layer string) string {
	tag := "c"
	if layer == wfengine.RewriteLayerPlot {
		tag = "p"
	}
	suffix := fmt.Sprintf("-rw-%s%02d", tag, chapterNum)
	maxParent := 63 - len(suffix)
	if maxParent < 1 {
		maxParent = 1
	}
	base := parent
	if len(base) > maxParent {
		base = strings.TrimRight(base[:maxParent], "-")
	}
	return base + suffix
}

func layerLabel(layer string) string {
	if layer == wfengine.RewriteLayerPlot {
		return "剧情"
	}
	return "正文"
}

func parseChapterNumFromPath(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, fmt.Errorf("chapter number required")
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid chapter number: %s", raw)
	}
	return n, nil
}