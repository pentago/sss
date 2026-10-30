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
	"sss/internal/urllib"
)

// login binds the session to user, records the login and redirects to redirectURL when it is a
// same-origin relative path, else to the user's home page.
func (h *handler) login(w http.ResponseWriter, r *http.Request, user *model.User, redirectURL string) {
	if err := h.store.SetLastLogin(user.ID); err != nil {
		slog.Warn("Unable to update last login date", slog.Int64("user_id", user.ID), slog.Any("error", err))
	}
	if err := authenticateWebSession(w, r, h.store, user); err != nil {
		response.HTMLServerError(w, r, err)
		return
	}
	response.HTMLRedirect(w, r, loginRedirect(redirectURL, user))
}

func loginRedirect(redirectURL string, user *model.User) string {
	if redirectURL != "" && urllib.IsRelativePath(redirectURL) {
		return redirectURL
	}
	return config.Opts.BasePath + "/" + user.DefaultHomePage
}

func (h *handler) showLoginPage(w http.ResponseWriter, r *http.Request) {
	if user := currentUser(r); user != nil {
		response.HTMLRedirect(w, r, config.Opts.BasePath+"/"+user.DefaultHomePage)
		return
	}
	if h.authProxyLogin(w, r) {
		return
	}
	h.renderLogin(w, r, "", r.URL.Query().Get("redirect_url"), "")
}

func (h *handler) renderLogin(w http.ResponseWriter, r *http.Request, username, redirectURL, errorMessage string) {
	data := map[string]any{
		"title":       "Sign in",
		"username":    username,
		"redirectURL": redirectURL,
		"oauth2":      config.Opts.OAuth2Provider,
		"oauth2Name":  oauth2DisplayName(),
		"webauthn":    true,
	}
	if errorMessage != "" {
		data["errorMessage"] = errorMessage
	}
	h.renderAccount(w, r, "auth_login", data)
}

func oauth2DisplayName() string {
	if config.Opts.OAuth2Provider == "google" {
		return "Google"
	}
	return config.Opts.OAuth2OIDCProviderName
}

func (h *handler) checkLogin(w http.ResponseWriter, r *http.Request) {
	username, password := r.FormValue("username"), r.FormValue("password")
	redirectURL := r.FormValue("redirect_url")
	if config.Opts.DisableLocalAuth {
		slog.Warn("Blocking local auth login attempt, local auth is disabled", slog.String("client_ip", request.ClientIP(r)))
		h.renderLogin(w, r, username, redirectURL, "")
		return
	}
	if username == "" || password == "" || h.store.CheckPassword(username, password) != nil {
		slog.Warn("Incorrect username or password",
			slog.Bool("authentication_failed", true),
			slog.String("client_ip", request.ClientIP(r)),
			slog.String("username", username),
		)
		h.renderLogin(w, r, username, redirectURL, "Invalid username or password.")
		return
	}
	user, err := h.store.UserByUsername(username)
	if err != nil || user == nil {
		response.HTMLServerError(w, r, err)
		return
	}
	slog.Info("User authenticated successfully with username/password",
		slog.String("client_ip", request.ClientIP(r)),
		slog.Int64("user_id", user.ID),
		slog.String("username", user.Username),
	)
	h.login(w, r, user, redirectURL)
}

func (h *handler) logout(w http.ResponseWriter, r *http.Request) {
	if s := request.WebSession(r); s != nil {
		if err := h.store.RemoveUserWebSession(request.UserID(r), s.ID); err != nil {
			response.HTMLServerError(w, r, err)
			return
		}
	}
	response.HTMLRedirect(w, r, config.Opts.BasePath+"/")
}

// authProxyLogin logs in the user named by AUTH_PROXY_HEADER when the request comes from a
// trusted reverse proxy network. It reports whether it wrote a response.
func (h *handler) authProxyLogin(w http.ResponseWriter, r *http.Request) bool {
	header := config.Opts.AuthProxyHeader
	if header == "" {
		return false
	}
	username := r.Header.Get(header)
	if username == "" {
		return false
	}
	remoteIP := request.FindRemoteIP(r)
	if !request.IsTrustedIP(remoteIP, config.Opts.TrustedReverseProxyNetworks) {
		slog.Warn("[AuthProxy] Rejecting authentication request from untrusted proxy", slog.String("remote_ip", remoteIP))
		return false
	}
	user, err := h.store.UserByUsername(username)
	if err != nil {
		response.HTMLServerError(w, r, err)
		return true
	}
	if user == nil {
		if !config.Opts.AuthProxyUserCreation {
			response.HTMLForbidden(w, r)
			return true
		}
		if user, err = h.store.CreateUser(&model.UserCreationRequest{Username: username}); err != nil {
			response.HTMLServerError(w, r, err)
			return true
		}
	}
	slog.Info("[AuthProxy] User authenticated successfully", slog.Int64("user_id", user.ID), slog.String("username", user.Username))
	h.login(w, r, user, "")
	return true
}
