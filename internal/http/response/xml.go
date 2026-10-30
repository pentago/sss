// Copyright The Miniflux Authors. Modified by The sss Authors.
// Derived from Miniflux (https://github.com/miniflux/v2).
// SPDX-License-Identifier: Apache-2.0

package response

import "net/http"

// XML writes a standard XML response with a status 200 OK.
func XML(w http.ResponseWriter, r *http.Request, body string) {
	NewBuilder(w, r).
		WithHeader("Content-Type", "text/xml; charset=utf-8").
		WithBodyAsString(body).
		Write()
}
