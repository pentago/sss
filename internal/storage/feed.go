// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"database/sql"
	"errors"
	"fmt"
	"math"
	"sort"
	"time"

	"sss/internal/model"
)

// FeedExists checks if the given feed exists. A missing feed is reported as (false, nil).
func (s *Storage) FeedExists(userID, feedID int64) (bool, error) {
	var result bool
	err := s.db.QueryRow(`SELECT true FROM feeds WHERE user_id=$1 AND id=$2 LIMIT 1`, userID, feedID).Scan(&result)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return false, fmt.Errorf("storage: unable to check if feed exists: %w", err)
	}
	return result, nil
}

// CheckedAt returns when the feed was last checked.
func (s *Storage) CheckedAt(userID, feedID int64) (time.Time, error) {
	var result time.Time
	err := s.db.QueryRow(`SELECT checked_at FROM feeds WHERE user_id=$1 AND id=$2 LIMIT 1`, userID, feedID).Scan(unixTime{&result})
	if err != nil {
		return time.Now(), err
	}
	return result, nil
}

// CategoryFeedExists returns true if the given feed exists and belongs to the given category.
func (s *Storage) CategoryFeedExists(userID, categoryID, feedID int64) (bool, error) {
	var result bool
	err := s.db.QueryRow(`SELECT true FROM feeds WHERE user_id=$1 AND category_id=$2 AND id=$3 LIMIT 1`, userID, categoryID, feedID).Scan(&result)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return false, fmt.Errorf("storage: unable to check if feed exists in category: %w", err)
	}
	return result, nil
}

// FeedURLExists returns true if the given feed URL already exists for the user.
func (s *Storage) FeedURLExists(userID int64, feedURL string) bool {
	var result bool
	s.db.QueryRow(`SELECT true FROM feeds WHERE user_id=$1 AND feed_url=$2 LIMIT 1`, userID, feedURL).Scan(&result)
	return result
}

// AnotherFeedURLExists returns true if another feed with the same URL exists for the user.
func (s *Storage) AnotherFeedURLExists(userID, feedID int64, feedURL string) bool {
	var result bool
	s.db.QueryRow(`SELECT true FROM feeds WHERE id <> $1 AND user_id=$2 AND feed_url=$3 LIMIT 1`, feedID, userID, feedURL).Scan(&result)
	return result
}

// Feeds returns all feeds that belong to the given user.
func (s *Storage) Feeds(userID int64) (model.Feeds, error) {
	return s.NewFeedQueryBuilder(userID).GetFeeds()
}

// getFeedsSorted sorts disabled feeds last, then by error count, unread count and title.
func getFeedsSorted(builder *FeedQueryBuilder) (model.Feeds, error) {
	feeds, err := builder.GetFeeds()
	if err != nil {
		return nil, err
	}
	sort.Slice(feeds, func(i, j int) bool {
		a, b := feeds[i], feeds[j]
		if a.Disabled != b.Disabled {
			return b.Disabled
		}
		if a.ParsingErrorCount != b.ParsingErrorCount {
			return a.ParsingErrorCount > b.ParsingErrorCount
		}
		if a.UnreadCount != b.UnreadCount {
			return a.UnreadCount > b.UnreadCount
		}
		return a.Title < b.Title
	})
	return feeds, nil
}

// FeedsWithCounters returns all feeds of the given user with read and unread entry counters.
func (s *Storage) FeedsWithCounters(userID int64) (model.Feeds, error) {
	return getFeedsSorted(s.NewFeedQueryBuilder(userID).WithCounters())
}

// FetchCounters returns the per-feed read and unread entry counts for the given user.
func (s *Storage) FetchCounters(userID int64) (model.FeedCounters, error) {
	reads, unreads, err := s.NewFeedQueryBuilder(userID).WithCounters().fetchFeedCounter()
	return model.FeedCounters{ReadCounters: reads, UnreadCounters: unreads}, err
}

// FeedsByCategoryWithCounters returns all feeds in the given category for the given user with read and unread entry counters.
func (s *Storage) FeedsByCategoryWithCounters(userID, categoryID int64) (model.Feeds, error) {
	return getFeedsSorted(s.NewFeedQueryBuilder(userID).
		WithCategoryID(categoryID).
		WithCounters())
}

