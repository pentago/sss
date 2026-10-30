package fever

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"sss/internal/config"
	"sss/internal/database"
	"sss/internal/model"
	"sss/internal/storage"
)

func TestFever(t *testing.T) {
	var err error
	if config.Opts, err = config.Parse(nil); err != nil {
		t.Fatal(err)
	}
	db, err := database.Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	store := storage.New(db)

	user, err := store.CreateUser(&model.UserCreationRequest{Username: "alice", Password: "secret123"})
	if err != nil {
		t.Fatal(err)
	}
	integ, err := store.Integration(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	integ.FeverEnabled, integ.FeverUsername, integ.FeverToken = true, "alice", "abc123"
	if err := store.UpdateIntegration(integ); err != nil {
		t.Fatal(err)
	}
	category, err := store.FirstCategory(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	f := &model.Feed{UserID: user.ID, FeedURL: "https://example.org/feed", SiteURL: "https://example.org", Title: "Example", Category: &model.Category{ID: category.ID}, Entries: model.Entries{
		{Hash: "a", Title: "A", Content: "<p>a</p>", URL: "https://example.org/a", Date: now},
		{Hash: "b", Title: "B", Content: "<p>b</p>", URL: "https://example.org/b", Date: now},
	}}
	if err := store.CreateFeed(f); err != nil {
		t.Fatal(err)
	}

	h := NewHandler(store)
	call := func(query string, form url.Values) map[string]any {
		t.Helper()
		form.Set("api_key", "abc123")
		req := httptest.NewRequest(http.MethodPost, "/fever/?api&"+query, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		var body map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("%s: %v: %s", query, err, rec.Body)
		}
		return body
	}

	// Wrong key: auth=0.
	req := httptest.NewRequest(http.MethodPost, "/fever/?api", strings.NewReader("api_key=wrong"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if !strings.Contains(rec.Body.String(), `"auth":0`) {
		t.Fatalf("bad key: %s", rec.Body)
	}

	if body := call("", url.Values{}); body["auth"] != 1.0 || body["api_version"] != 3.0 {
		t.Fatalf("auth: %v", body)
	}

	body := call("items", url.Values{})
	items := body["items"].([]any)
	if len(items) != 2 || body["total_items"] != 2.0 {
		t.Fatalf("items: %v", body)
	}
	first := items[0].(map[string]any)
	firstID := int64(first["id"].(float64))
	if first["is_read"] != 0.0 || first["title"] != "A" {
		t.Fatalf("first item: %v", first)
	}

	body = call("items&since_id="+strconv.FormatInt(firstID, 10), url.Values{})
	if items := body["items"].([]any); len(items) != 1 || items[0].(map[string]any)["title"] != "B" {
		t.Fatalf("since_id: %v", body)
	}

	call("", url.Values{"mark": {"item"}, "as": {"read"}, "id": {strconv.FormatInt(firstID, 10)}})
	call("", url.Values{"mark": {"item"}, "as": {"saved"}, "id": {strconv.FormatInt(firstID, 10)}})
	call("", url.Values{"mark": {"item"}, "as": {"saved"}, "id": {strconv.FormatInt(firstID, 10)}})

	unread := call("unread_item_ids", url.Values{})["unread_item_ids"].(string)
	if unread == "" || strings.Contains(","+unread+",", ","+strconv.FormatInt(firstID, 10)+",") {
		t.Fatalf("unread_item_ids after mark read: %q", unread)
	}
	if saved := call("saved_item_ids", url.Values{})["saved_item_ids"]; saved != strconv.FormatInt(firstID, 10) {
		t.Fatalf("saved_item_ids: %v", saved)
	}

	// Marking the feed read with before in the past leaves the remaining entry unread.
	call("", url.Values{"mark": {"feed"}, "as": {"read"}, "id": {strconv.FormatInt(f.ID, 10)}, "before": {strconv.FormatInt(now.Add(-time.Hour).Unix(), 10)}})
	if unread := call("unread_item_ids", url.Values{})["unread_item_ids"]; unread == "" {
		t.Fatal("before was ignored")
	}
	call("", url.Values{"mark": {"group"}, "as": {"read"}, "id": {"0"}})
	if unread := call("unread_item_ids", url.Values{})["unread_item_ids"]; unread != "" {
		t.Fatalf("group 0 should mark all read, got %v", unread)
	}

	groups := call("groups", url.Values{})
	fg := groups["feeds_groups"].([]any)[0].(map[string]any)
	if fg["group_id"] != float64(category.ID) || fg["feed_ids"] != strconv.FormatInt(f.ID, 10) {
		t.Fatalf("feeds_groups: %v", groups)
	}
}
