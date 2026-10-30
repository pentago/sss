// Copyright The Miniflux Authors. Modified by The sss Authors.
// Derived from Miniflux (https://github.com/miniflux/v2).
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"sss/internal/crypto"
	"sss/internal/model"
)

const iconColumns = `i.id, i.hash, i.mime_type, i.content, i.external_id`

func iconDest(icon *model.Icon) []any {
	return []any{&icon.ID, &icon.Hash, &icon.MimeType, &icon.Content, &icon.ExternalID}
}

// HasFeedIcon reports whether the specified feed already has an associated icon record.
func (s *Storage) HasFeedIcon(feedID int64) bool {
	var result bool
	s.db.QueryRow(`SELECT true FROM feed_icons WHERE feed_id=$1 LIMIT 1`, feedID).Scan(&result)
	return result
}

// IconByUserAndIconID fetches an icon by ID if it is used by a feed of the
// given user; it returns nil otherwise.
func (s *Storage) IconByUserAndIconID(userID, iconID int64) (*model.Icon, error) {
	return s.fetchIcon(`
		SELECT `+iconColumns+` FROM icons i
		WHERE i.id = $2 AND EXISTS (
			SELECT 1 FROM feeds f INNER JOIN feed_icons fi ON fi.feed_id = f.id
			WHERE f.user_id = $1 AND fi.icon_id = $2
		)`, userID, iconID)
}

// IconByExternalID fetches an icon using its external identifier, returning nil when no match exists.
func (s *Storage) IconByExternalID(externalIconID string) (*model.Icon, error) {
	return s.fetchIcon(`SELECT `+iconColumns+` FROM icons i WHERE i.external_id=$1`, externalIconID)
}

// IconByFeedID returns the icon linked to the given feed for the specified user, or nil if none is set.
func (s *Storage) IconByFeedID(userID, feedID int64) (*model.Icon, error) {
	return s.fetchIcon(`
		SELECT `+iconColumns+` FROM icons i
		INNER JOIN feed_icons fi ON fi.icon_id=i.id
		INNER JOIN feeds f ON f.id=fi.feed_id
		WHERE f.user_id=$1 AND f.id=$2
		LIMIT 1`, userID, feedID)
}

func (s *Storage) fetchIcon(query string, args ...any) (*model.Icon, error) {
	var icon model.Icon
	err := s.db.QueryRow(query, args...).Scan(iconDest(&icon)...)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return nil, nil
	case err != nil:
		return nil, fmt.Errorf("storage: unable to fetch icon: %w", err)
	}
	return &icon, nil
}

// StoreFeedIcon creates or reuses an icon by hash and associates it with the given feed atomically.
func (s *Storage) StoreFeedIcon(feedID int64, icon *model.Icon) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("storage: unable to start transaction: %w", err)
	}

	err = tx.QueryRow(`SELECT id FROM icons WHERE hash=$1`, icon.Hash).Scan(&icon.ID)
	if errors.Is(err, sql.ErrNoRows) {
		err = tx.QueryRow(`INSERT INTO icons (hash, mime_type, content, external_id) VALUES ($1, $2, $3, $4) RETURNING id`,
			icon.Hash, normalizeMimeType(icon.MimeType), icon.Content, crypto.GenerateRandomStringHex(20),
		).Scan(&icon.ID)
	}
	if err != nil {
		return rollback(tx, fmt.Errorf("storage: unable to store icon %q: %w", icon.Hash, err))
	}

	if _, err := tx.Exec(`DELETE FROM feed_icons WHERE feed_id=$1`, feedID); err != nil {
		return rollback(tx, fmt.Errorf("storage: unable to delete feed icon: %w", err))
	}

	if _, err := tx.Exec(`INSERT INTO feed_icons (feed_id, icon_id) VALUES ($1, $2)`, feedID, icon.ID); err != nil {
		return rollback(tx, fmt.Errorf("storage: unable to associate feed and icon: %w", err))
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("storage: unable to commit transaction: %w", err)
	}
	return nil
}

// CleanupOrphanIcons removes icons that are no longer associated with any feed.
func (s *Storage) CleanupOrphanIcons() (int64, error) {
	result, err := s.db.Exec(`DELETE FROM icons WHERE NOT EXISTS (SELECT 1 FROM feed_icons WHERE feed_icons.icon_id = icons.id)`)
	if err != nil {
		return 0, fmt.Errorf("storage: unable to clean orphan icons: %w", err)
	}
	return result.RowsAffected()
}

// Icons lists all icons currently associated with any feed owned by the given user.
func (s *Storage) Icons(userID int64) (model.Icons, error) {
	rows, err := s.db.Query(`
		SELECT `+iconColumns+` FROM icons i
		INNER JOIN feed_icons fi ON fi.icon_id=i.id
		INNER JOIN feeds f ON f.id=fi.feed_id
		WHERE f.user_id=$1`, userID)
	if err != nil {
		return nil, fmt.Errorf("storage: unable to fetch icons: %w", err)
	}
	defer rows.Close()

	var icons model.Icons
	for rows.Next() {
		var icon model.Icon
		if err := rows.Scan(iconDest(&icon)...); err != nil {
			return nil, fmt.Errorf("storage: unable to fetch icons row: %w", err)
		}
		icons = append(icons, &icon)
	}
	return icons, rows.Err()
}

func normalizeMimeType(mimeType string) string {
	mimeType = strings.ToLower(mimeType)
	switch mimeType {
	case "image/png", "image/jpeg", "image/jpg", "image/webp", "image/svg+xml", "image/x-icon", "image/gif":
		return mimeType
	default:
		return "image/x-icon"
	}
}
