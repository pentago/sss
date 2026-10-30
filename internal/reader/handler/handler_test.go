package handler

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
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
		if err := RefreshFeed(store, user.ID, feed.ID, false); err != nil {
			t.Fatal(err)
		}
	}
	if n := countEntries(); n != 3 {
		t.Fatalf("entries after refresh = %d, want 3", n)
	}

	if err := RefreshFeed(store, user.ID, feed.ID+1, false); err != ErrFeedNotFound {
		t.Errorf("unknown feed: got %v, want ErrFeedNotFound", err)
	}
}
