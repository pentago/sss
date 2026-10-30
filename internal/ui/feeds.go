// Copyright The Miniflux Authors. Modified by The sss Authors.
// Derived from Miniflux (https://github.com/miniflux/v2).
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"sss/internal/config"
	"sss/internal/http/request"
	"sss/internal/http/response"
	"sss/internal/model"
	feedHandler "sss/internal/reader/handler"
	"sss/internal/validator"
)

func (h *handler) showFeedsPage(w http.ResponseWriter, r *http.Request) {
	feeds, err := h.store.FeedsWithCounters(request.UserID(r))
	if err != nil {
		response.HTMLServerError(w, r, err)
		return
	}
	h.render(w, r, "feeds", map[string]any{"title": "Feeds", "menu": "feeds", "feeds": feeds})
}

func (h *handler) refreshAllFeeds(w http.ResponseWriter, r *http.Request) {
	if h.refreshFeeds(w, r, 0) {
		response.HTMLRedirect(w, r, route("/feeds"))
	}
}

// refreshFeeds queues a refresh of the user's feeds (of one category when
// categoryID != 0), at most once per FORCE_REFRESH_INTERVAL. It reports
// false when a server error response has been written.
func (h *handler) refreshFeeds(w http.ResponseWriter, r *http.Request, categoryID int64) bool {
	session := request.WebSession(r)
	if time.Since(session.LastForceRefresh()) < config.Opts.ForceRefreshInterval {
		session.SetErrorMessage(fmt.Sprintf("You have refreshed your feeds too recently. Please wait %s between manual refreshes.",
			plural(int(config.Opts.ForceRefreshInterval.Minutes()), "minute", "minutes")))
		return true
	}
	builder := h.store.NewBatchBuilder().
		WithoutDisabledFeeds().
		WithUserID(request.UserID(r)).
		WithLimitPerHost(config.Opts.PollingLimitPerHost)
	if categoryID != 0 {
		builder = builder.WithCategoryID(categoryID)
	}
	jobs, err := builder.FetchJobs()
	if err != nil {
		response.HTMLServerError(w, r, err)
		return false
	}
	slog.Info("Triggered a manual refresh of feeds from the web ui",
		slog.Int64("user_id", request.UserID(r)), slog.Int64("category_id", categoryID), slog.Int("nb_jobs", len(jobs)))
	go h.pool.Push(jobs)
	session.MarkForceRefreshed()
	session.SetSuccessMessage("Feeds are being refreshed in the background.")
	return true
}

func (h *handler) refreshFeed(w http.ResponseWriter, r *http.Request) {
	feedID := request.RouteInt64Param(r, "feedID")
	forceRefresh := request.QueryBoolParam(r, "forceRefresh", false)
	if err := feedHandler.RefreshFeed(h.store, request.UserID(r), feedID, forceRefresh); err != nil {
		slog.Warn("Unable to refresh feed", slog.Int64("user_id", request.UserID(r)), slog.Int64("feed_id", feedID), slog.Any("error", err))
		request.WebSession(r).SetErrorMessage(err.Error())
	}
	h.redirectBack(w, r, fmt.Sprintf("/feed/%d/entries", feedID))
}

func (h *handler) markFeedAsRead(w http.ResponseWriter, r *http.Request) {
	userID, feedID := request.UserID(r), request.RouteInt64Param(r, "feedID")
	checkedAt, err := h.store.CheckedAt(userID, feedID)
	if err != nil {
		response.HTMLNotFound(w, r)
		return
	}
	if err := h.store.MarkFeedAsRead(userID, feedID, checkedAt); err != nil {
		response.HTMLServerError(w, r, err)
		return
	}
	h.redirectBack(w, r, "/feeds")
}

func (h *handler) removeFeed(w http.ResponseWriter, r *http.Request) {
	userID, feedID := request.UserID(r), request.RouteInt64Param(r, "feedID")
	exists, err := h.store.FeedExists(userID, feedID)
	if err != nil {
		response.HTMLServerError(w, r, err)
		return
	}
	if !exists {
		response.HTMLNotFound(w, r)
		return
	}
	if err := h.store.RemoveFeed(userID, feedID); err != nil {
		response.HTMLServerError(w, r, err)
		return
	}
	response.HTMLRedirect(w, r, route("/feeds"))
}

func (h *handler) showEditFeedPage(w http.ResponseWriter, r *http.Request) {
	feed, err := h.store.FeedByID(request.UserID(r), request.RouteInt64Param(r, "feedID"))
	if err != nil {
		response.HTMLServerError(w, r, err)
		return
	}
	if feed == nil {
		response.HTMLNotFound(w, r)
		return
	}
	h.renderEditFeed(w, r, feed, "")
}

