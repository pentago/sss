// Copyright The Miniflux Authors. Modified by The sss Authors.
// Derived from Miniflux (https://github.com/miniflux/v2).
// SPDX-License-Identifier: Apache-2.0

// Package fever implements the Fever API.
package fever

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"sss/internal/http/request"
	"sss/internal/http/response"
	"sss/internal/integration"
	"sss/internal/mediaproxy"
	"sss/internal/model"
	"sss/internal/storage"
)

// NewHandler returns the authenticated Fever API handler, mounted at /fever/.
func NewHandler(store *storage.Storage) http.Handler {
	h := &handler{store: store}
	return http.HandlerFunc(h.authenticate)
}

type handler struct {
	store *storage.Storage
}

// authenticate resolves the api_key (md5 of "username:password") to a user and serves the request.
func (h *handler) authenticate(w http.ResponseWriter, r *http.Request) {
	apiKey := r.FormValue("api_key")
	if apiKey == "" {
		slog.Warn("[Fever] No API key provided",
			slog.Bool("authentication_failed", true),
			slog.String("client_ip", request.ClientIP(r)),
			slog.String("user_agent", r.UserAgent()),
		)
		response.JSON(w, r, newAuthFailureResponse())
		return
	}

	user, err := h.store.UserByFeverToken(apiKey)
	if err != nil || user == nil {
		slog.Warn("[Fever] Invalid API key",
			slog.Bool("authentication_failed", true),
			slog.String("client_ip", request.ClientIP(r)),
			slog.String("user_agent", r.UserAgent()),
			slog.Any("error", err),
		)
		response.JSON(w, r, newAuthFailureResponse())
		return
	}

	h.store.SetLastLogin(user.ID)

	ctx := r.Context()
	ctx = context.WithValue(ctx, request.UserIDContextKey, user.ID)
	ctx = context.WithValue(ctx, request.UserTimezoneContextKey, user.Timezone)
	ctx = context.WithValue(ctx, request.IsAdminUserContextKey, user.IsAdmin)
	ctx = context.WithValue(ctx, request.IsAuthenticatedContextKey, true)
	h.serve(w, r.WithContext(ctx))
}

func (h *handler) serve(w http.ResponseWriter, r *http.Request) {
	switch {
	case request.HasQueryParam(r, "groups"):
		h.handleGroups(w, r)
	case request.HasQueryParam(r, "feeds"):
		h.handleFeeds(w, r)
	case request.HasQueryParam(r, "favicons"):
		h.handleFavicons(w, r)
	case request.HasQueryParam(r, "unread_item_ids"):
		h.handleUnreadItems(w, r)
	case request.HasQueryParam(r, "saved_item_ids"):
		h.handleSavedItems(w, r)
	case request.HasQueryParam(r, "items"):
		h.handleItems(w, r)
	case request.HasQueryParam(r, "links"):
		// Hot links ("Sparks") are not supported: always empty.
		result := linksResponse{Links: []struct{}{}}
		result.SetCommonValues()
		response.JSON(w, r, result)
	case r.FormValue("mark") == "item":
		h.handleWriteItems(w, r)
	case r.FormValue("mark") == "feed":
		h.handleWriteFeeds(w, r)
	case r.FormValue("mark") == "group":
		h.handleWriteGroups(w, r)
	default:
		response.JSON(w, r, newBaseResponse())
	}
}

func (h *handler) handleGroups(w http.ResponseWriter, r *http.Request) {
	userID := request.UserID(r)
	categories, err := h.store.Categories(userID)
	if err != nil {
		response.JSONServerError(w, r, err)
		return
	}

	feeds, err := h.store.Feeds(userID)
	if err != nil {
		response.JSONServerError(w, r, err)
		return
	}

	var result groupsResponse
	result.Groups = make([]group, 0, len(categories))
	for _, category := range categories {
		result.Groups = append(result.Groups, group{ID: category.ID, Title: category.Title})
	}
	result.FeedsGroups = buildFeedGroups(feeds)
	result.SetCommonValues()
	response.JSON(w, r, result)
}

