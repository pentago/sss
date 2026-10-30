// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"sss/internal/config"
	"sss/internal/http/request"
	"sss/internal/http/response"
	"sss/internal/model"
	"sss/internal/storage"
)

// showEntries renders a paginated entry list. query returns a fresh builder
// with the page filters and sorting; data holds the page specific keys
// (title, menu, entryPath, page actions...).
func (h *handler) showEntries(w http.ResponseWriter, r *http.Request, query func() *storage.EntryQueryBuilder, data map[string]any) {
	user := currentUser(r)
	offset := request.QueryIntParam(r, "offset", 0)
	fetch := func(offset int) (model.Entries, int, error) {
		return query().WithoutContent().WithOffset(offset).WithLimit(user.EntriesPerPage).GetEntriesWithCount()
	}
	entries, total, err := fetch(offset)
	if err == nil && offset >= total && total > 0 {
		// Restart from the first page when marking entries as read leaves the offset past the end.
		offset = 0
		entries, total, err = fetch(offset)
	}
	if err != nil {
		response.HTMLServerError(w, r, err)
		return
	}
	data["entries"] = entries
	data["total"] = total
	data["prevURL"], data["nextURL"] = pageLinks(r, total, offset, user.EntriesPerPage)
	h.render(w, r, "entries", data)
}

// pageLinks returns the previous and next page URLs ("" when absent), keeping the other query parameters.
func pageLinks(r *http.Request, total, offset, limit int) (prev, next string) {
	link := func(offset int) string {
		q := r.URL.Query()
		if offset > 0 {
			q.Set("offset", strconv.Itoa(offset))
		} else {
			q.Del("offset")
		}
		u := url.URL{Path: config.Opts.BasePath + r.URL.Path, RawQuery: q.Encode()}
		return u.String()
	}
	if offset > 0 {
		prev = link(max(offset-limit, 0))
	}
	if total-offset > limit {
		next = link(offset + limit)
	}
	return prev, next
}

func (h *handler) showUnreadPage(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	h.showEntries(w, r, func() *storage.EntryQueryBuilder {
		return h.store.NewEntryQueryBuilder(user.ID).
			WithStatuses(model.EntryStatusUnread).
			WithSorting(user.EntryOrder, user.EntryDirection).
			WithSorting("id", user.EntryDirection).
			WithGloballyVisible()
	}, map[string]any{"title": "Unread", "menu": "unread", "page": "unread", "entryPath": "/unread/entry/%d"})
}

func (h *handler) showStarredPage(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	h.showEntries(w, r, func() *storage.EntryQueryBuilder {
		return h.store.NewEntryQueryBuilder(user.ID).
			WithStarred(true).
			WithSorting(user.EntryOrder, user.EntryDirection).
			WithSorting("id", user.EntryDirection)
	}, map[string]any{"title": "Starred", "menu": "starred", "page": "starred", "entryPath": "/starred/entry/%d"})
}

func (h *handler) showHistoryPage(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	h.showEntries(w, r, func() *storage.EntryQueryBuilder {
		return h.store.NewEntryQueryBuilder(user.ID).
			WithStatuses(model.EntryStatusRead).
			WithSorting("changed_at", "DESC").
			WithSorting("published_at", "DESC")
	}, map[string]any{"title": "History", "menu": "history", "page": "history", "entryPath": "/history/entry/%d"})
}

func (h *handler) sharedEntries(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	h.showEntries(w, r, func() *storage.EntryQueryBuilder {
		return h.store.NewEntryQueryBuilder(user.ID).
			WithShareCodeNotEmpty().
			WithSorting(user.EntryOrder, user.EntryDirection).
			WithSorting("id", user.EntryDirection)
	}, map[string]any{"title": "Shared entries", "menu": "history", "page": "shares", "entryPath": "/history/entry/%d"})
}

func (h *handler) showSearchPage(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	q := request.QueryStringParam(r, "q", "")
	unreadOnly := request.QueryBoolParam(r, "unread", false)
	data := map[string]any{"title": "Search", "menu": "search", "page": "search", "entryPath": "/search/entry/%d",
		"searchQuery": q, "unreadOnly": unreadOnly, "entryQuery": searchEntryQuery(q, unreadOnly)}
	if q == "" {
		data["entries"], data["total"] = model.Entries{}, 0
		h.render(w, r, "entries", data)
		return
	}
	h.showEntries(w, r, func() *storage.EntryQueryBuilder {
		b := h.store.NewEntryQueryBuilder(user.ID).WithSearchQuery(q)
		if unreadOnly {
			b = b.WithStatuses(model.EntryStatusUnread)
		}
		return b
	}, data)
}

