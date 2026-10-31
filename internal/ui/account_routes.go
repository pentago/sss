// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"net/http"
	"runtime"
	"strconv"
	"strings"

	"sss/internal/config"
	"sss/internal/http/request"
	"sss/internal/http/response"
	"sss/internal/model"
	"sss/internal/validator"
	"sss/internal/version"
)

func (h *handler) registerAccountRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /{$}", h.showLoginPage)
	mux.HandleFunc("POST /login", h.checkLogin)
	mux.HandleFunc("POST /logout", h.logout)

	if config.Opts.OAuth2Provider != "" {
		mux.HandleFunc("GET /oauth2/{provider}/redirect", h.oauth2Redirect)
		mux.HandleFunc("GET /oauth2/{provider}/callback", h.oauth2Callback)
		mux.HandleFunc("POST /oauth2/{provider}/unlink", h.oauth2Unlink)
	}

	if config.Opts.WebAuthn {
		mux.HandleFunc("GET /webauthn/register/begin", h.webAuthnRegisterBegin)
		mux.HandleFunc("POST /webauthn/register/finish", h.webAuthnRegisterFinish)
		mux.HandleFunc("GET /webauthn/login/begin", h.webAuthnLoginBegin)
		mux.HandleFunc("POST /webauthn/login/finish", h.webAuthnLoginFinish)
		mux.HandleFunc("POST /webauthn/deleteall", h.webAuthnDeleteAll)
		mux.HandleFunc("POST /webauthn/{credentialHandle}/delete", h.webAuthnDelete)
		mux.HandleFunc("GET /webauthn/{credentialHandle}/rename", h.webAuthnRename)
		mux.HandleFunc("POST /webauthn/{credentialHandle}/save", h.webAuthnSave)
	}

	mux.HandleFunc("GET /settings", h.showSettings)
	mux.HandleFunc("POST /settings", h.updateSettings)
	mux.HandleFunc("GET /authentication", h.showAuthentication)
	mux.HandleFunc("POST /authentication", h.updateAuthentication)
	mux.HandleFunc("GET /integrations", h.showIntegrations)
	mux.HandleFunc("POST /integration", h.updateIntegrations)
	mux.HandleFunc("GET /about", h.showAbout)

	mux.HandleFunc("GET /users", adminOnly(h.showUsers))
	mux.HandleFunc("GET /user/create", adminOnly(h.showCreateUser))
	mux.HandleFunc("POST /user/save", adminOnly(h.saveUser))
	mux.HandleFunc("GET /users/{userID}/edit", adminOnly(h.showEditUser))
	mux.HandleFunc("POST /users/{userID}/update", adminOnly(h.updateUser))
	mux.HandleFunc("POST /users/{userID}/remove", adminOnly(h.removeUser))

	mux.HandleFunc("GET /sessions", h.showSessions)
	mux.HandleFunc("POST /sessions/{sessionID}/remove", h.removeSession)

	if !config.Opts.DisableAPI {
		mux.HandleFunc("GET /keys", h.showAPIKeys)
		mux.HandleFunc("GET /keys/create", h.showCreateAPIKey)
		mux.HandleFunc("POST /keys/save", h.saveAPIKey)
		mux.HandleFunc("POST /keys/{keyID}/delete", h.deleteAPIKey)
	}
}

func (h *handler) showAPIKeys(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	keys, err := h.store.APIKeys(user.ID)
	if err != nil {
		response.HTMLServerError(w, r, err)
		return
	}
	h.renderAccount(w, r, "account_api_keys", map[string]any{"title": "API keys", "keys": keys, "apiEndpoint": config.Opts.BaseURL + "/v1/"})
}

func (h *handler) showCreateAPIKey(w http.ResponseWriter, r *http.Request) {
	h.renderAccount(w, r, "account_api_key_create", map[string]any{"title": "New API key"})
}

// saveAPIKey creates the key and shows its token once.
func (h *handler) saveAPIKey(w http.ResponseWriter, r *http.Request) {
	userID := request.UserID(r)
	req := &model.APIKeyCreationRequest{Description: r.FormValue("description")}
	if err := validator.ValidateAPIKeyCreation(h.store, userID, req); err != nil {
		h.renderAccount(w, r, "account_api_key_create", map[string]any{"title": "New API key", "description": req.Description, "errorMessage": err.Error()})
		return
	}
	key, err := h.store.CreateAPIKey(userID, req.Description)
	if err != nil {
		response.HTMLServerError(w, r, err)
		return
	}
	h.renderAccount(w, r, "account_api_key_created", map[string]any{"title": "API key created", "key": key, "apiEndpoint": config.Opts.BaseURL + "/v1/"})
}

func (h *handler) deleteAPIKey(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("keyID"), 10, 64)
	if err := h.store.DeleteAPIKey(request.UserID(r), id); err != nil {
		response.HTMLServerError(w, r, err)
		return
	}
	response.HTMLRedirect(w, r, route("/keys"))
}

func (h *handler) showSessions(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	sessions, err := h.store.WebSessionsByUserID(user.ID)
	if err != nil {
		response.HTMLServerError(w, r, err)
		return
	}
	for i := range sessions {
		sessions[i].UseTimezone(user.Timezone)
	}
	h.renderAccount(w, r, "account_sessions", map[string]any{"title": "Sessions", "sessions": sessions, "currentSessionID": request.WebSession(r).ID})
}

func (h *handler) removeSession(w http.ResponseWriter, r *http.Request) {
	if err := h.store.RemoveUserWebSession(request.UserID(r), r.PathValue("sessionID")); err != nil {
		response.HTMLServerError(w, r, err)
		return
	}
	response.HTMLRedirect(w, r, route("/sessions"))
}

func (h *handler) showAbout(w http.ResponseWriter, r *http.Request) {
	dbSize, err := h.store.DBSize()
	if err != nil {
		dbSize = err.Error()
	}
	h.renderAccount(w, r, "account_about", map[string]any{
		"title":     "About",
		"version":   version.Version,
		"commit":    version.Commit,
		"buildDate": version.BuildDate,
		"dbVersion": h.store.DatabaseVersion(),
		"dbSize":    dbSize,
		"goVersion": runtime.Version(),
		"config":    config.Opts.Dump(),
	})
}

// renderAccount renders an account page with the config flags the account templates need.
func (h *handler) renderAccount(w http.ResponseWriter, r *http.Request, name string, data map[string]any) {
	data["menu"] = "settings"
	data["apiEnabled"] = !config.Opts.DisableAPI
	data["webAuthnEnabled"] = config.Opts.WebAuthn
	data["localAuth"] = !config.Opts.DisableLocalAuth
	// Settings sub-navigation highlights the first path segment.
	page, _, _ := strings.Cut(strings.TrimPrefix(r.URL.Path, "/"), "/")
	switch page {
	case "user":
		page = "users"
	case "webauthn":
		page = "authentication"
	}
	data["page"] = page
	h.render(w, r, name, data)
}
