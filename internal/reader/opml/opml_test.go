package opml

import (
	"strings"
	"testing"

	"sss/internal/model"
)

func TestParse(t *testing.T) {
	data := `<?xml version="1.0" encoding="ISO-8859-1"?>
<opml version="2.0" xmlns:miniflux="https://miniflux.app/opml">
<head><title>x</title></head>
<body>
  <outline text="Top" xmlUrl="https://top/feed" />
  <outline text="Cat &amp; Dogs">
    <outline title="A" text="ignored" xmlUrl="https://a/feed" htmlUrl="https://a/" miniflux:crawler="true" miniflux:userAgent="UA"/>
    <outline text="Nested"><outline xmlUrl="https://b/feed"/></outline>
  </outline>
  <outline text="Empty"/>
</body></opml>`
	feeds, err := Parse(strings.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if len(feeds) != 3 {
		t.Fatalf("got %d feeds", len(feeds))
	}
	if f := feeds[0]; f.Title != "Top" || f.SiteURL != "https://top/feed" || f.Category.Title != "" {
		t.Errorf("top: %+v", f)
	}
	if f := feeds[1]; f.Title != "A" || f.Category.Title != "Cat & Dogs" || !f.Crawler || f.UserAgent != "UA" {
		t.Errorf("a: %+v", f)
	}
	if f := feeds[2]; f.Title != "https://b/feed" || f.Category.Title != "Nested" {
		t.Errorf("b: %+v", f)
	}

	if _, err := Parse(strings.NewReader(`<opml><body><outline xmlns:miniflux="https://miniflux.app/opml" xmlUrl="x" miniflux:crawler="maybe"/></body></opml>`)); err == nil {
		t.Error("expected invalid bool error")
	}
}

func TestSerializeRoundTrip(t *testing.T) {
	in := []*model.Feed{
		{Title: "B", FeedURL: "https://b/feed", SiteURL: "https://b/", Category: &model.Category{Title: "Z"}, DisableHTTP2: true, RewriteRules: "add_image_title"},
		{Title: "A", FeedURL: "https://a/feed", SiteURL: "https://a/", Category: &model.Category{Title: "Y"}},
	}
	out, err := Serialize(in)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `miniflux:disableHTTP2="true"`) || !strings.Contains(out, `type="rss"`) {
		t.Error(out)
	}
	feeds, err := Parse(strings.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	if len(feeds) != 2 || feeds[0].Title != "A" || feeds[1].Category.Title != "Z" || !feeds[1].DisableHTTP2 || feeds[1].RewriteRules != "add_image_title" {
		t.Errorf("round trip: %+v %+v", feeds[0], feeds[1])
	}
}
