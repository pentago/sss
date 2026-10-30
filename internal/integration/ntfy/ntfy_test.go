// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

package ntfy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"sss/internal/config"
	"sss/internal/model"
)

func TestSendMessages(t *testing.T) {
	opts, err := config.Parse([]string{"INTEGRATION_ALLOW_PRIVATE_NETWORKS=1", "BASE_URL=https://reader.example.org/sub"})
	if err != nil {
		t.Fatal(err)
	}
	prev := config.Opts
	config.Opts = opts
	t.Cleanup(func() { config.Opts = prev })

	var msgs []ntfyMessage
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		var m ntfyMessage
		if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
			t.Error(err)
		}
		msgs = append(msgs, m)
	}))
	defer srv.Close()

	feed := &model.Feed{Title: "Feed"}
	entries := model.Entries{{ID: 7, Title: "One", URL: "https://example.org/1"}, {ID: 8, Title: "Two"}}
	if err := NewClient(srv.URL, "topic", "tok", "", "", "https://icon", true, 4).SendMessages(feed, entries); err != nil {
		t.Fatal(err)
	}

	if auth != "Bearer tok" {
		t.Errorf("auth = %q", auth)
	}
	if len(msgs) != 2 {
		t.Fatalf("got %d messages", len(msgs))
	}
	want := ntfyMessage{Topic: "topic", Message: "One", Title: "Feed", Priority: 4, Click: "https://reader.example.org/sub/unread/entry/7", Icon: "https://icon"}
	if !reflect.DeepEqual(msgs[0], want) {
		t.Errorf("message = %+v, want %+v", msgs[0], want)
	}
}
