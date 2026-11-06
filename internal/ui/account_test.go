package ui

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strconv"
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

	// Custom CSS targets the login page by one selector, whichever route served it.
	if !strings.Contains(w.Body.String(), `<body data-page="login"`) {
		t.Error(`failed login lacks <body data-page="login">`)
	}
	root := httptest.NewRecorder()
	h.ServeHTTP(root, httptest.NewRequest(http.MethodGet, "/", nil))
	if !strings.Contains(root.Body.String(), `<body data-page="login"`) {
		t.Errorf(`GET / lacks <body data-page="login">: code %d`, root.Code)
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

// Anonymous page views (login page, shared entries, failed logins) must not
// store a web session each; only flows that need state across requests do.
func TestAnonymousVisitsDoNotPersistSessions(t *testing.T) {
	h, _ := newAccountTestServer(t, "WEBAUTHN=1", "BASE_URL=http://localhost")
	for _, r := range []*http.Request{
		httptest.NewRequest(http.MethodGet, "/", nil),
		httptest.NewRequest(http.MethodGet, "/share/unknown", nil),
		httptest.NewRequest(http.MethodPost, "/webauthn/login/begin", nil),
	} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if c := sessionCookie(w); c != nil {
			t.Errorf("%s %s set a session cookie", r.Method, r.URL)
		}
	}
	if w := postForm(h, "/login", url.Values{"username": {"alice"}, "password": {"wrong"}}); sessionCookie(w) != nil {
		t.Error("failed login set a session cookie")
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/webauthn/login/begin", nil))
	if sessionCookie(w) == nil {
		t.Errorf("WebAuthn login challenge did not persist a session: code %d", w.Code)
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

// A stolen admin session must not change the owner's password without the current one.
func TestAdminCannotEditSelf(t *testing.T) {
	h, store := newAccountTestServer(t)
	c := login(t, h)
	alice, _ := store.UserByUsername("alice")
	path := "/users/" + strconv.FormatInt(alice.ID, 10) + "/update"
	w := postForm(h, path, url.Values{"username": {"alice"}, "password": {"hijacked1"}, "confirmation": {"hijacked1"}, "is_admin": {"on"}}, c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("self update: code %d", w.Code)
	}
	if store.CheckPassword("alice", "secret123") != nil {
		t.Fatal("password was changed through the admin form")
	}
}

// Admins must keep their role when the provider only puts the group claim in the ID token.
func TestOIDCProfileGroups(t *testing.T) {
	user := map[string]any{"preferred_username": "alice"}
	with := func(m map[string]any, v any) map[string]any {
		c := map[string]any{"groups": v}
		for k, x := range m {
			c[k] = x
		}
		return c
	}
	for name, tc := range map[string]struct {
		info, id map[string]any
		want     []string
		wantErr  bool
	}{
		"userinfo list":      {info: with(user, []any{"admins", "users"}), id: with(nil, "ignored"), want: []string{"admins", "users"}},
		"id token string":    {info: user, id: with(nil, "admins"), want: []string{"admins"}},
		"missing everywhere": {info: user, id: map[string]any{}},
		"non-string entry":   {info: with(user, []any{"admins", 1.0}), wantErr: true},
		"wrong type":         {info: user, id: with(nil, 1.0), wantErr: true},
	} {
		p, err := oidcProfile("sub", tc.info, tc.id, "groups")
		if (err != nil) != tc.wantErr {
			t.Errorf("%s: err %v, wantErr %v", name, err, tc.wantErr)
			continue
		}
		if err == nil && !slices.Equal(p.groups, tc.want) {
			t.Errorf("%s: groups %v, want %v", name, p.groups, tc.want)
		}
	}
}

func TestFeverToken(t *testing.T) {
	if got := feverToken("user", "pass"); got != "21a39285884a268d6458e3fdeb08beac" {
		t.Fatalf("feverToken = %q", got)
	}
}
