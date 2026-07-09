package api

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	agentflowiov1alpha1 "agent-flow/api/v1alpha1"
	wfengine "agent-flow/internal/workflow"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (a *API) handleNovelOutline(w http.ResponseWriter, r *http.Request, namespace, name string) {
	wf, err := a.loadNovelWorkflow(r, namespace, name)
	if err != nil {
		writeJSON(w, Response{Success: false, Error: err.Error()})
		return
	}

	switch r.Method {
	case http.MethodGet:
		a.handleNovelOutlineGet(w, wf)
	case http.MethodPut:
		a.handleNovelOutlinePut(w, r, wf)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a *API) loadNovelWorkflow(r *http.Request, namespace, name string) (*agentflowiov1alpha1.Workflow, error) {
	wf := &agentflowiov1alpha1.Workflow{}
	if err := a.client.Get(r.Context(), client.ObjectKey{Namespace: namespace, Name: name}, wf); err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, fmt.Errorf("novel workflow not found")
		}
		return nil, err
	}
	return wf, nil
}

func (a *API) handleNovelOutlineGet(w http.ResponseWriter, wf *agentflowiov1alpha1.Workflow) {
	outline, err := wfengine.LoadOutline(wf)
	if err != nil {
		writeJSON(w, Response{Success: false, Error: fmt.Sprintf("outline not ready: %v", err)})
		return
	}
	writeJSON(w, Response{Success: true, Data: outline})
}

func (a *API) handleNovelOutlinePut(w http.ResponseWriter, r *http.Request, wf *agentflowiov1alpha1.Workflow) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 4<<20))
	if err != nil {
		writeJSON(w, Response{Success: false, Error: err.Error()})
		return
	}
	raw := strings.TrimSpace(string(body))
	if raw == "" {
		writeJSON(w, Response{Success: false, Error: "outline body is empty"})
		return
	}

	normalized, err := wfengine.ParseOutlineJSON(raw)
	if err != nil {
		writeJSON(w, Response{Success: false, Error: err.Error()})
		return
	}
	if err := wfengine.ValidateOutlineChapterCount(normalized, wfengine.IntParam(wf.Spec.Params, "chapterCount", 0)); err != nil {
		writeJSON(w, Response{Success: false, Error: err.Error()})
		return
	}
	if strings.TrimSpace(normalized.Title) == "" {
		writeJSON(w, Response{Success: false, Error: "title is required"})
		return
	}

	if err := wfengine.EnsureWorkspace(wf); err != nil {
		writeJSON(w, Response{Success: false, Error: err.Error()})
		return
	}
	content, err := wfengine.MarshalOutlineJSON(normalized)
	if err != nil {
		writeJSON(w, Response{Success: false, Error: err.Error()})
		return
	}
	if err := wfengine.WriteArtifact(wf, "outline.json", content); err != nil {
		writeJSON(w, Response{Success: false, Error: err.Error()})
		return
	}
	writeJSON(w, Response{Success: true, Message: "outline saved", Data: normalized})
}
