// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

// Package googlereader implements the Google Reader API.
package googlereader

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"sss/internal/config"
	"sss/internal/http/request"
	"sss/internal/http/response"
	"sss/internal/integration"
	"sss/internal/mediaproxy"
	"sss/internal/model"
	"sss/internal/reader/fetcher"
	feedhandler "sss/internal/reader/handler"
	"sss/internal/reader/subscription"
	"sss/internal/storage"
	"sss/internal/urllib"
	"sss/internal/validator"
)

var (
	errEmptyFeedTitle   = errors.New("googlereader: empty feed title")
	errFeedNotFound     = errors.New("googlereader: feed not found")
	errCategoryNotFound = errors.New("googlereader: category not found")
	errSimultaneously   = fmt.Errorf("googlereader: %s and %s should not be supplied simultaneously", keptUnreadStreamSuffix, readStreamSuffix)
)

// NewHandler returns the Google Reader API handler, mounted at /accounts/ClientLogin and
// /reader/api/0/. It expects the base path to be stripped from the request URL.
func NewHandler(store *storage.Storage) http.Handler {
	h := &handler{store: store}
	auth := func(fn http.HandlerFunc) http.Handler { return withAuth(store, fn) }

	mux := http.NewServeMux()
	mux.HandleFunc("POST /accounts/ClientLogin", h.clientLogin)
	mux.Handle("GET /reader/api/0/token", auth(h.token))
	mux.Handle("POST /reader/api/0/edit-tag", auth(h.editTag))
	mux.Handle("POST /reader/api/0/rename-tag", auth(h.renameTag))
	mux.Handle("POST /reader/api/0/disable-tag", auth(h.disableTag))
	mux.Handle("GET /reader/api/0/tag/list", auth(h.tagList))
	mux.Handle("GET /reader/api/0/user-info", auth(h.userInfo))
	mux.Handle("GET /reader/api/0/subscription/list", auth(h.subscriptionList))
	mux.Handle("POST /reader/api/0/subscription/edit", auth(h.editSubscription))
	mux.Handle("POST /reader/api/0/subscription/quickadd", auth(h.quickAdd))
	mux.Handle("GET /reader/api/0/stream/items/ids", auth(h.streamItemIDs))
	mux.Handle("POST /reader/api/0/stream/items/contents", auth(h.streamItemContents))
	mux.Handle("GET /reader/api/0/unread-count", auth(h.unreadCount))
	mux.Handle("POST /reader/api/0/mark-all-as-read", auth(h.markAllAsRead))
	// Unimplemented endpoints (preferences, friends...) answer an empty JSON list, as upstream.
	mux.Handle("GET /reader/api/0/", auth(h.fallback))
	mux.Handle("POST /reader/api/0/", auth(h.fallback))
	return mux
}

type handler struct {
	store *storage.Storage
}

func (h *handler) clientLogin(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		unauthorized(w, r, "could not parse request form data", err)
		return
	}

	username := r.Form.Get("Email")
	password := r.Form.Get("Passwd")
	if username == "" || password == "" {
		unauthorized(w, r, "empty username or password", nil)
		return
	}

	if err := h.store.GoogleReaderUserCheckPassword(username, password); err != nil {
		unauthorized(w, r, "invalid username or password", err)
		return
	}

	integration, err := h.store.GoogleReaderUserGetIntegration(username)
	if err != nil {
		response.JSONServerError(w, r, err)
		return
	}
	h.store.SetLastLogin(integration.UserID)

	token := getAuthToken(integration.GoogleReaderUsername, integration.GoogleReaderPassword)
	result := loginResponse{SID: token, LSID: token, Auth: token}
	if r.Form.Get("output") == "json" {
		response.JSON(w, r, result)
		return
	}
	response.Text(w, r, result.String())
}

func (h *handler) token(w http.ResponseWriter, r *http.Request) {
	response.Text(w, r, request.GoogleReaderToken(r))
}

