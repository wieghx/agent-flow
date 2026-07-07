package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// LibraryEntry is novel metadata for the library API.
type LibraryEntry struct {
	Namespace        string    `json:"namespace"`
	Name             string    `json:"name"`
	Title            string    `json:"title"`
	Synopsis         string    `json:"synopsis"`
	ChapterCount     int       `json:"chapter_count"`
	DoneChapters     int       `json:"chapters_done"`
	WritingChapters  int       `json:"chapters_writing"`
	FailedChapters   int       `json:"chapters_failed"`
	PromptTokens     int       `json:"prompt_tokens"`
	CompletionTokens int       `json:"completion_tokens"`
	TotalTokens      int       `json:"total_tokens"`
	WorkspacePath    string    `json:"workspace_path"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// ListLibrary returns all novel rows with chapter stats.
func (s *SQLStore) ListLibrary(ctx context.Context) ([]LibraryEntry, error) {
	rows, err := s.db.QueryContext(ctx, s.q(`
		SELECT n.namespace, n.name, COALESCE(n.title,''), COALESCE(n.synopsis,''),
			n.chapter_count, COALESCE(n.workspace_path,''), n.created_at, n.updated_at,
			COALESCE(n.prompt_tokens, 0), COALESCE(n.completion_tokens, 0), COALESCE(n.total_tokens, 0),
			COALESCE(SUM(CASE WHEN c.status = 'done' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN c.status = 'writing' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN c.status = 'failed' THEN 1 ELSE 0 END), 0)
		FROM novels n
		LEFT JOIN chapters c ON c.novel_id = n.id
		GROUP BY n.id
		ORDER BY n.updated_at DESC
	`, `
		SELECT n.namespace, n.name, COALESCE(n.title,''), COALESCE(n.synopsis,''),
			n.chapter_count, COALESCE(n.workspace_path,''), n.created_at, n.updated_at,
			COALESCE(n.prompt_tokens, 0), COALESCE(n.completion_tokens, 0), COALESCE(n.total_tokens, 0),
			COALESCE(SUM(CASE WHEN c.status = 'done' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN c.status = 'writing' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN c.status = 'failed' THEN 1 ELSE 0 END), 0)
		FROM novels n
		LEFT JOIN chapters c ON c.novel_id = n.id
		GROUP BY n.id, n.namespace, n.name, n.title, n.synopsis, n.chapter_count, n.workspace_path,
			n.created_at, n.updated_at, n.prompt_tokens, n.completion_tokens, n.total_tokens
		ORDER BY n.updated_at DESC
	`))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanLibraryRows(rows)
}

// GetLibrary returns one novel row with chapter stats.
func (s *SQLStore) GetLibrary(ctx context.Context, namespace, name string) (*LibraryEntry, error) {
	row := s.db.QueryRowContext(ctx, s.q(`
		SELECT n.namespace, n.name, COALESCE(n.title,''), COALESCE(n.synopsis,''),
			n.chapter_count, COALESCE(n.workspace_path,''), n.created_at, n.updated_at,
			COALESCE(n.prompt_tokens, 0), COALESCE(n.completion_tokens, 0), COALESCE(n.total_tokens, 0),
			COALESCE(SUM(CASE WHEN c.status = 'done' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN c.status = 'writing' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN c.status = 'failed' THEN 1 ELSE 0 END), 0)
		FROM novels n
		LEFT JOIN chapters c ON c.novel_id = n.id
		WHERE n.namespace = ? AND n.name = ?
		GROUP BY n.id
	`, `
		SELECT n.namespace, n.name, COALESCE(n.title,''), COALESCE(n.synopsis,''),
			n.chapter_count, COALESCE(n.workspace_path,''), n.created_at, n.updated_at,
			COALESCE(n.prompt_tokens, 0), COALESCE(n.completion_tokens, 0), COALESCE(n.total_tokens, 0),
			COALESCE(SUM(CASE WHEN c.status = 'done' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN c.status = 'writing' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN c.status = 'failed' THEN 1 ELSE 0 END), 0)
		FROM novels n
		LEFT JOIN chapters c ON c.novel_id = n.id
		WHERE n.namespace = $1 AND n.name = $2
		GROUP BY n.id, n.namespace, n.name, n.title, n.synopsis, n.chapter_count, n.workspace_path,
			n.created_at, n.updated_at, n.prompt_tokens, n.completion_tokens, n.total_tokens
	`), namespace, name)
	entry, err := scanLibraryRow(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return entry, nil
}

// DeleteNovelRecord removes SQLite metadata for a novel (cascades chapters/characters).
func (s *SQLStore) DeleteNovelRecord(ctx context.Context, namespace, name string) error {
	_, err := s.db.ExecContext(ctx, s.q(
		`DELETE FROM novels WHERE namespace = ? AND name = ?`,
		`DELETE FROM novels WHERE namespace = $1 AND name = $2`,
	), namespace, name)
	return err
}

func scanLibraryRows(rows *sql.Rows) ([]LibraryEntry, error) {
	var out []LibraryEntry
	for rows.Next() {
		entry, err := scanLibraryRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *entry)
	}
	return out, rows.Err()
}

type libraryScanner interface {
	Scan(dest ...any) error
}

func scanLibraryRow(row libraryScanner) (*LibraryEntry, error) {
	var e LibraryEntry
	var created, updated string
	err := row.Scan(
		&e.Namespace, &e.Name, &e.Title, &e.Synopsis,
		&e.ChapterCount, &e.WorkspacePath, &created, &updated,
		&e.PromptTokens, &e.CompletionTokens, &e.TotalTokens,
		&e.DoneChapters, &e.WritingChapters, &e.FailedChapters,
	)
	if err != nil {
		return nil, err
	}
	e.CreatedAt = parseDBTime(created)
	e.UpdatedAt = parseDBTime(updated)
	return &e, nil
}

func parseDBTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02T15:04:05Z"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// ListLibraryFromStore returns library entries when the store supports it.
func ListLibraryFromStore(ctx context.Context, s Store) ([]LibraryEntry, error) {
	if ls, ok := s.(interface {
		ListLibrary(context.Context) ([]LibraryEntry, error)
	}); ok && s.Enabled() {
		return ls.ListLibrary(ctx)
	}
	return nil, nil
}

// GetLibraryFromStore returns one library entry when supported.
func GetLibraryFromStore(ctx context.Context, s Store, namespace, name string) (*LibraryEntry, error) {
	if ls, ok := s.(interface {
		GetLibrary(context.Context, string, string) (*LibraryEntry, error)
	}); ok && s.Enabled() {
		return ls.GetLibrary(ctx, namespace, name)
	}
	return nil, nil
}

// DeleteNovelRecordFromStore removes DB metadata when supported.
func DeleteNovelRecordFromStore(ctx context.Context, s Store, namespace, name string) error {
	if ls, ok := s.(interface {
		DeleteNovelRecord(context.Context, string, string) error
	}); ok && s.Enabled() {
		return ls.DeleteNovelRecord(ctx, namespace, name)
	}
	return nil
}

// FormatLibraryTime formats library timestamps for API responses.
func FormatLibraryTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format("2006-01-02 15:04:05")
}

// ErrLibraryNotFound indicates the novel workflow does not exist.
var ErrLibraryNotFound = fmt.Errorf("novel not found")