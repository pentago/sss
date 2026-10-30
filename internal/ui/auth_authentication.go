// Copyright The Miniflux Authors. Modified by The sss Authors.
// Derived from Miniflux (https://github.com/miniflux/v2).
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"log/slog"
	"net/http"

	"sss/internal/config"
	"sss/internal/http/request"
	"sss/internal/http/response"
	"sss/internal/model"
	"sss/internal/validator"
)

func (h *handler) renderAuthentication(w http.ResponseWriter, r *http.Request, username, errorMessage string) {
	user := currentUser(r)
	credentials, err := h.store.WebAuthnCredentialsByUserID(user.ID)
	if err != nil {
		response.HTMLServerError(w, r, err)
		return
	}
	hasPassword, err := h.store.HasPassword(user.ID)
	if err != nil {
		response.HTMLServerError(w, r, err)
		return
	}
	data := map[string]any{
		"title":       "Authentication",
		"menu":        "settings",
		"username":    username,
		"hasPassword": hasPassword,
		"oauth2":      config.Opts.OAuth2Provider,
		"oauth2Name":  oauth2DisplayName(),
		"oauth2Linked": (config.Opts.OAuth2Provider == "google" && user.GoogleID != "") ||
			(config.Opts.OAuth2Provider == "oidc" && user.OpenIDConnectID != ""),
		"webauthn":    true,
		"credentials": credentials,
	}
	if errorMessage != "" {
		data["errorMessage"] = errorMessage
	}
	h.renderAccount(w, r, "auth_authentication", data)
}

func (h *handler) showAuthentication(w http.ResponseWriter, r *http.Request) {
	h.renderAuthentication(w, r, currentUser(r).Username, "")
}

func (h *handler) updateAuthentication(w http.ResponseWriter, r *http.Request) {
	if config.Opts.DisableLocalAuth {
		response.HTMLForbidden(w, r)
		return
	}
	user := currentUser(r)
	username := r.FormValue("username")
	password, confirmation := r.FormValue("password"), r.FormValue("confirmation")
	if confirmation == "" {
		// Browsers autofill the password field; a blank confirmation means no change intended.
		password = ""
	}
	if password != confirmation && password != "" {
		h.renderAuthentication(w, r, username, "Passwords are not the same.")
		return
	}
	changes := &model.UserModificationRequest{Username: &username}
	if password != "" {
		hasPassword, err := h.store.HasPassword(user.ID)
		if err != nil {
			response.HTMLServerError(w, r, err)
			return
		}
		// Users created through OAuth2 have no password yet, so there is nothing to confirm.
		if hasPassword && h.store.CheckPassword(user.Username, r.FormValue("current_password")) != nil {
			slog.Warn("Incorrect current password on password change", slog.Int64("user_id", user.ID))
			h.renderAuthentication(w, r, username, "The current password is incorrect.")
			return
		}
		changes.Password = &password
	}
	if err := validator.ValidateUserModification(h.store, user.ID, changes); err != nil {
		h.renderAuthentication(w, r, username, err.Error())
		return
	}
	updated := *user
	changes.Patch(&updated)
	if err := h.store.UpdateUser(&updated); err != nil {
		response.HTMLServerError(w, r, err)
		return
	}
	sess := request.WebSession(r)
	if password != "" {
		// Sign out every other session and rotate this one in case its cookie leaked with the old password.
		if err := h.store.RemoveUserWebSessionsExcept(user.ID, sess.ID); err != nil {
			response.HTMLServerError(w, r, err)
			return
		}
		if err := authenticateWebSession(w, r, h.store, &updated); err != nil {
			response.HTMLServerError(w, r, err)
			return
		}
	}
	sess.SetSuccessMessage("Preferences saved.")
	response.HTMLRedirect(w, r, route("/authentication"))
}
