// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

package googlereader

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"net/http"
	"strings"

	"sss/internal/crypto"
	"sss/internal/http/request"
	"sss/internal/storage"
)

// withAuth authenticates the request token ("T" form field for POST, "Authorization: GoogleLogin
// auth=<token>" header otherwise) and stores the user in the request context.
func withAuth(store *storage.Storage, next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, reason := requestToken(r)
		username, _, _ := strings.Cut(token, "/")
		if reason == "" && (username == "" || strings.Count(token, "/") != 1) {
			reason = "auth token does not have the expected structure username/hash"
		}
		if reason != "" {
			unauthorized(w, r, reason, nil)
			return
		}

		integration, err := store.GoogleReaderUserGetIntegration(username)
		if err != nil {
			unauthorized(w, r, "no user found with the given Google Reader username", err)
			return
		}
		if !crypto.ConstantTimeCmp(getAuthToken(integration.GoogleReaderUsername, integration.GoogleReaderPassword), token) {
			unauthorized(w, r, "token does not match", nil)
			return
		}
		user, err := store.UserByID(integration.UserID)
		if err != nil || user == nil {
			unauthorized(w, r, "unable to fetch user", err)
			return
		}

		store.SetLastLogin(user.ID)

		ctx := r.Context()
		ctx = context.WithValue(ctx, request.UserIDContextKey, user.ID)
		ctx = context.WithValue(ctx, request.UserNameContextKey, user.Username)
		ctx = context.WithValue(ctx, request.UserTimezoneContextKey, user.Timezone)
		ctx = context.WithValue(ctx, request.IsAdminUserContextKey, user.IsAdmin)
		ctx = context.WithValue(ctx, request.IsAuthenticatedContextKey, true)
		ctx = context.WithValue(ctx, request.GoogleReaderTokenKey, token)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// requestToken extracts the auth token, or returns a non-empty reason why it is missing/malformed.
func requestToken(r *http.Request) (token, reason string) {
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			return "", "could not parse request form data"
		}
		if token = r.Form.Get("T"); token == "" {
			return "", "post-form T field is empty"
		}
		return token, ""
	}

	fields := strings.Fields(r.Header.Get("Authorization"))
	if len(fields) != 2 || fields[0] != "GoogleLogin" {
		return "", "authorization header does not have the expected format GoogleLogin auth=xxxxxx"
	}
	token, ok := strings.CutPrefix(fields[1], "auth=")
	if !ok || strings.Contains(token, "=") {
		return "", "authorization header does not have the expected format GoogleLogin auth=xxxxxx"
	}
	return token, ""
}

func unauthorized(w http.ResponseWriter, r *http.Request, reason string, err error) {
	slog.Warn("[GoogleReader] Authentication failed: "+reason,
		slog.Bool("authentication_failed", true),
		slog.String("client_ip", request.ClientIP(r)),
		slog.String("user_agent", r.UserAgent()),
		slog.Any("error", err),
	)
	sendUnauthorizedResponse(w, r)
}

func getAuthToken(username, password string) string {
	return username + "/" + hex.EncodeToString(hmac.New(sha256.New, []byte(username+password)).Sum(nil))
}
