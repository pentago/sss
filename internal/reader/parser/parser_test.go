// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"sss/internal/crypto"
	"sss/internal/model"
)

type parseTest struct {
	name    string
	url     string
	data    string
	wantErr bool
	check   func(t *testing.T, feed *model.Feed)
}

func runParseTests(t *testing.T, tests []parseTest) {
	t.Helper()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			feed, err := ParseFeed(tc.url, strings.NewReader(tc.data))
			if tc.wantErr {
				if err == nil {
					t.Fatal("ParseFeed must return an error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if tc.check != nil {
				tc.check(t, feed)
			}
		})
	}
}

func BenchmarkParse(b *testing.B) {
	var testCases = map[string][]string{
		"large_atom.xml": {"https://dustri.org/b", ""},
		"large_rss.xml":  {"https://dustri.org/b", ""},
	}
	for filename := range testCases {
		data, err := os.ReadFile("./testdata/" + filename)
		if err != nil {
			b.Fatalf(`Unable to read file %q: %v`, filename, err)
		}
		testCases[filename][1] = string(data)
	}
	for b.Loop() {
		for _, v := range testCases {
			ParseFeed(v[0], strings.NewReader(v[1]))
		}
	}
}

func FuzzParse(f *testing.F) {
	f.Add("https://z.org", `<?xml version="1.0" encoding="utf-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
<title>Example Feed</title>
<link href="http://z.org/"/>
<link href="/k"/>
<updated>2003-12-13T18:30:02Z</updated>
<author><name>John Doe</name></author>
<id>urn:uuid:60a76c80-d399-11d9-b93C-0003939e0af6</id>
<entry>
<title>a</title>
<link href="http://example.org/b"/>
<id>urn:uuid:1225c695-cfb8-4ebb-aaaa-80da344efa6a</id>
<updated>2003-12-13T18:30:02Z</updated>
<summary>c</summary>
</entry>
</feed>`)
	f.Add("https://z.org", `<?xml version="1.0"?>
<rss version="2.0">
<channel>
<title>a</title>
<link>http://z.org</link>
<item>
<title>a</title>
<link>http://z.org</link>
<description>d</description>
<pubDate>Tue, 03 Jun 2003 09:39:21 GMT</pubDate>
<guid>l</guid>
</item>
</channel>
</rss>`)
	f.Add("https://z.org", `<?xml version="1.0" encoding="utf-8"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#" xmlns="http://purl.org/rss/1.0/">
<channel>
<title>a</title>
<link>http://z.org/</link>
</channel>
<item>
<title>a</title>
<link>/</link>
<description>c</description>
</item>
</rdf:RDF>`)
	f.Add("http://z.org", `{
"version": "http://jsonfeed.org/version/1",
"title": "a",
"home_page_url": "http://z.org/",
"feed_url": "http://z.org/a.json",
"items": [
{"id": "2","content_text": "a","url": "https://z.org/2"},
{"id": "1","content_html": "<a","url":"http://z.org/1"}]}`)
	f.Fuzz(func(t *testing.T, url string, data string) {
		ParseFeed(url, strings.NewReader(data))
	})
}

