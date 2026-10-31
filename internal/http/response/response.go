// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

package response

import (
	"context"
	"log/slog"
	"net/http"

	"sss/internal/http/request"
)

// ContentSecurityPolicyForUntrustedContent is the default CSP for untrusted content.
// default-src 'none' disables all content sources
// form-action 'none' disables all form submissions
// sandbox enables a sandbox for the requested resource
// https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Content-Security-Policy
// https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Content-Security-Policy/form-action
// https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Content-Security-Policy/sandbox
// https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Content-Security-Policy/default-src
const ContentSecurityPolicyForUntrustedContent = `default-src 'none'; form-action 'none'; sandbox;`

const noCacheControl = "no-cache, max-age=0, must-revalidate, no-store"

// NoContent sends a no content response to the client.
func NoContent(w http.ResponseWriter, r *http.Request) {
	NewBuilder(w, r).
		WithStatus(http.StatusNoContent).
		Write()
}

// writeError logs the failed request and writes the error response with the
// given status, body and header pairs. err is included in the log when non-nil.
func writeError(w http.ResponseWriter, r *http.Request, level slog.Level, status int, err error, body string, headers ...string) {
	attrs := make([]any, 0, 8)
	if err != nil {
		attrs = append(attrs, slog.Any("error", err))
	}
	attrs = append(attrs,
		slog.String("client_ip", request.ClientIP(r)),
		slog.Group("request",
			slog.String("method", r.Method),
			slog.String("uri", r.RequestURI),
			slog.String("user_agent", r.UserAgent()),
		),
		slog.Group("response",
			slog.Int("status_code", status),
		),
	)
	slog.Log(context.Background(), level, http.StatusText(status), attrs...)

	b := NewBuilder(w, r).WithStatus(status)
	for i := 0; i+1 < len(headers); i += 2 {
		b = b.WithHeader(headers[i], headers[i+1])
	}
	b.WithBodyAsString(body).Write()
}
