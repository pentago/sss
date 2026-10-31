// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"sss/internal/model"
)

const enclosureColumns = `id, user_id, entry_id, url, size, mime_type, media_progression`

func enclosureDest(e *model.Enclosure) []any {
	return []any{&e.ID, &e.UserID, &e.EntryID, &e.URL, &e.Size, &e.MimeType, &e.MediaProgression}
}

// EnclosuresByEntryIDs returns enclosures for the given entries, grouped by entry ID.
func (s *Storage) EnclosuresByEntryIDs(entryIDs []int64) (map[int64]model.EnclosureList, error) {
	rows, err := s.db.Query(`SELECT `+enclosureColumns+` FROM enclosures WHERE entry_id IN (SELECT value FROM json_each($1)) ORDER BY id ASC`, jsonArray(entryIDs))
	if err != nil {
		return nil, fmt.Errorf("storage: unable to fetch enclosures: %w", err)
	}
	defer rows.Close()

	enclosures := make(map[int64]model.EnclosureList)
	for rows.Next() {
		var enclosure model.Enclosure
		if err := rows.Scan(enclosureDest(&enclosure)...); err != nil {
			return nil, fmt.Errorf("storage: unable to scan enclosure row: %w", err)
		}
		enclosures[enclosure.EntryID] = append(enclosures[enclosure.EntryID], &enclosure)
	}
	return enclosures, rows.Err()
}

// EnclosureByID returns the enclosure for the given user and enclosure ID.
func (s *Storage) EnclosureByID(userID, enclosureID int64) (*model.Enclosure, error) {
	var enclosure model.Enclosure
	err := s.db.QueryRow(`SELECT `+enclosureColumns+` FROM enclosures WHERE id=$1 AND user_id=$2`, enclosureID, userID).Scan(enclosureDest(&enclosure)...)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return nil, nil
	case err != nil:
		return nil, fmt.Errorf("storage: unable to fetch enclosure row: %w", err)
	}
	return &enclosure, nil
}

func createEnclosure(tx *sql.Tx, enclosure *model.Enclosure) error {
	enclosureURL := strings.TrimSpace(enclosure.URL)
	if enclosureURL == "" {
		return nil
	}

	err := tx.QueryRow(`
		INSERT INTO enclosures (url, size, mime_type, entry_id, user_id, media_progression)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (entry_id, url) DO NOTHING
		RETURNING id`,
		enclosureURL, enclosure.Size, enclosure.MimeType, enclosure.EntryID, enclosure.UserID, enclosure.MediaProgression,
	).Scan(&enclosure.ID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("storage: unable to create enclosure: %w", err)
	}
	return nil
}

// updateEnclosures adds the entry's new enclosures and removes the ones no longer listed.
func updateEnclosures(tx *sql.Tx, entry *model.Entry) error {
	urls := make([]string, 0, len(entry.Enclosures))
	for _, enclosure := range entry.Enclosures {
		urls = append(urls, strings.TrimSpace(enclosure.URL))
		if err := createEnclosure(tx, enclosure); err != nil {
			return err
		}
	}

	_, err := tx.Exec(`DELETE FROM enclosures WHERE user_id=$1 AND entry_id=$2 AND url NOT IN (SELECT value FROM json_each($3))`,
		entry.UserID, entry.ID, jsonArray(urls))
	if err != nil {
		return fmt.Errorf("storage: unable to delete old enclosures: %w", err)
	}
	return nil
}

// UpdateEnclosure persists changes to the given enclosure.
func (s *Storage) UpdateEnclosure(enclosure *model.Enclosure) error {
	_, err := s.db.Exec(`
		UPDATE enclosures SET url=$1, size=$2, mime_type=$3, entry_id=$4, user_id=$5, media_progression=$6
		WHERE id=$7`,
		enclosure.URL, enclosure.Size, enclosure.MimeType, enclosure.EntryID, enclosure.UserID, enclosure.MediaProgression, enclosure.ID,
	)
	if err != nil {
		return fmt.Errorf("storage: unable to update enclosure #%d: %w", enclosure.ID, err)
	}
	return nil
}
