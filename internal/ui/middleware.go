// Copyright The Miniflux Authors. Modified by The sss Authors.
// Derived from Miniflux (https://github.com/miniflux/v2).
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"sss/internal/config"
	"sss/internal/http/request"
	"sss/internal/http/response"
	"sss/internal/model"
	"sss/internal/storage"
)

const sessionCookieName = "MinifluxSessionID"

type contextKey struct{}

// currentUser returns the authenticated user loaded by the session middleware, or nil.
func currentUser(r *http.Request) *model.User {
	user, _ := r.Context().Value(contextKey{}).(*model.User)
	return user
}

// withSecurityHeaders sets the UI Content-Security-Policy and Referrer-Policy.
// The CSP is computed per request because it depends on the user's font hosts,
// so it is set again by the session middleware once the user is known.
func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", contentSecurityPolicy(nil))
		w.Header().Set("Referrer-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}

func contentSecurityPolicy(user *model.User) string {
	styleSrc, fontSrc := "'self'", ""
	if user != nil && user.ExternalFontHosts != "" {
		styleSrc += " " + user.ExternalFontHosts
		fontSrc = "; font-src 'self' " + user.ExternalFontHosts
	}
	return "default-src 'self'; img-src * data:; media-src *; frame-src *; script-src 'self'; require-trusted-types-for 'script'; trusted-types html url; style-src " + styleSrc + fontSrc
}

// withSession loads the web session from the cookie, creates an anonymous one
// for public pages, puts the user in the request context and redirects
// unauthenticated visitors to the login page.
func (h *handler) withSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isStaticAssetRoute(r.URL.Path) {
			// Custom CSS/JS and the manifest depend on the user; other assets need no session.
			switch r.URL.Path {
			case "/custom.css", "/custom.js", "/manifest.json":
				if session, err := h.loadWebSession(r); err == nil && session != nil {
					r = h.withUser(r, session)
				}
			}
			next.ServeHTTP(w, r)
			return
		}

		session, err := h.loadWebSession(r)
		if err != nil {
			response.HTMLServerError(w, r, err)
			return
		}
		public := isPublicRoute(r.URL.Path)
		if session == nil {
			if !public {
				response.HTMLRedirect(w, r, loginRedirectURL(r.RequestURI))
				return
			}
			var secret string
			session, secret = model.NewWebSession(r.UserAgent(), request.ClientIP(r))
			if err := h.store.CreateWebSession(session); err != nil {
				response.HTMLServerError(w, r, err)
				return
			}
			setSessionCookie(w, r, session, secret)
		}

		r = h.withUser(r, session)
		if !request.IsAuthenticated(r) && !public {
			response.HTMLRedirect(w, r, loginRedirectURL(r.RequestURI))
			return
		}
		if user := currentUser(r); user != nil {
			w.Header().Set("Content-Security-Policy", contentSecurityPolicy(user))
		} else if session.IsAuthenticated() {
			// The session points to a deleted user.
			session.ClearUser()
			if !public {
				response.HTMLRedirect(w, r, loginRedirectURL(r.RequestURI))
				h.saveSession(session)
				return
			}
		}

		next.ServeHTTP(w, r)
		h.saveSession(session)
	})
}

func (h *handler) saveSession(session *model.WebSession) {
	if !session.IsDirty() {
		return
	}
	if err := h.store.UpdateWebSession(session); err != nil {
		slog.Error("Unable to persist web session changes", slog.String("session_id", session.ID), slog.Any("error", err))
	}
}

// withUser stores the session and, when authenticated, the user in the request context.
func (h *handler) withUser(r *http.Request, session *model.WebSession) *http.Request {
	ctx := context.WithValue(r.Context(), request.WebSessionContextKey, session)
	if userID, ok := session.UserID(); ok {
		user, err := h.store.UserByID(userID)
		if err != nil {
			slog.Error("Unable to load session user", slog.Int64("user_id", userID), slog.Any("error", err))
		}
		if user != nil {
			ctx = context.WithValue(ctx, contextKey{}, user)
			ctx = context.WithValue(ctx, request.UserIDContextKey, user.ID)
			ctx = context.WithValue(ctx, request.UserNameContextKey, user.Username)
			ctx = context.WithValue(ctx, request.UserTimezoneContextKey, user.Timezone)
			ctx = context.WithValue(ctx, request.IsAdminUserContextKey, user.IsAdmin)
		}
	}
	return r.WithContext(ctx)
}

func (h *handler) loadWebSession(r *http.Request) (*model.WebSession, error) {
	sessionID, secret, ok := strings.Cut(request.CookieValue(r, sessionCookieName), ".")
	if !ok || sessionID == "" || secret == "" {
		return nil, nil
	}
	session, err := h.store.WebSessionByID(sessionID)
	if err != nil || session == nil || !session.VerifySecret(secret) {
		return nil, err
	}
	return session, nil
}

// authenticateWebSession binds the current browser session to user, rotates
// its identifier and secret (session fixation) and refreshes the cookie.
func authenticateWebSession(w http.ResponseWriter, r *http.Request, store *storage.Storage, user *model.User) error {
	session := request.WebSession(r)
	session.SetUser(user)
	oldID, secret := session.Rotate()
	if err := store.RotateWebSession(oldID, session); err != nil {
		return err
	}
	setSessionCookie(w, r, session, secret)
	return nil
}

// setSessionCookie writes the session cookie (HttpOnly, SameSite=Lax, Secure when HTTPS).
func setSessionCookie(w http.ResponseWriter, r *http.Request, session *model.WebSession, secret string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    session.ID + "." + secret,
		Path:     cookiePath(),
		Secure:   request.IsHTTPS(r),
		HttpOnly: true,
		Expires:  time.Now().Add(config.Opts.CleanupRemoveSessionsInterval),
		SameSite: http.SameSiteLaxMode,
	})
}

func cookiePath() string {
	if config.Opts.BasePath != "" {
		return config.Opts.BasePath
	}
	return "/"
}

// isStaticAssetRoute reports whether path needs no session at all.
func isStaticAssetRoute(path string) bool {
	switch path {
	case "/favicon.ico", "/robots.txt", "/service-worker.js", "/custom.css", "/custom.js", "/manifest.json":
		return true
	}
	return strings.HasPrefix(path, "/stylesheets/") ||
		strings.HasPrefix(path, "/js/") ||
		strings.HasPrefix(path, "/icon/") ||
		strings.HasPrefix(path, "/feed-icon/") ||
		strings.HasPrefix(path, "/proxy/")
}

// isPublicRoute reports whether path is reachable without being logged in.
func isPublicRoute(path string) bool {
	switch path {
	case "/", "/login", "/offline", "/webauthn/login/begin", "/webauthn/login/finish":
		return true
	}
	return strings.HasPrefix(path, "/oauth2/") && (strings.HasSuffix(path, "/redirect") || strings.HasSuffix(path, "/callback")) ||
		strings.HasPrefix(path, "/share/")
}

func loginRedirectURL(requestURI string) string {
	return config.Opts.BasePath + "/?" + url.Values{"redirect_url": {requestURI}}.Encode()
}
