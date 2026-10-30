package mediaproxy

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"sss/internal/config"
	"sss/internal/model"
)

func TestServe(t *testing.T) {
	var err error
	if config.Opts, err = config.Parse([]string{"FETCHER_ALLOW_PRIVATE_NETWORKS=1", "MEDIA_PROXY_PRIVATE_KEY=test"}); err != nil {
		t.Fatal(err)
	}

	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write([]byte("PNG"))
	}))
	defer origin.Close()

	parts := strings.Split(ProxifyRelativeURL(origin.URL+"/a.png"), "/")
	digest, encodedURL := parts[2], parts[3]

	serve := func(digest, encodedURL string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		Serve(rec, httptest.NewRequest(http.MethodGet, "/proxy/x/y", nil), digest, encodedURL)
		return rec
	}

	rec := serve(digest, encodedURL)
	if rec.Code != http.StatusOK || rec.Body.String() != "PNG" || rec.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("unexpected response: %d %q %q", rec.Code, rec.Body.String(), rec.Header().Get("Content-Type"))
	}
	if rec.Header().Get("Content-Disposition") != `inline; filename=a.png` {
		t.Errorf("unexpected Content-Disposition %q", rec.Header().Get("Content-Disposition"))
	}

	otherURL := strings.Split(ProxifyRelativeURL(origin.URL+"/b.png"), "/")[3]
	if rec := serve(digest, otherURL); rec.Code != http.StatusForbidden {
		t.Errorf("tampered URL: got status %d, want 403", rec.Code)
	}
	if rec := serve("!!", encodedURL); rec.Code != http.StatusBadRequest {
		t.Errorf("bad digest: got status %d, want 400", rec.Code)
	}
}

func TestProxifyEnclosureURLs(t *testing.T) {
	var err error
	if config.Opts, err = config.Parse([]string{"MEDIA_PROXY_MODE=all", "MEDIA_PROXY_RESOURCE_TYPES=audio", "MEDIA_PROXY_PRIVATE_KEY=test", "BASE_URL=https://sss.example/app"}); err != nil {
		t.Fatal(err)
	}

	enclosures := model.EnclosureList{
		{URL: "https://example.org/a.mp3", MimeType: "audio/mpeg"},
		{URL: "https://example.org/b.png", MimeType: "image/png"},
	}
	ProxifyEnclosureURLs(enclosures)

	if !strings.HasPrefix(enclosures[0].URL, "https://sss.example/app/proxy/") {
		t.Errorf("audio enclosure not proxified: %q", enclosures[0].URL)
	}
	if enclosures[1].URL != "https://example.org/b.png" {
		t.Errorf("image enclosure should be unchanged: %q", enclosures[1].URL)
	}
}
