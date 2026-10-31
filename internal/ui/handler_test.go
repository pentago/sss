package ui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"sss/internal/config"
	"sss/internal/model"
	"sss/internal/storage"
)

func TestBackURL(t *testing.T) {
	config.Opts = &config.Options{BasePath: "/rss"}
	tests := []struct{ referer, want string }{
		{"http://example.org/rss/feed/1/entries?offset=10", "/rss/feed/1/entries?offset=10"},
		{"https://example.org/rss/unread", "/rss/unread"},
		{"http://evil.example/rss/unread", "/rss/starred"},
		{"javascript:alert(1)", "/rss/starred"},
		{"", "/rss/starred"},
		{"http://example.org//evil.example/x", "/rss/starred"},
	}
	for _, tt := range tests {
		r := httptest.NewRequest(http.MethodPost, "http://example.org/entry/status", nil)
		r.Header.Set("Referer", tt.referer)
		if got := backURL(r, "/starred"); got != tt.want {
			t.Errorf("backURL(%q) = %q, want %q", tt.referer, got, tt.want)
		}
	}
}

func TestPageAssets(t *testing.T) {
	config.Opts = &config.Options{}
	noon := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	night := time.Date(2026, 1, 1, 22, 0, 0, 0, time.UTC)
	timeUser := &model.User{Theme: "time_sans_serif", Timezone: "UTC", ThemeLightHour: 7, ThemeDarkHour: 19, GestureNav: "none"}

	tests := []struct {
		name        string
		user        *model.User
		now         time.Time
		styles      []string // file names + media
		colorScheme string
		scripts     []string
	}{
		{"anonymous default", nil, noon, []string{"base.css", "dark.css", "serif.css"}, "dark", nil},
		{"dark", &model.User{Theme: "dark_sans_serif", GestureNav: "none"}, noon, []string{"base.css", "dark.css"}, "dark", nil},
		{"dark serif", &model.User{Theme: "dark_serif", GestureNav: "none"}, noon, []string{"base.css", "dark.css", "serif.css"}, "dark", nil},
		{"time day", timeUser, noon, []string{"base.css", "light.css"}, "light", nil},
		{"time night", timeUser, night, []string{"base.css", "dark.css"}, "dark", nil},
		{"none with custom", &model.User{Theme: model.ThemeNone, Stylesheet: "body{}", GestureNav: "none"}, noon, []string{"custom.css"}, "light dark", nil},
		{"all scripts", &model.User{Theme: "light_sans_serif", KeyboardShortcuts: true, GestureNav: "tap", MarkReadOnScroll: true, ServiceWorker: true, CustomJS: "1"}, noon,
			[]string{"base.css", "light.css"}, "light", []string{"shortcuts.js", "swipe.js", "scroll.js", "sw-register.js", "custom.js"}},
		{"swipe only", &model.User{Theme: "light_sans_serif", EntrySwipe: true, GestureNav: "none"}, noon, []string{"base.css", "light.css"}, "light", []string{"swipe.js"}},
	}
	for _, tt := range tests {
		styles, colorScheme, scripts := pageAssets(tt.user, model.ThemeDefault, tt.now)
		var gotStyles []string
		for _, s := range styles {
			name := s.Href[strings.LastIndex(s.Href, "/")+1:]
			name, _, _ = strings.Cut(name, "?")
			if s.Media != "" {
				name += " " + s.Media
			}
			gotStyles = append(gotStyles, name)
		}
		var gotScripts []string
		for _, s := range scripts {
			name, _, _ := strings.Cut(s[strings.LastIndex(s, "/")+1:], "?")
			gotScripts = append(gotScripts, name)
		}
		if strings.Join(gotStyles, ",") != strings.Join(tt.styles, ",") || colorScheme != tt.colorScheme || strings.Join(gotScripts, ",") != strings.Join(tt.scripts, ",") {
			t.Errorf("%s: got %v %q %v, want %v %q %v", tt.name, gotStyles, colorScheme, gotScripts, tt.styles, tt.colorScheme, tt.scripts)
		}
	}
}

