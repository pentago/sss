// Copyright The Miniflux Authors. Modified by The sss Authors.
// Derived from Miniflux (https://github.com/miniflux/v2).
// SPDX-License-Identifier: Apache-2.0

package response

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestXMLResponse(t *testing.T) {
	r, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		XML(w, r, "Some XML")
	})

	handler.ServeHTTP(w, r)
	resp := w.Result()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf(`Unexpected status code, got %d instead of %d`, resp.StatusCode, http.StatusOK)
	}

	if actualBody := w.Body.String(); actualBody != "Some XML" {
		t.Fatalf(`Unexpected body, got %s instead of %s`, actualBody, "Some XML")
	}

	if actualContentType := resp.Header.Get("Content-Type"); actualContentType != "text/xml; charset=utf-8" {
		t.Fatalf(`Unexpected content type, got %q instead of %q`, actualContentType, "text/xml; charset=utf-8")
	}
}
