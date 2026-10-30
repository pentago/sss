// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

package validator

import (
	"errors"
	"fmt"
	"log/slog"

	"sss/internal/model"
	"sss/internal/reader/rewrite"
	"sss/internal/reader/scraper"
	"sss/internal/urllib"
)

var (
	errInvalidFeedURL           = errors.New("Invalid feed URL.")
	errInvalidSiteURL           = errors.New("Invalid site URL.")
	errFeedAlreadyExists        = errors.New("This feed already exists.")
	errFeedCategoryNotFound     = errors.New("This category does not exist or does not belong to this user.")
	errFeedInvalidBlocklistRule = errors.New("The block list rule is invalid.")
	errFeedInvalidKeeplistRule  = errors.New("The keep list rule is invalid.")
	errInvalidFeedProxyURL      = errors.New("Invalid proxy URL.")
	errFeedInvalidScraperRules  = errors.New("The scraper rules are invalid.")
	errFeedInvalidURLRewrite    = errors.New("The URL rewrite rules are invalid.")
)

// ValidateFeedCreation validates feed creation.
func ValidateFeedCreation(store Store, userID int64, request *model.FeedCreationRequest) error {
	if request.FeedURL == "" || request.CategoryID <= 0 {
		return errors.New("The URL and the category are mandatory.")
	}

	if !urllib.IsAbsoluteURL(request.FeedURL) {
		return errInvalidFeedURL
	}

	if store.FeedURLExists(userID, request.FeedURL) {
		return errFeedAlreadyExists
	}

	if err := validateCategoryID(store, userID, request.CategoryID); err != nil {
		return err
	}

	if err := validateFeedRules(request.ScraperRules, request.RewriteRules, request.UrlRewriteRules); err != nil {
		return err
	}

	if !IsValidRegex(request.BlocklistRules) {
		return errFeedInvalidBlocklistRule
	}

	if !IsValidRegex(request.KeeplistRules) {
		return errFeedInvalidKeeplistRule
	}

	if request.BlockFilterEntryRules != "" {
		if err := IsValidFilterRules(request.BlockFilterEntryRules, "block"); err != nil {
			return err
		}
	}

	if request.KeepFilterEntryRules != "" {
		if err := IsValidFilterRules(request.KeepFilterEntryRules, "keep"); err != nil {
			return err
		}
	}

	if request.ProxyURL != "" && !urllib.IsValidProxyURL(request.ProxyURL) {
		return errInvalidFeedProxyURL
	}

	return nil
}

// ValidateFeedModification validates feed modification.
func ValidateFeedModification(store Store, userID, feedID int64, request *model.FeedModificationRequest) error {
	if request.FeedURL != nil {
		if *request.FeedURL == "" {
			return errors.New("The feed URL cannot be empty.")
		}

		if !urllib.IsAbsoluteURL(*request.FeedURL) {
			return errInvalidFeedURL
		}

		if store.AnotherFeedURLExists(userID, feedID, *request.FeedURL) {
			return errFeedAlreadyExists
		}
	}

	if request.SiteURL != nil {
		if *request.SiteURL == "" {
			return errors.New("The site URL cannot be empty.")
		}

		if !urllib.IsAbsoluteURL(*request.SiteURL) {
			return errInvalidSiteURL
		}
	}

	if request.Title != nil && *request.Title == "" {
		return errors.New("The feed title cannot be empty.")
	}

	if request.CategoryID != nil {
		if err := validateCategoryID(store, userID, *request.CategoryID); err != nil {
			return err
		}
	}

	if err := validateFeedRules(deref(request.ScraperRules), deref(request.RewriteRules), deref(request.UrlRewriteRules)); err != nil {
		return err
	}

	if request.BlocklistRules != nil && !IsValidRegex(*request.BlocklistRules) {
		return errFeedInvalidBlocklistRule
	}

	if request.KeeplistRules != nil && !IsValidRegex(*request.KeeplistRules) {
		return errFeedInvalidKeeplistRule
	}

	if request.BlockFilterEntryRules != nil && *request.BlockFilterEntryRules != "" {
		if err := IsValidFilterRules(*request.BlockFilterEntryRules, "block"); err != nil {
			return err
		}
	}

	if request.KeepFilterEntryRules != nil && *request.KeepFilterEntryRules != "" {
		if err := IsValidFilterRules(*request.KeepFilterEntryRules, "keep"); err != nil {
			return err
		}
	}

	if request.ProxyURL != nil && *request.ProxyURL != "" && !urllib.IsValidProxyURL(*request.ProxyURL) {
		return errInvalidFeedProxyURL
	}

	return nil
}

// validateCategoryID checks the category belongs to the user. Storage
// failures are logged and reported as a missing category.
func validateCategoryID(store Store, userID, categoryID int64) error {
	exists, err := store.CategoryIDExists(userID, categoryID)
	if err != nil {
		slog.Error("validator: unable to check if feed category exists",
			slog.Int64("user_id", userID),
			slog.Int64("category_id", categoryID),
			slog.Any("error", err),
		)
	}
	if !exists {
		return errFeedCategoryNotFound
	}
	return nil
}

// validateFeedRules checks the scraper, content rewrite and URL rewrite rules.
func validateFeedRules(scraperRules, rewriteRules, urlRewriteRules string) error {
	if scraperRules != "" && !scraper.IsValidScraperRules(scraperRules) {
		return errFeedInvalidScraperRules
	}
	if errs := rewrite.ValidateRules(rewriteRules); len(errs) > 0 {
		e := errs[0]
		if e.Message != "" {
			return fmt.Errorf("Invalid rewrite rule %q: %s.", e.Rule, e.Message)
		}
		return fmt.Errorf("Invalid rewrite rule %q.", e.Rule)
	}
	if urlRewriteRules != "" && !rewrite.IsValidURLRewriteRules(urlRewriteRules) {
		return errFeedInvalidURLRewrite
	}
	return nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
