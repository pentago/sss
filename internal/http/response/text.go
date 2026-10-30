// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

package response

import "net/http"

// Text writes a standard text response with a status 200 OK.
func Text(w http.ResponseWriter, r *http.Request, body string) {
	NewBuilder(w, r).
		WithHeader("Content-Type", `text/plain; charset=utf-8`).
		WithBodyAsString(body).
		Write()
}
