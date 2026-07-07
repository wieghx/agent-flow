package store

import (
	"context"
	"time"

	agentflowiov1alpha1 "agent-flow/api/v1alpha1"
	"agent-flow/internal/ai"
	wfengine "agent-flow/internal/workflow"
)

// ChapterEntry is one chapter row for the library API.
type ChapterEntry struct {
	Num              int    `json:"num"`
	Title            string `json:"title"`
	Summary          string `json:"summary,omitempty"`
	Status           string `json:"status"`
	WordCount        int    `json:"word_count"`
	PromptTokens     int    `json:"prompt_tokens"`
	CompletionTokens int    `json:"completion_tokens"`
	TotalTokens      int    `json:"total_tokens"`
}

// RecordTaskTokens accumulates token usage onto the novel and optional chapter row.
func (s *SQLStore) RecordTaskTokens(ctx context.Context, wf *agentflowiov1alpha1.Workflow, stepID string, usage ai.TokenUsage) error {
	if usage.PromptTokens == 0 && usage.CompletionTokens == 0 && usage.TotalTokens == 0 {
		return nil
	}
	if err := s.EnsureNovel(ctx, wf); err != nil {
		return err
	}
	novelID, err := s.novelID(ctx, wf)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	if _, err := s.db.ExecContext(ctx, s.q(`
		UPDATE novels SET
			prompt_tokens = prompt_tokens + ?,
			completion_tokens = completion_tokens + ?,
			total_tokens = total_tokens + ?,
			updated_at = ?
		WHERE id = ?
	`, `
		UPDATE novels SET
			prompt_tokens = prompt_tokens + $1,
			completion_tokens = completion_tokens + $2,
			total_tokens = total_tokens + $3,
			updated_at = $4
		WHERE id = $5
	`), usage.PromptTokens, usage.CompletionTokens, usage.TotalTokens, now, novelID); err != nil {
		return err
	}

	chapterNum, ok := chapterNumFromStepID(stepID)
	if !ok {
		return nil
	}
	_, err = s.db.ExecContext(ctx, s.q(`
		UPDATE chapters SET
			prompt_tokens = prompt_tokens + ?,
			completion_tokens = completion_tokens + ?,
			total_tokens = total_tokens + ?,
			updated_at = ?
		WHERE novel_id = ? AND num = ?
	`, `
		UPDATE chapters SET
			prompt_tokens = prompt_tokens + $1,
			completion_tokens = completion_tokens + $2,
			total_tokens = total_tokens + $3,
			updated_at = $4
		WHERE novel_id = $5 AND num = $6
	`), usage.PromptTokens, usage.CompletionTokens, usage.TotalTokens, now, novelID, chapterNum)
	return err
}

func chapterNumFromStepID(stepID string) (int, bool) {
	if num, ok := wfengine.ChapterNumFromStepID(stepID); ok {
		return num, true
	}
	return wfengine.PlotNumFromStepID(stepID)
}

// ListChapters returns chapter rows including token stats.
func (s *SQLStore) ListChapters(ctx context.Context, namespace, name string) ([]ChapterEntry, error) {
	novelID, err := s.novelIDByName(ctx, namespace, name)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, s.q(`
		SELECT num, COALESCE(title,''), COALESCE(summary,''), status, word_count,
			prompt_tokens, completion_tokens, total_tokens
		FROM chapters WHERE novel_id = ? ORDER BY num
	`, `
		SELECT num, COALESCE(title,''), COALESCE(summary,''), status, word_count,
			prompt_tokens, completion_tokens, total_tokens
		FROM chapters WHERE novel_id = $1 ORDER BY num
	`), novelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ChapterEntry
	for rows.Next() {
		var e ChapterEntry
		if err := rows.Scan(
			&e.Num, &e.Title, &e.Summary, &e.Status, &e.WordCount,
			&e.PromptTokens, &e.CompletionTokens, &e.TotalTokens,
		); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *SQLStore) novelIDByName(ctx context.Context, namespace, name string) (int64, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, s.q(
		`SELECT id FROM novels WHERE namespace = ? AND name = ?`,
		`SELECT id FROM novels WHERE namespace = $1 AND name = $2`,
	), namespace, name).Scan(&id)
	return id, err
}

// RecordTaskTokensFromStore records usage when supported.
func RecordTaskTokensFromStore(ctx context.Context, st Store, wf *agentflowiov1alpha1.Workflow, stepID string, usage ai.TokenUsage) error {
	if ls, ok := st.(interface {
		RecordTaskTokens(context.Context, *agentflowiov1alpha1.Workflow, string, ai.TokenUsage) error
	}); ok && st.Enabled() {
		return ls.RecordTaskTokens(ctx, wf, stepID, usage)
	}
	return nil
}

// ListChaptersFromStore returns chapter rows when supported.
func ListChaptersFromStore(ctx context.Context, st Store, namespace, name string) ([]ChapterEntry, error) {
	if ls, ok := st.(interface {
		ListChapters(context.Context, string, string) ([]ChapterEntry, error)
	}); ok && st.Enabled() {
		return ls.ListChapters(ctx, namespace, name)
	}
	return nil, nil
}