func (h *handler) editTag(w http.ResponseWriter, r *http.Request) {
	userID := request.UserID(r)

	addTags, err := getStreams(r.PostForm[paramTagsAdd], userID)
	if err != nil {
		response.JSONServerError(w, r, err)
		return
	}
	removeTags, err := getStreams(r.PostForm[paramTagsRemove], userID)
	if err != nil {
		response.JSONServerError(w, r, err)
		return
	}
	if len(addTags) == 0 && len(removeTags) == 0 {
		response.JSONServerError(w, r, errors.New("googlereader: add or/and remove tags should be supplied"))
		return
	}
	tags, err := checkAndSimplifyTags(addTags, removeTags)
	if err != nil {
		response.JSONServerError(w, r, err)
		return
	}

	itemIDs, err := parseItemIDsFromRequest(r)
	if err != nil {
		response.JSONBadRequest(w, r, err)
		return
	}

	entries, err := h.store.NewEntryQueryBuilder(userID).WithEntryIDs(itemIDs...).GetEntries()
	if err != nil {
		response.JSONServerError(w, r, err)
		return
	}

	var readIDs, unreadIDs, starredIDs, unstarredIDs []int64
	var newlyStarred model.Entries
	for _, entry := range entries {
		if read, ok := tags[ReadStream]; ok {
			if read && entry.Status == model.EntryStatusUnread {
				readIDs = append(readIDs, entry.ID)
			} else if !read && entry.Status == model.EntryStatusRead {
				unreadIDs = append(unreadIDs, entry.ID)
			}
		}
		if starred, ok := tags[StarredStream]; ok {
			if starred && !entry.Starred {
				starredIDs = append(starredIDs, entry.ID)
				newlyStarred = append(newlyStarred, entry)
			} else if !starred && entry.Starred {
				unstarredIDs = append(unstarredIDs, entry.ID)
			}
		}
	}

	for _, err := range []error{
		setStatus(h.store, userID, readIDs, model.EntryStatusRead),
		setStatus(h.store, userID, unreadIDs, model.EntryStatusUnread),
		setStarred(h.store, userID, unstarredIDs, false),
		setStarred(h.store, userID, starredIDs, true),
	} {
		if err != nil {
			response.JSONServerError(w, r, err)
			return
		}
	}

	if len(newlyStarred) > 0 {
		settings, err := h.store.Integration(userID)
		if err != nil {
			response.JSONServerError(w, r, err)
			return
		}
		for _, entry := range newlyStarred {
			go integration.SendEntry(entry, settings)
		}
	}

	response.Text(w, r, "OK")
}

func setStatus(store *storage.Storage, userID int64, entryIDs []int64, status string) error {
	if len(entryIDs) == 0 {
		return nil
	}
	return store.SetEntriesStatus(userID, entryIDs, status)
}

func setStarred(store *storage.Storage, userID int64, entryIDs []int64, starred bool) error {
	if len(entryIDs) == 0 {
		return nil
	}
	return store.SetEntriesStarredState(userID, entryIDs, starred)
}

func (h *handler) quickAdd(w http.ResponseWriter, r *http.Request) {
	userID := request.UserID(r)

	feedURL := r.Form.Get(paramQuickAdd)
	if !urllib.IsAbsoluteURL(feedURL) {
		response.JSONBadRequest(w, r, fmt.Errorf("googlereader: invalid URL: %s", feedURL))
		return
	}

	user, err := h.store.UserByID(userID)
	if err != nil {
		response.JSONServerError(w, r, err)
		return
	}

	requestBuilder := fetcher.NewRequestBuilder().
		WithTimeout(config.Opts.HTTPClientTimeout).
		WithUserAgent("", user.DefaultUserAgent())

	var rssBridgeURL, rssBridgeToken string
	if intg, err := h.store.Integration(userID); err == nil && intg.RSSBridgeEnabled {
		rssBridgeURL, rssBridgeToken = intg.RSSBridgeURL, intg.RSSBridgeToken
	}

	subscriptions, err := subscription.NewSubscriptionFinder(requestBuilder).FindSubscriptions(feedURL, rssBridgeURL, rssBridgeToken)
	if err != nil {
		response.JSONServerError(w, r, err)
		return
	}
	if len(subscriptions) == 0 {
		response.JSON(w, r, quickAddResponse{NumResults: 0})
		return
	}

	newFeed, err := subscribe(h.store, userID, Stream{FeedStream, subscriptions[0].URL}, Stream{NoStream, ""}, "")
	if err != nil {
		writeSubscribeError(w, r, err)
		return
	}

	response.JSON(w, r, quickAddResponse{
		NumResults: 1,
		Query:      newFeed.FeedURL,
		StreamID:   feedPrefix + strconv.FormatInt(newFeed.ID, 10),
		StreamName: newFeed.Title,
	})
}

