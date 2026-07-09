package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
	"unicode/utf8"

	agentflowiov1alpha1 "agent-flow/api/v1alpha1"
	wfengine "agent-flow/internal/workflow"
)

// SQLStore implements Store with database/sql.
type SQLStore struct {
	db     *sql.DB
	driver string
}

func (s *SQLStore) Enabled() bool { return true }

func (s *SQLStore) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

func (s *SQLStore) Close() error {
	return s.db.Close()
}

func (s *SQLStore) q(sqliteQ, pgQ string) string {
	if s.driver == "pgx" {
		return pgQ
	}
	return sqliteQ
}

func (s *SQLStore) EnsureNovel(ctx context.Context, wf *agentflowiov1alpha1.Workflow) error {
	now := time.Now().UTC()
	chapterCount := wfengine.OutlineChapterCount(wf)
	if chapterCount <= 0 {
		if n := wfengine.IntParam(wf.Spec.Params, "chapterCount", 0); n > 0 {
			chapterCount = n
		}
	}
	_, err := s.db.ExecContext(ctx, s.q(`
		INSERT INTO novels(namespace, name, chapter_count, workspace_path, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(namespace, name) DO UPDATE SET
			chapter_count = excluded.chapter_count,
			workspace_path = excluded.workspace_path,
			updated_at = excluded.updated_at
	`, `
		INSERT INTO novels(namespace, name, chapter_count, workspace_path, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT(namespace, name) DO UPDATE SET
			chapter_count = EXCLUDED.chapter_count,
			workspace_path = EXCLUDED.workspace_path,
			updated_at = EXCLUDED.updated_at
	`), wf.Namespace, wf.Name, chapterCount, wfengine.WorkspacePath(wf), now, now)
	return err
}

func (s *SQLStore) HasOutline(ctx context.Context, wf *agentflowiov1alpha1.Workflow) (bool, error) {
	novelID, err := s.novelID(ctx, wf)
	if err != nil {
		return false, err
	}
	var n int
	err = s.db.QueryRowContext(ctx, s.q(
		`SELECT COUNT(1) FROM chapters WHERE novel_id = ?`,
		`SELECT COUNT(1) FROM chapters WHERE novel_id = $1`,
	), novelID).Scan(&n)
	return n > 0, err
}

func (s *SQLStore) HasNovel(ctx context.Context, namespace, name string) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, s.q(
		`SELECT COUNT(1) FROM novels WHERE namespace = ? AND name = ?`,
		`SELECT COUNT(1) FROM novels WHERE namespace = $1 AND name = $2`,
	), namespace, name).Scan(&n)
	return n > 0, err
}

func (s *SQLStore) novelID(ctx context.Context, wf *agentflowiov1alpha1.Workflow) (int64, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, s.q(
		`SELECT id FROM novels WHERE namespace = ? AND name = ?`,
		`SELECT id FROM novels WHERE namespace = $1 AND name = $2`,
	), wf.Namespace, wf.Name).Scan(&id)
	return id, err
}

