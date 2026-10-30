package handler

import (
	"path/filepath"
	"strings"
	"testing"

	"sss/internal/database"
	"sss/internal/model"
	"sss/internal/storage"
)

func TestImportOPML(t *testing.T) {
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
	first, err := store.FirstCategory(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateFeed(&model.Feed{UserID: user.ID, Category: first, FeedURL: "https://a.example/feed", Title: "A"}); err != nil {
		t.Fatal(err)
	}

	const doc = `<?xml version="1.0"?><opml version="2.0"><body>
<outline text="News">
  <outline type="rss" text="A" xmlUrl="https://a.example/feed"/>
  <outline type="rss" text="B" xmlUrl="https://b.example/feed"/>
</outline>
<outline type="rss" text="C" xmlUrl="https://c.example/feed"/>
</body></opml>`
	if err := ImportOPML(store, user.ID, strings.NewReader(doc)); err != nil {
		t.Fatal(err)
	}

	feeds, err := store.Feeds(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, f := range feeds {
		got[f.FeedURL] = f.Category.Title
	}
	want := map[string]string{
		"https://a.example/feed": first.Title, // already subscribed: left in place
		"https://b.example/feed": "News",
		"https://c.example/feed": first.Title,
	}
	if len(got) != len(want) {
		t.Fatalf("feeds = %v, want %v", got, want)
	}
	for url, title := range want {
		if got[url] != title {
			t.Errorf("%s category = %q, want %q", url, got[url], title)
		}
	}
}