func writeSubscribeError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, errCategoryNotFound) {
		response.JSONBadRequest(w, r, err)
	} else {
		response.JSONServerError(w, r, err)
	}
}

// getOrCreateCategory returns the first category for an empty label, else the category titled
// like the label, creating it when missing.
func getOrCreateCategory(store *storage.Storage, userID int64, label Stream) (*model.Category, error) {
	var category *model.Category
	var err error
	switch {
	case label.ID == "":
		category, err = store.FirstCategory(userID)
	case store.CategoryTitleExists(userID, label.ID):
		category, err = store.CategoryByTitle(userID, label.ID)
	default:
		category, err = store.CreateCategory(userID, &model.CategoryCreationRequest{Title: label.ID})
	}
	if errors.Is(err, storage.ErrNoCategory) || (err == nil && category == nil) {
		return nil, errCategoryNotFound
	}
	return category, err
}

func subscribe(store *storage.Storage, userID int64, newFeed, label Stream, title string) (*model.Feed, error) {
	category, err := getOrCreateCategory(store, userID, label)
	if err != nil {
		return nil, err
	}

	feedRequest := model.FeedCreationRequest{FeedURL: newFeed.ID, CategoryID: category.ID}
	if err := validator.ValidateFeedCreation(store, userID, &feedRequest); err != nil {
		return nil, err
	}

	created, err := feedhandler.CreateFeed(store, userID, &feedRequest)
	if err != nil {
		return nil, err
	}

	if title != "" {
		(&model.FeedModificationRequest{Title: &title}).Patch(created)
		if err := store.UpdateFeed(created); err != nil {
			return nil, err
		}
	}
	return created, nil
}

// editFeed renames the feed when title is set and moves it to label when label is non-nil.
func editFeed(store *storage.Storage, userID int64, feedStream Stream, title string, label *Stream) error {
	feedID, err := strconv.ParseInt(feedStream.ID, 10, 64)
	if err != nil {
		return err
	}
	feed, err := store.FeedByID(userID, feedID)
	if err != nil {
		return err
	}
	if feed == nil {
		return errFeedNotFound
	}

	var modification model.FeedModificationRequest
	if title != "" {
		modification.Title = &title
	}
	if label != nil {
		category, err := getOrCreateCategory(store, userID, *label)
		if err != nil {
			return err
		}
		modification.CategoryID = &category.ID
	}
	modification.Patch(feed)
	return store.UpdateFeed(feed)
}

func (h *handler) editSubscription(w http.ResponseWriter, r *http.Request) {
	userID := request.UserID(r)

	streams, err := getStreams(r.Form[paramStreamID], userID)
	if err != nil || len(streams) == 0 {
		response.JSONBadRequest(w, r, errors.New("googlereader: no valid stream IDs provided"))
		return
	}

	newLabel, err := getStream(r.Form.Get(paramTagsAdd), userID)
	if err != nil {
		response.JSONBadRequest(w, r, fmt.Errorf("googlereader: invalid data in %s", paramTagsAdd))
		return
	}

	title := r.Form.Get(paramTitle)
	switch action := r.Form.Get(paramSubscribeAction); action {
	case "subscribe":
		if _, err := subscribe(h.store, userID, streams[0], newLabel, title); err != nil {
			writeSubscribeError(w, r, err)
			return
		}
	case "unsubscribe":
		for _, s := range streams {
			feedID, err := strconv.ParseInt(s.ID, 10, 64)
			if err == nil {
				err = h.store.RemoveFeed(userID, feedID)
			}
			if err != nil {
				response.JSONServerError(w, r, err)
				return
			}
		}
	case "edit":
		if title == "" && !r.Form.Has(paramTagsAdd) {
			break
		}
		var label *Stream
		if r.Form.Has(paramTagsAdd) {
			if newLabel.Type != LabelStream {
				response.JSONBadRequest(w, r, errors.New("destination must be a label"))
				return
			}
			label = &newLabel
		}
		if err := editFeed(h.store, userID, streams[0], title, label); err != nil {
			if errors.Is(err, errFeedNotFound) || errors.Is(err, errCategoryNotFound) {
				response.JSONBadRequest(w, r, err)
			} else {
				response.JSONServerError(w, r, err)
			}
			return
		}
	default:
		response.JSONBadRequest(w, r, fmt.Errorf("googlereader: unrecognized action %s", action))
		return
	}

	response.Text(w, r, "OK")
}