func (h *handler) renderEditFeed(w http.ResponseWriter, r *http.Request, feed *model.Feed, errorMessage string) {
	categories, err := h.store.Categories(request.UserID(r))
	if err != nil {
		response.HTMLServerError(w, r, err)
		return
	}
	h.render(w, r, "feed_edit", map[string]any{
		"title": "Edit feed: " + feed.Title, "menu": "feeds", "feed": feed, "categories": categories,
		"errorMessage": errorMessage, "defaultUserAgent": config.Opts.HTTPClientUserAgent,
		"hasProxyConfigured": config.Opts.HTTPClientProxy != nil,
	})
}

func (h *handler) updateFeed(w http.ResponseWriter, r *http.Request) {
	userID := request.UserID(r)
	feed, err := h.store.FeedByID(userID, request.RouteInt64Param(r, "feedID"))
	if err != nil {
		response.HTMLServerError(w, r, err)
		return
	}
	if feed == nil {
		response.HTMLNotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		response.HTMLBadRequest(w, r, err)
		return
	}
	f := r.PostForm
	categoryID, _ := strconv.ParseInt(f.Get("category_id"), 10, 64)
	modification := &model.FeedModificationRequest{
		FeedURL:               model.OptionalString(f.Get("feed_url")),
		SiteURL:               model.OptionalString(f.Get("site_url")),
		Title:                 model.OptionalString(f.Get("title")),
		Description:           model.OptionalString(f.Get("description")),
		CategoryID:            model.OptionalNumber(categoryID),
		BlocklistRules:        model.OptionalString(f.Get("blocklist_rules")),
		KeeplistRules:         model.OptionalString(f.Get("keeplist_rules")),
		ScraperRules:          model.OptionalString(f.Get("scraper_rules")),
		RewriteRules:          model.OptionalString(f.Get("rewrite_rules")),
		UrlRewriteRules:       model.OptionalString(f.Get("urlrewrite_rules")),
		ProxyURL:              model.OptionalString(f.Get("proxy_url")),
		BlockFilterEntryRules: model.OptionalString(f.Get("block_filter_entry_rules")),
		KeepFilterEntryRules:  model.OptionalString(f.Get("keep_filter_entry_rules")),
	}

	feed.FeedURL = f.Get("feed_url")
	feed.SiteURL = f.Get("site_url")
	feed.Title = f.Get("title")
	feed.Description = f.Get("description")
	feed.WithCategoryID(categoryID)
	feed.ScraperRules = f.Get("scraper_rules")
	feed.RewriteRules = f.Get("rewrite_rules")
	feed.UrlRewriteRules = f.Get("urlrewrite_rules")
	feed.BlocklistRules = f.Get("blocklist_rules")
	feed.KeeplistRules = f.Get("keeplist_rules")
	feed.BlockFilterEntryRules = f.Get("block_filter_entry_rules")
	feed.KeepFilterEntryRules = f.Get("keep_filter_entry_rules")
	feed.UserAgent = f.Get("user_agent")
	feed.Cookie = f.Get("cookie")
	feed.Username = f.Get("feed_username")
	feed.Password = f.Get("feed_password")
	feed.ProxyURL = f.Get("proxy_url")
	feed.AppriseServiceURLs = f.Get("apprise_service_urls")
	feed.WebhookURL = f.Get("webhook_url")
	feed.NtfyTopic = f.Get("ntfy_topic")
	feed.NtfyPriority, _ = strconv.Atoi(f.Get("ntfy_priority"))
	feed.PushoverPriority, _ = strconv.Atoi(f.Get("pushover_priority"))
	checked := func(name string) bool { return f.Get(name) == "1" }
	feed.Crawler = checked("crawler")
	feed.IgnoreEntryUpdates = checked("ignore_entry_updates")
	feed.IgnoreHTTPCache = checked("ignore_http_cache")
	feed.AllowSelfSignedCertificates = checked("allow_self_signed_certificates")
	feed.FetchViaProxy = checked("fetch_via_proxy")
	feed.Disabled = checked("disabled")
	feed.NoMediaPlayer = checked("no_media_player")
	feed.HideGlobally = checked("hide_globally")
	feed.DisableHTTP2 = checked("disable_http2")
	feed.NtfyEnabled = checked("ntfy_enabled")
	feed.PushoverEnabled = checked("pushover_enabled")

	if err := validator.ValidateFeedModification(h.store, userID, feed.ID, modification); err != nil {
		h.renderEditFeed(w, r, feed, err.Error())
		return
	}
	feed.ResetErrorCounter()
	if err := h.store.UpdateFeed(feed); err != nil {
		response.HTMLServerError(w, r, err)
		return
	}
	response.HTMLRedirect(w, r, route("/feed/%d/entries", feed.ID))
}
