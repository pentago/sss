// Copyright The Miniflux Authors. Modified by The sss Authors.
// Derived from Miniflux (https://github.com/miniflux/v2).
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"errors"
	"net/http"

	"sss/internal/http/request"
	"sss/internal/http/response"
)

func (h *handler) getIconByFeedIDHandler(w http.ResponseWriter, r *http.Request) {
	feedID := request.RouteInt64Param(r, "feedID")
	if feedID == 0 {
		response.JSONBadRequest(w, r, errors.New("invalid feed ID"))
		return
	}

	icon, err := h.store.IconByFeedID(request.UserID(r), feedID)
	if err != nil {
		response.JSONServerError(w, r, err)
		return
	}

	if icon == nil {
		response.JSONNotFound(w, r)
		return
	}

	response.JSON(w, r, &feedIconResponse{
		ID:       icon.ID,
		MimeType: icon.MimeType,
		Data:     icon.DataURL(),
	})
}

func (h *handler) getIconByIconIDHandler(w http.ResponseWriter, r *http.Request) {
	iconID := request.RouteInt64Param(r, "iconID")
	if iconID == 0 {
		response.JSONBadRequest(w, r, errors.New("invalid icon ID"))
		return
	}

	icon, err := h.store.IconByUserAndIconID(request.UserID(r), iconID)
	if err != nil {
		response.JSONServerError(w, r, err)
		return
	}

	if icon == nil {
		response.JSONNotFound(w, r)
		return
	}

	response.JSON(w, r, &feedIconResponse{
		ID:       icon.ID,
		MimeType: icon.MimeType,
		Data:     icon.DataURL(),
	})
}