func (h *handler) streamItemContents(w http.ResponseWriter, r *http.Request) {
	userID := request.UserID(r)

	if err := checkOutputFormat(r); err != nil {
		response.JSONBadRequest(w, r, err)
		return
	}

	rm, err := parseStreamFilterFromRequest(r)
	if err != nil {
		response.JSONServerError(w, r, err)
		return
	}

	itemIDs, err := parseItemIDsFromRequest(r)
	if err != nil {
		response.JSONBadRequest(w, r, err)
		return
	}

	entries, err := h.store.NewEntryQueryBuilder(userID).
		WithEnclosures().
		WithEntryIDs(itemIDs...).
		WithSorting(model.DefaultSortingOrder, rm.SortDirection).
		GetEntries()
	if err != nil {
		response.JSONServerError(w, r, err)
		return
	}

	userStream := fmt.Sprintf(userStreamPrefix, userID)
	userLabel := fmt.Sprintf(userLabelPrefix, userID)
	result := streamContentItemsResponse{
		Direction: "ltr",
		ID:        "user/-/state/com.google/reading-list",
		Title:     "Reading List",
		Updated:   time.Now().Unix(),
		Self:      []contentHREF{{HREF: config.Opts.BaseURL + "/reader/api/0/stream/items/contents"}},
		Author:    request.UserName(r),
		Items:     make([]contentItem, len(entries)),
	}

	for i, entry := range entries {
		mediaproxy.ProxifyEnclosureURLs(entry.Enclosures)
		enclosures := make([]contentItemEnclosure, 0, len(entry.Enclosures))
		for _, enclosure := range entry.Enclosures {
			enclosures = append(enclosures, contentItemEnclosure{URL: enclosure.URL, Type: enclosure.MimeType})
		}

		categories := []string{userStream + readingListStreamSuffix}
		if entry.Feed.Category.Title != "" {
			categories = append(categories, userLabel+entry.Feed.Category.Title)
		}
		if entry.Status == model.EntryStatusRead {
			categories = append(categories, userStream+readStreamSuffix)
		}
		if entry.Starred {
			categories = append(categories, userStream+starredStreamSuffix)
		}

		content := contentItemContent{Direction: "ltr", Content: mediaproxy.RewriteDocumentWithAbsoluteProxyURL(entry.Content)}
		result.Items[i] = contentItem{
			ID:            convertEntryIDToLongFormItemID(entry.ID),
			Title:         entry.Title,
			Author:        entry.Author,
			TimestampUsec: strconv.FormatInt(entry.Date.UnixMicro(), 10),
			CrawlTimeMsec: strconv.FormatInt(entry.CreatedAt.UnixMilli(), 10),
			Published:     entry.Date.Unix(),
			Updated:       entry.ChangedAt.Unix(),
			Categories:    categories,
			Canonical:     []contentHREF{{HREF: entry.URL}},
			Alternate:     []contentHREFType{{HREF: entry.URL, Type: "text/html"}},
			Content:       content,
			Summary:       content,
			Origin: contentItemOrigin{
				StreamID: feedPrefix + strconv.FormatInt(entry.FeedID, 10),
				Title:    entry.Feed.Title,
				HTMLUrl:  entry.Feed.SiteURL,
			},
			Enclosure: enclosures,
		}
	}

	response.JSON(w, r, result)
}

