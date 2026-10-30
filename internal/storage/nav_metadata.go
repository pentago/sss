// Copyright The Miniflux Authors. Modified by The sss Authors.
// Derived from Miniflux (https://github.com/miniflux/v2).
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"fmt"

	"sss/internal/config"
)

// NavMetadata holds the counters shown in the navigation of every page.
type NavMetadata struct {
	CountUnread     int
	CountErrorFeeds int
	HasSaveEntry    bool
}

// GetNavMetadata returns the navigation metadata for the given user in a single SQL query.
// CountErrorFeeds is 0 when POLLING_PARSING_ERROR_LIMIT is 0 (unlimited errors).
func (s *Storage) GetNavMetadata(userID int64) (NavMetadata, error) {
	var meta NavMetadata
	err := s.db.QueryRow(`
		SELECT
			(SELECT count(*)
			   FROM entries e
			   JOIN feeds f ON f.id = e.feed_id
			   JOIN categories c ON c.id = f.category_id
			  WHERE e.user_id = $1 AND e.status = 'unread' AND f.hide_globally = 0 AND c.hide_globally = 0),
			(SELECT count(*) FROM feeds WHERE user_id = $1 AND $2 > 0 AND parsing_error_count >= $2),
			EXISTS (SELECT 1 FROM integrations WHERE user_id = $1 AND `+saveEntryIntegrations+`)`,
		userID, config.Opts.PollingParsingErrorLimit,
	).Scan(&meta.CountUnread, &meta.CountErrorFeeds, &meta.HasSaveEntry)
	if err != nil {
		return NavMetadata{}, fmt.Errorf("storage: unable to fetch navigation metadata: %w", err)
	}
	return meta, nil
}