func TestParseFeed(t *testing.T) {
	runParseTests(t, []parseTest{
		{
			name: "ParseAtom03Feed",
			url:  "https://example.org/",
			data: `<?xml version="1.0" encoding="utf-8"?>
	<feed version="0.3" xmlns="http://purl.org/atom/ns#">
		<title>dive into mark</title>
		<link rel="alternate" type="text/html" href="http://diveintomark.org/"/>
		<modified>2003-12-13T18:30:02Z</modified>
		<author><name>Mark Pilgrim</name></author>
		<entry>
			<title>Atom 0.3 snapshot</title>
			<link rel="alternate" type="text/html" href="http://diveintomark.org/2003/12/13/atom03"/>
			<id>tag:diveintomark.org,2003:3.2397</id>
			<issued>2003-12-13T08:29:29-04:00</issued>
			<modified>2003-12-13T18:30:02Z</modified>
			<summary type="text/plain">It&apos;s a test</summary>
			<content type="text/html" mode="escaped"><![CDATA[<p>HTML content</p>]]></content>
		</entry>
	</feed>`,
			check: func(t *testing.T, feed *model.Feed) {
				if feed.Title != "dive into mark" {
					t.Errorf("Incorrect title, got: %s", feed.Title)
				}
			},
		},
		{
			name: "ParseAtom10Feed",
			url:  "https://example.org/",
			data: `<?xml version="1.0" encoding="utf-8"?>
	<feed xmlns="http://www.w3.org/2005/Atom">

	  <title>Example Feed</title>
	  <link href="http://example.org/"/>
	  <updated>2003-12-13T18:30:02Z</updated>
	  <author>
		<name>John Doe</name>
	  </author>
	  <id>urn:uuid:60a76c80-d399-11d9-b93C-0003939e0af6</id>

	  <entry>
		<title>Atom-Powered Robots Run Amok</title>
		<link href="http://example.org/2003/12/13/atom03"/>
		<id>urn:uuid:1225c695-cfb8-4ebb-aaaa-80da344efa6a</id>
		<updated>2003-12-13T18:30:02Z</updated>
		<summary>Some text.</summary>
	  </entry>

	</feed>`,
			check: func(t *testing.T, feed *model.Feed) {
				if feed.Title != "Example Feed" {
					t.Errorf("Incorrect title, got: %s", feed.Title)
				}
			},
		},
		{
			name: "ParseAtomFeedWithRelativeURL",
			url:  "https://example.org/blog/atom.xml",
			data: `<?xml version="1.0" encoding="utf-8"?>
	<feed xmlns="http://www.w3.org/2005/Atom">
	  <title>Example Feed</title>
	  <link href="/blog/atom.xml" rel="self" type="application/atom+xml"/>
	  <link href="/blog"/>

	  <entry>
		<title>Test</title>
		<link href="/blog/article.html"/>
		<link href="/blog/article.html" rel="alternate" type="text/html"/>
		<id>/blog/article.html</id>
		<updated>2003-12-13T18:30:02Z</updated>
		<summary>Some text.</summary>
	  </entry>

	</feed>`,
			check: func(t *testing.T, feed *model.Feed) {
				if feed.FeedURL != "https://example.org/blog/atom.xml" {
					t.Errorf("Incorrect feed URL, got: %s", feed.FeedURL)
				}

				if feed.SiteURL != "https://example.org/blog" {
					t.Errorf("Incorrect site URL, got: %s", feed.SiteURL)
				}

				if feed.Entries[0].URL != "https://example.org/blog/article.html" {
					t.Errorf("Incorrect entry URL, got: %s", feed.Entries[0].URL)
				}
			},
		},
		{
			name: "ParseRSS",
			url:  "http://liftoff.msfc.nasa.gov/",
			data: `<?xml version="1.0"?>
	<rss version="2.0">
	<channel>
		<title>Liftoff News</title>
		<link>http://liftoff.msfc.nasa.gov/</link>
		<item>
			<title>Star City</title>
			<link>http://liftoff.msfc.nasa.gov/news/2003/news-starcity.asp</link>
			<description>How do Americans get ready to work with Russians aboard the International Space Station? They take a crash course in culture, language and protocol at Russia's &lt;a href="http://howe.iki.rssi.ru/GCTC/gctc_e.htm"&gt;Star City&lt;/a&gt;.</description>
			<pubDate>Tue, 03 Jun 2003 09:39:21 GMT</pubDate>
			<guid>http://liftoff.msfc.nasa.gov/2003/06/03.html#item573</guid>
		</item>
	</channel>
	</rss>`,
			check: func(t *testing.T, feed *model.Feed) {
				if feed.Title != "Liftoff News" {
					t.Errorf("Incorrect title, got: %s", feed.Title)
				}
			},
		},
		{
			name: "ParseRSSFeedWithRelativeURL",
			url:  "http://example.org/rss.xml",
			data: `<?xml version="1.0"?>
	<rss version="2.0">
	<channel>
		<title>Example Feed</title>
		<link>/blog</link>
		<item>
			<title>Example Entry</title>
			<link>/blog/article.html</link>
			<description>Something</description>
			<pubDate>Tue, 03 Jun 2003 09:39:21 GMT</pubDate>
			<guid>1234</guid>
		</item>
	</channel>
	</rss>`,
			check: func(t *testing.T, feed *model.Feed) {
				if feed.Title != "Example Feed" {
					t.Errorf("Incorrect title, got: %s", feed.Title)
				}

				if feed.FeedURL != "http://example.org/rss.xml" {
					t.Errorf("Incorrect feed URL, got: %s", feed.FeedURL)
				}

				if feed.SiteURL != "http://example.org/blog" {
					t.Errorf("Incorrect site URL, got: %s", feed.SiteURL)
				}

				if feed.Entries[0].URL != "http://example.org/blog/article.html" {
					t.Errorf("Incorrect entry URL, got: %s", feed.Entries[0].URL)
				}
			},
		},
		{
			name: "ParseRDF",
			url:  "http://example.org/",
			data: `<?xml version="1.0" encoding="utf-8"?>
		<rdf:RDF
		  xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
		  xmlns="http://purl.org/rss/1.0/"
		>

		  <channel>
			<title>RDF Example</title>
			<link>http://example.org/</link>
		  </channel>

		  <item>
			<title>Title</title>
			<link>http://example.org/item</link>
			<description>Test</description>
		  </item>
		</rdf:RDF>`,
			check: func(t *testing.T, feed *model.Feed) {
				if feed.Title != "RDF Example" {
					t.Errorf("Incorrect title, got: %s", feed.Title)
				}
			},
		},
		{
			name: "ParseRDFWithRelativeURL",
			url:  "http://example.org/rdf.xml",
			data: `<?xml version="1.0" encoding="utf-8"?>
		<rdf:RDF
		  xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
		  xmlns="http://purl.org/rss/1.0/"
		>

		  <channel>
			<title>RDF Example</title>
			<link>/blog</link>
		  </channel>

		  <item>
			<title>Title</title>
			<link>/blog/article.html</link>
			<description>Test</description>
		  </item>
		</rdf:RDF>`,
			check: func(t *testing.T, feed *model.Feed) {
				if feed.FeedURL != "http://example.org/rdf.xml" {
					t.Errorf("Incorrect feed URL, got: %s", feed.FeedURL)
				}

				if feed.SiteURL != "http://example.org/blog" {
					t.Errorf("Incorrect site URL, got: %s", feed.SiteURL)
				}

				if feed.Entries[0].URL != "http://example.org/blog/article.html" {
					t.Errorf("Incorrect entry URL, got: %s", feed.Entries[0].URL)
				}
			},
		},
		{
			name: "ParseJson",
			url:  "https://example.org/feed.json",
			data: `{
		"version": "https://jsonfeed.org/version/1",
		"title": "My Example Feed",
		"home_page_url": "https://example.org/",
		"feed_url": "https://example.org/feed.json",
		"items": [
			{
				"id": "2",
				"content_text": "This is a second item.",
				"url": "https://example.org/second-item"
			},
			{
				"id": "1",
				"content_html": "<p>Hello, world!</p>",
				"url": "https://example.org/initial-post"
			}
		]
	}`,
			check: func(t *testing.T, feed *model.Feed) {
				if feed.Title != "My Example Feed" {
					t.Errorf("Incorrect title, got: %s", feed.Title)
				}
			},
		},
		{
			name: "ParseJsonFeedWithRelativeURL",
			url:  "https://example.org/blog/feed.json",
			data: `{
		"version": "https://jsonfeed.org/version/1",
		"title": "My Example Feed",
		"home_page_url": "/blog",
		"feed_url": "/blog/feed.json",
		"items": [
			{
				"id": "2",
				"content_text": "This is a second item.",
				"url": "/blog/article.html"
			}
		]
	}`,
			check: func(t *testing.T, feed *model.Feed) {
				if feed.Title != "My Example Feed" {
					t.Errorf("Incorrect title, got: %s", feed.Title)
				}

				if feed.FeedURL != "https://example.org/blog/feed.json" {
					t.Errorf("Incorrect feed URL, got: %s", feed.FeedURL)
				}

				if feed.SiteURL != "https://example.org/blog" {
					t.Errorf("Incorrect site URL, got: %s", feed.SiteURL)
				}

				if feed.Entries[0].URL != "https://example.org/blog/article.html" {
					t.Errorf("Incorrect entry URL, got: %s", feed.Entries[0].URL)
				}
			},
		},
		{
			name: "ParseJsonFeedWithUTF8BOM",
			url:  "https://example.org/feed.json",
			data: "\xEF\xBB\xBF" + `{
		"version": "https://jsonfeed.org/version/1.1",
		"title": "My Example Feed",
		"home_page_url": "https://example.org/",
		"items": [
			{
				"id": "1",
				"content_text": "Hello, world!",
				"url": "https://example.org/initial-post"
			}
		]
	}`,
			check: func(t *testing.T, feed *model.Feed) {
				if feed.Title != "My Example Feed" {
					t.Errorf("Incorrect title, got: %s", feed.Title)
				}

				if len(feed.Entries) != 1 {
					t.Fatalf("Incorrect number of entries, got: %d", len(feed.Entries))
				}

				if feed.Entries[0].URL != "https://example.org/initial-post" {
					t.Errorf("Incorrect entry URL, got: %s", feed.Entries[0].URL)
				}
			},
		},
		{
			name: "ParseUnknownFeed",
			url:  "https://example.org/",
			data: `
		<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0 Transitional//EN" "http://www.w3.org/TR/xhtml1/DTD/xhtml1-transitional.dtd">
		<html xmlns="http://www.w3.org/1999/xhtml">
			<head>
				<title>Title of document</title>
			</head>
			<body>
				some content
			</body>
		</html>
	`,
			wantErr: true,
		},
		{
			name:    "ParseEmptyFeed",
			url:     "",
			data:    "",
			wantErr: true,
		},
	})
}

