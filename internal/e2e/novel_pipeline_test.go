package e2e_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	agentflowiov1alpha1 "agent-flow/api/v1alpha1"
	"agent-flow/internal/ai"
	"agent-flow/internal/config"
	"agent-flow/internal/flow"
	"agent-flow/internal/testutil/mockllm"
	wfengine "agent-flow/internal/workflow"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestTwoChapterNovelPipeline exercises outline expansion, mock LLM worker/monitor, and book merge.
func TestTwoChapterNovelPipeline(t *testing.T) {
	mock := mockllm.New()
	defer mock.Close()

	root := t.TempDir()
	wf := &agentflowiov1alpha1.Workflow{
		ObjectMeta: metav1.ObjectMeta{Name: "e2e-novel-2ch", Namespace: "default"},
		Spec: agentflowiov1alpha1.WorkflowSpec{
			Template: "novel-outline-chapters",
			Prompt:   "写一部两章荒岛生存小说",
			Params: map[string]string{
				"chapterCount":    "2",
				"teamMode":        "false",
				"wordsPerChapter": "800",
			},
		},
		Status: agentflowiov1alpha1.WorkflowStatus{WorkspacePath: root},
	}

	outlineJSON, err := workerChat(mock.URL, "请生成小说大纲 JSON")
	if err != nil {
		t.Fatalf("outline chat: %v", err)
	}
	if !strings.Contains(outlineJSON, "荒岛") {
		t.Fatalf("unexpected outline: %s", outlineJSON)
	}
	if err := os.WriteFile(filepath.Join(root, "outline.json"), []byte(outlineJSON), 0644); err != nil {
		t.Fatal(err)
	}

	steps, err := wfengine.ResolveSteps(wf)
	if err != nil {
		t.Fatalf("ResolveSteps: %v", err)
	}
	if len(steps) < 5 {
		t.Fatalf("expected expanded steps, got %d", len(steps))
	}

	chaptersDir := filepath.Join(root, "chapters")
	if err := os.MkdirAll(chaptersDir, 0755); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 2; i++ {
		chapter, err := workerChat(mock.URL, fmt.Sprintf("撰写第%d章正文", i))
		if err != nil {
			t.Fatalf("chapter %d chat: %v", i, err)
		}
		monitorRaw, err := monitorChat(mock.URL, fmt.Sprintf("质量检查第%d章", i))
		if err != nil {
			t.Fatalf("monitor %d: %v", i, err)
		}
		eval, err := flow.ParseMonitorResult(monitorRaw, wfengine.DefaultQualityThreshold)
		if err != nil || !eval.Passed {
			t.Fatalf("monitor parse chapter %d: eval=%+v err=%v", i, eval, err)
		}
		name := filepath.Join(chaptersDir, fmt.Sprintf("chapter-%02d.md", i))
		if err := os.WriteFile(name, []byte(fmt.Sprintf("# 第%d章\n\n%s", i, chapter)), 0644); err != nil {
			t.Fatal(err)
		}
	}

	book, err := wfengine.MergeChapterFiles(wf)
	if err != nil {
		t.Fatalf("MergeChapterFiles: %v", err)
	}
	bookPath := filepath.Join(root, "book.md")
	if err := os.WriteFile(bookPath, []byte(book), 0644); err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{"荒岛求生记", "第1章", "第2章", "海风呼啸"} {
		if !strings.Contains(book, want) {
			t.Fatalf("book missing %q:\n%s", want, book)
		}
	}
	if mock.Calls() < 5 {
		t.Fatalf("expected multiple LLM calls, got %d", mock.Calls())
	}
}

func workerChat(baseURL, user string) (string, error) {
	cfg := &config.RemoteConfig{
		BaseURL:        baseURL,
		APIKey:         "test",
		Model:          "mock",
		Temperature:    0.7,
		MaxTokens:      2048,
		TimeoutSeconds: 30,
	}
	client := ai.NewRemoteClient(cfg)
	res, err := client.Chat(context.Background(), "你是小说执笔者，只输出最终结果。", user)
	if err != nil {
		return "", err
	}
	return res.Content, nil
}

func monitorChat(baseURL, user string) (string, error) {
	cfg := &config.RemoteConfig{
		BaseURL:        baseURL,
		APIKey:         "test",
		Model:          "mock",
		Temperature:    0.2,
		MaxTokens:      1024,
		TimeoutSeconds: 30,
	}
	client := ai.NewRemoteClient(cfg)
	res, err := client.Chat(context.Background(), "你是质量检查监工，返回 JSON：score, passed, feedback。", user)
	if err != nil {
		return "", err
	}
	return res.Content, nil
}