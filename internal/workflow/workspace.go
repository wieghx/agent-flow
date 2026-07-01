package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	agentflowiov1alpha1 "agent-flow/api/v1alpha1"
)

const defaultWorkspaceRoot = "/data/outputs/workflows"

// WorkspacePath returns the on-disk workspace for a workflow.
func WorkspacePath(wf *agentflowiov1alpha1.Workflow) string {
	if wf.Status.WorkspacePath != "" {
		return wf.Status.WorkspacePath
	}
	base := defaultWorkspaceRoot
	if wf.Spec.Workspace.BasePath != "" {
		base = wf.Spec.Workspace.BasePath
	}
	return filepath.Join(base, wf.Namespace, wf.Name)
}

// EnsureWorkspace creates workflow workspace directories.
func EnsureWorkspace(wf *agentflowiov1alpha1.Workflow) error {
	root := WorkspacePath(wf)
	for _, sub := range []string{"chapters", "volumes", "arcs", "imports", "imports/chapters", "rag"} {
		if err := os.MkdirAll(filepath.Join(root, sub), 0755); err != nil {
			return err
		}
	}
	return os.MkdirAll(root, 0755)
}

// WriteArtifact stores step output into workspace.
func WriteArtifact(wf *agentflowiov1alpha1.Workflow, relativePath, content string) error {
	if relativePath == "" {
		return fmt.Errorf("output path is empty")
	}
	target := filepath.Join(WorkspacePath(wf), filepath.Clean(relativePath))
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}
	return os.WriteFile(target, []byte(content), 0644)
}

// ReadArtifact loads a workspace file.
func ReadArtifact(wf *agentflowiov1alpha1.Workflow, relativePath string) (string, error) {
	target := filepath.Join(WorkspacePath(wf), filepath.Clean(relativePath))
	data, err := os.ReadFile(target)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// MergeChapterFiles concatenates chapter markdown files.
func MergeChapterFiles(wf *agentflowiov1alpha1.Workflow) (string, error) {
	dir := filepath.Join(WorkspacePath(wf), "chapters")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), "chapter-") || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		files = append(files, e.Name())
	}
	SortChapterFiles(files)
	if len(files) == 0 {
		return "", fmt.Errorf("no chapter files found")
	}

	outlineRaw, _ := ReadArtifact(wf, "outline.json")
	outline, _ := ParseOutlineJSON(outlineRaw)

	var b strings.Builder
	if outline != nil {
		fmt.Fprintf(&b, "# %s\n\n%s\n\n## 目录\n\n", outline.Title, outline.Synopsis)
		for _, ch := range outline.Chapters {
			fmt.Fprintf(&b, "- 第%d章 %s\n", ch.Num, ch.Title)
		}
		b.WriteString("\n")
	}

	for _, name := range files {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return "", err
		}
		b.WriteString("\n\n")
		b.Write(data)
	}
	return b.String(), nil
}