// WeeklyFeedEntryCount returns a virtual weekly entry count for a feed based
// on the average interval between its entries published during the last week.
// It returns 0 when there are fewer than two such entries.
func (s *Storage) WeeklyFeedEntryCount(userID, feedID int64) (int, error) {
	var count int64
	var oldest, newest sql.NullInt64
	err := s.db.QueryRow(`
		SELECT count(*), min(published_at), max(published_at)
		FROM entries
		WHERE user_id=$1 AND feed_id=$2 AND published_at >= unixepoch() - 604800`,
		userID, feedID,
	).Scan(&count, &oldest, &newest)
	if err != nil {
		return 0, fmt.Errorf("storage: unable to fetch weekly count for feed #%d: %w", feedID, err)
	}

	if count < 2 || newest.Int64 == oldest.Int64 {
		return 0, nil
	}
	averageInterval := float64(newest.Int64-oldest.Int64) / float64(count-1)
	return int(math.Ceil(604800 / averageInterval)), nil
}

// FeedByID returns the feed with the given ID.
func (s *Storage) FeedByID(userID, feedID int64) (*model.Feed, error) {
	feed, err := s.NewFeedQueryBuilder(userID).WithFeedID(feedID).GetFeed()
	if err != nil {
		return nil, fmt.Errorf("storage: unable to fetch feed #%d: %w", feedID, err)
	}
	return feed, nil
}

// CreateFeed creates a new feed and stores its entries as unread, skipping
// entries whose hash already exists or is tombstoned.
func (s *Storage) CreateFeed(feed *model.Feed) error {
	err := s.db.QueryRow(`
		INSERT INTO feeds (
			feed_url,
			site_url,
			title,
			category_id,
			user_id,
			etag_header,
			last_modified_header,
			crawler,
			user_agent,
			cookie,
			username,
			password,
			disabled,
			scraper_rules,
			rewrite_rules,
			blocklist_rules,
			keeplist_rules,
			block_filter_entry_rules,
			keep_filter_entry_rules,
			ignore_http_cache,
			allow_self_signed_certificates,
			fetch_via_proxy,
			hide_globally,
			url_rewrite_rules,
			no_media_player,
			apprise_service_urls,
			webhook_url,
			disable_http2,
			description,
			proxy_url,
			ignore_entry_updates,
			language
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20,
			$21, $22, $23, $24, $25, $26, $27, $28, $29, $30, $31, $32)
		RETURNING id`,
		feed.FeedURL,
		feed.SiteURL,
		feed.Title,
		feed.Category.ID,
		feed.UserID,
		feed.EtagHeader,
		feed.LastModifiedHeader,
		feed.Crawler,
		feed.UserAgent,
		feed.Cookie,
		feed.Username,
		feed.Password,
		feed.Disabled,
		feed.ScraperRules,
		feed.RewriteRules,
		feed.BlocklistRules,
		feed.KeeplistRules,
		feed.BlockFilterEntryRules,
		feed.KeepFilterEntryRules,
		feed.IgnoreHTTPCache,
		feed.AllowSelfSignedCertificates,
		feed.FetchViaProxy,
		feed.HideGlobally,
		feed.UrlRewriteRules,
		feed.NoMediaPlayer,
		feed.AppriseServiceURLs,
		feed.WebhookURL,
		feed.DisableHTTP2,
		feed.Description,
		feed.ProxyURL,
		feed.IgnoreEntryUpdates,
		feed.Language,
	).Scan(&feed.ID)
	if err != nil {
		return fmt.Errorf("storage: unable to create feed %q: %w", feed.FeedURL, err)
	}

	_, err = s.RefreshFeedEntries(feed.UserID, feed.ID, feed.Entries, false)
	return err
}

