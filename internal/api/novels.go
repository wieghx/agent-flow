package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	agentflowiov1alpha1 "agent-flow/api/v1alpha1"
	"agent-flow/internal/flow"
	"agent-flow/internal/store"
	wfengine "agent-flow/internal/workflow"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NovelSummary is a merged view of Workflow CRD + SQLite metadata.
type NovelSummary struct {
	Namespace       string            `json:"namespace"`
	Name            string            `json:"name"`
	Title           string            `json:"title"`
	Synopsis        string            `json:"synopsis,omitempty"`
	Phase           string            `json:"phase"`
	Progress        int32             `json:"progress"`
	CurrentStep     string            `json:"currentStep,omitempty"`
	Message         string            `json:"message,omitempty"`
	ChapterCount    int               `json:"chapter_count"`
	ChaptersDone    int               `json:"chapters_done"`
	ChaptersWriting int               `json:"chapters_writing"`
	ChaptersFailed  int               `json:"chapters_failed"`
	PlotsDone       int               `json:"plots_done"`
	PlotsWriting    int               `json:"plots_writing"`
	PlotsFailed     int               `json:"plots_failed"`
	PipelineStage   string            `json:"pipeline_stage,omitempty"`
	Prompt          string            `json:"prompt,omitempty"`
	Template        string            `json:"template,omitempty"`
	Params          map[string]string `json:"params,omitempty"`
	WorkspacePath   string            `json:"workspace_path,omitempty"`
	BookURL         string            `json:"book_url,omitempty"`
	OutlineURL      string            `json:"outline_url,omitempty"`
	CreatedAt       string            `json:"created_at"`
	UpdatedAt       string            `json:"updated_at,omitempty"`
	CompletionAt    string            `json:"completion_at,omitempty"`
}

// ImportNovelRequest imports an existing novel text and runs 拆书 + optional续写.
type ImportNovelRequest struct {
	Name             string            `json:"name"`
	Namespace        string            `json:"namespace"`
	Title            string            `json:"title"`
	Prompt           string            `json:"prompt"`
	Text             string            `json:"text"`
	ContinueWriting  bool              `json:"continue_writing"`
	Params           map[string]string `json:"params"`
}

// CreateNovelRequest creates a novel workflow from the library UI.
type CreateNovelRequest struct {
	Name            string            `json:"name"`
	Namespace       string            `json:"namespace"`
	Title           string            `json:"title"`
	Prompt          string            `json:"prompt"`
	ChapterCount    int               `json:"chapter_count"`
	WordsPerChapter int               `json:"words_per_chapter"`
	QualityThreshold int              `json:"quality_threshold"`
	Params          map[string]string `json:"params"`
}

func (a *API) handleNovelList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	summaries, err := a.listNovelSummaries(r.Context())
	if err != nil {
		writeJSON(w, Response{Success: false, Error: err.Error()})
		return
	}
	writeJSON(w, Response{
		Success: true,
		Data: map[string]interface{}{
			"count":  len(summaries),
			"novels": summaries,
		},
	})
}

func (a *API) handleNovelRoutes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/novels/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 2 {
		http.Error(w, "path must be /novels/{namespace}/{name}[/resume]", http.StatusBadRequest)
		return
	}
	namespace, name := parts[0], parts[1]
	action := ""
	if len(parts) >= 3 {
		action = parts[2]
	}

	switch {
	case action == "rag" && len(parts) >= 4 && parts[3] == "search" && r.Method == http.MethodGet:
		a.handleNovelRAGSearch(w, r, namespace, name)
	case action == "resume" && r.Method == http.MethodPost:
		a.handleNovelResume(w, r, namespace, name)
	case action == "" && r.Method == http.MethodGet:
		a.handleNovelGet(w, r, namespace, name)
	case action == "" && r.Method == http.MethodDelete:
		a.handleNovelDelete(w, r, namespace, name)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a *API) handleNovelGet(w http.ResponseWriter, r *http.Request, namespace, name string) {
	summary, err := a.getNovelSummary(r.Context(), namespace, name)
	if err != nil {
		writeJSON(w, Response{Success: false, Error: err.Error()})
		return
	}
	if summary == nil {
		writeJSON(w, Response{Success: false, Error: "novel not found"})
		return
	}
	writeJSON(w, Response{Success: true, Data: summary})
}

