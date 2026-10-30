package subscription

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"sss/internal/config"
	"sss/internal/reader/fetcher"
)

func setup(t *testing.T) {
	opts, err := config.Parse([]string{"FETCHER_ALLOW_PRIVATE_NETWORKS=1", "INTEGRATION_ALLOW_PRIVATE_NETWORKS=1"})
	if err != nil {
		t.Fatal(err)
	}
	old := config.Opts
	config.Opts = opts
	t.Cleanup(func() { config.Opts = old })
}

func find(t *testing.T, h http.HandlerFunc, path, bridge string) Subscriptions {
	t.Helper()
	s := httptest.NewServer(h)
	defer s.Close()
	if bridge == "bridge" {
		bridge = s.URL + "/bridge/"
	}
	subs, err := NewSubscriptionFinder(fetcher.NewRequestBuilder()).FindSubscriptions(s.URL+path, bridge, "")
	if err != nil {
		t.Fatal(err)
	}
	return subs
}

func TestFindSubscriptions(t *testing.T) {
	setup(t)

	// URL is a feed.
	subs := find(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<?xml version="1.0"?><feed xmlns="http://www.w3.org/2005/Atom"></feed>`)
	}, "/feed", "")
	if len(subs) != 1 || subs[0].Type != "atom" {
		t.Errorf("feed: %v", subs)
	}

	// HTML link tags, relative to <base>, deduplicated, wp-json ignored.
	subs = find(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><head><base href="https://example.org/blog/">
<link rel="alternate" type="application/rss+xml" title="RSS" href="rss">
<link rel="alternate" type="application/rss+xml" href="rss">
<link rel="alternate" type="application/json" href="/wp-json/x">
<link rel="alternate" type="application/feed+json" href="/feed.json">
</head></html>`)
	}, "/", "")
	if len(subs) != 2 || *subs[0] != (Subscription{"RSS", "https://example.org/blog/rss", "rss"}) ||
		*subs[1] != (Subscription{"https://example.org/feed.json", "https://example.org/feed.json", "json"}) {
		t.Errorf("html: %v %v", subs[0], subs)
	}

	// RSS-Bridge, then well-known URLs (redirects ignored).
	subs = find(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/bridge/":
			if r.URL.Query().Get("action") != "findfeed" {
				t.Error("bad bridge query")
			}
			fmt.Fprint(w, `[{"url":"./?action=display&bridge=X","bridgeMeta":{"name":"X"}}]`)
		default:
			fmt.Fprint(w, `<html></html>`)
		}
	}, "/", "bridge")
	if len(subs) != 1 || subs[0].Title != "X" {
		t.Errorf("bridge: %v", subs)
	}

	subs = find(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rss.xml":
			fmt.Fprint(w, "<rss/>")
		case "/atom.xml":
			http.Redirect(w, r, "/", http.StatusFound)
		case "/":
			fmt.Fprint(w, `<html></html>`)
		default:
			http.NotFound(w, r)
		}
	}, "/", "")
	if len(subs) != 1 || subs[0].Type != "rss" {
		t.Errorf("well-known: %v", subs)
	}
}

func TestYouTubeAndGitHub(t *testing.T) {
	if s := findSubscriptionsFromYouTube("https://www.youtube.com/channel/UC123"); len(s) != 4 ||
		s[0].URL != "https://www.youtube.com/feeds/videos.xml?channel_id=UC123" || s[1].URL != "https://www.youtube.com/feeds/videos.xml?playlist_id=UULF123" {
		t.Errorf("channel: %v", s)
	}
	if s := findSubscriptionsFromYouTube("https://www.youtube.com/watch?v=a&list=PL1"); len(s) != 1 || s[0].URL != "https://www.youtube.com/feeds/videos.xml?playlist_id=PL1" {
		t.Errorf("playlist: %v", s)
	}
	if s := findSubscriptionsFromGitHub("https://github.com/o/r"); len(s) != 3 || s[1].URL != "https://github.com/o/r/releases.atom" {
		t.Errorf("github: %v", s)
	}
	if s := findSubscriptionsFromGitHub("https://github.com/o/r/tree/main"); s != nil {
		t.Errorf("github deep: %v", s)
	}
}