// UpdateFeed updates an existing feed.
func (s *Storage) UpdateFeed(feed *model.Feed) error {
	_, err := s.db.Exec(`
		UPDATE feeds SET
			feed_url=$1,
			site_url=$2,
			title=$3,
			category_id=$4,
			etag_header=$5,
			last_modified_header=$6,
			checked_at=$7,
			parsing_error_msg=$8,
			parsing_error_count=$9,
			scraper_rules=$10,
			rewrite_rules=$11,
			blocklist_rules=$12,
			keeplist_rules=$13,
			block_filter_entry_rules=$14,
			keep_filter_entry_rules=$15,
			crawler=$16,
			user_agent=$17,
			cookie=$18,
			username=$19,
			password=$20,
			disabled=$21,
			next_check_at=$22,
			ignore_http_cache=$23,
			allow_self_signed_certificates=$24,
			fetch_via_proxy=$25,
			hide_globally=$26,
			url_rewrite_rules=$27,
			no_media_player=$28,
			apprise_service_urls=$29,
			webhook_url=$30,
			disable_http2=$31,
			description=$32,
			ntfy_enabled=$33,
			ntfy_priority=$34,
			ntfy_topic=$35,
			pushover_enabled=$36,
			pushover_priority=$37,
			proxy_url=$38,
			ignore_entry_updates=$39,
			language=$40
		WHERE id=$41 AND user_id=$42`,
		feed.FeedURL,
		feed.SiteURL,
		feed.Title,
		feed.Category.ID,
		feed.EtagHeader,
		feed.LastModifiedHeader,
		feed.CheckedAt,
		feed.ParsingErrorMsg,
		feed.ParsingErrorCount,
		feed.ScraperRules,
		feed.RewriteRules,
		feed.BlocklistRules,
		feed.KeeplistRules,
		feed.BlockFilterEntryRules,
		feed.KeepFilterEntryRules,
		feed.Crawler,
		feed.UserAgent,
		feed.Cookie,
		feed.Username,
		feed.Password,
		feed.Disabled,
		feed.NextCheckAt,
		feed.IgnoreHTTPCache,
		feed.AllowSelfSignedCertificates,
		feed.FetchViaProxy,
		feed.HideGlobally,
		feed.UrlRewriteRules,
		feed.NoMediaPlayer,
		feed.AppriseServiceURLs,
		feed.WebhookURL,
		feed.DisableHTTP2,
		feed.Description,
		feed.NtfyEnabled,
		feed.NtfyPriority,
		feed.NtfyTopic,
		feed.PushoverEnabled,
		feed.PushoverPriority,
		feed.ProxyURL,
		feed.IgnoreEntryUpdates,
		feed.Language,
		feed.ID,
		feed.UserID,
	)
	if err != nil {
		return fmt.Errorf("storage: unable to update feed #%d (%s): %w", feed.ID, feed.FeedURL, err)
	}
	return nil
}

// UpdateFeedError persists the parsing error fields for the given feed.
func (s *Storage) UpdateFeedError(feed *model.Feed) error {
	_, err := s.db.Exec(`
		UPDATE feeds SET parsing_error_msg=$1, parsing_error_count=$2, checked_at=$3, next_check_at=$4
		WHERE id=$5 AND user_id=$6`,
		feed.ParsingErrorMsg, feed.ParsingErrorCount, feed.CheckedAt, feed.NextCheckAt, feed.ID, feed.UserID,
	)
	if err != nil {
		return fmt.Errorf("storage: unable to update feed error #%d (%s): %w", feed.ID, feed.FeedURL, err)
	}
	return nil
}

// RemoveFeed removes the given feed along with its entries and enclosures.
func (s *Storage) RemoveFeed(userID, feedID int64) error {
	if _, err := s.db.Exec(`DELETE FROM feeds WHERE id=$1 AND user_id=$2`, feedID, userID); err != nil {
		return fmt.Errorf("storage: unable to delete feed #%d: %w", feedID, err)
	}
	return nil
}

// ResetFeedErrors clears the parsing error fields for all feeds.
func (s *Storage) ResetFeedErrors() error {
	_, err := s.db.Exec(`UPDATE feeds SET parsing_error_count=0, parsing_error_msg=''`)
	return err
}

// ResetNextCheckAt schedules all feeds to be checked immediately.
func (s *Storage) ResetNextCheckAt() error {
	_, err := s.db.Exec(`UPDATE feeds SET next_check_at=unixepoch()`)
	return err
}