func TestParseRSS(t *testing.T) {
	data := `<?xml version="1.0"?>
<rss version="2.0" xmlns:media="http://search.yahoo.com/mrss/" xmlns:itunes="http://www.itunes.com/dtds/podcast-1.0.dtd" xmlns:atom="http://www.w3.org/2005/Atom" xmlns:dc="http://purl.org/dc/elements/1.1/">
<channel>
  <title>Example &amp; Co</title>
  <link>/blog</link>
  <atom:link rel="self" href="/feed.xml"/>
  <ttl>60</ttl>
  <image><url>/icon.png</url></image>
  <category>Feed</category>
  <item>
    <title>Item 1</title>
    <link>/post/1</link>
    <guid isPermaLink="false">id-1</guid>
    <pubDate>Tue, 03 Jun 2003 09:39:21 GMT</pubDate>
    <description>desc</description>
    <comments>https://example.org/c/1</comments>
    <dc:creator>Jane &lt;b&gt;Doe&lt;/b&gt;</dc:creator>
    <category> b </category><category>a</category><category>a</category>
    <enclosure url="/a.mp3" length="123" type="audio/mpeg"/>
    <media:thumbnail url="https://example.org/t.jpg"/>
    <media:group><media:content url="https://example.org/v.mp4" medium="video"/></media:group>
    <itunes:duration>1:02:30</itunes:duration>
  </item>
  <item>
    <guid>https://example.org/post/2</guid>
    <description>&lt;p&gt;Only &lt;b&gt;content&lt;/b&gt;&lt;/p&gt;</description>
  </item>
  <item><guid isPermaLink="false">dup</guid><link>https://example.org/x</link></item>
  <item><guid isPermaLink="false">dup</guid><link>https://example.org/y</link></item>
</channel></rss>`
	feed, err := ParseFeed("https://example.org/", strings.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if feed.Title != "Example & Co" || feed.SiteURL != "https://example.org/blog" || feed.FeedURL != "https://example.org/feed.xml" {
		t.Errorf("feed: %q %q %q", feed.Title, feed.SiteURL, feed.FeedURL)
	}
	if feed.TTL != time.Hour || feed.IconURL != "https://example.org/icon.png" {
		t.Errorf("ttl/icon: %v %q", feed.TTL, feed.IconURL)
	}
	e := feed.Entries[0]
	if e.URL != "https://example.org/post/1" || e.Hash != crypto.SHA256("id-1") || e.CommentsURL != "https://example.org/c/1" {
		t.Errorf("entry: %q %q %q", e.URL, e.Hash, e.CommentsURL)
	}
	if e.Author != "Jane Doe" || e.Content != "desc" || e.ReadingTime != 62 {
		t.Errorf("entry: %q %q %d", e.Author, e.Content, e.ReadingTime)
	}
	if !e.Date.Equal(time.Date(2003, 6, 3, 9, 39, 21, 0, time.UTC)) {
		t.Errorf("date: %v", e.Date)
	}
	if strings.Join(e.Tags, ",") != "a,b" {
		t.Errorf("tags: %v", e.Tags)
	}
	if len(e.Enclosures) != 3 || e.Enclosures[0].MimeType != "image/*" || e.Enclosures[1].URL != "https://example.org/a.mp3" ||
		e.Enclosures[1].Size != 123 || e.Enclosures[2].MimeType != "video/*" {
		t.Errorf("enclosures: %+v %+v %+v", e.Enclosures[0], e.Enclosures[1], e.Enclosures[2])
	}

	e = feed.Entries[1]
	if e.URL != "https://example.org/post/2" || e.Title != "Only content" || strings.Join(e.Tags, ",") != "Feed" {
		t.Errorf("entry 2: %q %q %v", e.URL, e.Title, e.Tags)
	}
	if feed.Entries[2].Hash != crypto.SHA256("dup") || feed.Entries[3].Hash != crypto.SHA256("dup|https://example.org/y") {
		t.Error("duplicate guid hashing")
	}
}

func TestParseAtom(t *testing.T) {
	data := `<?xml version="1.0" encoding="utf-8"?>
<feed xmlns="http://www.w3.org/2005/Atom" xmlns:media="http://search.yahoo.com/mrss/">
  <title>Atom</title>
  <link href="https://example.org/"/>
  <author><name>Feed Author</name></author>
  <entry>
    <id>urn:1</id>
    <title type="html">A &amp;amp; B</title>
    <link href="/e/1"/>
    <link rel="replies" type="text/html" href="https://example.org/e/1#comments"/>
    <updated>2024-01-02T03:04:05Z</updated>
    <content type="text">1 &lt; 2</content>
  </entry>
  <entry>
    <id>yt:video:x</id>
    <title>Video</title>
    <link rel="alternate" href="https://www.youtube.com/watch?v=x"/>
    <author><name>A</name></author><author><name>B</name></author>
    <media:group><media:description>Hello &lt;world&gt;</media:description></media:group>
  </entry>
</feed>`
	feed, err := ParseFeed("https://example.org/atom.xml", strings.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	e := feed.Entries[0]
	if e.Title != "A & B" || e.URL != "https://example.org/e/1" || e.CommentsURL != "https://example.org/e/1#comments" ||
		e.Content != "1 < 2" || e.Author != "Feed Author" || e.Hash != crypto.SHA256("urn:1") || e.Date.Year() != 2024 {
		t.Errorf("entry: %+v", e)
	}
	e = feed.Entries[1]
	if e.Content != "Hello <world>" || e.Author != "A, B" {
		t.Errorf("youtube entry: %q %q", e.Content, e.Author)
	}
}

func TestParseJSON(t *testing.T) {
	data := `{"version":"https://jsonfeed.org/version/1.1","title":"JSON","home_page_url":"https://example.org/","icon":"https://example.org/i.png",
	"items":[{"id":"1","url":"https://example.org/1","content_text":"a < b","tags":["x"],"authors":[{"name":"Ann"}],
	"attachments":[{"url":"https://example.org/a.mp3","mime_type":"audio/mpeg","size_in_bytes":9}]}]}`
	feed, err := ParseFeed("https://example.org/feed.json", strings.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	e := feed.Entries[0]
	if feed.IconURL != "https://example.org/i.png" || e.Content != "a &lt; b" || e.Author != "Ann" || e.Title != "a < b" ||
		e.Hash != crypto.SHA256("1") || e.Enclosures[0].Size != 9 || e.Tags[0] != "x" {
		t.Errorf("entry: %+v %q", e, feed.IconURL)
	}
}

func TestParseRDFAndCharset(t *testing.T) {
	data := []byte(`<?xml version="1.0" encoding="ISO-8859-1"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#" xmlns="http://purl.org/rss/1.0/">
  <channel><title>RDF</title><link>https://example.org/</link></channel>
  <item><title>Caf` + "\xe9" + `</title><link>https://example.org/1</link></item>
</rdf:RDF>`)
	feed, err := ParseFeed("https://example.org/rdf", bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if e := feed.Entries[0]; e.Title != "Café" || e.Hash != crypto.SHA256("https://example.org/1") {
		t.Errorf("entry: %q", e.Title)
	}
}

func TestParseNotAFeed(t *testing.T) {
	if _, err := ParseFeed("https://example.org/", strings.NewReader("<html><body>hi</body></html>")); err != ErrFeedFormatNotDetected {
		t.Errorf("got %v", err)
	}
}
