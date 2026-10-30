// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"sss/internal/crypto"
	"sss/internal/model"
)

// ErrEntryTombstoned is returned when an entry cannot be created because its
// (feed_id, hash) pair has a tombstone recording a prior deletion.
var ErrEntryTombstoned = errors.New("storage: entry is tombstoned")

// UpdateEntryTitleAndContent updates entry title, content and reading time.
func (s *Storage) UpdateEntryTitleAndContent(entry *model.Entry) error {
	_, err := s.db.Exec(`UPDATE entries SET title=$1, content=$2, reading_time=$3 WHERE id=$4 AND user_id=$5`,
		entry.Title, entry.Content, entry.ReadingTime, entry.ID, entry.UserID)
	if err != nil {
		return fmt.Errorf("storage: unable to update entry #%d: %w", entry.ID, err)
	}
	return nil
}

// createEntry adds a new unread entry unless its (feed_id, hash) is tombstoned.
func (s *Storage) createEntry(tx *sql.Tx, entry *model.Entry) error {
	err := tx.QueryRow(`
		INSERT INTO entries
			(title, hash, url, comments_url, published_at, content, author, user_id, feed_id, reading_time, tags, language)
		SELECT $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
		WHERE NOT EXISTS (SELECT 1 FROM entry_tombstones WHERE feed_id=$9 AND hash=$2)
		RETURNING id, status, created_at, changed_at`,
		entry.Title,
		entry.Hash,
		entry.URL,
		entry.CommentsURL,
		entry.Date,
		entry.Content,
		entry.Author,
		entry.UserID,
		entry.FeedID,
		entry.ReadingTime,
		jsonArray(entry.Tags),
		entry.Language,
	).Scan(&entry.ID, &entry.Status, unixTime{&entry.CreatedAt}, unixTime{&entry.ChangedAt})
	if errors.Is(err, sql.ErrNoRows) {
		return ErrEntryTombstoned
	}
	if err != nil {
		return fmt.Errorf("storage: unable to create entry %q (feed #%d): %w", entry.URL, entry.FeedID, err)
	}

	for _, enclosure := range entry.Enclosures {
		enclosure.EntryID = entry.ID
		enclosure.UserID = entry.UserID
		if err := createEnclosure(tx, enclosure); err != nil {
			return err
		}
	}
	return nil
}

// updateEntry updates an entry when a feed is refreshed.
// The published date is kept: feeds without dates default to now, which would reorder history.
func (s *Storage) updateEntry(tx *sql.Tx, entry *model.Entry) error {
	err := tx.QueryRow(`
		UPDATE entries SET
			title=$1,
			url=$2,
			comments_url=$3,
			content=$4,
			author=$5,
			reading_time=$6,
			tags=$7,
			language=$8
		WHERE user_id=$9 AND feed_id=$10 AND hash=$11
		RETURNING id`,
		entry.Title,
		entry.URL,
		entry.CommentsURL,
		entry.Content,
		entry.Author,
		entry.ReadingTime,
		jsonArray(entry.Tags),
		entry.Language,
		entry.UserID,
		entry.FeedID,
		entry.Hash,
	).Scan(&entry.ID)
	if err != nil {
		return fmt.Errorf("storage: unable to update entry %q: %w", entry.URL, err)
	}

	for _, enclosure := range entry.Enclosures {
		enclosure.UserID = entry.UserID
		enclosure.EntryID = entry.ID
	}
	return updateEnclosures(tx, entry)
}

// entryIDByHash returns the ID of the entry with the given hash in the feed, or 0.
func entryIDByHash(tx *sql.Tx, feedID int64, hash string) (int64, error) {
	var entryID int64
	err := tx.QueryRow(`SELECT id FROM entries WHERE feed_id=$1 AND hash=$2`, feedID, hash).Scan(&entryID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("storage: unable to fetch entry ID: %w", err)
	}
	return entryID, nil
}