func (h *handler) disableTag(w http.ResponseWriter, r *http.Request) {
	userID := request.UserID(r)

	streams, err := getStreams(r.Form[paramStreamID], userID)
	if err != nil {
		response.JSONBadRequest(w, r, fmt.Errorf("googlereader: invalid data in %s", paramStreamID))
		return
	}

	titles := make([]string, len(streams))
	for i, s := range streams {
		if s.Type != LabelStream {
			response.JSONBadRequest(w, r, errors.New("googlereader: only labels are supported"))
			return
		}
		titles[i] = s.ID
	}

	if err := h.store.RemoveAndReplaceCategoriesByName(userID, titles); err != nil {
		response.JSONServerError(w, r, err)
		return
	}
	response.Text(w, r, "OK")
}

func (h *handler) renameTag(w http.ResponseWriter, r *http.Request) {
	userID := request.UserID(r)

	source, err := getStream(r.Form.Get(paramStreamID), userID)
	if err != nil {
		response.JSONBadRequest(w, r, fmt.Errorf("googlereader: invalid data in %s", paramStreamID))
		return
	}
	destination, err := getStream(r.Form.Get(paramDestination), userID)
	if err != nil {
		response.JSONBadRequest(w, r, fmt.Errorf("googlereader: invalid data in %s", paramDestination))
		return
	}
	if source.Type != LabelStream || destination.Type != LabelStream {
		response.JSONBadRequest(w, r, errors.New("googlereader: only labels supported"))
		return
	}
	if destination.ID == "" {
		response.JSONBadRequest(w, r, errors.New("googlereader: empty destination name"))
		return
	}

	category, err := h.store.CategoryByTitle(userID, source.ID)
	if err != nil {
		response.JSONServerError(w, r, err)
		return
	}
	if category == nil {
		response.JSONNotFound(w, r)
		return
	}

	modification := model.CategoryModificationRequest{Title: &destination.ID}
	if err := validator.ValidateCategoryModification(h.store, userID, category.ID, &modification); err != nil {
		response.JSONBadRequest(w, r, err)
		return
	}
	modification.Patch(category)
	if err := h.store.UpdateCategory(category); err != nil {
		response.JSONServerError(w, r, err)
		return
	}
	response.Text(w, r, "OK")
}

func (h *handler) tagList(w http.ResponseWriter, r *http.Request) {
	userID := request.UserID(r)

	if err := checkOutputFormat(r); err != nil {
		response.JSONBadRequest(w, r, err)
		return
	}

	categories, err := h.store.Categories(userID)
	if err != nil {
		response.JSONServerError(w, r, err)
		return
	}

	userLabel := fmt.Sprintf(userLabelPrefix, userID)
	var result tagsResponse
	result.Tags = make([]subscriptionCategoryResponse, 0, 1+len(categories))
	result.Tags = append(result.Tags, subscriptionCategoryResponse{
		ID: fmt.Sprintf(userStreamPrefix, userID) + starredStreamSuffix,
	})
	for _, category := range categories {
		result.Tags = append(result.Tags, subscriptionCategoryResponse{
			ID:    userLabel + category.Title,
			Label: category.Title,
			Type:  "folder",
		})
	}
	response.JSON(w, r, result)
}

func (h *handler) subscriptionList(w http.ResponseWriter, r *http.Request) {
	userID := request.UserID(r)

	if err := checkOutputFormat(r); err != nil {
		response.JSONBadRequest(w, r, err)
		return
	}

	feeds, err := h.store.Feeds(userID)
	if err != nil {
		response.JSONServerError(w, r, err)
		return
	}

	userLabel := fmt.Sprintf(userLabelPrefix, userID)
	var result subscriptionsResponse
	result.Subscriptions = make([]subscriptionResponse, 0, len(feeds))
	for _, feed := range feeds {
		var iconURL string
		if feed.Icon != nil && feed.Icon.ExternalIconID != "" {
			iconURL = config.Opts.BaseURL + "/feed-icon/" + feed.Icon.ExternalIconID
		}
		result.Subscriptions = append(result.Subscriptions, subscriptionResponse{
			ID:         feedPrefix + strconv.FormatInt(feed.ID, 10),
			Title:      feed.Title,
			URL:        feed.FeedURL,
			Categories: []subscriptionCategoryResponse{{userLabel + feed.Category.Title, feed.Category.Title, "folder"}},
			HTMLURL:    feed.SiteURL,
			IconURL:    iconURL,
		})
	}
	response.JSON(w, r, result)
}