func (s *SQLStore) SyncOutline(ctx context.Context, wf *agentflowiov1alpha1.Workflow, outline *wfengine.NovelOutline) error {
	if outline == nil || len(outline.Chapters) == 0 {
		return fmt.Errorf("outline is empty")
	}
	if err := s.EnsureNovel(ctx, wf); err != nil {
		return err
	}
	novelID, err := s.novelID(ctx, wf)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	_, err = s.db.ExecContext(ctx, s.q(
		`UPDATE novels SET title = ?, synopsis = ?, chapter_count = ?, updated_at = ? WHERE id = ?`,
		`UPDATE novels SET title = $1, synopsis = $2, chapter_count = $3, updated_at = $4 WHERE id = $5`,
	), outline.Title, outline.Synopsis, len(outline.Chapters), now, novelID)
	if err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, s.q(
		`DELETE FROM characters WHERE novel_id = ?`,
		`DELETE FROM characters WHERE novel_id = $1`,
	), novelID); err != nil {
		return err
	}
	for _, ch := range outline.Characters {
		name, _ := ch["name"].(string)
		if name == "" {
			continue
		}
		role, _ := ch["role"].(string)
		trait, _ := ch["trait"].(string)
		if _, err := tx.ExecContext(ctx, s.q(
			`INSERT INTO characters(novel_id, name, role, trait) VALUES (?, ?, ?, ?)`,
			`INSERT INTO characters(novel_id, name, role, trait) VALUES ($1, $2, $3, $4)`,
		), novelID, name, role, trait); err != nil {
			return err
		}
	}

	width := wfengine.ChapterPaddingWidth(len(outline.Chapters))
	for _, ch := range outline.Chapters {
		stepID := wfengine.ChapterStepID("chapter", ch.Num, width)
		if _, err := tx.ExecContext(ctx, s.q(`
			INSERT INTO chapters(novel_id, num, title, summary, status, step_id, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(novel_id, num) DO UPDATE SET
				title = excluded.title,
				summary = excluded.summary,
				step_id = excluded.step_id,
				updated_at = excluded.updated_at
		`, `
			INSERT INTO chapters(novel_id, num, title, summary, status, step_id, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT(novel_id, num) DO UPDATE SET
				title = EXCLUDED.title,
				summary = EXCLUDED.summary,
				step_id = EXCLUDED.step_id,
				updated_at = EXCLUDED.updated_at
		`), novelID, ch.Num, ch.Title, ch.Summary, ChapterStatusPending, stepID, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *SQLStore) MarkChapterWriting(ctx context.Context, wf *agentflowiov1alpha1.Workflow, num int, stepID string) error {
	novelID, err := s.novelID(ctx, wf)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	_, err = s.db.ExecContext(ctx, s.q(
		`UPDATE chapters SET status = ?, step_id = ?, updated_at = ? WHERE novel_id = ? AND num = ?`,
		`UPDATE chapters SET status = $1, step_id = $2, updated_at = $3 WHERE novel_id = $4 AND num = $5`,
	), ChapterStatusWriting, stepID, now, novelID, num)
	return err
}

func (s *SQLStore) MarkChapterDone(ctx context.Context, wf *agentflowiov1alpha1.Workflow, num int, bodyPath, summaryPath string, wordCount int, qcScore *int) error {
	novelID, err := s.novelID(ctx, wf)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	_, err = s.db.ExecContext(ctx, s.q(
		`UPDATE chapters SET status = ?, word_count = ?, qc_score = ?, body_path = ?, summary_path = ?, updated_at = ?
		WHERE novel_id = ? AND num = ?`,
		`UPDATE chapters SET status = $1, word_count = $2, qc_score = $3, body_path = $4, summary_path = $5, updated_at = $6
		WHERE novel_id = $7 AND num = $8`,
	), ChapterStatusDone, wordCount, qcScore, bodyPath, summaryPath, now, novelID, num)
	return err
}

func (s *SQLStore) MarkChapterFailed(ctx context.Context, wf *agentflowiov1alpha1.Workflow, num int, stepID string) error {
	novelID, err := s.novelID(ctx, wf)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	_, err = s.db.ExecContext(ctx, s.q(
		`UPDATE chapters SET status = ?, step_id = ?, updated_at = ? WHERE novel_id = ? AND num = ?`,
		`UPDATE chapters SET status = $1, step_id = $2, updated_at = $3 WHERE novel_id = $4 AND num = $5`,
	), ChapterStatusFailed, stepID, now, novelID, num)
	return err
}

func (s *SQLStore) MissingChapterNumbers(ctx context.Context, wf *agentflowiov1alpha1.Workflow) ([]int, error) {
	novelID, err := s.novelID(ctx, wf)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, s.q(
		`SELECT num FROM chapters WHERE novel_id = ? AND status != ? ORDER BY num`,
		`SELECT num FROM chapters WHERE novel_id = $1 AND status != $2 ORDER BY num`,
	), novelID, ChapterStatusDone)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var missing []int
	for rows.Next() {
		var num int
		if err := rows.Scan(&num); err != nil {
			return nil, err
		}
		missing = append(missing, num)
	}
	return missing, rows.Err()
}

// CountRunes returns prose length for persistence.
func CountRunes(content string) int {
	return utf8.RuneCountInString(content)
}

// SyncOutlineFromWorkflow reads outline.json and upserts DB rows.
func SyncOutlineFromWorkflow(ctx context.Context, s Store, wf *agentflowiov1alpha1.Workflow) error {
	if s == nil || !s.Enabled() {
		return nil
	}
	raw, err := wfengine.ReadArtifact(wf, "outline.json")
	if err != nil {
		return err
	}
	outline, err := wfengine.ParseOutlineJSON(raw)
	if err != nil {
		return err
	}
	return s.SyncOutline(ctx, wf, outline)
}
