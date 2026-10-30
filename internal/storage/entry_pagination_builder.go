// Copyright The Miniflux Authors. Modified by The sss Authors.
// Derived from Miniflux (https://github.com/miniflux/v2).
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"sss/internal/model"
)

// EntryPaginationBuilder finds the previous and next entries of an entry.
type EntryPaginationBuilder struct {
	db         *sql.DB
	conditions []string
	args       []any
	entryID    int64
	order      string
	direction  string
}

// NewEntryPaginationBuilder returns a new EntryPaginationBuilder for the
// entry, ordered by the given entries column and direction.
func (s *Storage) NewEntryPaginationBuilder(userID, entryID int64, order, direction string) *EntryPaginationBuilder {
	return &EntryPaginationBuilder{
		db:         s.db,
		args:       []any{userID},
		conditions: []string{"e.user_id = $1"},
		entryID:    entryID,
		order:      quoteIdent(order),
		direction:  direction,
	}
}

func (e *EntryPaginationBuilder) nextArg(value any) string {
	e.args = append(e.args, value)
	return "$" + strconv.Itoa(len(e.args))
}

// WithSearchQuery adds full-text search query to the condition.
func (e *EntryPaginationBuilder) WithSearchQuery(query string) *EntryPaginationBuilder {
	if match := ftsQuery(query); match != "" {
		e.conditions = append(e.conditions, "e.id IN (SELECT rowid FROM entries_fts WHERE entries_fts MATCH "+e.nextArg(match)+")")
	}
	return e
}

// WithStarred adds starred to the condition.
func (e *EntryPaginationBuilder) WithStarred() *EntryPaginationBuilder {
	e.conditions = append(e.conditions, "e.starred = 1")
	return e
}

// WithFeedID adds feed_id to the condition.
func (e *EntryPaginationBuilder) WithFeedID(feedID int64) *EntryPaginationBuilder {
	if feedID != 0 {
		e.conditions = append(e.conditions, "e.feed_id = "+e.nextArg(feedID))
	}
	return e
}

// WithCategoryID adds category_id to the condition.
func (e *EntryPaginationBuilder) WithCategoryID(categoryID int64) *EntryPaginationBuilder {
	if categoryID != 0 {
		e.conditions = append(e.conditions, "f.category_id = "+e.nextArg(categoryID))
	}
	return e
}

// WithStatus adds status to the condition.
func (e *EntryPaginationBuilder) WithStatus(status string) *EntryPaginationBuilder {
	if status != "" {
		e.conditions = append(e.conditions, "e.status = "+e.nextArg(status))
	}
	return e
}

// WithStatusOrEntryID adds a status condition that always includes a specific entry ID.
func (e *EntryPaginationBuilder) WithStatusOrEntryID(status string, entryID int64) *EntryPaginationBuilder {
	switch {
	case status == "":
	case entryID == 0:
		e.WithStatus(status)
	default:
		e.conditions = append(e.conditions, "(e.status = "+e.nextArg(status)+" OR e.id = "+e.nextArg(entryID)+")")
	}
	return e
}

// WithTags adds a condition matching entries having all the given tags.
func (e *EntryPaginationBuilder) WithTags(tags []string) *EntryPaginationBuilder {
	for _, tag := range tags {
		e.conditions = append(e.conditions, tagCondition(e.nextArg(tag)))
	}
	return e
}

// WithGloballyVisible adds global visibility to the condition.
func (e *EntryPaginationBuilder) WithGloballyVisible() *EntryPaginationBuilder {
	e.conditions = append(e.conditions, "c.hide_globally = 0", "f.hide_globally = 0")
	return e
}

// Entries returns previous and next entries (only ID and title are set).
func (e *EntryPaginationBuilder) Entries() (*model.Entry, *model.Entry, error) {
	prevID, nextID, err := e.getPrevNextID()
	if err != nil {
		return nil, nil, err
	}

	prevEntry, err := e.getEntry(prevID)
	if err != nil {
		return nil, nil, err
	}

	nextEntry, err := e.getEntry(nextID)
	if err != nil {
		return nil, nil, err
	}

	if e.direction == "desc" {
		return nextEntry, prevEntry, nil
	}
	return prevEntry, nextEntry, nil
}

func (e *EntryPaginationBuilder) getPrevNextID() (prevID, nextID int64, err error) {
	window := fmt.Sprintf("ORDER BY e.%[1]s ASC, e.created_at ASC, e.id DESC", e.order)
	query := `
		WITH entry_pagination AS (
			SELECT
				e.id,
				lag(e.id) OVER (` + window + `) AS prev_id,
				lead(e.id) OVER (` + window + `) AS next_id
			FROM entries e
			JOIN feeds f ON f.id=e.feed_id
			JOIN categories c ON c.id=f.category_id
			WHERE ` + strings.Join(e.conditions, " AND ") + `
		)
		SELECT prev_id, next_id FROM entry_pagination WHERE id = ` + e.nextArg(e.entryID)

	var pID, nID sql.NullInt64
	err = e.db.QueryRow(query, e.args...).Scan(&pID, &nID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return 0, 0, nil
	case err != nil:
		return 0, 0, fmt.Errorf("storage: entry pagination: %w", err)
	}
	return pID.Int64, nID.Int64, nil
}

func (e *EntryPaginationBuilder) getEntry(entryID int64) (*model.Entry, error) {
	var entry model.Entry
	err := e.db.QueryRow(`SELECT id, title FROM entries WHERE id = $1`, entryID).Scan(&entry.ID, &entry.Title)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return nil, nil
	case err != nil:
		return nil, fmt.Errorf("storage: fetching sibling entry: %w", err)
	}
	return &entry, nil
}
