// Copyright The Miniflux Authors. Modified by The sss Authors.
// Derived from Miniflux (https://github.com/miniflux/v2).
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"sss/internal/config"
	"sss/internal/model"
)

func TestSendNewEntriesWebhookEvent(t *testing.T) {
	opts, err := config.Parse([]string{"INTEGRATION_ALLOW_PRIVATE_NETWORKS=1"})
	if err != nil {
		t.Fatal(err)
	}
	prev := config.Opts
	config.Opts = opts
	t.Cleanup(func() { config.Opts = prev })

	var body []byte
	var hdr http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ = io.ReadAll(r.Body)
		hdr = r.Header
	}))
	defer srv.Close()

	feed := &model.Feed{ID: 1, UserID: 2, Title: "F", Category: &model.Category{ID: 3, Title: "C"}}
	entries := model.Entries{{ID: 10, Title: "E", URL: "https://example.org/e", Tags: []string{"t"}}}
	if err := NewClient(srv.URL, "secret").SendNewEntriesWebhookEvent(feed, entries); err != nil {
		t.Fatal(err)
	}

	mac := hmac.New(sha256.New, []byte("secret"))
	mac.Write(body)
	if got, want := hdr.Get("X-Miniflux-Signature"), hex.EncodeToString(mac.Sum(nil)); got != want {
		t.Errorf("signature = %q, want %q", got, want)
	}
	if got := hdr.Get("X-Miniflux-Event-Type"); got != NewEntriesEventType {
		t.Errorf("event type header = %q", got)
	}
	if got := hdr.Get("Content-Type"); got != "application/json" {
		t.Errorf("content type = %q", got)
	}

	var ev map[string]any
	if err := json.Unmarshal(body, &ev); err != nil {
		t.Fatal(err)
	}
	if ev["event_type"] != "new_entries" {
		t.Errorf("event_type = %v", ev["event_type"])
	}
	f := ev["feed"].(map[string]any)
	if f["category_id"] != 3.0 || f["category"].(map[string]any)["title"] != "C" {
		t.Errorf("feed = %v", f)
	}
	e := ev["entries"].([]any)[0].(map[string]any)
	for _, k := range []string{"id", "user_id", "feed_id", "status", "hash", "title", "url", "comments_url", "published_at", "created_at", "changed_at", "content", "author", "share_code", "starred", "reading_time", "enclosures", "tags"} {
		if _, ok := e[k]; !ok {
			t.Errorf("entry missing %q", k)
		}
	}
}