func (a *API) handleCreateNovel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req CreateNovelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, Response{Success: false, Error: err.Error()})
		return
	}
	namespace := req.Namespace
	if namespace == "" {
		namespace = "default"
	}
	chapters := req.ChapterCount
	if chapters <= 0 {
		chapters = 20
	}
	params := wfengine.DefaultNovelParams(chapters)
	if req.WordsPerChapter > 0 {
		params["wordsPerChapter"] = fmt.Sprintf("%d", req.WordsPerChapter)
	}
	if req.QualityThreshold > 0 {
		params["qualityThreshold"] = fmt.Sprintf("%d", req.QualityThreshold)
	}
	params = wfengine.MergeParams(params, req.Params)

	prompt := strings.TrimSpace(req.Prompt)
	if title := strings.TrimSpace(req.Title); title != "" && !strings.Contains(prompt, title) {
		if prompt != "" {
			prompt = fmt.Sprintf("书名：%s\n\n%s", title, prompt)
		} else {
			prompt = fmt.Sprintf("书名：%s", title)
		}
	}
	if prompt == "" {
		prompt = fmt.Sprintf("写一部长篇小说，共 %d 章。", chapters)
	}

	wfName := strings.TrimSpace(req.Name)
	if wfName == "" {
		wfName = flow.ProposeWorkflowName(prompt)
	}

	template := wfengine.DefaultNovelTemplate(params, prompt)
	wf, err := flow.NewWorkflowCRD(template, prompt, params, wfName, namespace)
	if err != nil {
		writeJSON(w, Response{Success: false, Error: err.Error()})
		return
	}
	if err := a.client.Create(r.Context(), wf); err != nil {
		writeJSON(w, Response{Success: false, Error: err.Error()})
		return
	}
	summary := a.buildSummaryFromWorkflow(wf, nil)
	writeJSON(w, Response{Success: true, Message: "Novel workflow created", Data: summary})
}

func (a *API) handleImportNovel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req ImportNovelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, Response{Success: false, Error: err.Error()})
		return
	}
	text := strings.TrimSpace(req.Text)
	if text == "" {
		writeJSON(w, Response{Success: false, Error: "text is required"})
		return
	}
	namespace := req.Namespace
	if namespace == "" {
		namespace = "default"
	}
	chapters, err := wfengine.ParseImportedNovel(text)
	if err != nil {
		writeJSON(w, Response{Success: false, Error: err.Error()})
		return
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = "导入小说"
	}
	params := wfengine.ImportNovelParams(len(chapters), title)
	if !req.ContinueWriting {
		params["continueWriting"] = "false"
	}
	params = wfengine.MergeParams(params, req.Params)

	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		prompt = fmt.Sprintf("导入并拆书：《%s》，共 %d 章", title, len(chapters))
	}
	wfName := strings.TrimSpace(req.Name)
	if wfName == "" {
		wfName = flow.ProposeWorkflowName(title)
	}

	wf, err := flow.NewWorkflowCRD("novel-import-deconstruct", prompt, params, wfName, namespace)
	if err != nil {
		writeJSON(w, Response{Success: false, Error: err.Error()})
		return
	}
	if err := a.client.Create(r.Context(), wf); err != nil {
		writeJSON(w, Response{Success: false, Error: err.Error()})
		return
	}
	if err := wfengine.EnsureWorkspace(wf); err != nil {
		writeJSON(w, Response{Success: false, Error: err.Error()})
		return
	}
	if _, err := wfengine.WriteImportedNovel(wf, text, title); err != nil {
		writeJSON(w, Response{Success: false, Error: err.Error()})
		return
	}
	summary := a.buildSummaryFromWorkflow(wf, nil)
	writeJSON(w, Response{Success: true, Message: "Import workflow created", Data: summary})
}

