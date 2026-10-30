// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"sss/internal/config"
	"sss/internal/http/request"
	"sss/internal/http/response"
	"sss/internal/integration"
	"sss/internal/model"
	"sss/internal/reader/processor"
	"sss/internal/storage"
	"sss/internal/validator"
)

// entryView describes the list an entry page belongs to.
type entryView struct {
	menu     string
	prefix   string // route prefix of the entry pages, e.g. "/unread/feed/4"
	query    string // query string kept on prev/next links (search)
	fallback string // where to go when the entry is not in the list
	// filter restricts the entry lookup and the pagination to the list.
	filter func(q *storage.EntryQueryBuilder, p *storage.EntryPaginationBuilder) (*storage.EntryQueryBuilder, *storage.EntryPaginationBuilder)
	// unread keeps the pagination over unread entries even when the entry is read.
	unread bool
	order  string // overrides the user sorting order (history)
}

func (h *handler) showEntry(w http.ResponseWriter, r *http.Request, v entryView) {
	user := currentUser(r)
	entryID := request.RouteInt64Param(r, "entryID")
	order, direction := user.EntryOrder, user.EntryDirection
	if v.order != "" {
		order, direction = v.order, "desc"
	}
	q, p := v.filter(
		h.store.NewEntryQueryBuilder(user.ID).WithEntryIDs(entryID).WithEnclosures(),
		h.store.NewEntryPaginationBuilder(user.ID, entryID, order, direction),
	)
	entry, err := q.GetEntry()
	if err != nil {
		response.HTMLServerError(w, r, err)
		return
	}
	if entry == nil {
		response.HTMLRedirect(w, r, config.Opts.BasePath+v.fallback)
		return
	}

	// Coming back to the page after one of its forms (e.g. "Mark as unread")
	// must not mark it as read again.
	markRead := entry.ShouldMarkAsReadOnView(user) && !refererIsPage(r)
	if v.unread && entry.Status == model.EntryStatusRead {
		// Keep the pagination in unread mode: temporarily restore the unread status.
		if err := h.store.SetEntriesStatus(user.ID, []int64{entry.ID}, model.EntryStatusUnread); err != nil {
			response.HTMLServerError(w, r, err)
			return
		}
	}
	prev, next, err := p.Entries()
	if err != nil {
		response.HTMLServerError(w, r, err)
		return
	}
	if markRead {
		entry.Status = model.EntryStatusRead
	}
	if markRead || (v.unread && entry.Status == model.EntryStatusRead) {
		if err := h.store.SetEntriesStatus(user.ID, []int64{entry.ID}, model.EntryStatusRead); err != nil {
			response.HTMLServerError(w, r, err)
			return
		}
	}

	if user.AlwaysOpenExternalLinks {
		response.HTMLRedirect(w, r, entry.URL)
		return
	}

	data := map[string]any{"title": entry.Title, "menu": v.menu, "entry": entry}
	if prev != nil {
		data["prevURL"] = route("%s/entry/%d%s", v.prefix, prev.ID, v.query)
	}
	if next != nil {
		data["nextURL"] = route("%s/entry/%d%s", v.prefix, next.ID, v.query)
	}
	h.render(w, r, "entry", data)
}

// refererIsPage reports whether the request comes from the same page, i.e. a
// redirect back after a form submission or a reload.
func refererIsPage(r *http.Request) bool {
	ref, err := url.Parse(r.Referer())
	return err == nil && ref.Host == r.Host && ref.Path == config.Opts.BasePath+r.URL.Path
}

func noFilter(q *storage.EntryQueryBuilder, p *storage.EntryPaginationBuilder) (*storage.EntryQueryBuilder, *storage.EntryPaginationBuilder) {
	return q, p
}

func (h *handler) showUnreadEntryPage(w http.ResponseWriter, r *http.Request) {
	h.showEntry(w, r, entryView{menu: "unread", prefix: "/unread", fallback: "/unread", unread: true,
		filter: func(q *storage.EntryQueryBuilder, p *storage.EntryPaginationBuilder) (*storage.EntryQueryBuilder, *storage.EntryPaginationBuilder) {
			return q, p.WithStatus(model.EntryStatusUnread).WithGloballyVisible()
		}})
}

func (h *handler) showStarredEntryPage(w http.ResponseWriter, r *http.Request) {
	h.showEntry(w, r, entryView{menu: "starred", prefix: "/starred", fallback: "/starred",
		filter: func(q *storage.EntryQueryBuilder, p *storage.EntryPaginationBuilder) (*storage.EntryQueryBuilder, *storage.EntryPaginationBuilder) {
			return q, p.WithStarred()
		}})
}

