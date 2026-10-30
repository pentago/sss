// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"errors"
	"fmt"
	"io"

	"sss/internal/model"
	"sss/internal/reader/opml"
	"sss/internal/storage"
	"sss/internal/validator"
)

// ImportOPML creates the feeds of an OPML document the user isn't subscribed
// to yet, creating missing categories by title.
func ImportOPML(store *storage.Storage, userID int64, r io.Reader) error {
	feeds, err := opml.Parse(r)
	if err != nil {
		return err
	}
	if len(feeds) == 0 {
		return errors.New("This file contains no feeds.")
	}

	for _, feed := range feeds {
		if store.FeedURLExists(userID, feed.FeedURL) {
			continue
		}

		category, err := opmlCategory(store, userID, feed.Category.Title)
		if err != nil {
			return err
		}

		if err := validator.ValidateFeedCreation(store, userID, &model.FeedCreationRequest{
			FeedURL:                     feed.FeedURL,
			CategoryID:                  category.ID,
			UserAgent:                   feed.UserAgent,
			Crawler:                     feed.Crawler,
			IgnoreEntryUpdates:          feed.IgnoreEntryUpdates,
			Disabled:                    feed.Disabled,
			NoMediaPlayer:               feed.NoMediaPlayer,
			IgnoreHTTPCache:             feed.IgnoreHTTPCache,
			AllowSelfSignedCertificates: feed.AllowSelfSignedCertificates,
			FetchViaProxy:               feed.FetchViaProxy,
			HideGlobally:                feed.HideGlobally,
			DisableHTTP2:                feed.DisableHTTP2,
			ScraperRules:                feed.ScraperRules,
			RewriteRules:                feed.RewriteRules,
			BlocklistRules:              feed.BlocklistRules,
			KeeplistRules:               feed.KeeplistRules,
			BlockFilterEntryRules:       feed.BlockFilterEntryRules,
			KeepFilterEntryRules:        feed.KeepFilterEntryRules,
			UrlRewriteRules:             feed.UrlRewriteRules,
		}); err != nil {
			return fmt.Errorf("Invalid feed settings for %q: %w", feed.FeedURL, err)
		}

		feed.UserID = userID
		feed.Category = category
		if err := store.CreateFeed(feed); err != nil {
			return fmt.Errorf("Unable to create the feed %q: %w", feed.FeedURL, err)
		}
	}
	return nil
}

// opmlCategory returns the category named title, creating it if needed; an
// empty title means the user's first category (storage.ErrNoCategory if none).
func opmlCategory(store *storage.Storage, userID int64, title string) (*model.Category, error) {
	if title == "" {
		return store.FirstCategory(userID)
	}
	category, err := store.CategoryByTitle(userID, title)
	if err != nil || category != nil {
		return category, err
	}
	return store.CreateCategory(userID, &model.CategoryCreationRequest{Title: title})
}