func (a *API) handleNovelResume(w http.ResponseWriter, r *http.Request, namespace, name string) {
	wf := &agentflowiov1alpha1.Workflow{}
	if err := a.client.Get(r.Context(), client.ObjectKey{Namespace: namespace, Name: name}, wf); err != nil {
		writeJSON(w, Response{Success: false, Error: err.Error()})
		return
	}

	taskList := &agentflowiov1alpha1.TaskList{}
	if err := a.client.List(r.Context(), taskList,
		client.InNamespace(namespace),
		client.MatchingLabels{"agentflow.io/workflow": name},
	); err == nil {
		for i := range taskList.Items {
			task := &taskList.Items[i]
			if task.Status.Phase == agentflowiov1alpha1.TaskPhaseFailed {
				_ = a.client.Delete(r.Context(), task)
			}
		}
	}

	wf.Status.Phase = agentflowiov1alpha1.WorkflowPhaseRunning
	wf.Status.Message = "resumed via library API"
	wf.Status.CompletionTime = nil
	if err := a.client.Status().Update(r.Context(), wf); err != nil {
		writeJSON(w, Response{Success: false, Error: err.Error()})
		return
	}
	summary, _ := a.getNovelSummary(r.Context(), namespace, name)
	writeJSON(w, Response{Success: true, Message: "Novel resumed", Data: summary})
}

func (a *API) handleNovelDelete(w http.ResponseWriter, r *http.Request, namespace, name string) {
	wf := &agentflowiov1alpha1.Workflow{}
	err := a.client.Get(r.Context(), client.ObjectKey{Namespace: namespace, Name: name}, wf)
	if err != nil && !k8serrors.IsNotFound(err) {
		writeJSON(w, Response{Success: false, Error: err.Error()})
		return
	}

	taskList := &agentflowiov1alpha1.TaskList{}
	if err := a.client.List(r.Context(), taskList,
		client.InNamespace(namespace),
		client.MatchingLabels{"agentflow.io/workflow": name},
	); err == nil {
		for i := range taskList.Items {
			_ = a.client.Delete(r.Context(), &taskList.Items[i])
		}
	}

	if err == nil {
		if err := a.client.Delete(r.Context(), wf); err != nil && !k8serrors.IsNotFound(err) {
			writeJSON(w, Response{Success: false, Error: err.Error()})
			return
		}
	}

	_ = store.DeleteNovelRecordFromStore(r.Context(), a.novelStore, namespace, name)
	writeJSON(w, Response{Success: true, Message: "Novel deleted (chapter files on PVC retained)"})
}

func (a *API) listNovelSummaries(ctx context.Context) ([]NovelSummary, error) {
	wfList := &agentflowiov1alpha1.WorkflowList{}
	if err := a.client.List(ctx, wfList); err != nil {
		return nil, err
	}

	dbMap := map[string]store.LibraryEntry{}
	if entries, err := store.ListLibraryFromStore(ctx, a.novelStore); err == nil {
		for _, e := range entries {
			dbMap[e.Namespace+"/"+e.Name] = e
		}
	}

	seen := map[string]bool{}
	var out []NovelSummary
	for i := range wfList.Items {
		wf := &wfList.Items[i]
		if wf.Spec.Template != "" && wf.Spec.Template != "novel-outline-chapters" && wf.Spec.Template != "novel-team-chapters" && wf.Spec.Template != "novel-team-historical" {
			continue
		}
		key := wf.Namespace + "/" + wf.Name
		seen[key] = true
		var lib *store.LibraryEntry
		if e, ok := dbMap[key]; ok {
			copy := e
			lib = &copy
		}
		out = append(out, a.buildSummaryFromWorkflow(wf, lib))
	}

	for key, e := range dbMap {
		if seen[key] {
			continue
		}
		copy := e
		out = append(out, a.buildSummaryFromDB(&copy))
	}
	return out, nil
}

func (a *API) getNovelSummary(ctx context.Context, namespace, name string) (*NovelSummary, error) {
	wf := &agentflowiov1alpha1.Workflow{}
	err := a.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, wf)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			lib, _ := store.GetLibraryFromStore(ctx, a.novelStore, namespace, name)
			if lib == nil {
				return nil, nil
			}
			s := a.buildSummaryFromDB(lib)
			return &s, nil
		}
		return nil, err
	}
	lib, _ := store.GetLibraryFromStore(ctx, a.novelStore, namespace, name)
	s := a.buildSummaryFromWorkflow(wf, lib)
	return &s, nil
}

