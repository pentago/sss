package fetcher

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"sss/internal/config"
)

func setAllowPrivate(t *testing.T, allow string) {
	t.Helper()
	opts, err := config.Parse([]string{"FETCHER_ALLOW_PRIVATE_NETWORKS=" + allow})
	if err != nil {
		t.Fatal(err)
	}
	old := config.Opts
	config.Opts = opts
	t.Cleanup(func() { config.Opts = old })
}

func proxyServer(t *testing.T, got chan<- string) *httptest.Server {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case got <- r.Host + "|" + r.Header.Get("Cookie"):
		default:
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(s.Close)
	return s
}

func TestCookieOnlySentToSameSite(t *testing.T) {
	got := make(chan string, 1)
	p, _ := url.Parse(proxyServer(t, got).URL)
	setAllowPrivate(t, "1")
	config.Opts.HTTPClientProxy = p
	b := NewRequestBuilder().UseCustomApplicationProxyURL(true).
		WithCookie("s=1", "https://feeds.example.org/rss.xml")
	for u, want := range map[string]string{
		"http://feeds.example.org/rss.xml":             "s=1",
		"http://www.example.org/a":                     "s=1",
		"http://example.net/a":                         "",
		"http://feeds.example.org.example.net/rss.xml": "",
	} {
		resp, err := b.ExecuteRequest(u)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if c := strings.SplitN(<-got, "|", 2)[1]; c != want {
			t.Errorf("%s: cookie %q, want %q", u, c, want)
		}
	}
}

func TestIsSameSite(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		want bool
	}{
		{"www.example.org", "feeds.example.org", true},
		{"news.example.co.uk", "www.example.co.uk", true},
		{"xn--bcher-kva.example", "www.bücher.example", true},
		{"attacker.co.uk", "example.co.uk", false},
		{"bob.github.io", "alice.github.io", false},
		{"10.0.0.1", "192.168.0.1", false},
		{"", "", false},
	} {
		if got := isSameSite(tc.a, tc.b); got != tc.want {
			t.Errorf("isSameSite(%q,%q)=%v", tc.a, tc.b, got)
		}
	}
}

func TestPrivateNetworks(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer target.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer redirect.Close()

	setAllowPrivate(t, "0")
	for _, u := range []string{target.URL, redirect.URL} {
		if _, err := NewRequestBuilder().ExecuteRequest(u); err == nil || !strings.Contains(err.Error(), "private network") {
			t.Errorf("%s: expected private network error, got %v", u, err)
		}
	}

	// Application proxy is trusted; per-feed proxy is not.
	got := make(chan string, 1)
	ps := proxyServer(t, got)
	p, _ := url.Parse(ps.URL)
	config.Opts.HTTPClientProxy = p
	resp, err := NewRequestBuilder().UseCustomApplicationProxyURL(true).ExecuteRequest("http://feed.invalid/")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if _, err := NewRequestBuilder().WithCustomFeedProxyURL(ps.URL).ExecuteRequest("http://feed.invalid/"); err == nil {
		t.Error("expected feed proxy on private network to be refused")
	}

	setAllowPrivate(t, "1")
	resp, err = NewRequestBuilder().ExecuteRequest(target.URL)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
}

func TestIsModified(t *testing.T) {
	const etag, lm = "abc", "Wed, 21 Oct 2015 07:28:00 GMT"
	for _, tc := range []struct {
		status   int
		lm, etag string
		want     bool
	}{
		{304, lm, etag, false},
		{200, lm, etag, false},
		{200, "Thu, 22 Oct 2015 07:28:00 GMT", etag, false},
		{200, lm, "xyz", true},
		{200, "", "", true},
	} {
		h := http.Header{}
		h.Set("Last-Modified", tc.lm)
		h.Set("ETag", tc.etag)
		rh := NewResponseHandler(&http.Response{StatusCode: tc.status, Header: h}, nil)
		if rh.IsModified(etag, lm) != tc.want {
			t.Errorf("%+v", tc)
		}
	}
}

func TestCacheHeaders(t *testing.T) {
	h := http.Header{}
	h.Set("Cache-Control", "no-cache, max-age=300")
	h.Set("Retry-After", "42")
	h.Set("Expires", time.Now().Add(10*time.Minute).Format(time.RFC1123))
	rh := NewResponseHandler(&http.Response{Header: h}, nil)
	if rh.CacheControlMaxAge() != 5*time.Minute || rh.ParseRetryDelay() != 42*time.Second || rh.Expires() != 10*time.Minute {
		t.Error(rh.CacheControlMaxAge(), rh.ParseRetryDelay(), rh.Expires())
	}
}

func TestLocalizedError(t *testing.T) {
	for _, tc := range []struct {
		status int
		header string
		want   string
	}{
		{200, "", ""},
		{404, "", "resource not found (404"},
		{403, "challenge", "Cloudflare"},
		{418, "", "unexpected status code (418"},
	} {
		h := http.Header{}
		if tc.header != "" {
			h.Set("cf-mitigated", tc.header)
			h.Set("Content-Type", "text/html")
		}
		err := NewResponseHandler(&http.Response{StatusCode: tc.status, Header: h, ContentLength: -1}, nil).LocalizedError()
		if (err == nil) != (tc.want == "") || (err != nil && !strings.Contains(err.Error(), tc.want)) {
			t.Errorf("%d: %v", tc.status, err)
		}
	}
}

func TestReadBodyTooLarge(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(strings.Repeat("x", 100)))
	}))
	defer s.Close()
	rh := NewResponseHandler(NewRequestBuilder().ExecuteRequest(s.URL))
	defer rh.Close()
	if _, err := rh.ReadBody(10); err == nil || !strings.Contains(err.Error(), "too large") {
		t.Error(err)
	}
}
