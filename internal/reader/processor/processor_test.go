package processor

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"sss/internal/config"
	"sss/internal/model"
)

func TestProcessFeedEntries(t *testing.T) {
	var err error
	if config.Opts, err = config.Parse([]string{"FETCHER_ALLOW_PRIVATE_NETWORKS=1"}); err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><body><article><p>Scraped <script>alert(1)</script></p></article></body></html>`))
	}))
	defer server.Close()

	feed := &model.Feed{
		FeedURL:        server.URL + "/feed.xml",
		SiteURL:        server.URL,
		Crawler:        true,
		ScraperRules:   "article",
		BlocklistRules: "(?i)spam",
		Entries: model.Entries{
			{Hash: "new", URL: server.URL + "/new?utm_source=x", Title: "New", Content: "<p>Original</p>"},
			{Hash: "old", URL: server.URL + "/old", Title: "Old", Content: `<p onclick="x()">Kept</p>`},
			{Hash: "spam", URL: server.URL + "/spam", Title: "Spam offer"},
		},
	}
	user := &model.User{ShowReadingTime: true, DefaultReadingSpeed: 265, CJKReadingSpeed: 500}

	ProcessFeedEntries(fakeStore{"new": true}, feed, user, false)

	if len(feed.Entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(feed.Entries))
	}
	byHash := map[string]*model.Entry{}
	for _, e := range feed.Entries {
		byHash[e.Hash] = e
	}
	if e := byHash["new"]; e.Content != "<p>Scraped </p>" || e.URL != server.URL+"/new" || e.ReadingTime != 1 {
		t.Errorf("new entry: content %q, url %q, reading time %d", e.Content, e.URL, e.ReadingTime)
	}
	if e := byHash["old"]; e.Content != "<p>Kept</p>" {
		t.Errorf("old entry should not be scraped, got %q", e.Content)
	}
}

// fakeStore treats hashes mapped to true as new and returns stored reading time 7 for the others.
type fakeStore map[string]bool

func (s fakeStore) IsNewEntry(_ int64, hash string) bool { return s[hash] }
func (s fakeStore) GetReadTime(int64, string) int        { return 7 }
