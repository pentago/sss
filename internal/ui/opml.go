// Copyright The Miniflux Authors. Modified by The sss Authors.
// Derived from Miniflux (https://github.com/miniflux/v2).
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"log/slog"
	"net/http"
	"strings"

	"sss/internal/config"
	"sss/internal/http/request"
	"sss/internal/http/response"
	"sss/internal/reader/fetcher"
	feedHandler "sss/internal/reader/handler"
	"sss/internal/reader/opml"
)

func (h *handler) exportFeeds(w http.ResponseWriter, r *http.Request) {
	feeds, err := h.store.Feeds(request.UserID(r))
	if err != nil {
		response.HTMLServerError(w, r, err)
		return
	}
	export, err := opml.Serialize(feeds)
	if err != nil {
		response.HTMLServerError(w, r, err)
		return
	}
	response.NewBuilder(w, r).
		WithHeader("Content-Type", "text/xml; charset=utf-8").
		WithAttachment("feeds.opml").
		WithBodyAsString(export).
		Write()
}

func (h *handler) showImportPage(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, "import", map[string]any{"title": "Import", "menu": "feeds"})
}

func (h *handler) uploadOPML(w http.ResponseWriter, r *http.Request) {
	file, header, err := r.FormFile("file")
	if err != nil {
		h.render(w, r, "import", map[string]any{"title": "Import", "menu": "feeds", "errorMessage": "Please choose an OPML file."})
		return
	}
	defer file.Close()
	slog.Info("OPML file uploaded", slog.Int64("user_id", request.UserID(r)), slog.String("file_name", header.Filename), slog.Int64("file_size", header.Size))
	h.finishImport(w, r, feedHandler.ImportOPML(h.store, request.UserID(r), file))
}

func (h *handler) fetchOPML(w http.ResponseWriter, r *http.Request) {
	opmlURL := strings.TrimSpace(r.FormValue("url"))
	if opmlURL == "" {
		response.HTMLRedirect(w, r, route("/import"))
		return
	}
	slog.Info("Fetching OPML file remotely", slog.Int64("user_id", request.UserID(r)), slog.String("opml_file_url", opmlURL))
	rh := fetcher.NewResponseHandler(fetcher.NewRequestBuilder().WithTimeout(config.Opts.HTTPClientTimeout).ExecuteRequest(opmlURL))
	defer rh.Close()
	err := rh.LocalizedError()
	if err == nil {
		err = feedHandler.ImportOPML(h.store, request.UserID(r), rh.Body(config.Opts.HTTPClientMaxBodySize))
	}
	h.finishImport(w, r, err)
}

func (h *handler) finishImport(w http.ResponseWriter, r *http.Request, err error) {
	if err != nil {
		h.render(w, r, "import", map[string]any{"title": "Import", "menu": "feeds", "errorMessage": err.Error()})
		return
	}
	request.WebSession(r).SetSuccessMessage("Feeds imported. They will be refreshed in the background.")
	response.HTMLRedirect(w, r, route("/feeds"))
}
