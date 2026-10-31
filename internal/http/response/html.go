// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

package response

import (
	"fmt"
	"html"
	"log/slog"
	"net/http"

	"sss/internal/urllib"
)

// HTML creates a new HTML response with a 200 status code.
func HTML(w http.ResponseWriter, r *http.Request, body []byte) {
	NewBuilder(w, r).
		WithHeader("Content-Type", "text/html; charset=utf-8").
		WithHeader("Cache-Control", noCacheControl).
		WithBodyAsBytes(body).
		Write()
}

// HTMLServerError sends an internal error to the client.
func HTMLServerError(w http.ResponseWriter, r *http.Request, err error) {
	writeError(w, r, slog.LevelError, http.StatusInternalServerError, err, html.EscapeString(err.Error()),
		"Content-Security-Policy", ContentSecurityPolicyForUntrustedContent,
		"Content-Type", "text/plain; charset=utf-8",
		"Cache-Control", noCacheControl)
}

// HTMLBadRequest sends a bad request error to the client.
func HTMLBadRequest(w http.ResponseWriter, r *http.Request, err error) {
	writeError(w, r, slog.LevelWarn, http.StatusBadRequest, err, html.EscapeString(err.Error()),
		"Content-Security-Policy", ContentSecurityPolicyForUntrustedContent,
		"Content-Type", "text/plain; charset=utf-8",
		"Cache-Control", noCacheControl)
}

// HTMLForbidden sends a forbidden error to the client.
func HTMLForbidden(w http.ResponseWriter, r *http.Request) {
	writeError(w, r, slog.LevelWarn, http.StatusForbidden, nil, "Access Forbidden",
		"Content-Type", "text/html; charset=utf-8",
		"Cache-Control", noCacheControl)
}

// HTMLNotFound sends a page not found error to the client.
func HTMLNotFound(w http.ResponseWriter, r *http.Request) {
	writeError(w, r, slog.LevelWarn, http.StatusNotFound, nil, "Page Not Found",
		"Content-Type", "text/html; charset=utf-8",
		"Cache-Control", noCacheControl)
}

// HTMLRedirect redirects the user with 303 See Other to a relative path or an absolute http(s) URL.
func HTMLRedirect(w http.ResponseWriter, r *http.Request, uri string) {
	if !urllib.IsRelativePath(uri) && !urllib.IsAbsoluteURL(uri) {
		HTMLBadRequest(w, r, fmt.Errorf("invalid redirect URL: %q", uri))
		return
	}
	http.Redirect(w, r, uri, http.StatusSeeOther)
}
