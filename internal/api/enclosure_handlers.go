// Copyright The Miniflux Authors. Modified by The sss Authors.
// Derived from Miniflux (https://github.com/miniflux/v2).
// SPDX-License-Identifier: Apache-2.0

package api

import (
	json_parser "encoding/json"
	"errors"
	"net/http"

	"sss/internal/http/request"
	"sss/internal/http/response"
	"sss/internal/mediaproxy"
	"sss/internal/model"
	"sss/internal/validator"
)

func (h *handler) getEnclosureByIDHandler(w http.ResponseWriter, r *http.Request) {
	enclosureID := request.RouteInt64Param(r, "enclosureID")
	if enclosureID == 0 {
		response.JSONBadRequest(w, r, errors.New("invalid enclosure ID"))
		return
	}

	enclosure, err := h.store.EnclosureByID(request.UserID(r), enclosureID)
	if err != nil {
		response.JSONServerError(w, r, err)
		return
	}

	if enclosure == nil {
		response.JSONNotFound(w, r)
		return
	}

	mediaproxy.ProxifyEnclosureURLs(model.EnclosureList{enclosure})

	response.JSON(w, r, enclosure)
}

func (h *handler) updateEnclosureByIDHandler(w http.ResponseWriter, r *http.Request) {
	enclosureID := request.RouteInt64Param(r, "enclosureID")
	if enclosureID == 0 {
		response.JSONBadRequest(w, r, errors.New("invalid enclosure ID"))
		return
	}

	var enclosureUpdateRequest model.EnclosureUpdateRequest
	if err := json_parser.NewDecoder(r.Body).Decode(&enclosureUpdateRequest); err != nil {
		response.JSONBadRequest(w, r, err)
		return
	}

	if err := validator.ValidateEnclosureUpdateRequest(&enclosureUpdateRequest); err != nil {
		response.JSONBadRequest(w, r, err)
		return
	}

	enclosure, err := h.store.EnclosureByID(request.UserID(r), enclosureID)
	if err != nil {
		response.JSONServerError(w, r, err)
		return
	}

	if enclosure == nil {
		response.JSONNotFound(w, r)
		return
	}

	enclosure.MediaProgression = enclosureUpdateRequest.MediaProgression
	if err := h.store.UpdateEnclosure(enclosure); err != nil {
		response.JSONServerError(w, r, err)
		return
	}

	response.NoContent(w, r)
}