// seedEntries creates a feed with three unread entries for alice and returns their IDs.
func seedEntries(t *testing.T, store *storage.Storage) []int64 {
	t.Helper()
	user, err := store.UserByUsername("alice")
	if err != nil {
		t.Fatal(err)
	}
	category, err := store.FirstCategory(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	feed := &model.Feed{UserID: user.ID, FeedURL: "https://example.org/feed", SiteURL: "https://example.org/", Title: "Example",
		Category: &model.Category{ID: category.ID}}
	for _, hash := range []string{"a", "b", "c"} {
		feed.Entries = append(feed.Entries, &model.Entry{Hash: hash, Title: "Entry " + hash, Content: "<p>" + hash + "</p>", URL: "https://example.org/" + hash, Date: now})
	}
	if err := store.CreateFeed(feed); err != nil {
		t.Fatal(err)
	}
	ids, err := store.NewEntryQueryBuilder(user.ID).WithSorting("id", "asc").GetEntryIDs()
	if err != nil || len(ids) != 3 {
		t.Fatalf("seed: %v %v", ids, err)
	}
	return ids
}

func TestUpdateEntriesStatus(t *testing.T) {
	h, store := newAccountTestServer(t)
	cookie := login(t, h)
	ids := seedEntries(t, store)

	form := url.Values{"id": {itoa(ids[0]), itoa(ids[2])}, "status": {"read"}}
	r := httptest.NewRequest(http.MethodPost, "/entry/status", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("Referer", "http://example.com/feed/1/entries?offset=0")
	r.AddCookie(cookie)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/feed/1/entries?offset=0" {
		t.Fatalf("got %d %q", w.Code, w.Header().Get("Location"))
	}
	entries, err := store.NewEntryQueryBuilder(1).WithSorting("id", "asc").GetEntries()
	if err != nil {
		t.Fatal(err)
	}
	var statuses []string
	for _, e := range entries {
		statuses = append(statuses, e.Status)
	}
	if got := strings.Join(statuses, ","); got != "read,unread,read" {
		t.Errorf("statuses = %s", got)
	}

	// Invalid status and invalid IDs are rejected.
	for _, form := range []url.Values{{"id": {itoa(ids[1])}, "status": {"removed"}}, {"id": {"x"}, "status": {"read"}}} {
		if w := postForm(h, "/entry/status", form, cookie); w.Code != http.StatusBadRequest {
			t.Errorf("%v: code %d, want 400", form, w.Code)
		}
	}
}

// Marking a feed or category as read lands on the fresh list, not back on the
// now-empty entries page, whose stale twin a "back" would restore from cache.
func TestMarkAllAsReadRedirectsToList(t *testing.T) {
	h, store := newAccountTestServer(t)
	cookie := login(t, h)
	seedEntries(t, store)
	category, err := store.FirstCategory(1)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct{ path, referer, want string }{
		{"/feed/1/mark-all-as-read", "http://example.com/feed/1/entries", "/feeds"},
		{"/category/" + itoa(category.ID) + "/mark-all-as-read", "http://example.com/category/" + itoa(category.ID) + "/entries", "/categories"},
	}
	for _, tt := range tests {
		r := httptest.NewRequest(http.MethodPost, tt.path, nil)
		r.Header.Set("Referer", tt.referer)
		r.AddCookie(cookie)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != http.StatusSeeOther || w.Header().Get("Location") != tt.want {
			t.Errorf("%s: got %d %q, want 303 %q", tt.path, w.Code, w.Header().Get("Location"), tt.want)
		}
	}
}

func TestCrossOriginPostRejected(t *testing.T) {
	h, store := newAccountTestServer(t)
	cookie := login(t, h)
	ids := seedEntries(t, store)
	r := httptest.NewRequest(http.MethodPost, "/entry/star/"+itoa(ids[0]), nil)
	r.Header.Set("Sec-Fetch-Site", "cross-site")
	r.AddCookie(cookie)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("code %d, want 403", w.Code)
	}
}

// TestPages renders every reading page to catch template errors.
func TestPages(t *testing.T) {
	h, store := newAccountTestServer(t)
	cookie := login(t, h)
	ids := seedEntries(t, store)
	id := itoa(ids[1])
	shareCode, err := store.EntryShareCode(1, ids[1])
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		"/unread", "/unread/entry/" + id, "/starred", "/history", "/search?q=entry", "/search/entry/" + id + "?q=entry",
		"/feeds", "/feed/1/entries", "/feed/1/entries/all", "/feed/1/entry/" + id, "/unread/feed/1/entry/" + id, "/feed/1/edit",
		"/categories", "/category/create", "/category/1/edit", "/category/1/feeds", "/category/1/entries", "/category/1/entries/all",
		"/category/1/entries/starred", "/category/1/entry/" + id, "/subscribe", "/bookmarklet?uri=https://example.org", "/import",
		"/shares", "/offline", "/manifest.json", "/export", "/share/" + shareCode, "/tags/tag/entries/all",
	} {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		r.AddCookie(cookie)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != http.StatusOK {
			t.Errorf("GET %s: %d %s", path, w.Code, w.Body.String())
		}
	}
}

// TestAppIcons checks that browsers get a real favicon and that every manifest icon resolves
// with the type it advertises, without a session (install prompts fetch them anonymously).
func TestAppIcons(t *testing.T) {
	h, _ := newAccountTestServer(t)
	get := func(path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		return w
	}
	if w := get("/favicon.ico"); w.Code != http.StatusOK || w.Header().Get("Content-Type") != "image/x-icon" {
		t.Fatalf("favicon: %d %q", w.Code, w.Header().Get("Content-Type"))
	}
	var manifest struct {
		Icons []struct{ Src, Type string }
	}
	if err := json.Unmarshal(get("/manifest.json").Body.Bytes(), &manifest); err != nil || len(manifest.Icons) == 0 {
		t.Fatalf("manifest icons: %v %v", manifest.Icons, err)
	}
	for _, icon := range manifest.Icons {
		if w := get(icon.Src); w.Code != http.StatusOK || w.Header().Get("Content-Type") != icon.Type {
			t.Errorf("%s: %d %q, want %q", icon.Src, w.Code, w.Header().Get("Content-Type"), icon.Type)
		}
	}
	if w := get("/icon/x/missing.png"); w.Code != http.StatusNotFound {
		t.Errorf("missing icon: %d", w.Code)
	}
}

func itoa(id int64) string { return strconv.FormatInt(id, 10) }

func TestPageLinks(t *testing.T) {
	config.Opts = &config.Options{BasePath: "/rss"}
	tests := []struct {
		uri                  string
		total, offset, limit int
		wantPrev, wantNext   string
	}{
		{"/unread", 25, 0, 10, "", "/rss/unread?offset=10"},
		{"/unread?offset=10", 25, 10, 10, "/rss/unread", "/rss/unread?offset=20"},
		{"/search?q=go&offset=20", 25, 20, 10, "/rss/search?offset=10&q=go", ""},
		{"/starred", 10, 0, 10, "", ""},
	}
	for _, tt := range tests {
		r := httptest.NewRequest(http.MethodGet, tt.uri, nil)
		prev, next := pageLinks(r, tt.total, tt.offset, tt.limit)
		if prev != tt.wantPrev || next != tt.wantNext {
			t.Errorf("%s: got %q %q, want %q %q", tt.uri, prev, next, tt.wantPrev, tt.wantNext)
		}
	}
}
