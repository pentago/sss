// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

// Package handler creates and refreshes feeds: fetch, parse, process and store.
package handler

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"

	"sss/internal/config"
	"sss/internal/integration"
	"sss/internal/model"
	"sss/internal/reader/fetcher"
	"sss/internal/reader/icon"
	"sss/internal/reader/parser"
	"sss/internal/reader/processor"
	"sss/internal/storage"
)

// User-presentable errors returned by the handlers.
var (
	ErrCategoryNotFound = errors.New("This category does not exist or does not belong to this user.")
	ErrFeedNotFound     = errors.New("This feed does not exist or does not belong to this user.")
	ErrDuplicatedFeed   = errors.New("This feed already exists.")
)

func databaseError(err error) error {
	return fmt.Errorf("Database error: %w.", err)
}

func parseError(err error) error {
	if errors.Is(err, parser.ErrFeedFormatNotDetected) {
		return fmt.Errorf("Unable to detect feed format: %w.", err)
	}
	return fmt.Errorf("Unable to parse this feed: %w.", err)
}

// requestBuilder returns a fetcher configured with the feed's HTTP settings,
// falling back to the user's global user agent.
func requestBuilder(feed *model.Feed, user *model.User) *fetcher.RequestBuilder {
	return fetcher.ForFeed(feed, user.DefaultUserAgent()).
		WithUsernameAndPassword(feed.Username, feed.Password)
}

// applyRequest copies the user-provided settings of req into feed.
func applyRequest(feed *model.Feed, userID int64, req *model.FeedCreationRequest) {
	feed.UserID = userID
	feed.UserAgent = req.UserAgent
	feed.Cookie = req.Cookie
	feed.Username = req.Username
	feed.Password = req.Password
	feed.Crawler = req.Crawler
	feed.IgnoreEntryUpdates = req.IgnoreEntryUpdates
	feed.Disabled = req.Disabled
	feed.NoMediaPlayer = req.NoMediaPlayer
	feed.IgnoreHTTPCache = req.IgnoreHTTPCache
	feed.AllowSelfSignedCertificates = req.AllowSelfSignedCertificates
	feed.FetchViaProxy = req.FetchViaProxy
	feed.HideGlobally = req.HideGlobally
	feed.DisableHTTP2 = req.DisableHTTP2
	feed.ScraperRules = req.ScraperRules
	feed.RewriteRules = req.RewriteRules
	feed.BlocklistRules = req.BlocklistRules
	feed.KeeplistRules = req.KeeplistRules
	feed.BlockFilterEntryRules = req.BlockFilterEntryRules
	feed.KeepFilterEntryRules = req.KeepFilterEntryRules
	feed.UrlRewriteRules = req.UrlRewriteRules
	feed.ProxyURL = req.ProxyURL
	feed.WithCategoryID(req.CategoryID)
}

func checkCategory(store *storage.Storage, userID, categoryID int64) error {
	exists, err := store.CategoryIDExists(userID, categoryID)
	if err != nil {
		return databaseError(err)
	}
	if !exists {
		return ErrCategoryNotFound
	}
	return nil
}

// CreateFeedFromSubscriptionDiscovery stores a new feed from content already
// downloaded during subscription discovery.
func CreateFeedFromSubscriptionDiscovery(store *storage.Storage, userID int64, req *model.FeedCreationRequestFromSubscriptionDiscovery) (*model.Feed, error) {
	if err := checkCategory(store, userID, req.CategoryID); err != nil {
		return nil, err
	}
	if store.FeedURLExists(userID, req.FeedURL) {
		return nil, ErrDuplicatedFeed
	}
	user, err := store.UserByID(userID)
	if err != nil {
		return nil, databaseError(err)
	}
	return createFeed(store, user, &req.FeedCreationRequest, req.FeedURL, req.Content, req.ETag, req.LastModified)
}

