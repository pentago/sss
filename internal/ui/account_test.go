package ui

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"sss/internal/config"
	"sss/internal/database"
	"sss/internal/model"
	"sss/internal/storage"
)

func newAccountTestServer(t *testing.T, env ...string) (http.Handler, *storage.Storage) {
	t.Helper()
	opts, err := config.Parse(env)
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
	if _, err := store.CreateUser(&model.UserCreationRequest{Username: "alice", Password: "secret123"}); err != nil {
		t.Fatal(err)
	}
	return NewHandler(store, nil), store
}

func postForm(h http.Handler, path string, form url.Values, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, c := range cookies {
		r.AddCookie(c)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

// sessionCookie returns the last session cookie set (login rotates the anonymous one).
func sessionCookie(w *httptest.ResponseRecorder) (last *http.Cookie) {
	for _, c := range w.Result().Cookies() {
		if c.Name == sessionCookieName && c.Value != "" {
			last = c
		}
	}
	return last
}

func login(t *testing.T, h http.Handler) *http.Cookie {
	t.Helper()
	w := postForm(h, "/login", url.Values{"username": {"alice"}, "password": {"secret123"}})
	c := sessionCookie(w)
	if w.Code != http.StatusSeeOther || c == nil {
		t.Fatalf("login: code %d, cookie %v", w.Code, c)
	}
	return c
}

func TestLogin(t *testing.T) {
	h, _ := newAccountTestServer(t)

	w := postForm(h, "/login", url.Values{"username": {"alice"}, "password": {"wrong"}})
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `role="alert"`) {
		t.Fatalf("bad password: code %d, body lacks alert", w.Code)
	}

	w = postForm(h, "/login", url.Values{"username": {"alice"}, "password": {"secret123"}, "redirect_url": {"/feeds"}})
	if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/feeds" {
		t.Fatalf("good password: code %d, location %q", w.Code, w.Header().Get("Location"))
	}
	c := sessionCookie(w)
	if c == nil || !c.HttpOnly || c.SameSite != http.SameSiteLaxMode {
		t.Fatalf("session cookie = %+v", c)
	}

	r := httptest.NewRequest(http.MethodGet, "/settings", nil)
	r.AddCookie(c)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("settings with session cookie: code %d", w.Code)
	}
}

func TestLoginRedirect(t *testing.T) {
	config.Opts, _ = config.Parse(nil)
	user := &model.User{DefaultHomePage: "unread"}
	for in, want := range map[string]string{
		"":                     "/unread",
		"/starred":             "/starred",
		"//evil.example":       "/unread",
		"https://evil.example": "/unread",
		`/\evil.example`:       "/unread",
	} {
		if got := loginRedirect(in, user); got != want {
			t.Errorf("loginRedirect(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSettingsValidation(t *testing.T) {
	h, store := newAccountTestServer(t)
	c := login(t, h)
	form := url.Values{
		"theme": {"dark_serif"}, "theme_light_hour": {"7"}, "theme_dark_hour": {"19"}, "timezone": {"UTC"},
		"entry_direction": {"asc"}, "entry_order": {"published_at"}, "entries_per_page": {"50"},
		"default_home_page": {"unread"}, "categories_sorting_order": {"unread_count"}, "display_mode": {"standalone"},
		"default_reading_speed": {"265"}, "cjk_reading_speed": {"500"}, "gesture_nav": {"none"},
		"keyboard_shortcuts": {"on"},
	}

	bad := url.Values{}
	for k, v := range form {
		bad[k] = v
	}
	bad.Set("block_filter_entry_rules", "EntryTitle=(")
	w := postForm(h, "/settings", bad, c)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `role="alert"`) {
		t.Fatalf("invalid rules: code %d", w.Code)
	}
	user, _ := store.UserByUsername("alice")
	if user.Theme == "dark" {
		t.Fatal("invalid settings were saved")
	}

	if w := postForm(h, "/settings", form, c); w.Code != http.StatusSeeOther {
		t.Fatalf("valid settings: code %d %s", w.Code, w.Body.String())
	}
	user, _ = store.UserByUsername("alice")
	if user.Theme != "dark_serif" || !user.KeyboardShortcuts || user.EntryDirection != "asc" {
		t.Fatalf("settings not saved: %+v", user)
	}
}

func TestAuthProxy(t *testing.T) {
	h, _ := newAccountTestServer(t, "AUTH_PROXY_HEADER=X-User", "TRUSTED_REVERSE_PROXY_NETWORKS=10.0.0.0/8")
	for remote, wantLogin := range map[string]bool{"10.1.2.3:1234": true, "192.0.2.1:1234": false} {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = remote
		r.Header.Set("X-User", "alice")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		loggedIn := w.Code == http.StatusSeeOther && w.Header().Get("Location") == "/unread"
		if loggedIn != wantLogin {
			t.Errorf("remote %s: code %d location %q, want login %v", remote, w.Code, w.Header().Get("Location"), wantLogin)
		}
	}
}

// The config dump may reveal deployment details, so only admins see it.
func TestAboutConfigAdminOnly(t *testing.T) {
	h, store := newAccountTestServer(t, "OAUTH2_CLIENT_SECRET=hunter2")
	c := login(t, h)
	about := func() string {
		r := httptest.NewRequest(http.MethodGet, "/about", nil)
		r.AddCookie(c)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Body.String()
	}
	// alice is the first user, so she starts as an admin.
	body := about()
	if !strings.Contains(body, "OAUTH2_CLIENT_SECRET=&lt;redacted&gt;") || strings.Contains(body, "hunter2") {
		t.Fatal("admin config missing or secret leaked")
	}
	user, err := store.UserByUsername("alice")
	if err != nil {
		t.Fatal(err)
	}
	user.IsAdmin = false
	if err := store.UpdateUser(user); err != nil {
		t.Fatal(err)
	}
	if body := about(); strings.Contains(body, "Global configuration options") {
		t.Fatal("non-admin sees the config")
	}
}

func TestFeverToken(t *testing.T) {
	if got := feverToken("user", "pass"); got != "21a39285884a268d6458e3fdeb08beac" {
		t.Fatalf("feverToken = %q", got)
	}
}