func (h *handler) showHistoryEntryPage(w http.ResponseWriter, r *http.Request) {
	h.showEntry(w, r, entryView{menu: "history", prefix: "/history", fallback: "/history", order: "changed_at",
		filter: func(q *storage.EntryQueryBuilder, p *storage.EntryPaginationBuilder) (*storage.EntryQueryBuilder, *storage.EntryPaginationBuilder) {
			return q, p.WithStatus(model.EntryStatusRead)
		}})
}

func (h *handler) showSearchEntryPage(w http.ResponseWriter, r *http.Request) {
	q := request.QueryStringParam(r, "q", "")
	unreadOnly := request.QueryBoolParam(r, "unread", false)
	entryID := request.RouteInt64Param(r, "entryID")
	h.showEntry(w, r, entryView{menu: "search", prefix: "/search", query: searchEntryQuery(q, unreadOnly), fallback: "/search",
		filter: func(qb *storage.EntryQueryBuilder, p *storage.EntryPaginationBuilder) (*storage.EntryQueryBuilder, *storage.EntryPaginationBuilder) {
			p = p.WithSearchQuery(q)
			if unreadOnly {
				p = p.WithStatusOrEntryID(model.EntryStatusUnread, entryID)
			}
			return qb.WithSearchQuery(q), p
		}})
}

func (h *handler) showFeedEntryPage(w http.ResponseWriter, r *http.Request) {
	h.showFeedEntry(w, r, false)
}

func (h *handler) showUnreadFeedEntryPage(w http.ResponseWriter, r *http.Request) {
	h.showFeedEntry(w, r, true)
}

func (h *handler) showFeedEntry(w http.ResponseWriter, r *http.Request, unread bool) {
	feedID := request.RouteInt64Param(r, "feedID")
	prefix := "/feed/" + strconv.FormatInt(feedID, 10)
	if unread {
		prefix = "/unread" + prefix
	}
	h.showEntry(w, r, entryView{menu: "feeds", prefix: prefix, fallback: "/feed/" + strconv.FormatInt(feedID, 10) + "/entries", unread: unread,
		filter: func(q *storage.EntryQueryBuilder, p *storage.EntryPaginationBuilder) (*storage.EntryQueryBuilder, *storage.EntryPaginationBuilder) {
			p = p.WithFeedID(feedID)
			if unread {
				p = p.WithStatus(model.EntryStatusUnread)
			}
			return q.WithFeedID(feedID), p
		}})
}

func (h *handler) showCategoryEntryPage(w http.ResponseWriter, r *http.Request) {
	h.showCategoryEntry(w, r, "all")
}

func (h *handler) showUnreadCategoryEntryPage(w http.ResponseWriter, r *http.Request) {
	h.showCategoryEntry(w, r, "unread")
}

func (h *handler) showStarredCategoryEntryPage(w http.ResponseWriter, r *http.Request) {
	h.showCategoryEntry(w, r, "starred")
}

// showCategoryEntry shows a category entry; filter is "all", "unread" or "starred".
func (h *handler) showCategoryEntry(w http.ResponseWriter, r *http.Request, filter string) {
	categoryID := request.RouteInt64Param(r, "categoryID")
	prefix := "/category/" + strconv.FormatInt(categoryID, 10)
	if filter != "all" {
		prefix = "/" + filter + prefix
	}
	h.showEntry(w, r, entryView{menu: "categories", prefix: prefix, fallback: "/category/" + strconv.FormatInt(categoryID, 10) + "/entries", unread: filter == "unread",
		filter: func(q *storage.EntryQueryBuilder, p *storage.EntryPaginationBuilder) (*storage.EntryQueryBuilder, *storage.EntryPaginationBuilder) {
			p = p.WithCategoryID(categoryID)
			switch filter {
			case "unread":
				p = p.WithStatus(model.EntryStatusUnread)
			case "starred":
				p = p.WithStarred()
			}
			return q.WithCategoryID(categoryID), p
		}})
}

func (h *handler) showTagEntryPage(w http.ResponseWriter, r *http.Request) {
	tag := r.PathValue("tagName")
	escaped := strings.ReplaceAll(url.PathEscape(tag), "%", "%%")
	h.showEntry(w, r, entryView{prefix: "/tags/" + escaped, fallback: "/unread",
		filter: func(q *storage.EntryQueryBuilder, p *storage.EntryPaginationBuilder) (*storage.EntryQueryBuilder, *storage.EntryPaginationBuilder) {
			return q.WithTags(tag), p.WithTags([]string{tag})
		}})
}

