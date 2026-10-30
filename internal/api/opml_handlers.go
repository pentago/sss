// Copyright The Miniflux Authors. Modified by The sss Authors.
// Derived from Miniflux (https://github.com/miniflux/v2).
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"errors"
	"net/http"

	"sss/internal/http/request"
	"sss/internal/http/response"
	feedHandler "sss/internal/reader/handler"
	"sss/internal/reader/opml"
	"sss/internal/storage"
)

func (h *handler) exportFeedsHandler(w http.ResponseWriter, r *http.Request) {
	feeds, err := h.store.Feeds(request.UserID(r))
	if err != nil {
		response.JSONServerError(w, r, err)
		return
	}

	opmlExport, err := opml.Serialize(feeds)
	if err != nil {
		response.JSONServerError(w, r, err)
		return
	}

	response.XML(w, r, opmlExport)
}

func (h *handler) importFeedsHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	if err := feedHandler.ImportOPML(h.store, request.UserID(r), r.Body); err != nil {
		if errors.Is(err, storage.ErrNoCategory) {
			response.JSONBadRequest(w, r, err)
			return
		}
		response.JSONServerError(w, r, err)
		return
	}

	response.JSONCreated(w, r, importFeedsResponse{Message: "Feeds imported successfully"})
}
