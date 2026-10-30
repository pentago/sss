// Copyright The Miniflux Authors. Modified by The sss Authors.
// Derived from Miniflux (https://github.com/miniflux/v2).
// SPDX-License-Identifier: Apache-2.0

// Package database opens the SQLite database and applies schema migrations.
package database

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"modernc.org/sqlite"
)

func init() {
	// SQLite's built-in lower() only folds ASCII; match PostgreSQL's Unicode-aware lower().
	sqlite.MustRegisterDeterministicScalarFunction("lower", 1, func(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
		if s, ok := args[0].(string); ok {
			return strings.ToLower(s), nil
		}
		return args[0], nil
	})
}

// Open opens the SQLite database at dsn (a file path or file: URI) with the
// pragmas sss relies on, creating the parent directory of a plain file path.
// time.Time arguments are stored as unix seconds.
func Open(dsn string) (*sql.DB, error) {
	if !strings.HasPrefix(dsn, "file:") {
		if err := os.MkdirAll(filepath.Dir(dsn), 0o700); err != nil {
			return nil, fmt.Errorf("database: unable to create directory: %w", err)
		}
	}
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	dsn += sep + "_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)" +
		"&_pragma=synchronous(NORMAL)&_txlock=immediate&_time_integer_format=unix"

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("database: unable to open: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("database: unable to connect: %w", err)
	}
	return db, nil
}

// Migrate applies pending migrations, each in its own transaction.
func Migrate(db *sql.DB) error {
	current, err := version(db)
	if err != nil {
		return err
	}

	slog.Info("Running database migrations",
		slog.Int("current_version", current),
		slog.Int("latest_version", len(migrations)),
	)

	for v := current; v < len(migrations); v++ {
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("database: migration v%d: %w", v+1, err)
		}
		if _, err := tx.Exec(migrations[v]); err != nil {
			tx.Rollback()
			return fmt.Errorf("database: migration v%d: %w", v+1, err)
		}
		// PRAGMA does not accept bound parameters.
		if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", v+1)); err != nil {
			tx.Rollback()
			return fmt.Errorf("database: migration v%d: %w", v+1, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("database: migration v%d: %w", v+1, err)
		}
	}
	return nil
}

// IsSchemaUpToDate reports whether every migration has been applied.
func IsSchemaUpToDate(db *sql.DB) (bool, error) {
	current, err := version(db)
	if err != nil {
		return false, err
	}
	return current >= len(migrations), nil
}

func version(db *sql.DB) (int, error) {
	var v int
	if err := db.QueryRow("PRAGMA user_version").Scan(&v); err != nil {
		return 0, fmt.Errorf("database: unable to read schema version: %w", err)
	}
	return v, nil
}
