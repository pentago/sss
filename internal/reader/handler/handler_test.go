package handler

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"

	"sss/internal/config"
	"sss/internal/database"
	"sss/internal/model"
	"sss/internal/storage"
)

func TestCreateAndRefreshFeed(t *testing.T) {
	opts, err := config.Parse([]string{"FETCHER_ALLOW_PRIVATE_NETWORKS=1"})
	if err != nil {
		t.Fatal(err)
	}
	config.Opts = opts

	items := 2
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/feed.xml" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/rss+xml")
		fmt.Fprint(w, `<?xml version="1.0"?><rss version="2.0"><channel><title>Test</title><link>`+"http://"+r.Host+`/</link>`)
		for i := 1; i <= items; i++ {
			fmt.Fprintf(w, `<item><title>Item %d</title><link>http://%s/%d</link><guid>%d</guid><description>Body %d</description></item>`, i, r.Host, i, i, i)
		}
		fmt.Fprint(w, `</channel></rss>`)
	}))
	defer srv.Close()

	db, err := database.Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	store := storage.New(db)

	user, err := store.CreateUser(&model.UserCreationRequest{Username: "u", Password: "password123"})
	if err != nil {
		t.Fatal(err)
	}
	category, err := store.FirstCategory(user.ID)
	if err != nil {
		t.Fatal(err)
	}

	req := &model.FeedCreationRequest{FeedURL: srv.URL + "/feed.xml", CategoryID: category.ID}
	feed, err := CreateFeed(store, user.ID, req)
	if err != nil {
		t.Fatal(err)
	}
	if feed.Title != "Test" {
		t.Errorf("title = %q", feed.Title)
	}
	if _, err := CreateFeed(store, user.ID, req); err != ErrDuplicatedFeed {
		t.Errorf("second create: got %v, want ErrDuplicatedFeed", err)
	}

	countEntries := func() int {
		n, err := store.NewEntryQueryBuilder(user.ID).WithFeedID(feed.ID).CountEntries()
		if err != nil {
			t.Fatal(err)
		}
		return n
	}
	if n := countEntries(); n != 2 {
		t.Fatalf("entries after create = %d, want 2", n)
	}

	items = 3
	for range 2 {
		if err := RefreshFeed(t.Context(), store, user.ID, feed.ID, false); err != nil {
			t.Fatal(err)
		}
	}
	if n := countEntries(); n != 3 {
		t.Fatalf("entries after refresh = %d, want 3", n)
	}

	if err := RefreshFeed(t.Context(), store, user.ID, feed.ID+1, false); err != ErrFeedNotFound {
		t.Errorf("unknown feed: got %v, want ErrFeedNotFound", err)
	}
}

// Global settings fill in for feeds that leave them unset, but a feed's own
// user agent wins, and the global values are never copied into the feed.
func TestGlobalFeedSettings(t *testing.T) {
	prev := config.Opts
	t.Cleanup(func() { config.Opts = prev })
	opts, err := config.Parse([]string{"FETCHER_ALLOW_PRIVATE_NETWORKS=1", "HTTP_CLIENT_USER_AGENT=env-ua"})
	if err != nil {
		t.Fatal(err)
	}
	config.Opts = opts

	var mu sync.Mutex
	userAgents := map[string]string{} // path -> last User-Agent
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		userAgents[r.URL.Path] = r.UserAgent()
		mu.Unlock()
		if r.URL.Path == "/feed.xml" {
			w.Header().Set("Content-Type", "application/rss+xml")
			fmt.Fprintf(w, `<?xml version="1.0"?><rss version="2.0"><channel><title>T</title><link>http://%[1]s/</link><item><title>A</title><link>http://%[1]s/article</link><guid>1</guid></item></channel></rss>`, r.Host)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><body><article><p>Original</p></article></body></html>`)
	}))
	defer srv.Close()
	lastUA := func(path string) string {
		mu.Lock()
		defer mu.Unlock()
		ua := userAgents[path]
		delete(userAgents, path)
		return ua
	}

	db, err := database.Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	store := storage.New(db)

	user, err := store.CreateUser(&model.UserCreationRequest{Username: "u", Password: "password123"})
	if err != nil {
		t.Fatal(err)
	}
	user.GlobalUserAgent, user.GlobalCrawler = "global-ua", true
	if err := store.UpdateUser(user); err != nil {
		t.Fatal(err)
	}
	category, err := store.FirstCategory(user.ID)
	if err != nil {
		t.Fatal(err)
	}

	feed, err := CreateFeed(store, user.ID, &model.FeedCreationRequest{FeedURL: srv.URL + "/feed.xml", CategoryID: category.ID})
	if err != nil {
		t.Fatal(err)
	}
	if ua := lastUA("/feed.xml"); ua != "global-ua" {
		t.Errorf("feed fetched with %q, want global-ua", ua)
	}
	if ua := lastUA("/article"); ua != "global-ua" {
		t.Errorf("article fetched with %q, want global-ua (global crawler on)", ua)
	}

	feed, err = store.FeedByID(user.ID, feed.ID)
	if err != nil {
		t.Fatal(err)
	}
	if feed.UserAgent != "" || feed.Crawler {
		t.Fatalf("global settings were copied into the feed: user_agent=%q crawler=%v", feed.UserAgent, feed.Crawler)
	}

	feed.UserAgent = "feed-ua"
	if err := store.UpdateFeed(feed); err != nil {
		t.Fatal(err)
	}
	if err := RefreshFeed(t.Context(), store, user.ID, feed.ID, true); err != nil {
		t.Fatal(err)
	}
	if ua := lastUA("/feed.xml"); ua != "feed-ua" {
		t.Errorf("feed with its own user agent fetched with %q, want feed-ua", ua)
	}
	if ua := lastUA("/article"); ua != "feed-ua" {
		t.Errorf("article of feed with its own user agent fetched with %q, want feed-ua", ua)
	}

	feed.UserAgent = ""
	if err := store.UpdateFeed(feed); err != nil {
		t.Fatal(err)
	}
	user.GlobalUserAgent, user.GlobalCrawler = "", false
	if err := store.UpdateUser(user); err != nil {
		t.Fatal(err)
	}
	if err := RefreshFeed(t.Context(), store, user.ID, feed.ID, true); err != nil {
		t.Fatal(err)
	}
	if ua := lastUA("/feed.xml"); ua != "env-ua" {
		t.Errorf("feed fetched with %q, want env-ua", ua)
	}
	if ua := lastUA("/article"); ua != "" {
		t.Errorf("article fetched with crawler off everywhere")
	}
}
