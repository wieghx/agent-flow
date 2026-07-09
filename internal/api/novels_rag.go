package api

import (
	"net/http"

	"agent-flow/internal/rag"
	wfengine "agent-flow/internal/workflow"

	agentflowiov1alpha1 "agent-flow/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (a *API) handleNovelRAGSearch(w http.ResponseWriter, r *http.Request, namespace, name string) {
	query := r.URL.Query().Get("q")
	if query == "" {
		writeJSON(w, Response{Success: false, Error: "q is required"})
		return
	}
	wf := &agentflowiov1alpha1.Workflow{}
	if err := a.client.Get(r.Context(), client.ObjectKey{Namespace: namespace, Name: name}, wf); err != nil {
		writeJSON(w, Response{Success: false, Error: err.Error()})
		return
	}
	if wf.Status.WorkspacePath == "" {
		wf.Status.WorkspacePath = wfengine.WorkspacePath(wf)
	}
	topK := rag.TopK(wf.Spec.Params)
	chunks, err := rag.SearchAtForParams(wfengine.WorkspacePath(wf), query, topK, wf.Spec.Params)
	if err != nil {
		writeJSON(w, Response{Success: false, Error: err.Error()})
		return
	}
	writeJSON(w, Response{Success: true, Data: map[string]interface{}{
		"query":  query,
		"count":  len(chunks),
		"chunks": chunks,
	}})
}
