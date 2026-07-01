package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

const (
	envDBDSN  = "AGENTFLOW_DB_DSN"
	envDBPath = "AGENTFLOW_DB_PATH"
	defaultDB = "/data/outputs/agentflow/novels.db"
)

// OpenFromEnv returns a Store from environment configuration.
// AGENTFLOW_DB_DSN=disable uses a noop store.
// postgres://... opens PostgreSQL; otherwise SQLite (DSN or file path).
func OpenFromEnv() (Store, error) {
	dsn := strings.TrimSpace(os.Getenv(envDBDSN))
	if strings.EqualFold(dsn, "disable") || strings.EqualFold(dsn, "off") {
		return Noop(), nil
	}
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv(envDBPath))
	}
	if dsn == "" {
		dsn = defaultDB
	}
	return Open(dsn)
}

// Open connects to SQLite or PostgreSQL and runs migrations.
func Open(dsn string) (Store, error) {
	driver, conn, err := parseDSN(dsn)
	if err != nil {
		return nil, err
	}
	if driver == "sqlite" {
		if err := os.MkdirAll(filepath.Dir(conn), 0755); err != nil {
			return nil, fmt.Errorf("create db dir: %w", err)
		}
	}
	db, err := sql.Open(driver, conn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(5)
	if err := migrate(db, driver); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &SQLStore{db: db, driver: driver}, nil
}

func parseDSN(dsn string) (driver, conn string, err error) {
	dsn = strings.TrimSpace(dsn)
	switch {
	case strings.HasPrefix(dsn, "postgres://"), strings.HasPrefix(dsn, "postgresql://"):
		return "pgx", dsn, nil
	case strings.HasPrefix(dsn, "sqlite://"):
		path := strings.TrimPrefix(dsn, "sqlite://")
		return "sqlite", path, nil
	default:
		return "sqlite", dsn, nil
	}
}

// PingStore checks connectivity when the store is enabled.
func PingStore(ctx context.Context, s Store) error {
	if s == nil || !s.Enabled() {
		return nil
	}
	return s.Ping(ctx)
}