func (h *handler) fallback(w http.ResponseWriter, r *http.Request) {
	response.JSON(w, r, []string{})
}

func (h *handler) userInfo(w http.ResponseWriter, r *http.Request) {
	user, err := h.store.UserByID(request.UserID(r))
	if err != nil {
		response.JSONServerError(w, r, err)
		return
	}
	if user == nil {
		response.JSONNotFound(w, r)
		return
	}

	id := strconv.FormatInt(user.ID, 10)
	response.JSON(w, r, userInfoResponse{UserID: id, UserName: user.Username, UserProfileID: id, UserEmail: user.Username})
}

// unreadCount returns unread counts per feed, per label and for the whole reading list.
func (h *handler) unreadCount(w http.ResponseWriter, r *http.Request) {
	userID := request.UserID(r)

	if err := checkOutputFormat(r); err != nil {
		response.JSONBadRequest(w, r, err)
		return
	}

	feeds, err := h.store.FeedsWithCounters(userID)
	if err != nil {
		response.JSONServerError(w, r, err)
		return
	}

	userLabel := fmt.Sprintf(userLabelPrefix, userID)
	var total int
	var labels []string
	labelCounts := make(map[string]int)
	result := unreadCountResponse{Max: model.MaxEntryIDsLimit, UnreadCounts: make([]unreadCount, 0, len(feeds)+1)}
	for _, feed := range feeds {
		result.UnreadCounts = append(result.UnreadCounts, unreadCount{ID: feedPrefix + strconv.FormatInt(feed.ID, 10), Count: feed.UnreadCount})
		if _, ok := labelCounts[feed.Category.Title]; !ok {
			labels = append(labels, feed.Category.Title)
		}
		labelCounts[feed.Category.Title] += feed.UnreadCount
		total += feed.UnreadCount
	}
	for _, label := range labels {
		result.UnreadCounts = append(result.UnreadCounts, unreadCount{ID: userLabel + label, Count: labelCounts[label]})
	}
	result.UnreadCounts = append(result.UnreadCounts, unreadCount{ID: fmt.Sprintf(userStreamPrefix, userID) + readingListStreamSuffix, Count: total})
	response.JSON(w, r, result)
}

func (h *handler) streamItemIDs(w http.ResponseWriter, r *http.Request) {
	if err := checkOutputFormat(r); err != nil {
		response.JSONBadRequest(w, r, err)
		return
	}

	rm, err := parseStreamFilterFromRequest(r)
	if err != nil {
		response.JSONServerError(w, r, err)
		return
	}
	if len(rm.Streams) != 1 {
		response.JSONServerError(w, r, errors.New("googlereader: only one stream type expected"))
		return
	}

	builder := h.store.NewEntryQueryBuilder(rm.UserID).
		WithLimitAndMaximum(rm.Count, model.MaxEntryIDsLimit).
		WithOffset(rm.Offset).
		WithSorting(model.DefaultSortingOrder, rm.SortDirection)

	switch s := rm.Streams[0]; s.Type {
	case ReadingListStream:
		for _, x := range rm.ExcludeTargets {
			if x.Type == ReadStream {
				builder = builder.WithStatuses(model.EntryStatusUnread)
			}
		}
	case StarredStream:
		builder = builder.WithStarred(true)
	case ReadStream:
		builder = builder.WithStatuses(model.EntryStatusRead)
	case FeedStream:
		feedID, err := strconv.ParseInt(s.ID, 10, 64)
		if err != nil {
			response.JSONServerError(w, r, err)
			return
		}
		builder = builder.WithFeedID(feedID)
		for _, x := range rm.ExcludeTargets {
			if x.Type == ReadStream {
				builder = builder.WithoutStatus(model.EntryStatusRead)
			}
		}
	default:
		response.JSONServerError(w, r, fmt.Errorf("googlereader: unknown stream type %s", s.Type))
		return
	}

	if rm.StartTime > 0 {
		builder = builder.AfterPublishedDate(time.Unix(rm.StartTime, 0))
	}
	if rm.StopTime > 0 {
		builder = builder.BeforePublishedDate(time.Unix(rm.StopTime, 0))
	}

	entryIDs, total, err := builder.GetEntryIDsWithCount()
	if err != nil {
		response.JSONServerError(w, r, err)
		return
	}

	itemRefs := make([]itemRef, 0, len(entryIDs))
	for _, entryID := range entryIDs {
		itemRefs = append(itemRefs, itemRef{ID: strconv.FormatInt(entryID, 10)})
	}
	continuation := 0
	if len(itemRefs)+rm.Offset < total {
		continuation = len(itemRefs) + rm.Offset
	}
	response.JSON(w, r, streamIDResponse{itemRefs, continuation})
}

