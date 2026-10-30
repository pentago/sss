package icon

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"sss/internal/config"
	"sss/internal/model"
)

// 1x1 transparent GIF.
var gif1x1 = []byte("GIF89a\x01\x00\x01\x00\x80\x00\x00\x00\x00\x00\xff\xff\xff!\xf9\x04\x01\x00\x00\x00\x00,\x00\x00\x00\x00\x01\x00\x01\x00\x00\x02\x02D\x01\x00;")

func setup(t *testing.T) {
	opts, err := config.Parse([]string{"FETCHER_ALLOW_PRIVATE_NETWORKS=1"})
	if err != nil {
		t.Fatal(err)
	}
	old := config.Opts
	config.Opts = opts
	t.Cleanup(func() { config.Opts = old })
}

func TestFindIcon(t *testing.T) {
	setup(t)
	for name, tc := range map[string]struct {
		routes  map[string]string // path -> body ("" = 404); bodies starting with "<" are HTML
		feedURL string
		want    string // expected icon content
	}{
		"feed icon":        {map[string]string{"/f.gif": "gif"}, "/f.gif", "gif"},
		"html link":        {map[string]string{"/blog/": `<link rel="Shortcut Icon" href="i.gif">`, "/blog/i.gif": "gif"}, "", "gif"},
		"root html":        {map[string]string{"/blog/": `<p>`, "/": `<link rel="icon" href="/r.gif">`, "/r.gif": "gif"}, "", "gif"},
		"data url":         {map[string]string{"/blog/": `<link rel="icon" href="data:image/svg+xml,%3Csvg%2F%3E">`}, "", "<svg/>"},
		"favicon.ico":      {map[string]string{"/blog/": `<p>`, "/": `<p>`, "/favicon.ico": "gif"}, "", "gif"},
		"html not an icon": {map[string]string{"/blog/": `<p>`, "/": `<p>`, "/favicon.ico": "<html>"}, "", ""},
	} {
		t.Run(name, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch body := tc.routes[r.URL.Path]; body {
				case "":
					http.NotFound(w, r)
				case "gif":
					w.Write(gif1x1)
				default:
					w.Header().Set("Content-Type", "text/html")
					fmt.Fprint(w, body)
				}
			}))
			defer s.Close()
			feed := &model.Feed{SiteURL: s.URL + "/blog/"}
			if tc.feedURL != "" {
				feed.IconURL = s.URL + tc.feedURL
			}
			icon, _ := FindIcon(feed)
			got := ""
			if icon != nil {
				got = string(icon.Content)
				if got == string(gif1x1) {
					got = "gif"
					if icon.MimeType != "image/gif" {
						t.Errorf("mime %q", icon.MimeType)
					}
				}
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestParseImageDataURL(t *testing.T) {
	for in, want := range map[string]string{
		"data:image/png;base64,aGk=":    "hi",
		"data:image/svg+xml;utf8,<svg>": "<svg>",
		"data:text/html,x":              "",
		"data:image/png;base64,!!":      "",
	} {
		icon, err := parseImageDataURL(in)
		if (err != nil) != (want == "") || (icon != nil && string(icon.Content) != want) {
			t.Errorf("%s: %v %v", in, icon, err)
		}
	}
}