// updateEntriesStatus handles POST /entry/status with repeated "id" fields
// and a "status" field (read or unread).
func (h *handler) updateEntriesStatus(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		response.HTMLBadRequest(w, r, err)
		return
	}
	req := model.EntriesStatusUpdateRequest{Status: r.PostForm.Get("status")}
	for _, value := range r.PostForm["id"] {
		id, err := strconv.ParseInt(value, 10, 64)
		if err != nil || id <= 0 {
			response.HTMLBadRequest(w, r, err)
			return
		}
		req.EntryIDs = append(req.EntryIDs, id)
	}
	if len(req.EntryIDs) > 0 {
		if err := validator.ValidateEntriesStatusUpdateRequest(&req); err != nil {
			response.HTMLBadRequest(w, r, err)
			return
		}
		if err := h.store.SetEntriesStatus(request.UserID(r), req.EntryIDs, req.Status); err != nil {
			response.HTMLServerError(w, r, err)
			return
		}
	}
	h.redirectBack(w, r, "/unread")
}

func (h *handler) toggleStarred(w http.ResponseWriter, r *http.Request) {
	if err := h.store.ToggleStarred(request.UserID(r), request.RouteInt64Param(r, "entryID")); err != nil {
		response.HTMLServerError(w, r, err)
		return
	}
	h.redirectBack(w, r, "/starred")
}

func (h *handler) saveEntry(w http.ResponseWriter, r *http.Request) {
	userID := request.UserID(r)
	entry, err := h.store.NewEntryQueryBuilder(userID).WithEntryIDs(request.RouteInt64Param(r, "entryID")).GetEntry()
	if err != nil {
		response.HTMLServerError(w, r, err)
		return
	}
	if entry == nil {
		response.HTMLNotFound(w, r)
		return
	}
	integrations, err := h.store.Integration(userID)
	if err != nil {
		response.HTMLServerError(w, r, err)
		return
	}
	go integration.SendEntry(entry, integrations)
	request.WebSession(r).SetSuccessMessage("The entry has been sent to your integrations.")
	h.redirectBack(w, r, "/unread")
}

// fetchContent replaces the entry content with the original web page content.
func (h *handler) fetchContent(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	entry, err := h.store.NewEntryQueryBuilder(user.ID).WithEntryIDs(request.RouteInt64Param(r, "entryID")).GetEntry()
	if err != nil {
		response.HTMLServerError(w, r, err)
		return
	}
	if entry == nil {
		response.HTMLNotFound(w, r)
		return
	}
	feed, err := h.store.FeedByID(user.ID, entry.FeedID)
	if err != nil {
		response.HTMLServerError(w, r, err)
		return
	}
	if feed == nil {
		response.HTMLNotFound(w, r)
		return
	}
	session := request.WebSession(r)
	if err := processor.ProcessEntryWebPage(feed, entry, user); err != nil {
		slog.Warn("Unable to fetch original content", slog.Int64("entry_id", entry.ID), slog.Any("error", err))
		session.SetErrorMessage("Unable to fetch the original content: " + err.Error())
	} else if err := h.store.UpdateEntryTitleAndContent(entry); err != nil {
		response.HTMLServerError(w, r, err)
		return
	}
	h.redirectBack(w, r, "/unread")
}

func (h *handler) createSharedEntry(w http.ResponseWriter, r *http.Request) {
	shareCode, err := h.store.EntryShareCode(request.UserID(r), request.RouteInt64Param(r, "entryID"))
	if err != nil {
		response.HTMLServerError(w, r, err)
		return
	}
	response.HTMLRedirect(w, r, route("/share/%s", shareCode))
}

func (h *handler) unshareEntry(w http.ResponseWriter, r *http.Request) {
	if err := h.store.UnshareEntry(request.UserID(r), request.RouteInt64Param(r, "entryID")); err != nil {
		response.HTMLServerError(w, r, err)
		return
	}
	h.redirectBack(w, r, "/shares")
}

// sharedEntry is the public page of a shared entry.
func (h *handler) sharedEntry(w http.ResponseWriter, r *http.Request) {
	entry, err := h.store.NewAnonymousQueryBuilder().WithShareCode(r.PathValue("shareCode")).WithEnclosures().GetEntry()
	if err != nil {
		response.HTMLServerError(w, r, err)
		return
	}
	if entry == nil {
		response.HTMLNotFound(w, r)
		return
	}
	h.render(w, r, "share", map[string]any{"title": entry.Title, "entry": entry})
}