// searchEntryQuery is the query string carried by search result entry links.
func searchEntryQuery(q string, unreadOnly bool) string {
	values := url.Values{"q": {q}}
	if unreadOnly {
		values.Set("unread", "1")
	}
	return "?" + values.Encode()
}

func (h *handler) showFeedEntriesPage(w http.ResponseWriter, r *http.Request) {
	h.showFeedEntries(w, r, true)
}

func (h *handler) showFeedEntriesAllPage(w http.ResponseWriter, r *http.Request) {
	h.showFeedEntries(w, r, false)
}

func (h *handler) showFeedEntries(w http.ResponseWriter, r *http.Request, unreadOnly bool) {
	user := currentUser(r)
	feed, err := h.store.FeedByID(user.ID, request.RouteInt64Param(r, "feedID"))
	if err != nil {
		response.HTMLServerError(w, r, err)
		return
	}
	if feed == nil {
		response.HTMLNotFound(w, r)
		return
	}
	entryPath := "/feed/" + strconv.FormatInt(feed.ID, 10) + "/entry/%d"
	if unreadOnly {
		entryPath = "/unread" + entryPath
	}
	h.showEntries(w, r, func() *storage.EntryQueryBuilder {
		b := h.store.NewEntryQueryBuilder(user.ID).
			WithFeedID(feed.ID).
			WithSorting(user.EntryOrder, user.EntryDirection).
			WithSorting("id", user.EntryDirection)
		if unreadOnly {
			b = b.WithStatuses(model.EntryStatusUnread)
		}
		return b
	}, map[string]any{"title": feed.Title, "menu": "feeds", "page": "feed", "feed": feed, "unreadOnly": unreadOnly, "entryPath": entryPath})
}

func (h *handler) showCategoryEntriesPage(w http.ResponseWriter, r *http.Request) {
	h.showCategoryEntries(w, r, "unread")
}

func (h *handler) showCategoryEntriesAllPage(w http.ResponseWriter, r *http.Request) {
	h.showCategoryEntries(w, r, "all")
}

func (h *handler) showCategoryEntriesStarredPage(w http.ResponseWriter, r *http.Request) {
	h.showCategoryEntries(w, r, "starred")
}

// showCategoryEntries lists category entries; filter is "unread", "all" or "starred".
func (h *handler) showCategoryEntries(w http.ResponseWriter, r *http.Request, filter string) {
	user := currentUser(r)
	category, err := h.store.Category(user.ID, request.RouteInt64Param(r, "categoryID"))
	if err != nil {
		response.HTMLServerError(w, r, err)
		return
	}
	if category == nil {
		response.HTMLNotFound(w, r)
		return
	}
	entryPath := "/category/" + strconv.FormatInt(category.ID, 10) + "/entry/%d"
	if filter != "all" {
		entryPath = "/" + filter + entryPath
	}
	h.showEntries(w, r, func() *storage.EntryQueryBuilder {
		b := h.store.NewEntryQueryBuilder(user.ID).
			WithCategoryID(category.ID).
			WithSorting(user.EntryOrder, user.EntryDirection).
			WithSorting("id", user.EntryDirection)
		switch filter {
		case "unread":
			b = b.WithStatuses(model.EntryStatusUnread)
		case "starred":
			b = b.WithStarred(true)
		}
		return b
	}, map[string]any{"title": category.Title, "menu": "categories", "page": "category", "category": category, "filter": filter, "entryPath": entryPath})
}

func (h *handler) showTagEntriesAllPage(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	tag := r.PathValue("tagName")
	h.showEntries(w, r, func() *storage.EntryQueryBuilder {
		return h.store.NewEntryQueryBuilder(user.ID).
			WithTags(tag).
			WithSorting("status", "asc").
			WithSorting(user.EntryOrder, user.EntryDirection).
			WithSorting("id", user.EntryDirection)
	}, map[string]any{"title": tag, "page": "tag", "tag": tag, "entryPath": "/tags/" + strings.ReplaceAll(url.PathEscape(tag), "%", "%%") + "/entry/%d"})
}

func (h *handler) markAllAsRead(w http.ResponseWriter, r *http.Request) {
	if err := h.store.MarkGloballyVisibleFeedsAsRead(request.UserID(r)); err != nil {
		response.HTMLServerError(w, r, err)
		return
	}
	response.HTMLRedirect(w, r, route("/unread"))
}

func (h *handler) flushHistory(w http.ResponseWriter, r *http.Request) {
	if err := h.store.FlushHistory(request.UserID(r)); err != nil {
		response.HTMLServerError(w, r, err)
		return
	}
	response.HTMLRedirect(w, r, route("/history"))
}
