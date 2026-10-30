// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

package integration

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"sss/internal/config"
	"sss/internal/model"
)

func TestPushEntriesOnlyEnabled(t *testing.T) {
	opts, err := config.Parse([]string{"INTEGRATION_ALLOW_PRIVATE_NETWORKS=1"})
	if err != nil {
		t.Fatal(err)
	}
	prev := config.Opts
	config.Opts = opts
	t.Cleanup(func() { config.Opts = prev })

	var mu sync.Mutex
	hits := map[string]int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hits[r.URL.Path]++
		mu.Unlock()
	}))
	defer srv.Close()

	feed := &model.Feed{ID: 1, Title: "F", Category: &model.Category{ID: 1}}
	entries := model.Entries{{ID: 1, Title: "E", URL: "https://example.org/e"}}
	ui := &model.Integration{
		DiscordEnabled:     true,
		DiscordWebhookLink: srv.URL + "/discord",
		SlackEnabled:       false,
		SlackWebhookLink:   srv.URL + "/slack",
		WebhookEnabled:     true,
		WebhookURL:         srv.URL + "/webhook",
		NtfyEnabled:        true, // feed.NtfyEnabled is false: must not fire
		NtfyURL:            srv.URL + "/ntfy",
		NtfyTopic:          "t",
	}
	PushEntries(feed, entries, ui)

	feed.WebhookURL = srv.URL + "/feed-webhook"
	PushEntries(feed, entries, ui)

	want := map[string]int{"/discord": 2, "/webhook": 1, "/feed-webhook": 1}
	if len(hits) != len(want) {
		t.Fatalf("hits = %v, want %v", hits, want)
	}
	for k, v := range want {
		if hits[k] != v {
			t.Errorf("hits = %v, want %v", hits, want)
		}
	}
}