// InsertEntryForFeed inserts a single entry into a feed unless one with the same hash exists.
// Returns true if a new entry was created, false if an existing one was reused.
func (s *Storage) InsertEntryForFeed(userID, feedID int64, entry *model.Entry) (bool, error) {
	entry.UserID = userID
	entry.FeedID = feedID

	tx, err := s.db.Begin()
	if err != nil {
		return false, fmt.Errorf("storage: unable to start transaction: %w", err)
	}
	defer tx.Rollback()

	entryID, err := entryIDByHash(tx, entry.FeedID, entry.Hash)
	if err != nil {
		return false, err
	}
	if entryID > 0 {
		entry.ID = entryID
	} else if err := s.createEntry(tx, entry); err != nil {
		return false, err
	}

	if err := tx.Commit(); err != nil {
		return false, err
	}
	return entryID == 0, nil
}

// IsNewEntry reports whether an entry with this hash is neither stored nor tombstoned.
func (s *Storage) IsNewEntry(feedID int64, entryHash string) bool {
	var known bool
	s.db.QueryRow(`
		SELECT EXISTS (SELECT 1 FROM entries WHERE feed_id=$1 AND hash=$2)
			OR EXISTS (SELECT 1 FROM entry_tombstones WHERE feed_id=$1 AND hash=$2)`,
		feedID, entryHash,
	).Scan(&known)
	return !known
}

// GetReadTime returns the stored reading time of the entry with the given hash, or 0.
func (s *Storage) GetReadTime(feedID int64, entryHash string) int {
	var result int
	s.db.QueryRow(`SELECT reading_time FROM entries WHERE feed_id=$1 AND hash=$2`, feedID, entryHash).Scan(&result)
	return result
}

// RefreshFeedEntries stores new entries as unread (skipping tombstoned ones)
// and, when updateExistingEntries is set, updates entries already stored.
// Status, starred and changed_at of existing entries are kept.
func (s *Storage) RefreshFeedEntries(userID, feedID int64, entries model.Entries, updateExistingEntries bool) (newEntries model.Entries, err error) {
	for _, entry := range entries {
		entry.UserID = userID
		entry.FeedID = feedID

		tx, err := s.db.Begin()
		if err != nil {
			return nil, fmt.Errorf("storage: unable to start transaction: %w", err)
		}

		entryID, err := entryIDByHash(tx, feedID, entry.Hash)
		if err != nil {
			return nil, rollback(tx, err)
		}

		if entryID > 0 {
			if updateExistingEntries {
				err = s.updateEntry(tx, entry)
			}
		} else {
			err = s.createEntry(tx, entry)
			switch {
			case errors.Is(err, ErrEntryTombstoned):
				err = nil
			case err == nil:
				newEntries = append(newEntries, entry)
			}
		}
		if err != nil {
			return nil, rollback(tx, err)
		}

		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("storage: unable to commit transaction: %w", err)
		}
	}
	return newEntries, nil
}

