// Copyright The Miniflux Authors. Modified by The sss Authors.
// Derived from Miniflux (https://github.com/miniflux/v2).
// SPDX-License-Identifier: Apache-2.0

// Package storage holds every SQL query of the application.
package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"sss/internal/validator"
)

var _ validator.Store = (*Storage)(nil)

// Storage handles all operations related to the database.
type Storage struct {
	db *sql.DB
}

// New returns a Storage backed by db (see database.Open).
func New(db *sql.DB) *Storage {
	return &Storage{db}
}

// DatabaseVersion returns the SQLite library version.
func (s *Storage) DatabaseVersion() string {
	var version string
	if err := s.db.QueryRow(`SELECT sqlite_version()`).Scan(&version); err != nil {
		return err.Error()
	}
	return "SQLite " + version
}

// Ping checks if the database connection works.
func (s *Storage) Ping() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.db.PingContext(ctx)
}

// DBSize returns the size of the database file in a human readable form.
func (s *Storage) DBSize() (string, error) {
	var size int64
	err := s.db.QueryRow(`SELECT page_count * page_size FROM pragma_page_count(), pragma_page_size()`).Scan(&size)
	if err != nil {
		return "", fmt.Errorf("storage: unable to compute database size: %w", err)
	}

	units := []string{"bytes", "kB", "MB", "GB", "TB"}
	value, unit := float64(size), 0
	for value >= 1024 && unit < len(units)-1 {
		value /= 1024
		unit++
	}
	if unit == 0 {
		return fmt.Sprintf("%d bytes", size), nil
	}
	return fmt.Sprintf("%.0f %s", value, units[unit]), nil
}

// unixTime scans an INTEGER unix timestamp into a UTC time.Time (zero for NULL).
type unixTime struct{ t *time.Time }

func (u unixTime) Scan(v any) error {
	switch x := v.(type) {
	case int64:
		*u.t = time.Unix(x, 0).UTC()
	case nil:
		*u.t = time.Time{}
	default:
		return fmt.Errorf("storage: cannot scan %T as unix time", v)
	}
	return nil
}

// nullUnixTime scans a nullable INTEGER unix timestamp into a *time.Time.
type nullUnixTime struct{ t **time.Time }

func (u nullUnixTime) Scan(v any) error {
	if v == nil {
		*u.t = nil
		return nil
	}
	var t time.Time
	if err := (unixTime{&t}).Scan(v); err != nil {
		return err
	}
	*u.t = &t
	return nil
}

// jsonStrings scans a JSON array of strings.
type jsonStrings struct{ s *[]string }

func (j jsonStrings) Scan(v any) error {
	switch x := v.(type) {
	case string:
		return json.Unmarshal([]byte(x), j.s)
	case []byte:
		return json.Unmarshal(x, j.s)
	case nil:
		*j.s = []string{}
		return nil
	}
	return fmt.Errorf("storage: cannot scan %T as JSON array", v)
}

// jsonArray encodes values as a JSON array for json_each() or JSON columns.
func jsonArray[T any](values []T) string {
	if len(values) == 0 {
		return "[]"
	}
	b, _ := json.Marshal(values)
	return string(b)
}

// quoteIdent quotes an SQL identifier.
func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// sortDirection normalizes direction to ASC/DESC, or "" when invalid.
func sortDirection(direction string) string {
	switch {
	case strings.EqualFold(direction, "asc"):
		return "ASC"
	case strings.EqualFold(direction, "desc"):
		return "DESC"
	}
	return ""
}

// rollback wraps err after rolling back tx.
func rollback(tx *sql.Tx, err error) error {
	tx.Rollback()
	return err
}
