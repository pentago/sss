// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

package request

import (
	"cmp"
	"net/http"

	"sss/internal/model"
)

// ContextKey represents a context key.
type ContextKey int

// List of context keys.
const (
	UserIDContextKey ContextKey = iota
	UserNameContextKey
	UserTimezoneContextKey
	IsAdminUserContextKey
	IsAuthenticatedContextKey
	WebSessionContextKey
	ClientIPContextKey
	GoogleReaderTokenKey
	IsHTTPSContextKey
)

// WebSession returns the current web session from the request context, if present.
func WebSession(r *http.Request) *model.WebSession {
	if v := r.Context().Value(WebSessionContextKey); v != nil {
		if value, valid := v.(*model.WebSession); valid {
			return value
		}
	}
	return nil
}

// GoogleReaderToken returns the Google Reader token from the request context, if present.
func GoogleReaderToken(r *http.Request) string {
	return ctxValue[string](r, GoogleReaderTokenKey)
}

// IsAdminUser reports whether the logged-in user is an administrator.
func IsAdminUser(r *http.Request) bool {
	return ctxValue[bool](r, IsAdminUserContextKey)
}

// IsAuthenticated reports whether the user is authenticated.
func IsAuthenticated(r *http.Request) bool {
	if ctxValue[bool](r, IsAuthenticatedContextKey) {
		return true
	}
	if session := WebSession(r); session != nil {
		return session.IsAuthenticated()
	}
	return false
}

// UserID returns the logged-in user's ID from the request context.
func UserID(r *http.Request) int64 {
	if userID := ctxValue[int64](r, UserIDContextKey); userID != 0 {
		return userID
	}
	if session := WebSession(r); session != nil {
		if id, ok := session.UserID(); ok {
			return id
		}
	}
	return 0
}

// UserName returns the logged-in user's username, or "unknown" when unset.
func UserName(r *http.Request) string {
	return cmp.Or(ctxValue[string](r, UserNameContextKey), "unknown")
}

// UserTimezone returns the user's timezone, defaulting to "UTC" when unset.
func UserTimezone(r *http.Request) string {
	return cmp.Or(ctxValue[string](r, UserTimezoneContextKey), "UTC")
}

// ClientIP returns the client IP address stored in the request context.
func ClientIP(r *http.Request) string {
	return ctxValue[string](r, ClientIPContextKey)
}

// IsHTTPS reports whether the request reached the server over HTTPS (directly, via the HTTPS
// setting, or through a trusted reverse proxy sending X-Forwarded-Proto: https).
func IsHTTPS(r *http.Request) bool {
	return ctxValue[bool](r, IsHTTPSContextKey)
}

// ctxValue returns the value stored in the request context at key, or the
// zero value of T when the key is unset or holds a different type.
func ctxValue[T any](r *http.Request, key ContextKey) T {
	if v, valid := r.Context().Value(key).(T); valid {
		return v
	}
	var zero T
	return zero
}
