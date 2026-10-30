// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

package discord

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"sss/internal/config"
	"sss/internal/model"
)

func TestSendDiscordMsg(t *testing.T) {
	opts, err := config.Parse([]string{"INTEGRATION_ALLOW_PRIVATE_NETWORKS=1"})
	if err != nil {
		t.Fatal(err)
	}
	prev := config.Opts
	config.Opts = opts
	t.Cleanup(func() { config.Opts = prev })

	var got discordMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("method=%s content-type=%s", r.Method, r.Header.Get("Content-Type"))
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Error(err)
		}
	}))
	defer srv.Close()

	feed := &model.Feed{Title: "Feed", SiteURL: "https://example.org/blog/"}
	entries := model.Entries{{Title: "Post", URL: "https://example.org/blog/post", Author: "Me"}}
	if err := NewClient(srv.URL).SendDiscordMsg(feed, entries); err != nil {
		t.Fatal(err)
	}

	if len(got.Embeds) != 1 {
		t.Fatalf("embeds = %+v", got.Embeds)
	}
	f := got.Embeds[0].Fields
	if len(f) != 4 || f[0].Value != "Feed" || f[1].Value != "[Post](https://example.org/blog/post)" || f[2].Value != "Me" || f[3].Value != "https://example.org/" {
		t.Errorf("fields = %+v", f)
	}
}
