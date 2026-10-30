package processor

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"sss/internal/config"
	"sss/internal/model"
)

func TestParseISO8601Duration(t *testing.T) {
	for duration, want := range map[string]time.Duration{
		"PT0M0S":   0,
		"PT6M20S":  6*time.Minute + 20*time.Second,
		"PT80M42S": time.Hour + 20*time.Minute + 42*time.Second,
		"PT2H45S":  2*time.Hour + 45*time.Second,
		"PT":       0,
	} {
		if got, err := parseISO8601Duration(duration); err != nil || got != want {
			t.Errorf("%s: got %v, %v; want %v", duration, got, err, want)
		}
	}
	for _, duration := range []string{"P1D", "PT1D", "PT1X", "1M"} {
		if _, err := parseISO8601Duration(duration); err == nil {
			t.Errorf("%s: expected error", duration)
		}
	}
}

func TestYouTubeVideoURL(t *testing.T) {
	url := "https://www.youtube.com/watch?v=HLrqNhgdiC0&feature=youtu.be"
	if !isYouTubeVideoURL(url) || getVideoIDFromYouTubeURL(url) != "HLrqNhgdiC0" {
		t.Error("expected YouTube video HLrqNhgdiC0")
	}
	if isYouTubeVideoURL("https://example.org/watch?v=x") {
		t.Error("non-YouTube URL matched")
	}
}

func setYouTubeConfig(t *testing.T, env ...string) {
	t.Helper()
	opts, err := config.Parse(append(env, "FETCHER_ALLOW_PRIVATE_NETWORKS=1", "FETCH_YOUTUBE_WATCH_TIME=1"))
	if err != nil {
		t.Fatal(err)
	}
	old := config.Opts
	config.Opts = opts
	t.Cleanup(func() { config.Opts = old })
}

func TestFetchYouTubeWatchTime(t *testing.T) {
	setYouTubeConfig(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><head><meta itemprop="duration" content="PT51M52S"></head></html>`))
	}))
	defer server.Close()

	if got, err := fetchYouTubeWatchTime(server.URL); err != nil || got != 51 {
		t.Fatalf("got %d, %v; want 51", got, err)
	}
}

func TestUpdateEntryReadingTimeYouTube(t *testing.T) {
	setYouTubeConfig(t)
	user := &model.User{ShowReadingTime: true, DefaultReadingSpeed: 265, CJKReadingSpeed: 500}
	entry := &model.Entry{Hash: "old", URL: "https://www.youtube.com/watch?v=x", Content: "<p>text</p>"}
	updateEntryReadingTime(fakeStore{}, &model.Feed{}, entry, false, user)
	if entry.ReadingTime != 7 {
		t.Errorf("existing entry: got reading time %d, want stored 7", entry.ReadingTime)
	}
}

func TestFetchYouTubeWatchTimeInBulk(t *testing.T) {
	setYouTubeConfig(t, "YOUTUBE_API_KEY=secret")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("key") != "secret" || q.Get("id") != "a,b" || q.Get("part") != "contentDetails" {
			t.Errorf("unexpected query %q", r.URL.RawQuery)
		}
		w.Write([]byte(`{"items":[{"id":"a","contentDetails":{"duration":"PT1H2M"}},{"id":"b","contentDetails":{"duration":"bad"}}]}`))
	}))
	defer server.Close()
	old := youtubeAPIURL
	youtubeAPIURL = server.URL
	t.Cleanup(func() { youtubeAPIURL = old })

	entries := model.Entries{
		{URL: "https://www.youtube.com/watch?v=a"},
		{URL: "https://www.youtube.com/watch?v=b", ReadingTime: 3},
		{URL: "https://example.org/", ReadingTime: 4},
	}
	fetchYouTubeWatchTimeInBulk(entries)
	if entries[0].ReadingTime != 62 || entries[1].ReadingTime != 3 || entries[2].ReadingTime != 4 {
		t.Errorf("got reading times %d, %d, %d", entries[0].ReadingTime, entries[1].ReadingTime, entries[2].ReadingTime)
	}
}