func (a *API) buildSummaryFromWorkflow(wf *agentflowiov1alpha1.Workflow, lib *store.LibraryEntry) NovelSummary {
	s := NovelSummary{
		Namespace:    wf.Namespace,
		Name:         wf.Name,
		Phase:        string(wf.Status.Phase),
		Progress:     wf.Status.Progress.Percent,
		CurrentStep:  wf.Status.CurrentStep,
		Message:      wf.Status.Message,
		Prompt:       wf.Spec.Prompt,
		Template:     wf.Spec.Template,
		Params:       wf.Spec.Params,
		WorkspacePath: wf.Status.WorkspacePath,
		CreatedAt:    wf.CreationTimestamp.Time.Format("2006-01-02 15:04:05"),
	}
	if wf.Status.CompletionTime != nil {
		s.CompletionAt = wf.Status.CompletionTime.Time.Format("2006-01-02 15:04:05")
	}
	if lib != nil {
		a.applyLibraryEntry(&s, lib)
	} else {
		s.ChapterCount = wfengine.OutlineChapterCount(wf)
		if s.ChapterCount <= 0 {
			s.ChapterCount = wfengine.IntParam(wf.Spec.Params, "chapterCount", 0)
		}
		a.enrichTitleFromOutline(wf, &s)
	}
	if s.WorkspacePath == "" {
		s.WorkspacePath = wfengine.WorkspacePath(wf)
	}
	s.BookURL, s.OutlineURL = artifactURLs(s.WorkspacePath)
	enrichNovelProgress(wf, &s)
	return s
}

func (a *API) buildSummaryFromDB(lib *store.LibraryEntry) NovelSummary {
	s := NovelSummary{
		Namespace:       lib.Namespace,
		Name:            lib.Name,
		Title:           lib.Title,
		Synopsis:        lib.Synopsis,
		Phase:           "Unknown",
		ChapterCount:    lib.ChapterCount,
		ChaptersDone:    lib.DoneChapters,
		ChaptersWriting: lib.WritingChapters,
		ChaptersFailed:  lib.FailedChapters,
		WorkspacePath:   lib.WorkspacePath,
		CreatedAt:       store.FormatLibraryTime(lib.CreatedAt),
		UpdatedAt:       store.FormatLibraryTime(lib.UpdatedAt),
	}
	if lib.DoneChapters > 0 && lib.ChapterCount > 0 {
		s.Progress = int32(lib.DoneChapters * 100 / lib.ChapterCount)
	}
	s.BookURL, s.OutlineURL = artifactURLs(s.WorkspacePath)
	return s
}

func (a *API) applyLibraryEntry(s *NovelSummary, lib *store.LibraryEntry) {
	if lib.Title != "" {
		s.Title = lib.Title
	}
	if lib.Synopsis != "" {
		s.Synopsis = lib.Synopsis
	}
	if lib.ChapterCount > 0 {
		s.ChapterCount = lib.ChapterCount
	}
	s.ChaptersDone = lib.DoneChapters
	s.ChaptersWriting = lib.WritingChapters
	s.ChaptersFailed = lib.FailedChapters
	s.UpdatedAt = store.FormatLibraryTime(lib.UpdatedAt)
	if s.WorkspacePath == "" {
		s.WorkspacePath = lib.WorkspacePath
	}
}

func (a *API) enrichTitleFromOutline(wf *agentflowiov1alpha1.Workflow, s *NovelSummary) {
	if s.Title != "" {
		return
	}
	raw, err := wfengine.ReadArtifact(wf, "outline.json")
	if err != nil {
		return
	}
	outline, err := wfengine.ParseOutlineJSON(raw)
	if err != nil {
		return
	}
	s.Title = outline.Title
	s.Synopsis = outline.Synopsis
	if len(outline.Chapters) > 0 {
		s.ChapterCount = len(outline.Chapters)
	}
}

func artifactURLs(workspace string) (bookURL, outlineURL string) {
	if workspace == "" {
		return "", ""
	}
	const prefix = "/data/outputs"
	if !strings.HasPrefix(workspace, prefix) {
		return "", ""
	}
	rel := strings.TrimPrefix(workspace, prefix)
	bookURL = "/outputs" + rel + "/book.md"
	outlineURL = "/outputs" + rel + "/outline.json"
	return bookURL, outlineURL
}

func writeJSON(w http.ResponseWriter, resp Response) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}