func (h *handler) markAllAsRead(w http.ResponseWriter, r *http.Request) {
	userID := request.UserID(r)

	s, err := getStream(r.Form.Get(paramStreamID), userID)
	if err != nil {
		response.JSONBadRequest(w, r, err)
		return
	}

	var before time.Time
	if value := r.Form.Get(paramTimestamp); value != "" {
		ts, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			response.JSONBadRequest(w, r, err)
			return
		}
		if ts > 0 {
			// Clients send seconds or microseconds: tell them apart by length.
			if len(value) >= 16 {
				before = time.UnixMicro(ts)
			} else {
				before = time.Unix(ts, 0)
			}
		}
	}
	if before.IsZero() {
		before = time.Now()
	}

	switch s.Type {
	case FeedStream:
		feedID, err := strconv.ParseInt(s.ID, 10, 64)
		if err != nil {
			response.JSONBadRequest(w, r, err)
			return
		}
		if err := h.store.MarkFeedAsRead(userID, feedID, before); err != nil {
			response.JSONServerError(w, r, err)
			return
		}
	case LabelStream:
		category, err := h.store.CategoryByTitle(userID, s.ID)
		if err != nil {
			response.JSONServerError(w, r, err)
			return
		}
		if category == nil {
			response.JSONNotFound(w, r)
			return
		}
		if err := h.store.MarkCategoryAsRead(userID, category.ID, before); err != nil {
			response.JSONServerError(w, r, err)
			return
		}
	case ReadingListStream:
		if err := h.store.MarkAllAsReadBeforeDate(userID, before); err != nil {
			response.JSONServerError(w, r, err)
			return
		}
	}

	response.Text(w, r, "OK")
}

func checkAndSimplifyTags(addTags, removeTags []Stream) (map[StreamType]bool, error) {
	tags := make(map[StreamType]bool)
	for _, s := range addTags {
		switch s.Type {
		case ReadStream:
			if _, ok := tags[KeptUnreadStream]; ok {
				return nil, errSimultaneously
			}
			tags[ReadStream] = true
		case KeptUnreadStream:
			if _, ok := tags[ReadStream]; ok {
				return nil, errSimultaneously
			}
			tags[ReadStream] = false
		case StarredStream:
			tags[StarredStream] = true
		case BroadcastStream, LikeStream:
			// Not supported: ignored.
		default:
			return nil, fmt.Errorf("googlereader: unsupported tag type: %s", s.Type)
		}
	}
	for _, s := range removeTags {
		switch s.Type {
		case ReadStream, KeptUnreadStream:
			if _, ok := tags[ReadStream]; ok {
				return nil, errSimultaneously
			}
			tags[ReadStream] = s.Type == KeptUnreadStream
		case StarredStream:
			if _, ok := tags[StarredStream]; ok {
				return nil, fmt.Errorf("googlereader: %s should not be supplied for add and remove simultaneously", starredStreamSuffix)
			}
			tags[StarredStream] = false
		case BroadcastStream, LikeStream:
			// Not supported: ignored.
		default:
			return nil, fmt.Errorf("googlereader: unsupported tag type: %s", s.Type)
		}
	}
	return tags, nil
}

func checkOutputFormat(r *http.Request) error {
	output := r.URL.Query().Get("output")
	if r.Method == http.MethodPost {
		output = r.Form.Get("output")
	}
	if output != "json" {
		return errors.New("googlereader: only json output is supported")
	}
	return nil
}
