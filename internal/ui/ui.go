// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

// Package ui serves the classless HTML web interface.
package ui

import (
	"net/http"

	"sss/internal/storage"
	"sss/internal/worker"
)

// NewHandler returns the web UI handler, to be mounted at "/" with the base
// path already stripped.
func NewHandler(store *storage.Storage, pool *worker.Pool) http.Handler {
	h := &handler{store: store, pool: pool, pages: parseTemplates()}
	mux := http.NewServeMux()

	// Static assets.
	mux.HandleFunc("GET /stylesheets/{checksum}/{path...}", h.showStylesheet)
	mux.HandleFunc("GET /js/{checksum}/{filename}", h.showJavascript)
	mux.HandleFunc("GET /custom.css", h.showCustomCSS)
	mux.HandleFunc("GET /custom.js", h.showCustomJS)
	mux.HandleFunc("GET /favicon.ico", h.showFavicon)
	mux.HandleFunc("GET /icon/{checksum}/{filename}", h.showFavicon)
	mux.HandleFunc("GET /manifest.json", h.showWebManifest)
	mux.HandleFunc("GET /service-worker.js", h.showServiceWorker)
	mux.HandleFunc("GET /offline", h.showOfflinePage)
	mux.HandleFunc("GET /robots.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("User-agent: *\nDisallow: /"))
	})
	mux.HandleFunc("GET /feed-icon/{externalIconID}", h.showFeedIcon)
	mux.HandleFunc("GET /proxy/{encodedDigest}/{encodedURL}", h.mediaProxy)

	// Subscriptions.
	mux.HandleFunc("GET /subscribe", h.showAddSubscriptionPage)
	mux.HandleFunc("POST /subscribe", h.submitSubscription)
	mux.HandleFunc("POST /subscriptions", h.chooseSubscription)
	mux.HandleFunc("GET /bookmarklet", h.bookmarklet)

	// Unread, history, starred, search.
	mux.HandleFunc("POST /mark-all-as-read", h.markAllAsRead)
	mux.HandleFunc("GET /unread", h.showUnreadPage)
	mux.HandleFunc("GET /unread/entry/{entryID}", h.showUnreadEntryPage)
	mux.HandleFunc("GET /history", h.showHistoryPage)
	mux.HandleFunc("GET /history/entry/{entryID}", h.showHistoryEntryPage)
	mux.HandleFunc("POST /history/flush", h.flushHistory)
	mux.HandleFunc("GET /starred", h.showStarredPage)
	mux.HandleFunc("GET /starred/entry/{entryID}", h.showStarredEntryPage)
	mux.HandleFunc("GET /search", h.showSearchPage)
	mux.HandleFunc("GET /search/entry/{entryID}", h.showSearchEntryPage)

	// Feeds.
	mux.HandleFunc("GET /feeds", h.showFeedsPage)
	mux.HandleFunc("POST /feeds/refresh", h.refreshAllFeeds)
	mux.HandleFunc("POST /feed/{feedID}/refresh", h.refreshFeed)
	mux.HandleFunc("GET /feed/{feedID}/edit", h.showEditFeedPage)
	mux.HandleFunc("POST /feed/{feedID}/remove", h.removeFeed)
	mux.HandleFunc("POST /feed/{feedID}/update", h.updateFeed)
	mux.HandleFunc("GET /feed/{feedID}/entries", h.showFeedEntriesPage)
	mux.HandleFunc("GET /feed/{feedID}/entries/all", h.showFeedEntriesAllPage)
	mux.HandleFunc("GET /feed/{feedID}/entry/{entryID}", h.showFeedEntryPage)
	mux.HandleFunc("GET /unread/feed/{feedID}/entry/{entryID}", h.showUnreadFeedEntryPage)
	mux.HandleFunc("POST /feed/{feedID}/mark-all-as-read", h.markFeedAsRead)

	// Categories.
	mux.HandleFunc("GET /categories", h.showCategoryListPage)
	mux.HandleFunc("GET /category/create", h.showCreateCategoryPage)
	mux.HandleFunc("POST /category/save", h.saveCategory)
	mux.HandleFunc("GET /category/{categoryID}/entry/{entryID}", h.showCategoryEntryPage)
	mux.HandleFunc("GET /unread/category/{categoryID}/entry/{entryID}", h.showUnreadCategoryEntryPage)
	mux.HandleFunc("GET /starred/category/{categoryID}/entry/{entryID}", h.showStarredCategoryEntryPage)
	mux.HandleFunc("GET /category/{categoryID}/feeds", h.showCategoryFeedsPage)
	mux.HandleFunc("POST /category/{categoryID}/feed/{feedID}/remove", h.removeCategoryFeed)
	mux.HandleFunc("POST /category/{categoryID}/feed/{feedID}/mark-all-as-read", h.markCategoryFeedAsRead)
	mux.HandleFunc("POST /category/{categoryID}/feeds/refresh", h.refreshCategoryFeedsPage)
	mux.HandleFunc("GET /category/{categoryID}/entries", h.showCategoryEntriesPage)
	mux.HandleFunc("POST /category/{categoryID}/entries/refresh", h.refreshCategoryEntriesPage)
	mux.HandleFunc("GET /category/{categoryID}/entries/all", h.showCategoryEntriesAllPage)
	mux.HandleFunc("GET /category/{categoryID}/entries/starred", h.showCategoryEntriesStarredPage)
	mux.HandleFunc("GET /category/{categoryID}/edit", h.showEditCategoryPage)
	mux.HandleFunc("POST /category/{categoryID}/update", h.updateCategory)
	mux.HandleFunc("POST /category/{categoryID}/remove", h.removeCategory)
	mux.HandleFunc("POST /category/{categoryID}/mark-all-as-read", h.markCategoryAsRead)

	// Tags.
	mux.HandleFunc("GET /tags/{tagName}/entries/all", h.showTagEntriesAllPage)
	mux.HandleFunc("GET /tags/{tagName}/entry/{entryID}", h.showTagEntryPage)

	// Entry actions.
	mux.HandleFunc("POST /entry/status", h.updateEntriesStatus)
	mux.HandleFunc("POST /entry/save/{entryID}", h.saveEntry)
	mux.HandleFunc("POST /entry/download/{entryID}", h.fetchContent)
	mux.HandleFunc("POST /entry/star/{entryID}", h.toggleStarred)

	// Sharing.
	mux.HandleFunc("POST /entry/share/{entryID}", h.createSharedEntry)
	mux.HandleFunc("POST /entry/unshare/{entryID}", h.unshareEntry)
	mux.HandleFunc("GET /share/{shareCode}", h.sharedEntry)
	mux.HandleFunc("GET /shares", h.sharedEntries)

	// OPML.
	mux.HandleFunc("GET /export", h.exportFeeds)
	mux.HandleFunc("GET /import", h.showImportPage)
	mux.HandleFunc("POST /upload", h.uploadOPML)
	mux.HandleFunc("POST /fetch", h.fetchOPML)

	h.registerAccountRoutes(mux)

	return withSecurityHeaders(http.NewCrossOriginProtection().Handler(h.withSession(mux)))
}
