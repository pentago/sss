// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

// Package processor applies filters, scraping, rewrite rules and sanitization to parsed feed entries.
package processor

import (
	"log/slog"
	"net/url"
	"slices"

	"sss/internal/config"
	"sss/internal/model"
	"sss/internal/reader/fetcher"
	"sss/internal/reader/filter"
	"sss/internal/reader/readingtime"
	"sss/internal/reader/rewrite"
	"sss/internal/reader/sanitizer"
	"sss/internal/reader/scraper"
	"sss/internal/reader/urlcleaner"
)

// EntryStore reports what is already stored for a feed; *storage.Storage implements it.
type EntryStore interface {
	// IsNewEntry reports whether no entry with this hash is stored for the feed.
	IsNewEntry(feedID int64, entryHash string) bool
	// GetReadTime returns the stored reading time of the entry with this hash, or 0.
	GetReadTime(feedID int64, entryHash string) int
}

// ProcessFeedEntries filters feed.Entries in place and downloads, rewrites and sanitizes their content.
func ProcessFeedEntries(store EntryStore, feed *model.Feed, user *model.User, forceRefresh bool) {
	var filteredEntries model.Entries

	// The errors are handled in RemoveTrackingParameters.
	parsedFeedURL, _ := url.Parse(feed.FeedURL)
	parsedSiteURL, _ := url.Parse(feed.SiteURL)

	blockRules := filter.ParseRules(user.BlockFilterEntryRules, feed.BlockFilterEntryRules)
	allowRules := filter.ParseRules(user.KeepFilterEntryRules, feed.KeepFilterEntryRules)

	requestBuilder := newRequestBuilder(feed, user)

	// Processing older entries first ensures that their creation timestamp is lower than newer entries.
	for _, entry := range slices.Backward(feed.Entries) {
		if filter.IsBlockedEntry(blockRules, allowRules, feed, entry) {
			continue
		}

		parsedInputURL, _ := url.Parse(entry.URL)
		if cleanedURL, err := urlcleaner.RemoveTrackingParameters(parsedFeedURL, parsedSiteURL, parsedInputURL); err == nil {
			entry.URL = cleanedURL
		}

		webpageBaseURL := ""
		entry.URL = rewrite.RewriteEntryURL(feed, entry)
		entryIsNew := store.IsNewEntry(feed.ID, entry.Hash)
		contentExtractedSuccessfully := false
		if (feed.Crawler || user.GlobalCrawler) && (forceRefresh || entryIsNew) {
			scrapedPageBaseURL, extractedContent, scraperErr := scraper.ScrapeWebsite(requestBuilder, entry.URL, feed.ScraperRules)
			if scrapedPageBaseURL != "" {
				webpageBaseURL = scrapedPageBaseURL
			}

			if scraperErr != nil {
				slog.Warn("Unable to scrape entry",
					slog.Int64("user_id", user.ID),
					slog.String("entry_url", entry.URL),
					slog.Int64("feed_id", feed.ID),
					slog.Any("error", scraperErr),
				)
			} else if extractedContent != "" {
				// We replace the entry content only if the scraper doesn't return any error.
				entry.Content = extractedContent
				contentExtractedSuccessfully = true
			}
		}

		rewrite.ApplyContentRewriteRules(entry, feed.RewriteRules)

		// Re-run filters only when extracted content replaced entry.Content.
		if contentExtractedSuccessfully && filter.IsBlockedEntry(blockRules, allowRules, feed, entry) {
			continue
		}

		if webpageBaseURL == "" {
			webpageBaseURL = entry.URL
		}

		// The sanitizer should always run at the end of the process to make sure unsafe HTML is filtered out.
		entry.Content = sanitizer.SanitizeHTML(webpageBaseURL, entry.Content, &sanitizer.SanitizerOptions{OpenLinksInNewTab: user.OpenExternalLinksInNewTab})

		updateEntryReadingTime(store, feed, entry, entryIsNew, user)

		filteredEntries = append(filteredEntries, entry)
	}

	if user.ShowReadingTime && config.Opts.FetchYouTubeWatchTime && config.Opts.YouTubeAPIKey != "" {
		fetchYouTubeWatchTimeInBulk(filteredEntries)
	}

	feed.Entries = filteredEntries
}

// ProcessEntryWebPage downloads the entry web page and applies rewrite rules and the sanitizer.
func ProcessEntryWebPage(feed *model.Feed, entry *model.Entry, user *model.User) error {
	entry.URL = rewrite.RewriteEntryURL(feed, entry)

	webpageBaseURL, extractedContent, scraperErr := scraper.ScrapeWebsite(newRequestBuilder(feed, user), entry.URL, feed.ScraperRules)
	if scraperErr != nil {
		return scraperErr
	}

	if extractedContent != "" {
		entry.Content = extractedContent
		if user.ShowReadingTime {
			entry.ReadingTime = readingtime.EstimateReadingTime(entry.Content, user.DefaultReadingSpeed, user.CJKReadingSpeed)
		}
	}

	rewrite.ApplyContentRewriteRules(entry, feed.RewriteRules)
	entry.Content = sanitizer.SanitizeHTML(webpageBaseURL, entry.Content, &sanitizer.SanitizerOptions{OpenLinksInNewTab: user.OpenExternalLinksInNewTab})

	return nil
}

func newRequestBuilder(feed *model.Feed, user *model.User) *fetcher.RequestBuilder {
	return fetcher.ForFeed(feed, user.DefaultUserAgent())
}