func (h *handler) handleFeeds(w http.ResponseWriter, r *http.Request) {
	feeds, err := h.store.Feeds(request.UserID(r))
	if err != nil {
		response.JSONServerError(w, r, err)
		return
	}

	var result feedsResponse
	result.Feeds = make([]feed, 0, len(feeds))
	for _, f := range feeds {
		subscription := feed{
			ID:          f.ID,
			Title:       f.Title,
			URL:         f.FeedURL,
			SiteURL:     f.SiteURL,
			LastUpdated: f.CheckedAt.Unix(),
		}
		if f.Icon != nil {
			subscription.FaviconID = f.Icon.IconID
		}
		result.Feeds = append(result.Feeds, subscription)
	}
	result.FeedsGroups = buildFeedGroups(feeds)
	result.SetCommonValues()
	response.JSON(w, r, result)
}

func (h *handler) handleFavicons(w http.ResponseWriter, r *http.Request) {
	icons, err := h.store.Icons(request.UserID(r))
	if err != nil {
		response.JSONServerError(w, r, err)
		return
	}

	var result faviconsResponse
	result.Favicons = make([]favicon, 0, len(icons))
	for _, i := range icons {
		result.Favicons = append(result.Favicons, favicon{ID: i.ID, Data: i.DataURL()})
	}
	result.SetCommonValues()
	response.JSON(w, r, result)
}

// handleItems returns at most 50 items: after since_id (ascending), before max_id (descending),
// the listed with_ids, or the oldest items.
func (h *handler) handleItems(w http.ResponseWriter, r *http.Request) {
	userID := request.UserID(r)
	builder := h.store.NewEntryQueryBuilder(userID).WithLimit(50)

	switch {
	case request.HasQueryParam(r, "since_id"):
		if sinceID := request.QueryInt64Param(r, "since_id", 0); sinceID > 0 {
			builder = builder.AfterEntryID(sinceID)
		}
		builder = builder.WithSorting("id", "ASC")
	case request.HasQueryParam(r, "max_id"):
		if maxID := request.QueryInt64Param(r, "max_id", 0); maxID > 0 {
			builder = builder.BeforeEntryID(maxID)
		}
		builder = builder.WithSorting("id", "DESC")
	case request.HasQueryParam(r, "with_ids"):
		if csvItemIDs := request.QueryStringParam(r, "with_ids", ""); csvItemIDs != "" {
			var itemIDs []int64
			for strItemID := range strings.SplitSeq(csvItemIDs, ",") {
				itemID, _ := strconv.ParseInt(strings.TrimSpace(strItemID), 10, 64)
				itemIDs = append(itemIDs, itemID)
			}
			builder = builder.WithEntryIDs(itemIDs...)
		}
		builder = builder.WithSorting("id", "ASC")
	default:
		builder = builder.WithSorting("id", "ASC")
	}

	entries, err := builder.GetEntries()
	if err != nil {
		response.JSONServerError(w, r, err)
		return
	}

	var result itemsResponse
	result.Total, err = h.store.NewEntryQueryBuilder(userID).CountEntries()
	if err != nil {
		response.JSONServerError(w, r, err)
		return
	}

	result.Items = make([]item, 0, len(entries))
	for _, entry := range entries {
		result.Items = append(result.Items, item{
			ID:        entry.ID,
			FeedID:    entry.FeedID,
			Title:     entry.Title,
			Author:    entry.Author,
			HTML:      mediaproxy.RewriteDocumentWithAbsoluteProxyURL(entry.Content),
			URL:       entry.URL,
			IsSaved:   boolInt(entry.Starred),
			IsRead:    boolInt(entry.Status == model.EntryStatusRead),
			CreatedAt: entry.Date.Unix(),
		})
	}
	result.SetCommonValues()
	response.JSON(w, r, result)
}

func (h *handler) handleUnreadItems(w http.ResponseWriter, r *http.Request) {
	entryIDs, err := h.store.NewEntryQueryBuilder(request.UserID(r)).
		WithStatuses(model.EntryStatusUnread).
		GetEntryIDs()
	if err != nil {
		response.JSONServerError(w, r, err)
		return
	}

	result := unreadResponse{ItemIDs: joinIDs(entryIDs)}
	result.SetCommonValues()
	response.JSON(w, r, result)
}

