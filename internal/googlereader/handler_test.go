package googlereader

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"sss/internal/config"
	"sss/internal/crypto"
	"sss/internal/database"
	"sss/internal/model"
	"sss/internal/storage"
)

func TestItemIDFormats(t *testing.T) {
	if got := convertEntryIDToLongFormItemID(344691561); got != "tag:google.com,2005:reader/item/00000000148b9369" {
		t.Fatalf("long form: %s", got)
	}

	r := &http.Request{Form: url.Values{"i": {
		"12345",
		"tag:google.com,2005:reader/item/00000000148b9369",
		"tag:google.com,2005:reader/item/2f2",
		"000000000000046f",
	}}}
	got, err := parseItemIDsFromRequest(r)
	if err != nil {
		t.Fatal(err)
	}
	if want := []int64{12345, 344691561, 754, 1135}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}

	for _, bad := range []string{"", "invalid_id", "tag:google.com,2005:reader/item/000000000000000g", "tag:google.com,2005:reader/item/0"} {
		if _, err := parseItemID(bad); err == nil {
			t.Errorf("%q: expected error", bad)
		}
	}
	if _, err := parseItemIDsFromRequest(&http.Request{Form: url.Values{}}); err == nil {
		t.Error("expected error without items")
	}
}

func TestGoogleReaderFlow(t *testing.T) {
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
	hash, err := crypto.HashPassword("greader-pw")
	if err != nil {
		t.Fatal(err)
	}
	integ.GoogleReaderEnabled, integ.GoogleReaderUsername, integ.GoogleReaderPassword = true, "alice-gr", hash
	if err := store.UpdateIntegration(integ); err != nil {
		t.Fatal(err)
	}
	category, err := store.FirstCategory(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	feed := &model.Feed{UserID: user.ID, FeedURL: "https://example.org/feed", SiteURL: "https://example.org", Title: "Example", Category: &model.Category{ID: category.ID}, Entries: model.Entries{
		{Hash: "a", Title: "A", Content: "<p>a</p>", URL: "https://example.org/a", Date: now.Add(-2 * time.Hour)},
		{Hash: "b", Title: "B", Content: "<p>b</p>", URL: "https://example.org/b", Date: now.Add(-time.Hour)},
		{Hash: "c", Title: "C", Content: "<p>c</p>", URL: "https://example.org/c", Date: now},
	}}
	if err := store.CreateFeed(feed); err != nil {
		t.Fatal(err)
	}

	h := NewHandler(store)
	do := func(method, target string, form url.Values, auth string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, target, strings.NewReader(form.Encode()))
		if method == http.MethodPost {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
		if auth != "" {
			req.Header.Set("Authorization", "GoogleLogin auth="+auth)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	decode := func(rec *httptest.ResponseRecorder, v any) {
		t.Helper()
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d: %s", rec.Code, rec.Body)
		}
		if err := json.Unmarshal(rec.Body.Bytes(), v); err != nil {
			t.Fatalf("%v: %s", err, rec.Body)
		}
	}

	if rec := do(http.MethodPost, "/accounts/ClientLogin", url.Values{"Email": {"alice-gr"}, "Passwd": {"wrong"}}, ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("bad password: %d", rec.Code)
	}
	rec := do(http.MethodPost, "/accounts/ClientLogin", url.Values{"Email": {"alice-gr"}, "Passwd": {"greader-pw"}}, "")
	_, auth, ok := strings.Cut(rec.Body.String(), "Auth=")
	auth = strings.TrimSpace(auth)
	if rec.Code != http.StatusOK || !ok || !strings.HasPrefix(auth, "alice-gr/") {
		t.Fatalf("ClientLogin: %d %s", rec.Code, rec.Body)
	}

	if rec := do(http.MethodGet, "/reader/api/0/token", nil, "alice-gr/bogus"); rec.Code != http.StatusUnauthorized || rec.Header().Get("X-Reader-Google-Bad-Token") != "true" {
		t.Fatalf("bogus token accepted: %d", rec.Code)
	}
	if rec := do(http.MethodGet, "/reader/api/0/token", nil, auth); rec.Body.String() != auth {
		t.Fatalf("token: %s", rec.Body)
	}

	var subs subscriptionsResponse
	decode(do(http.MethodGet, "/reader/api/0/subscription/list?output=json", nil, auth), &subs)
	feedStream := "feed/" + strconv.FormatInt(feed.ID, 10)
	if len(subs.Subscriptions) != 1 || subs.Subscriptions[0].ID != feedStream || subs.Subscriptions[0].Categories[0].Label != category.Title {
		t.Fatalf("subscription/list: %+v", subs)
	}

	itemIDs := func(query string) []string {
		t.Helper()
		var res streamIDResponse
		decode(do(http.MethodGet, "/reader/api/0/stream/items/ids?output=json&s=user/-/state/com.google/reading-list"+query, nil, auth), &res)
		ids := make([]string, 0, len(res.ItemRefs))
		for _, ref := range res.ItemRefs {
			ids = append(ids, ref.ID)
		}
		return ids
	}
	ids := itemIDs("&r=o")
	if len(ids) != 3 {
		t.Fatalf("reading-list: %v", ids)
	}
	var page streamIDResponse
	decode(do(http.MethodGet, "/reader/api/0/stream/items/ids?output=json&s=user/-/state/com.google/reading-list&n=2", nil, auth), &page)
	if len(page.ItemRefs) != 2 || page.Continuation != 2 {
		t.Fatalf("pagination: %+v", page)
	}

	firstID, _ := strconv.ParseInt(ids[0], 10, 64)
	longID := convertEntryIDToLongFormItemID(firstID)
	var contents streamContentItemsResponse
	decode(do(http.MethodPost, "/reader/api/0/stream/items/contents", url.Values{"T": {auth}, "output": {"json"}, "i": {longID}}, ""), &contents)
	if len(contents.Items) != 1 || contents.Items[0].ID != longID || contents.Items[0].Title != "A" || contents.Items[0].Origin.StreamID != feedStream {
		t.Fatalf("contents: %+v", contents)
	}
	if slices.Contains(contents.Items[0].Categories, "user/"+strconv.FormatInt(user.ID, 10)+"/state/com.google/read") {
		t.Fatalf("entry should be unread: %v", contents.Items[0].Categories)
	}

	if rec := do(http.MethodPost, "/reader/api/0/edit-tag", url.Values{"T": {auth}, "i": {longID}, "a": {"user/-/state/com.google/read"}}, ""); rec.Body.String() != "OK" {
		t.Fatalf("edit-tag: %d %s", rec.Code, rec.Body)
	}

	unread := itemIDs("&xt=user/-/state/com.google/read")
	if len(unread) != 2 || slices.Contains(unread, ids[0]) {
		t.Fatalf("unread exclusion: %v", unread)
	}

	var counts unreadCountResponse
	decode(do(http.MethodGet, "/reader/api/0/unread-count?output=json", nil, auth), &counts)
	want := map[string]int{
		feedStream: 2,
		"user/" + strconv.FormatInt(user.ID, 10) + "/label/" + category.Title:       2,
		"user/" + strconv.FormatInt(user.ID, 10) + "/state/com.google/reading-list": 2,
	}
	got := map[string]int{}
	for _, c := range counts.UnreadCounts {
		got[c.ID] = c.Count
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unread-count: got %v, want %v", got, want)
	}

	// Unimplemented endpoints answer an empty JSON list.
	if rec := do(http.MethodGet, "/reader/api/0/preference/list?output=json", nil, auth); strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Fatalf("fallback: %s", rec.Body)
	}
}