// deleteEntriesWithTombstones deletes the entries matching where and records
// tombstones so they are not re-ingested on the next refresh.
func (s *Storage) deleteEntriesWithTombstones(where string, args ...any) (int64, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}

	_, err = tx.Exec(`
		INSERT INTO entry_tombstones (feed_id, hash)
		SELECT feed_id, hash FROM entries WHERE (`+where+`) AND hash <> ''
		ON CONFLICT DO NOTHING`, args...)
	if err != nil {
		return 0, rollback(tx, err)
	}

	result, err := tx.Exec(`DELETE FROM entries WHERE `+where, args...)
	if err != nil {
		return 0, rollback(tx, err)
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// ArchiveEntries deletes up to limit non-starred, non-shared entries with the given
// status created more than interval ago (at least one day), recording tombstones.
func (s *Storage) ArchiveEntries(status string, interval time.Duration, limit int) (int64, error) {
	if interval < 0 || limit <= 0 {
		return 0, nil
	}

	days := max(int64(interval/(24*time.Hour)), 1)
	count, err := s.deleteEntriesWithTombstones(`
		id IN (
			SELECT id FROM entries
			WHERE status=$1 AND starred=0 AND share_code='' AND created_at < unixepoch() - $2
			ORDER BY created_at ASC, id ASC
			LIMIT $3
		)`, status, days*86400, limit)
	if err != nil {
		return 0, fmt.Errorf("storage: unable to archive %s entries: %w", status, err)
	}
	return count, nil
}

// FlushHistory deletes all read entries (non-starred, non-shared) and records tombstones to prevent re-ingestion.
func (s *Storage) FlushHistory(userID int64) error {
	_, err := s.deleteEntriesWithTombstones(`user_id=$1 AND status=$2 AND starred=0 AND share_code=''`, userID, model.EntryStatusRead)
	if err != nil {
		return fmt.Errorf("storage: unable to flush history: %w", err)
	}
	return nil
}

// SetEntriesStatus update the status of the given list of entries.
func (s *Storage) SetEntriesStatus(userID int64, entryIDs []int64, status string) error {
	_, err := s.db.Exec(`UPDATE entries SET status=$1, changed_at=unixepoch() WHERE user_id=$2 AND id IN (SELECT value FROM json_each($3))`,
		status, userID, jsonArray(entryIDs))
	if err != nil {
		return fmt.Errorf("storage: unable to update entries statuses %v: %w", entryIDs, err)
	}
	return nil
}

// SetEntriesStatusAndCountVisible updates the status of the given entries and returns how many are visible in global views.
func (s *Storage) SetEntriesStatusAndCountVisible(userID int64, entryIDs []int64, status string) (int, error) {
	ids := jsonArray(entryIDs)
	tx, err := s.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("storage: unable to start transaction: %w", err)
	}

	_, err = tx.Exec(`UPDATE entries SET status=$1, changed_at=unixepoch() WHERE user_id=$2 AND id IN (SELECT value FROM json_each($3))`,
		status, userID, ids)
	if err != nil {
		return 0, rollback(tx, fmt.Errorf("storage: unable to update entries status %v: %w", entryIDs, err))
	}

	var visible int
	err = tx.QueryRow(`
		SELECT count(*)
		FROM entries e
		JOIN feeds f ON f.id = e.feed_id
		JOIN categories c ON c.id = f.category_id
		WHERE e.user_id=$1 AND e.id IN (SELECT value FROM json_each($2)) AND f.hide_globally=0 AND c.hide_globally=0`,
		userID, ids,
	).Scan(&visible)
	if err != nil {
		return 0, rollback(tx, fmt.Errorf("storage: unable to count visible entries: %w", err))
	}
	return visible, tx.Commit()
}

// SetEntriesStarredState updates the starred state for the given list of entries.
func (s *Storage) SetEntriesStarredState(userID int64, entryIDs []int64, starred bool) error {
	_, err := s.db.Exec(`UPDATE entries SET starred=$1, changed_at=unixepoch() WHERE user_id=$2 AND id IN (SELECT value FROM json_each($3))`,
		starred, userID, jsonArray(entryIDs))
	if err != nil {
		return fmt.Errorf("storage: unable to update the starred state %v: %w", entryIDs, err)
	}
	return nil
}

// ToggleStarred toggles entry starred value.
func (s *Storage) ToggleStarred(userID int64, entryID int64) error {
	result, err := s.db.Exec(`UPDATE entries SET starred = NOT starred, changed_at=unixepoch() WHERE user_id=$1 AND id=$2`, userID, entryID)
	if err != nil {
		return fmt.Errorf("storage: unable to toggle starred flag for entry #%d: %w", entryID, err)
	}
	if count, _ := result.RowsAffected(); count == 0 {
		return errors.New("storage: nothing has been updated")
	}
	return nil
}