func (h *handler) handleSavedItems(w http.ResponseWriter, r *http.Request) {
	entryIDs, err := h.store.NewEntryQueryBuilder(request.UserID(r)).
		WithStarred(true).
		GetEntryIDs()
	if err != nil {
		response.JSONServerError(w, r, err)
		return
	}

	result := savedResponse{ItemIDs: joinIDs(entryIDs)}
	result.SetCommonValues()
	response.JSON(w, r, result)
}

// handleWriteItems handles mark=item&as=read|unread|saved|unsaved&id=N.
func (h *handler) handleWriteItems(w http.ResponseWriter, r *http.Request) {
	userID := request.UserID(r)
	entryID := request.FormInt64Value(r, "id")
	if entryID <= 0 {
		return
	}

	entry, err := h.store.NewEntryQueryBuilder(userID).WithEntryIDs(entryID).GetEntry()
	if err != nil {
		response.JSONServerError(w, r, err)
		return
	}
	if entry == nil {
		response.JSON(w, r, newBaseResponse())
		return
	}

	switch r.FormValue("as") {
	case "read":
		err = h.store.SetEntriesStatus(userID, []int64{entryID}, model.EntryStatusRead)
	case "unread":
		err = h.store.SetEntriesStatus(userID, []int64{entryID}, model.EntryStatusUnread)
	case "saved":
		if err = h.store.SetEntriesStarredState(userID, []int64{entryID}, true); err != nil {
			break
		}
		var settings *model.Integration
		if settings, err = h.store.Integration(userID); err == nil {
			go integration.SendEntry(entry, settings)
		}
	case "unsaved":
		err = h.store.SetEntriesStarredState(userID, []int64{entryID}, false)
	}
	if err != nil {
		response.JSONServerError(w, r, err)
		return
	}

	response.JSON(w, r, newBaseResponse())
}

// handleWriteFeeds handles mark=feed&as=read&id=N&before=UNIX.
func (h *handler) handleWriteFeeds(w http.ResponseWriter, r *http.Request) {
	feedID := request.FormInt64Value(r, "id")
	if feedID <= 0 {
		return
	}

	before := time.Unix(request.FormInt64Value(r, "before"), 0)
	if err := h.store.MarkFeedAsRead(request.UserID(r), feedID, before); err != nil {
		response.JSONServerError(w, r, err)
		return
	}

	response.JSON(w, r, newBaseResponse())
}

// handleWriteGroups handles mark=group&as=read&id=N&before=UNIX; group 0 ("Kindling") is everything.
func (h *handler) handleWriteGroups(w http.ResponseWriter, r *http.Request) {
	userID := request.UserID(r)
	groupID := request.FormInt64Value(r, "id")
	if groupID < 0 {
		return
	}

	var err error
	if groupID == 0 {
		err = h.store.MarkAllAsRead(userID)
	} else {
		err = h.store.MarkCategoryAsRead(userID, groupID, time.Unix(request.FormInt64Value(r, "before"), 0))
	}
	if err != nil {
		response.JSONServerError(w, r, err)
		return
	}

	response.JSON(w, r, newBaseResponse())
}

func buildFeedGroups(feeds model.Feeds) []feedsGroups {
	var categoryIDs []int64
	feedIDsByCategory := make(map[int64][]int64)
	for _, f := range feeds {
		if _, ok := feedIDsByCategory[f.Category.ID]; !ok {
			categoryIDs = append(categoryIDs, f.Category.ID)
		}
		feedIDsByCategory[f.Category.ID] = append(feedIDsByCategory[f.Category.ID], f.ID)
	}

	result := make([]feedsGroups, 0, len(categoryIDs))
	for _, categoryID := range categoryIDs {
		result = append(result, feedsGroups{GroupID: categoryID, FeedIDs: joinIDs(feedIDsByCategory[categoryID])})
	}
	return result
}

func joinIDs(ids []int64) string {
	s := make([]string, len(ids))
	for i, id := range ids {
		s[i] = strconv.FormatInt(id, 10)
	}
	return strings.Join(s, ",")
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
