package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"sss/internal/config"
	"sss/internal/database"
	"sss/internal/model"
	"sss/internal/storage"
)

type testEnv struct {
	srv   *httptest.Server
	store *storage.Storage
	user  *model.User
	token string
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	opts, err := config.Parse(nil)
	if err != nil {
		t.Fatal(err)
	}
	config.Opts = opts

	db, err := database.Open(t.TempDir() + "/db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	store := storage.New(db)

	user, err := store.CreateUser(&model.UserCreationRequest{Username: "alice", Password: "secret123", IsAdmin: true})
	if err != nil {
		t.Fatal(err)
	}
	key, err := store.CreateAPIKey(user.ID, "test")
	if err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(NewHandler(store, nil))
	t.Cleanup(srv.Close)
	return &testEnv{srv: srv, store: store, user: user, token: key.Token}
}

func (e *testEnv) do(t *testing.T, method, path, body string) (*http.Response, []byte) {
	t.Helper()
	req, _ := http.NewRequest(method, e.srv.URL+path, strings.NewReader(body))
	req.Header.Set("X-Auth-Token", e.token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return resp, data
}

// seedFeed creates a category and a feed with two unread entries.
func (e *testEnv) seedFeed(t *testing.T) *model.Feed {
	t.Helper()
	category, err := e.store.CreateCategory(e.user.ID, &model.CategoryCreationRequest{Title: "News"})
	if err != nil {
		t.Fatal(err)
	}
	feed := &model.Feed{UserID: e.user.ID, Category: category, Title: "Feed", FeedURL: "https://example.org/feed.xml", SiteURL: "https://example.org/"}
	for _, title := range []string{"Hello world", "Other"} {
		feed.Entries = append(feed.Entries, &model.Entry{
			Title: title, URL: "https://example.org/" + title, Hash: title, Status: model.EntryStatusUnread,
			Date: time.Now(), Content: "<p>" + title + "</p>",
		})
	}
	if err := e.store.CreateFeed(feed); err != nil {
		t.Fatal(err)
	}
	return feed
}

func TestAuth(t *testing.T) {
	env := newTestEnv(t)

	tests := []struct {
		name string
		set  func(*http.Request)
		want int
	}{
		{"token", func(r *http.Request) { r.Header.Set("X-Auth-Token", env.token) }, http.StatusOK},
		{"bad token", func(r *http.Request) { r.Header.Set("X-Auth-Token", "nope") }, http.StatusUnauthorized},
		{"basic", func(r *http.Request) { r.SetBasicAuth("alice", "secret123") }, http.StatusOK},
		{"bad basic", func(r *http.Request) { r.SetBasicAuth("alice", "wrong") }, http.StatusUnauthorized},
		{"none", func(*http.Request) {}, http.StatusUnauthorized},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", env.srv.URL+"/v1/me", nil)
			tt.set(req)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
			if resp.StatusCode != tt.want {
				t.Fatalf("status = %d, want %d", resp.StatusCode, tt.want)
			}
		})
	}

	config.Opts.DisableLocalAuth = true
	req, _ := http.NewRequest("GET", env.srv.URL+"/v1/me", nil)
	req.SetBasicAuth("alice", "secret123")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("basic auth with DISABLE_LOCAL_AUTH: status = %d", resp.StatusCode)
	}
}

func TestCreateCategory(t *testing.T) {
	env := newTestEnv(t)

	resp, body := env.do(t, "POST", "/v1/categories", `{"title":"Tech"}`)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d: %s", resp.StatusCode, body)
	}
	var category model.Category
	if err := json.Unmarshal(body, &category); err != nil {
		t.Fatal(err)
	}
	if category.ID == 0 || category.Title != "Tech" || category.UserID != env.user.ID {
		t.Fatalf("unexpected category: %+v", category)
	}

	resp, body = env.do(t, "POST", "/v1/categories", `{"title":"Tech"}`)
	if resp.StatusCode != http.StatusBadRequest || !strings.Contains(string(body), "error_message") {
		t.Fatalf("duplicate: status = %d: %s", resp.StatusCode, body)
	}
}

func TestListFeedsShape(t *testing.T) {
	env := newTestEnv(t)
	env.seedFeed(t)

	resp, body := env.do(t, "GET", "/v1/feeds", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d: %s", resp.StatusCode, body)
	}
	var feeds []map[string]any
	if err := json.Unmarshal(body, &feeds); err != nil {
		t.Fatal(err)
	}
	if len(feeds) != 1 {
		t.Fatalf("got %d feeds", len(feeds))
	}
	for _, key := range []string{"id", "user_id", "feed_url", "site_url", "title", "checked_at", "parsing_error_count", "category", "icon", "hide_globally"} {
		if _, ok := feeds[0][key]; !ok {
			t.Errorf("missing key %q in %s", key, body)
		}
	}
	if cat, _ := feeds[0]["category"].(map[string]any); cat["title"] != "News" {
		t.Errorf("category = %v", feeds[0]["category"])
	}
}

func TestEntries(t *testing.T) {
	env := newTestEnv(t)
	feed := env.seedFeed(t)

	var result entriesResponse
	list := func(query string) {
		t.Helper()
		resp, body := env.do(t, "GET", "/v1/entries?"+query, "")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d: %s", resp.StatusCode, body)
		}
		result = entriesResponse{}
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatal(err)
		}
	}

	list("status=unread&order=id&direction=asc&limit=1")
	if result.Total != 2 || len(result.Entries) != 1 || result.Entries[0].Title != "Hello world" {
		t.Fatalf("unexpected list: total=%d entries=%d", result.Total, len(result.Entries))
	}
	if result.Entries[0].Feed == nil || result.Entries[0].Feed.ID != feed.ID {
		t.Fatalf("entry feed not set: %+v", result.Entries[0].Feed)
	}
	id := result.Entries[0].ID

	list("search=hello")
	if result.Total != 1 {
		t.Fatalf("search total = %d", result.Total)
	}

	resp, body := env.do(t, "PUT", "/v1/entries", `{"entry_ids":[`+itoa(id)+`],"status":"read"}`)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status update = %d: %s", resp.StatusCode, body)
	}
	list("status=read")
	if result.Total != 1 || result.Entries[0].ID != id {
		t.Fatalf("read total = %d", result.Total)
	}

	resp, _ = env.do(t, "PUT", "/v1/entries/"+itoa(id)+"/bookmark", "")
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("toggle = %d", resp.StatusCode)
	}
	list("starred=true")
	if result.Total != 1 || !result.Entries[0].Starred {
		t.Fatalf("starred total = %d", result.Total)
	}
	env.do(t, "PUT", "/v1/entries/"+itoa(id)+"/star", "")
	list("starred=true")
	if result.Total != 0 {
		t.Fatalf("starred after untoggle = %d", result.Total)
	}

	resp, body = env.do(t, "GET", "/v1/entries?status=bogus", "")
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("bogus status = %d: %s", resp.StatusCode, body)
	}
}

func TestExportOPML(t *testing.T) {
	env := newTestEnv(t)
	env.seedFeed(t)

	resp, body := env.do(t, "GET", "/v1/export", "")
	if resp.StatusCode != http.StatusOK || !strings.Contains(resp.Header.Get("Content-Type"), "xml") {
		t.Fatalf("status = %d type = %s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	for _, want := range []string{`xmlUrl="https://example.org/feed.xml"`, `text="News"`} {
		if !strings.Contains(string(body), want) {
			t.Errorf("export missing %s:\n%s", want, body)
		}
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