// CreateFeed downloads, parses and stores a new feed.
func CreateFeed(store *storage.Storage, userID int64, req *model.FeedCreationRequest) (*model.Feed, error) {
	if err := checkCategory(store, userID, req.CategoryID); err != nil {
		return nil, err
	}
	user, err := store.UserByID(userID)
	if err != nil {
		return nil, databaseError(err)
	}

	settings := &model.Feed{FeedURL: req.FeedURL}
	applyRequest(settings, userID, req)
	rh := fetcher.NewResponseHandler(requestBuilder(settings, user).ExecuteRequest(req.FeedURL))
	defer rh.Close()
	if err := rh.LocalizedError(); err != nil {
		slog.Warn("Unable to fetch feed", slog.String("feed_url", req.FeedURL), slog.Any("error", err))
		return nil, err
	}
	body, err := rh.ReadBody(config.Opts.HTTPClientMaxBodySize)
	if err != nil {
		slog.Warn("Unable to fetch feed", slog.String("feed_url", req.FeedURL), slog.Any("error", err))
		return nil, err
	}
	if store.FeedURLExists(userID, rh.EffectiveURL()) {
		return nil, ErrDuplicatedFeed
	}
	return createFeed(store, user, req, rh.EffectiveURL(), bytes.NewReader(body), rh.ETag(), rh.LastModified())
}

// createFeed parses content as the feed at feedURL, applies req and stores it.
func createFeed(store *storage.Storage, user *model.User, req *model.FeedCreationRequest, feedURL string, content io.ReadSeeker, etag, lastModified string) (*model.Feed, error) {
	feed, err := parser.ParseFeed(feedURL, content)
	if err != nil {
		return nil, parseError(err)
	}
	applyRequest(feed, user.ID, req)
	feed.FeedURL = feedURL
	feed.EtagHeader = etag
	feed.LastModifiedHeader = lastModified
	feed.CheckedNow()

	processor.ProcessFeedEntries(store, feed, user, true)

	if err := store.CreateFeed(feed); err != nil {
		return nil, databaseError(err)
	}
	slog.Debug("Created feed", slog.Int64("user_id", user.ID), slog.Int64("feed_id", feed.ID), slog.String("feed_url", feed.FeedURL))

	updateIcon(store, feed, user)
	return feed, nil
}

