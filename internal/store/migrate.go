package store

import (
	"database/sql"
	"fmt"
)

const schemaVersion = 2

func migrate(db *sql.DB, driver string) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER NOT NULL)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}
	var version int
	err := db.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&version)
	if err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	if version >= schemaVersion {
		return nil
	}

	migrations := sqliteMigrations
	if driver == "postgres" {
		migrations = postgresMigrations
	}
	for v := version + 1; v <= schemaVersion; v++ {
		stmts, ok := migrations[v]
		if !ok {
			continue
		}
		for _, stmt := range stmts {
			if _, err := db.Exec(stmt); err != nil {
				return fmt.Errorf("migrate v%d: %w", v, err)
			}
		}
		insert := `INSERT INTO schema_migrations(version) VALUES (?)`
		if driver == "pgx" {
			insert = `INSERT INTO schema_migrations(version) VALUES ($1)`
		}
		if _, err := db.Exec(insert, v); err != nil {
			return fmt.Errorf("record schema version %d: %w", v, err)
		}
	}
	return nil
}

var sqliteMigrations = map[int][]string{
	1: {
		`CREATE TABLE IF NOT EXISTS novels (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			namespace TEXT NOT NULL,
			name TEXT NOT NULL,
			title TEXT,
			synopsis TEXT,
			chapter_count INTEGER NOT NULL DEFAULT 0,
			workspace_path TEXT,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			UNIQUE(namespace, name)
		)`,
		`CREATE TABLE IF NOT EXISTS characters (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			novel_id INTEGER NOT NULL REFERENCES novels(id) ON DELETE CASCADE,
			name TEXT NOT NULL,
			role TEXT,
			trait TEXT,
			UNIQUE(novel_id, name)
		)`,
		`CREATE TABLE IF NOT EXISTS chapters (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			novel_id INTEGER NOT NULL REFERENCES novels(id) ON DELETE CASCADE,
			num INTEGER NOT NULL,
			title TEXT,
			summary TEXT,
			status TEXT NOT NULL DEFAULT 'pending',
			word_count INTEGER NOT NULL DEFAULT 0,
			qc_score INTEGER,
			body_path TEXT,
			summary_path TEXT,
			step_id TEXT,
			updated_at TEXT NOT NULL,
			UNIQUE(novel_id, num)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_chapters_novel_status ON chapters(novel_id, status)`,
	},
	2: {
		`ALTER TABLE novels ADD COLUMN prompt_tokens INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE novels ADD COLUMN completion_tokens INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE novels ADD COLUMN total_tokens INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE chapters ADD COLUMN prompt_tokens INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE chapters ADD COLUMN completion_tokens INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE chapters ADD COLUMN total_tokens INTEGER NOT NULL DEFAULT 0`,
	},
}

var postgresMigrations = map[int][]string{
	1: {
		`CREATE TABLE IF NOT EXISTS novels (
			id BIGSERIAL PRIMARY KEY,
			namespace TEXT NOT NULL,
			name TEXT NOT NULL,
			title TEXT,
			synopsis TEXT,
			chapter_count INTEGER NOT NULL DEFAULT 0,
			workspace_path TEXT,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL,
			UNIQUE(namespace, name)
		)`,
		`CREATE TABLE IF NOT EXISTS characters (
			id BIGSERIAL PRIMARY KEY,
			novel_id BIGINT NOT NULL REFERENCES novels(id) ON DELETE CASCADE,
			name TEXT NOT NULL,
			role TEXT,
			trait TEXT,
			UNIQUE(novel_id, name)
		)`,
		`CREATE TABLE IF NOT EXISTS chapters (
			id BIGSERIAL PRIMARY KEY,
			novel_id BIGINT NOT NULL REFERENCES novels(id) ON DELETE CASCADE,
			num INTEGER NOT NULL,
			title TEXT,
			summary TEXT,
			status TEXT NOT NULL DEFAULT 'pending',
			word_count INTEGER NOT NULL DEFAULT 0,
			qc_score INTEGER,
			body_path TEXT,
			summary_path TEXT,
			step_id TEXT,
			updated_at TIMESTAMPTZ NOT NULL,
			UNIQUE(novel_id, num)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_chapters_novel_status ON chapters(novel_id, status)`,
	},
	2: {
		`ALTER TABLE novels ADD COLUMN IF NOT EXISTS prompt_tokens INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE novels ADD COLUMN IF NOT EXISTS completion_tokens INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE novels ADD COLUMN IF NOT EXISTS total_tokens INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE chapters ADD COLUMN IF NOT EXISTS prompt_tokens INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE chapters ADD COLUMN IF NOT EXISTS completion_tokens INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE chapters ADD COLUMN IF NOT EXISTS total_tokens INTEGER NOT NULL DEFAULT 0`,
	},
}