// markAsRead marks the user's unread entries matching the extra condition as read.
func (s *Storage) markAsRead(userID int64, condition string, args ...any) error {
	query := `UPDATE entries SET status='read', changed_at=unixepoch() WHERE user_id=$1 AND status='unread'`
	if condition != "" {
		query += " AND " + condition
	}
	_, err := s.db.Exec(query, append([]any{userID}, args...)...)
	return err
}

// MarkAllAsRead updates all user entries to the read status.
func (s *Storage) MarkAllAsRead(userID int64) error {
	if err := s.markAsRead(userID, ""); err != nil {
		return fmt.Errorf("storage: unable to mark all entries as read: %w", err)
	}
	return nil
}

// MarkAllAsReadBeforeDate updates all user entries published before the given date to the read status.
func (s *Storage) MarkAllAsReadBeforeDate(userID int64, before time.Time) error {
	if err := s.markAsRead(userID, "published_at < $2", before); err != nil {
		return fmt.Errorf("storage: unable to mark all entries as read before %s: %w", before.Format(time.RFC3339), err)
	}
	return nil
}

// MarkGloballyVisibleFeedsAsRead marks as read the unread entries that are
// visible in the global unread view, i.e. those belonging to a feed and a
// category that are both not hidden globally.
func (s *Storage) MarkGloballyVisibleFeedsAsRead(userID int64) error {
	err := s.markAsRead(userID, `feed_id IN (
		SELECT f.id FROM feeds f JOIN categories c ON c.id = f.category_id
		WHERE f.user_id=$1 AND f.hide_globally=0 AND c.hide_globally=0
	)`)
	if err != nil {
		return fmt.Errorf("storage: unable to mark globally visible feeds as read: %w", err)
	}
	return nil
}

// MarkFeedAsRead updates the feed entries published before the given date to the read status.
func (s *Storage) MarkFeedAsRead(userID, feedID int64, before time.Time) error {
	if err := s.markAsRead(userID, "feed_id=$2 AND published_at < $3", feedID, before); err != nil {
		return fmt.Errorf("storage: unable to mark feed entries as read: %w", err)
	}
	return nil
}

// MarkCategoryAsRead updates the category entries published before the given date to the read status.
func (s *Storage) MarkCategoryAsRead(userID, categoryID int64, before time.Time) error {
	err := s.markAsRead(userID, "published_at < $2 AND feed_id IN (SELECT id FROM feeds WHERE user_id=$1 AND category_id=$3)", before, categoryID)
	if err != nil {
		return fmt.Errorf("storage: unable to mark category entries as read: %w", err)
	}
	return nil
}

// EntryShareCode returns the share code of the provided entry.
// It generates a new one if not already defined.
func (s *Storage) EntryShareCode(userID int64, entryID int64) (shareCode string, err error) {
	err = s.db.QueryRow(`SELECT share_code FROM entries WHERE user_id=$1 AND id=$2`, userID, entryID).Scan(&shareCode)
	if err != nil {
		return "", fmt.Errorf("storage: unable to get share code for entry #%d: %w", entryID, err)
	}

	if shareCode == "" {
		shareCode = crypto.GenerateRandomStringHex(20)
		if _, err := s.db.Exec(`UPDATE entries SET share_code=$1 WHERE user_id=$2 AND id=$3`, shareCode, userID, entryID); err != nil {
			return "", fmt.Errorf("storage: unable to set share code for entry #%d: %w", entryID, err)
		}
	}
	return shareCode, nil
}

// UnshareEntry removes the share code for the given entry.
func (s *Storage) UnshareEntry(userID int64, entryID int64) error {
	if _, err := s.db.Exec(`UPDATE entries SET share_code='' WHERE user_id=$1 AND id=$2`, userID, entryID); err != nil {
		return fmt.Errorf("storage: unable to remove share code for entry #%d: %w", entryID, err)
	}
	return nil
}