// RefreshFeed downloads a feed again and stores its new and updated entries.
func RefreshFeed(store *storage.Storage, userID, feedID int64, forceRefresh bool) error {
	slog.Debug("Begin feed refresh process", slog.Int64("user_id", userID), slog.Int64("feed_id", feedID), slog.Bool("force_refresh", forceRefresh))

	feed, err := store.FeedByID(userID, feedID)
	if err != nil {
		return databaseError(err)
	}
	if feed == nil {
		return ErrFeedNotFound
	}
	user, err := store.UserByID(userID)
	if err != nil {
		return databaseError(err)
	}

	weeklyEntryCount := 0
	if config.Opts.PollingScheduler == model.SchedulerEntryFrequency {
		if weeklyEntryCount, err = store.WeeklyFeedEntryCount(userID, feedID); err != nil {
			return databaseError(err)
		}
	}

	feed.CheckedNow()
	feed.ScheduleNextCheck(weeklyEntryCount, 0)

	rb := requestBuilder(feed, user)
	ignoreHTTPCache := feed.IgnoreHTTPCache || forceRefresh
	if !ignoreHTTPCache {
		rb = rb.WithETag(feed.EtagHeader).WithLastModified(feed.LastModifiedHeader)
	}

	rh := fetcher.NewResponseHandler(rb.ExecuteRequest(feed.FeedURL))
	defer rh.Close()

	if rh.IsRateLimited() {
		retryDelay := rh.ParseRetryDelay()
		feed.ScheduleNextCheck(weeklyEntryCount, retryDelay)
		slog.Warn("Feed is rate limited", slog.String("feed_url", feed.FeedURL), slog.Duration("retry_delay", retryDelay), slog.Time("new_next_check_at", feed.NextCheckAt))
	}

	if err := rh.LocalizedError(); err != nil {
		slog.Warn("Unable to fetch feed", slog.Int64("user_id", userID), slog.Int64("feed_id", feedID), slog.String("feed_url", feed.FeedURL), slog.Any("error", err))
		return recordError(store, feed, err)
	}

	if store.AnotherFeedURLExists(userID, feed.ID, rh.EffectiveURL()) {
		return recordError(store, feed, ErrDuplicatedFeed)
	}

	if ignoreHTTPCache || rh.IsModified(feed.EtagHeader, feed.LastModifiedHeader) {
		body, err := rh.ReadBody(config.Opts.HTTPClientMaxBodySize)
		if err != nil {
			slog.Warn("Unable to fetch feed", slog.String("feed_url", feed.FeedURL), slog.Any("error", err))
			return recordError(store, feed, err)
		}

		updated, err := parser.ParseFeed(rh.EffectiveURL(), bytes.NewReader(body))
		if err != nil {
			return recordError(store, feed, parseError(err))
		}

		// Use the RSS TTL, Cache-Control max-age or Expires header when longer than the scheduler interval.
		refreshDelay := max(updated.TTL, rh.CacheControlMaxAge(), rh.Expires())
		feed.ScheduleNextCheck(weeklyEntryCount, refreshDelay)

		feed.Entries = updated.Entries
		processor.ProcessFeedEntries(store, feed, user, forceRefresh)

		// Existing entries are not updated when crawling or ignoring entry updates, unless forced.
		updateExisting := forceRefresh || (!feed.Crawler && !user.GlobalCrawler && !feed.IgnoreEntryUpdates)
		newEntries, err := store.RefreshFeedEntries(feed.UserID, feed.ID, feed.Entries, updateExisting)
		if err != nil {
			return recordError(store, feed, databaseError(err))
		}

		if integ, err := store.Integration(userID); err != nil {
			slog.Error("Fetching integrations failed; no integrations will run this time", slog.Int64("user_id", userID), slog.Int64("feed_id", feedID), slog.Any("error", err))
		} else if integ != nil && len(newEntries) > 0 {
			go integration.PushEntries(feed, newEntries, integ)
		}

		feed.EtagHeader = rh.ETag()
		feed.LastModifiedHeader = rh.LastModified()
		feed.Language = updated.Language
		feed.IconURL = updated.IconURL
		if forceRefresh || !store.HasFeedIcon(feed.ID) {
			updateIcon(store, feed, user)
		}
	} else if rh.LastModified() != "" {
		// Per RFC 9111 §4.3.4 a 304 may carry an updated Last-Modified.
		feed.LastModifiedHeader = rh.LastModified()
	}

	feed.ResetErrorCounter()
	if err := store.UpdateFeed(feed); err != nil {
		return recordError(store, feed, databaseError(err))
	}
	return nil
}

// recordError stores err as the feed's parsing error and returns it.
func recordError(store *storage.Storage, feed *model.Feed, err error) error {
	feed.WithErrorMessage(err.Error())
	if storeErr := store.UpdateFeedError(feed); storeErr != nil {
		slog.Error("Unable to update feed error", slog.Int64("feed_id", feed.ID), slog.Any("error", storeErr))
	}
	return err
}

// updateIcon downloads the feed icon and stores it.
func updateIcon(store *storage.Storage, feed *model.Feed, user *model.User) {
	ic, err := icon.FindIcon(feed, user.DefaultUserAgent())
	switch {
	case err != nil:
		slog.Debug("Unable to find feed icon", slog.Int64("feed_id", feed.ID), slog.String("website_url", feed.SiteURL), slog.Any("error", err))
	case ic == nil:
		slog.Debug("No icon found", slog.Int64("feed_id", feed.ID), slog.String("website_url", feed.SiteURL))
	default:
		if err := store.StoreFeedIcon(feed.ID, ic); err != nil {
			slog.Error("Unable to store feed icon", slog.Int64("feed_id", feed.ID), slog.Any("error", err))
		}
	}
}
