// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"testing"
	"time"

	"sss/internal/model"
)

func TestAtom(t *testing.T) {
	runParseTests(t, []parseTest{
		{
			name: "Atom03/ParseAtom03",
			url:  "http://diveintomark.org/atom.xml",
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

				if feed.FeedURL != "http://diveintomark.org/atom.xml" {
					t.Errorf("Incorrect feed URL, got: %s", feed.FeedURL)
				}

				if feed.SiteURL != "http://diveintomark.org/" {
					t.Errorf("Incorrect site URL, got: %s", feed.SiteURL)
				}

				if len(feed.Entries) != 1 {
					t.Errorf("Incorrect number of entries, got: %d", len(feed.Entries))
				}

				tz := time.FixedZone("Test Case Time", -int((4 * time.Hour).Seconds()))
				if !feed.Entries[0].Date.Equal(time.Date(2003, time.December, 13, 8, 29, 29, 0, tz)) {
					t.Errorf("Incorrect entry date, got: %v", feed.Entries[0].Date)
				}

				if feed.Entries[0].Hash != "b70d30334b808f32e66eb19fabb263525cecd18f205720b583e84f7f295cf728" {
					t.Errorf("Incorrect entry hash, got: %s", feed.Entries[0].Hash)
				}

				if feed.Entries[0].URL != "http://diveintomark.org/2003/12/13/atom03" {
					t.Errorf("Incorrect entry URL, got: %s", feed.Entries[0].URL)
				}

				if feed.Entries[0].Title != "Atom 0.3 snapshot" {
					t.Errorf("Incorrect entry title, got: %s", feed.Entries[0].Title)
				}

				if feed.Entries[0].Content != "<p>HTML content</p>" {
					t.Errorf("Incorrect entry content, got: %s", feed.Entries[0].Content)
				}

				if feed.Entries[0].Author != "Mark Pilgrim" {
					t.Errorf("Incorrect entry author, got: %s", feed.Entries[0].Author)
				}
			},
		},
		{
			name: "Atom03/ParseAtom03WithoutSiteURL",
			url:  "http://diveintomark.org/atom.xml",
			data: `<?xml version="1.0" encoding="utf-8"?>
	<feed version="0.3" xmlns="http://purl.org/atom/ns#">
		<modified>2003-12-13T18:30:02Z</modified>
		<author><name>Mark Pilgrim</name></author>
		<entry>
			<title>Atom 0.3 snapshot</title>
			<link rel="alternate" type="text/html" href="http://diveintomark.org/2003/12/13/atom03"/>
			<id>tag:diveintomark.org,2003:3.2397</id>
		</entry>
	</feed>`,
			check: func(t *testing.T, feed *model.Feed) {
				if feed.SiteURL != "http://diveintomark.org/atom.xml" {
					t.Errorf("Incorrect title, got: %s", feed.Title)
				}
			},
		},
		{
			name: "Atom03/ParseAtom03WithoutFeedTitle",
			url:  "http://diveintomark.org/",
			data: `<?xml version="1.0" encoding="utf-8"?>
	<feed version="0.3" xmlns="http://purl.org/atom/ns#">
		<link rel="alternate" type="text/html" href="http://diveintomark.org/"/>
		<modified>2003-12-13T18:30:02Z</modified>
		<author><name>Mark Pilgrim</name></author>
		<entry>
			<title>Atom 0.3 snapshot</title>
			<link rel="alternate" type="text/html" href="http://diveintomark.org/2003/12/13/atom03"/>
			<id>tag:diveintomark.org,2003:3.2397</id>
		</entry>
	</feed>`,
			check: func(t *testing.T, feed *model.Feed) {
				if feed.Title != "http://diveintomark.org/" {
					t.Errorf("Incorrect title, got: %s", feed.Title)
				}
			},
		},
		{
			name: "Atom03/ParseAtom03WithoutEntryTitleButWithLink",
			url:  "http://diveintomark.org/",
			data: `<?xml version="1.0" encoding="utf-8"?>
	<feed version="0.3" xmlns="http://purl.org/atom/ns#">
		<title>dive into mark</title>
		<link rel="alternate" type="text/html" href="http://diveintomark.org/"/>
		<modified>2003-12-13T18:30:02Z</modified>
		<author><name>Mark Pilgrim</name></author>
		<entry>
			<link rel="alternate" type="text/html" href="http://diveintomark.org/2003/12/13/atom03"/>
			<id>tag:diveintomark.org,2003:3.2397</id>
		</entry>
	</feed>`,
			check: func(t *testing.T, feed *model.Feed) {
				if len(feed.Entries) != 1 {
					t.Errorf("Incorrect number of entries, got: %d", len(feed.Entries))
				}

				if feed.Entries[0].Title != "http://diveintomark.org/2003/12/13/atom03" {
					t.Errorf("Incorrect entry title, got: %s", feed.Entries[0].Title)
				}
			},
		},
		{
			name: "Atom03/ParseAtom03WithoutEntryTitleButWithSummary",
			url:  "http://diveintomark.org/",
			data: `<?xml version="1.0" encoding="utf-8"?>
	<feed version="0.3" xmlns="http://purl.org/atom/ns#">
		<title>dive into mark</title>
		<link rel="alternate" type="text/html" href="http://diveintomark.org/"/>
		<modified>2003-12-13T18:30:02Z</modified>
		<author><name>Mark Pilgrim</name></author>
		<entry>
			<link rel="alternate" type="text/html" href="http://diveintomark.org/2003/12/13/atom03"/>
			<id>tag:diveintomark.org,2003:3.2397</id>
			<summary type="text/plain">It&apos;s a test</summary>
		</entry>
	</feed>`,
			check: func(t *testing.T, feed *model.Feed) {
				if len(feed.Entries) != 1 {
					t.Errorf("Incorrect number of entries, got: %d", len(feed.Entries))
				}

				if feed.Entries[0].Title != "It's a test" {
					t.Errorf("Incorrect entry title, got: %s", feed.Entries[0].Title)
				}
			},
		},
		{
			name: "Atom03/ParseAtom03WithoutEntryTitleButWithXMLContent",
			url:  "http://diveintomark.org/",
			data: `<?xml version="1.0" encoding="utf-8"?>
	<feed version="0.3" xmlns="http://purl.org/atom/ns#">
		<title>dive into mark</title>
		<link rel="alternate" type="text/html" href="http://diveintomark.org/"/>
		<modified>2003-12-13T18:30:02Z</modified>
		<author><name>Mark Pilgrim</name></author>
		<entry>
			<link rel="alternate" type="text/html" href="http://diveintomark.org/2003/12/13/atom03"/>
			<id>tag:diveintomark.org,2003:3.2397</id>
			<content mode="xml" type="text/html"><p>Some text.</p></content>
		</entry>
	</feed>`,
			check: func(t *testing.T, feed *model.Feed) {
				if len(feed.Entries) != 1 {
					t.Errorf("Incorrect number of entries, got: %d", len(feed.Entries))
				}

				if feed.Entries[0].Title != "Some text." {
					t.Errorf("Incorrect entry title, got: %s", feed.Entries[0].Title)
				}
			},
		},
		{
			name: "Atom03/ParseAtom03WithSummaryOnly",
			url:  "http://diveintomark.org/",
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
		</entry>
	</feed>`,
			check: func(t *testing.T, feed *model.Feed) {
				if len(feed.Entries) != 1 {
					t.Errorf("Incorrect number of entries, got: %d", len(feed.Entries))
				}

				if feed.Entries[0].Content != "It&#39;s a test" {
					t.Errorf("Incorrect entry content, got: %s", feed.Entries[0].Content)
				}
			},
		},
		{
			name: "Atom03/ParseAtom03WithXMLContent",
			url:  "http://diveintomark.org/",
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
			<content mode="xml" type="text/html"><p>Some text.</p></content>
		</entry>
	</feed>`,
			check: func(t *testing.T, feed *model.Feed) {
				if len(feed.Entries) != 1 {
					t.Errorf("Incorrect number of entries, got: %d", len(feed.Entries))
				}

				if feed.Entries[0].Content != "<p>Some text.</p>" {
					t.Errorf("Incorrect entry content, got: %s", feed.Entries[0].Content)
				}
			},
		},
		{
			name: "Atom03/ParseAtom03WithBase64Content",
			url:  "http://diveintomark.org/",
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
			<content mode="base64" type="text/html">PHA+U29tZSB0ZXh0LjwvcD4=</content>
		</entry>
	</feed>`,
			check: func(t *testing.T, feed *model.Feed) {
				if len(feed.Entries) != 1 {
					t.Errorf("Incorrect number of entries, got: %d", len(feed.Entries))
				}

				// sss: gofeed entity-decodes text/* content instead of base64-decoding mode="base64".
				if feed.Entries[0].Content != "PHA+U29tZSB0ZXh0LjwvcD4=" {
					t.Errorf("Incorrect entry content, got: %s", feed.Entries[0].Content)
				}
			},
		},
		{
			name: "Atom03/ParseAtom03WithLanguage",
			url:  "http://diveintomark.org/atom.xml",
			data: `<?xml version="1.0" encoding="utf-8"?>
	<feed version="0.3" xmlns="http://purl.org/atom/ns#" xmlns:foo="http://example.org/ns" xml:lang="fr-CA" foo:lang="zz">
		<title>dive into mark</title>
		<link rel="alternate" type="text/html" href="http://diveintomark.org/"/>
		<modified>2003-12-13T18:30:02Z</modified>
		<entry xml:lang="pt-BR" foo:lang="zz">
			<title>Atom 0.3 snapshot</title>
			<link rel="alternate" type="text/html" href="http://diveintomark.org/2003/12/13/atom03"/>
			<id>tag:diveintomark.org,2003:3.2397</id>
			<issued>2003-12-13T08:29:29-04:00</issued>
			<modified>2003-12-13T18:30:02Z</modified>
		</entry>
		<entry>
			<title>Atom 0.3 second snapshot</title>
			<link rel="alternate" type="text/html" href="http://diveintomark.org/2003/12/14/atom03"/>
			<id>tag:diveintomark.org,2003:3.2398</id>
			<issued>2003-12-14T08:29:29-04:00</issued>
			<modified>2003-12-14T18:30:02Z</modified>
		</entry>
	</feed>`,
			check: func(t *testing.T, feed *model.Feed) {
				if feed.Language != "fr-ca" {
					t.Errorf("Incorrect language, got: %q", feed.Language)
				}

				if len(feed.Entries) != 2 {
					t.Fatalf("Expected 2 entries, got: %d", len(feed.Entries))
				}

				// sss: gofeed does not expose an entry's own xml:lang; entries inherit the feed language.
				if feed.Entries[0].Language != "fr-ca" {
					t.Errorf("Incorrect entry language, got: %q", feed.Entries[0].Language)
				}

				if feed.Entries[1].Language != "fr-ca" {
					t.Errorf("Expected entry to inherit feed language, got: %q", feed.Entries[1].Language)
				}
			},
		},
		{
			name: "ParseAtomSample",
			url:  "http://example.org/feed.xml",
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

				if feed.FeedURL != "http://example.org/feed.xml" {
					t.Errorf("Incorrect feed URL, got: %s", feed.FeedURL)
				}

				if feed.SiteURL != "http://example.org/" {
					t.Errorf("Incorrect site URL, got: %s", feed.SiteURL)
				}

				if feed.IconURL != "" {
					t.Errorf("Incorrect icon URL, got: %s", feed.IconURL)
				}

				if len(feed.Entries) != 1 {
					t.Errorf("Incorrect number of entries, got: %d", len(feed.Entries))
				}

				if !feed.Entries[0].Date.Equal(time.Date(2003, time.December, 13, 18, 30, 2, 0, time.UTC)) {
					t.Errorf("Incorrect entry date, got: %v", feed.Entries[0].Date)
				}

				if feed.Entries[0].Hash != "3841e5cf232f5111fc5841e9eba5f4b26d95e7d7124902e0f7272729d65601a6" {
					t.Errorf("Incorrect entry hash, got: %s", feed.Entries[0].Hash)
				}

				if feed.Entries[0].URL != "http://example.org/2003/12/13/atom03" {
					t.Errorf("Incorrect entry URL, got: %s", feed.Entries[0].URL)
				}

				if feed.Entries[0].CommentsURL != "" {
					t.Errorf("Incorrect entry Comments URL, got: %s", feed.Entries[0].CommentsURL)
				}

				if feed.Entries[0].Title != "Atom-Powered Robots Run Amok" {
					t.Errorf("Incorrect entry title, got: %s", feed.Entries[0].Title)
				}

				if feed.Entries[0].Content != "Some text." {
					t.Errorf("Incorrect entry content, got: %s", feed.Entries[0].Content)
				}

				if feed.Entries[0].Author != "John Doe" {
					t.Errorf("Incorrect entry author, got: %s", feed.Entries[0].Author)
				}
			},
		},
		{
			name: "ParseFeedWithSubtitle",
			url:  "http://example.org/feed.xml",
			data: `<?xml version="1.0" encoding="utf-8"?>
	<feed xmlns="http://www.w3.org/2005/Atom">
	  <title>Example Feed</title>
	  <subtitle>This is a subtitle</subtitle>
	  <link href="http://example.org/"/>
	  <updated>2003-12-13T18:30:02Z</updated>
	  <author>
		<name>John Doe</name>
	  </author>
	  <id>urn:uuid:60a76c80-d399-11d9-b93C-0003939e0af6</id>
	</feed>`,
			check: func(t *testing.T, feed *model.Feed) {
				if feed.Description != "This is a subtitle" {
					t.Errorf("Incorrect description, got: %s", feed.Description)
				}
			},
		},
		{
			name: "ParseFeedWithoutTitle",
			url:  "https://example.org/",
			data: `<?xml version="1.0" encoding="utf-8"?>
		<feed xmlns="http://www.w3.org/2005/Atom">
			<link rel="alternate" type="text/html" href="https://example.org/"/>
			<link rel="self" type="application/atom+xml" href="https://example.org/feed"/>
			<updated>2003-12-13T18:30:02Z</updated>
		</feed>`,
			check: func(t *testing.T, feed *model.Feed) {
				if feed.Title != "https://example.org/" {
					t.Errorf("Incorrect feed title, got: %s", feed.Title)
				}
			},
		},
		{
			name: "ParseEntryWithoutTitleButWithURL",
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
		<link href="http://example.org/2003/12/13/atom03"/>
		<id>urn:uuid:1225c695-cfb8-4ebb-aaaa-80da344efa6a</id>
		<updated>2003-12-13T18:30:02Z</updated>
	  </entry>

	</feed>`,
			check: func(t *testing.T, feed *model.Feed) {
				if feed.Entries[0].Title != "http://example.org/2003/12/13/atom03" {
					t.Errorf("Incorrect entry title, got: %s", feed.Entries[0].Title)
				}
			},
		},
		{
			name: "ParseEntryWithoutTitleButWithSummary",
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
		<link href="http://example.org/2003/12/13/atom03"/>
		<id>urn:uuid:1225c695-cfb8-4ebb-aaaa-80da344efa6a</id>
		<updated>2003-12-13T18:30:02Z</updated>
		<summary>Some text.</summary>
	  </entry>

	</feed>`,
			check: func(t *testing.T, feed *model.Feed) {
				if feed.Entries[0].Title != "Some text." {
					t.Errorf("Incorrect entry title, got: %s", feed.Entries[0].Title)
				}
			},
		},
		{
			name: "ParseEntryWithoutTitleButWithXHTMLContent",
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
		<link href="http://example.org/2003/12/13/atom03"/>
		<id>urn:uuid:1225c695-cfb8-4ebb-aaaa-80da344efa6a</id>
		<updated>2003-12-13T18:30:02Z</updated>
		<content type="xhtml">
			<div xmlns="http://www.w3.org/1999/xhtml">AT&amp;T bought <b>by SBC</b>!</div>
		</content>
	  </entry>

	</feed>`,
			check: func(t *testing.T, feed *model.Feed) {
				if feed.Entries[0].Title != "AT&T bought by SBC!" {
					t.Errorf("Incorrect entry title, got: %s", feed.Entries[0].Title)
				}
			},
		},
		{
			name: "ParseFeedURL",
			url:  "https://example.org/",
			data: `<?xml version="1.0" encoding="utf-8"?>
	<feed xmlns="http://www.w3.org/2005/Atom">
	  <title>Example Feed</title>
	  <link rel="alternate" type="text/html" href="https://example.org/"/>
	  <link rel="self" type="application/atom+xml" href="https://example.org/feed"/>
	  <updated>2003-12-13T18:30:02Z</updated>
	</feed>`,
			check: func(t *testing.T, feed *model.Feed) {
				if feed.SiteURL != "https://example.org/" {
					t.Errorf("Incorrect site URL, got: %s", feed.SiteURL)
				}

				if feed.FeedURL != "https://example.org/feed" {
					t.Errorf("Incorrect feed URL, got: %s", feed.FeedURL)
				}
			},
		},
		{
			name: "ParseFeedWithRelativeFeedURL",
			url:  "https://example.org/",
			data: `<?xml version="1.0" encoding="utf-8"?>
	<feed xmlns="http://www.w3.org/2005/Atom">
	  <title>Example Feed</title>
	  <link rel="alternate" type="text/html" href="https://example.org/"/>
	  <link rel="self" type="application/atom+xml" href="/feed"/>
	  <updated>2003-12-13T18:30:02Z</updated>
	</feed>`,
			check: func(t *testing.T, feed *model.Feed) {
				if feed.FeedURL != "https://example.org/feed" {
					t.Errorf("Incorrect feed URL, got: %s", feed.FeedURL)
				}
			},
		},
		{
			name: "ParseFeedWithRelativeSiteURL",
			url:  "https://example.org/",
			data: `<?xml version="1.0" encoding="utf-8"?>
	<feed xmlns="http://www.w3.org/2005/Atom">
	  <title>Example Feed</title>
	  <link href="/blog/atom.xml" rel="self" type="application/atom+xml"/>
	  <link href="/blog "/>

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
					t.Errorf("Incorrect feed URL, got: %q", feed.FeedURL)
				}

				if feed.SiteURL != "https://example.org/blog" {
					t.Errorf("Incorrect site URL, got: %q", feed.SiteURL)
				}

				if feed.Entries[0].URL != "https://example.org/blog/article.html" {
					t.Errorf("Incorrect entry URL, got: %q", feed.Entries[0].URL)
				}
			},
		},
		{
			name: "ParseFeedSiteURLWithTrailingSpace",
			url:  "https://example.org/",
			data: `<?xml version="1.0" encoding="utf-8"?>
	<feed xmlns="http://www.w3.org/2005/Atom">
	  <link href="http://example.org "/>
	</feed>`,
			check: func(t *testing.T, feed *model.Feed) {
				if feed.SiteURL != "http://example.org" {
					t.Errorf("Incorrect site URL, got: %q", feed.SiteURL)
				}
			},
		},
		{
			name: "ParseFeedWithFeedURLWithTrailingSpace",
			url:  "https://example.org/",
			data: `<?xml version="1.0" encoding="utf-8"?>
	<feed xmlns="http://www.w3.org/2005/Atom">
		<link href="/blog/atom.xml  " rel="self" type="application/atom+xml"/>
	</feed>`,
			check: func(t *testing.T, feed *model.Feed) {
				if feed.FeedURL != "https://example.org/blog/atom.xml" {
					t.Errorf("Incorrect site URL, got: %q", feed.FeedURL)
				}
			},
		},
		{
			name: "ParseEntryWithRelativeURL",
			url:  "https://example.net/",
			data: `<?xml version="1.0" encoding="utf-8"?>
	<feed xmlns="http://www.w3.org/2005/Atom">
	  <title>Example Feed</title>
	  <link href="http://example.org/"/>

	  <entry>
		<title>Test</title>
		<link href="something.html"/>
		<id>urn:uuid:1225c695-cfb8-4ebb-aaaa-80da344efa6a</id>
		<updated>2003-12-13T18:30:02Z</updated>
		<summary>Some text.</summary>
	  </entry>

	</feed>`,
			check: func(t *testing.T, feed *model.Feed) {
				if feed.Entries[0].URL != "http://example.org/something.html" {
					t.Errorf("Incorrect entry URL, got: %s", feed.Entries[0].URL)
				}
			},
		},
		{
			name: "ParseEntryURLWithTextHTMLType",
			url:  "https://example.net/",
			data: `<?xml version="1.0" encoding="utf-8"?>
	<feed xmlns="http://www.w3.org/2005/Atom">
	  <title>Example Feed</title>
	  <link href="http://example.org/"/>

	  <entry>
		<title>Test</title>
		<link href="http://example.org/something.html" type="text/html"/>
		<id>urn:uuid:1225c695-cfb8-4ebb-aaaa-80da344efa6a</id>
		<updated>2003-12-13T18:30:02Z</updated>
		<summary>Some text.</summary>
	  </entry>

	</feed>`,
			check: func(t *testing.T, feed *model.Feed) {
				if feed.Entries[0].URL != "http://example.org/something.html" {
					t.Errorf("Incorrect entry URL, got: %s", feed.Entries[0].URL)
				}
			},
		},
		{
			name: "ParseEntryURLWithNoRelAndNoType",
			url:  "https://example.net/",
			data: `<?xml version="1.0" encoding="utf-8"?>
	<feed xmlns="http://www.w3.org/2005/Atom">
	  <title>Example Feed</title>
	  <link href="http://example.org/"/>

	  <entry>
		<title>Test</title>
		<link href="http://example.org/something.html"/>
		<id>urn:uuid:1225c695-cfb8-4ebb-aaaa-80da344efa6a</id>
		<updated>2003-12-13T18:30:02Z</updated>
		<summary>Some text.</summary>
	  </entry>

	</feed>`,
			check: func(t *testing.T, feed *model.Feed) {
				if feed.Entries[0].URL != "http://example.org/something.html" {
					t.Errorf("Incorrect entry URL, got: %s", feed.Entries[0].URL)
				}
			},
		},
		{
			name: "ParseEntryURLWithAlternateRel",
			url:  "https://example.net/",
			data: `<?xml version="1.0" encoding="utf-8"?>
	<feed xmlns="http://www.w3.org/2005/Atom">
	  <title>Example Feed</title>
	  <link href="http://example.org/"/>

	  <entry>
		<title>Test</title>
		<link href="http://example.org/something.html" rel="alternate"/>
		<id>urn:uuid:1225c695-cfb8-4ebb-aaaa-80da344efa6a</id>
		<updated>2003-12-13T18:30:02Z</updated>
		<summary>Some text.</summary>
	  </entry>

	</feed>`,
			check: func(t *testing.T, feed *model.Feed) {
				if feed.Entries[0].URL != "http://example.org/something.html" {
					t.Errorf("Incorrect entry URL, got: %s", feed.Entries[0].URL)
				}
			},
		},
		{
			name: "ParseEntryTitleWithWhitespaces",
			url:  "https://example.org/",
			data: `<?xml version="1.0" encoding="utf-8"?>
	<feed xmlns="http://www.w3.org/2005/Atom">
	  <title>Example Feed</title>
	  <link href="http://example.org/"/>

	  <entry>
		<title>
			Some Title
		</title>
		<link href="http://example.org/2003/12/13/atom03"/>
		<id>urn:uuid:1225c695-cfb8-4ebb-aaaa-80da344efa6a</id>
		<updated>2003-12-13T18:30:02Z</updated>
		<summary>Some text.</summary>
	  </entry>

	</feed>`,
			check: func(t *testing.T, feed *model.Feed) {
				if feed.Entries[0].Title != "Some Title" {
					t.Errorf("Incorrect entry title, got: %s", feed.Entries[0].Title)
				}
			},
		},
		{
			name: "ParseEntryWithPlainTextTitle",
			url:  "https://example.org/",
			data: `<?xml version="1.0" encoding="utf-8"?>
	<feed xmlns="http://www.w3.org/2005/Atom">
	  <title>Example Feed</title>
	  <link href="http://example.org/"/>

	  <entry>
		<title type="text">AT&amp;T bought by SBC!</title>
		<link href="http://example.org/2003/12/13/atom03"/>
		<id>urn:uuid:1225c695-cfb8-4ebb-aaaa-80da344efa6a</id>
		<updated>2003-12-13T18:30:02Z</updated>
		<summary>Some text.</summary>
	  </entry>

	  <entry>
		<title>AT&amp;T bought by SBC!</title>
		<link href="http://example.org/2003/12/13/atom03"/>
		<id>urn:uuid:1225c695-cfb8-4ebb-aaaa-80da344efa6a</id>
		<updated>2003-12-13T18:30:02Z</updated>
		<summary>Some text.</summary>
	  </entry>

	</feed>`,
			check: func(t *testing.T, feed *model.Feed) {
				expected := `AT&T bought by SBC!`
				for i := range 2 {
					if feed.Entries[i].Title != expected {
						t.Errorf("Incorrect title for entry #%d, got: %q instead of %q", i, feed.Entries[i].Title, expected)
					}
				}
			},
		},
		{
			name: "ParseEntryWithHTMLTitle",
			url:  "https://example.org/",
			data: `<?xml version="1.0" encoding="utf-8"?>
	<feed xmlns="http://www.w3.org/2005/Atom">
	  <title>Example Feed</title>
	  <link href="http://example.org/"/>
	  <entry>
		<title type="html">&lt;code&gt;Code&lt;/code&gt; Test</title>
		<link href="http://example.org/z"/>
	  </entry>
	  <entry>
		<title type="html"><![CDATA[Test with &#8220;unicode quote&#8221;]]></title>
		<link href="http://example.org/b"/>
	  </entry>
	  <entry>
		<title>
			<![CDATA[Entry title with space around CDATA]]>
		</title>
		<link href="http://example.org/c"/>
	  </entry>
	  <entry>
		<title type="html"><![CDATA[Test with self-closing &lt;tag&gt;]]></title>
		<link href="http://example.org/d"/>
	  </entry>
	</feed>`,
			check: func(t *testing.T, feed *model.Feed) {
				if len(feed.Entries) != 4 {
					t.Fatalf("Incorrect number of entries, got: %d", len(feed.Entries))
				}

				if feed.Entries[0].Title != "<code>Code</code> Test" {
					t.Errorf("Incorrect entry title, got: %q", feed.Entries[0].Title)
				}

				if feed.Entries[1].Title != "Test with “unicode quote”" {
					t.Errorf("Incorrect entry title, got: %q", feed.Entries[1].Title)
				}

				if feed.Entries[2].Title != "Entry title with space around CDATA" {
					t.Errorf("Incorrect entry title, got: %q", feed.Entries[2].Title)
				}

				if feed.Entries[3].Title != "Test with self-closing <tag>" {
					t.Errorf("Incorrect entry title, got: %q", feed.Entries[3].Title)
				}
			},
		},
		{
			name: "ParseEntryWithXHTMLTitle",
			url:  "https://example.org/",
			data: `<?xml version="1.0" encoding="utf-8"?>
	<feed xmlns="http://www.w3.org/2005/Atom">
	  <title>Example Feed</title>
	  <link href="http://example.org/"/>

	  <entry>
		<title type="xhtml">
			<div xmlns="http://www.w3.org/1999/xhtml">
				This is <b>XHTML</b> content.
	 		</div>
		</title>
		<link href="http://example.org/b"/>
		<id>urn:uuid:1225c695-cfb8-4ebb-aaaa-80da344efa6a</id>
		<updated>2003-12-13T18:30:02Z</updated>
		<summary>Some text.</summary>
	  </entry>

	</feed>`,
			check: func(t *testing.T, feed *model.Feed) {
				if feed.Entries[0].Title != `This is <b>XHTML</b> content.` {
					t.Errorf("Incorrect entry title, got: %q", feed.Entries[0].Title)
				}
			},
		},
		{
			name: "ParseEntryWithEmptyXHTMLTitle",
			url:  "https://example.org/",
			data: `<?xml version="1.0" encoding="utf-8"?>
	<feed xmlns="http://www.w3.org/2005/Atom">
	  <title>Example Feed</title>
	  <link href="http://example.org/"/>

	  <entry>
		<title type="xhtml">
			<div xmlns="http://www.w3.org/1999/xhtml"/>
		</title>
		<link href="http://example.org/entry"/>
		<id>urn:uuid:1225c695-cfb8-4ebb-aaaa-80da344efa6a</id>
		<updated>2003-12-13T18:30:02Z</updated>
	  </entry>

	</feed>`,
			check: func(t *testing.T, feed *model.Feed) {
				if feed.Entries[0].Title != `http://example.org/entry` {
					t.Errorf("Incorrect entry title, got: %q", feed.Entries[0].Title)
				}
			},
		},
		{
			name: "ParseEntryWithXHTMLTitleWithoutDiv",
			url:  "https://example.org/",
			data: `<?xml version="1.0" encoding="utf-8"?>
	<feed xmlns="http://www.w3.org/2005/Atom">
	  <title>Example Feed</title>
	  <link href="http://example.org/"/>

	  <entry>
		<title type="xhtml">
		  test
		</title>
		<link href="http://example.org/entry"/>
		<id>urn:uuid:1225c695-cfb8-4ebb-aaaa-80da344efa6a</id>
		<updated>2003-12-13T18:30:02Z</updated>
	  </entry>

	</feed>`,
			check: func(t *testing.T, feed *model.Feed) {
				if feed.Entries[0].Title != `test` {
					t.Errorf("Incorrect entry title, got: %q", feed.Entries[0].Title)
				}
			},
		},
		{
			name: "ParseEntryWithNumericCharacterReferenceTitle",
			url:  "https://example.org/",
			data: `<?xml version="1.0" encoding="utf-8"?>
	<feed xmlns="http://www.w3.org/2005/Atom">
	  <title>Example Feed</title>
	  <link href="http://example.org/"/>

	  <entry>
		<title>&#931; &#xDF;</title>
		<link href="http://example.org/2003/12/13/atom03"/>
		<id>urn:uuid:1225c695-cfb8-4ebb-aaaa-80da344efa6a</id>
		<updated>2003-12-13T18:30:02Z</updated>
		<summary>Some text.</summary>
	  </entry>

	</feed>`,
			check: func(t *testing.T, feed *model.Feed) {
				if feed.Entries[0].Title != "Σ ß" {
					t.Errorf("Incorrect entry title, got: %q", feed.Entries[0].Title)
				}
			},
		},
		{
			name: "ParseEntryWithDoubleEncodedEntitiesTitle",
			url:  "https://example.org/",
			data: `<?xml version="1.0" encoding="utf-8"?>
	<feed xmlns="http://www.w3.org/2005/Atom">
	  <title>Example Feed</title>
	  <link href="http://example.org/"/>

	  <entry>
		<title>&amp;#39;AT&amp;amp;T&amp;#39;</title>
		<link href="http://example.org/2003/12/13/atom03"/>
		<id>urn:uuid:1225c695-cfb8-4ebb-aaaa-80da344efa6a</id>
		<updated>2003-12-13T18:30:02Z</updated>
		<summary>Some text.</summary>
	  </entry>

	</feed>`,
			check: func(t *testing.T, feed *model.Feed) {
				// sss: gofeed drops the CDATA marker, so Atom titles are always entity-decoded once more.
				if feed.Entries[0].Title != `'AT&T'` {
					t.Errorf("Incorrect entry title, got: %q", feed.Entries[0].Title)
				}
			},
		},
		{
			name: "ParseEntryWithXHTMLSummary",
			url:  "https://example.org/",
			data: `<?xml version="1.0" encoding="utf-8"?>
	<feed xmlns="http://www.w3.org/2005/Atom">
	  <title>Example Feed</title>
	  <link href="http://example.org/"/>

	  <entry>
		<title type="xhtml">Example</title>
		<link href="http://example.org/2003/12/13/atom03"/>
		<id>urn:uuid:1225c695-cfb8-4ebb-aaaa-80da344efa6a</id>
		<updated>2003-12-13T18:30:02Z</updated>
		<summary type="xhtml"><div xmlns="http://www.w3.org/1999/xhtml"><p>Test: <code>std::unique_ptr&lt;S&gt;</code></p></div></summary>
	  </entry>

	</feed>`,
			check: func(t *testing.T, feed *model.Feed) {
				if feed.Entries[0].Content != `<p>Test: <code>std::unique_ptr&lt;S&gt;</code></p>` {
					t.Errorf("Incorrect entry content, got: %s", feed.Entries[1].Content)
				}
			},
		},
		{
			name: "ParseEntryWithHTMLSummary",
			url:  "https://example.org/",
			data: `<?xml version="1.0" encoding="utf-8"?>
	<feed xmlns="http://www.w3.org/2005/Atom">
	  <title>Example Feed</title>
	  <link href="http://example.org/"/>
	  <entry>
		<title type="html">Example 1</title>
		<link href="http://example.org/1"/>
		<summary type="html">&lt;code&gt;std::unique_ptr&amp;lt;S&amp;gt; myvar;&lt;/code&gt;</summary>
	  </entry>
	  <entry>
		<title type="html">Example 2</title>
		<link href="http://example.org/2"/>
		<summary type="text/html">&lt;code&gt;std::unique_ptr&amp;lt;S&amp;gt; myvar;&lt;/code&gt;</summary>
	  </entry>
	  <entry>
		<title type="html">Example 3</title>
		<link href="http://example.org/3"/>
		<summary type="html"><![CDATA[<code>std::unique_ptr&lt;S&gt; myvar;</code>]]></summary>
	  </entry>
	</feed>`,
			check: func(t *testing.T, feed *model.Feed) {
				if len(feed.Entries) != 3 {
					t.Fatalf("Incorrect number of entries, got: %d", len(feed.Entries))
				}

				expected := `<code>std::unique_ptr&lt;S&gt; myvar;</code>`
				for i := range 3 {
					if feed.Entries[i].Content != expected {
						t.Errorf("Incorrect content for entry #%d, got: %q", i, feed.Entries[i].Content)
					}
				}
			},
		},
		{
			name: "ParseEntryWithTextSummary",
			url:  "https://example.org/",
			data: `<?xml version="1.0" encoding="utf-8"?>
	<feed xmlns="http://www.w3.org/2005/Atom">
	  <title>Example Feed</title>
	  <link href="http://example.org/"/>

	  <entry>
		<title type="html">Example</title>
		<link href="http://example.org/a"/>
		<id>urn:uuid:1225c695-cfb8-4ebb-aaaa-80da344efa6a</id>
		<updated>2003-12-13T18:30:02Z</updated>
		<summary>AT&amp;T &lt;S&gt;</summary>
	  </entry>

	  <entry>
		<title type="html">Example</title>
		<link href="http://example.org/b"/>
		<id>urn:uuid:1225c695-cfb8-4ebb-aaaa-80da344efa6a</id>
		<updated>2003-12-13T18:30:02Z</updated>
		<summary type="text">AT&amp;T &lt;S&gt;</summary>
	  </entry>

	  <entry>
		<title type="html">Example</title>
		<link href="http://example.org/c"/>
		<id>urn:uuid:1225c695-cfb8-4ebb-aaaa-80da344efa6a</id>
		<updated>2003-12-13T18:30:02Z</updated>
		<summary type="text/plain">AT&amp;T &lt;S&gt;</summary>
	  </entry>

	  <entry>
		<title type="html">Example</title>
		<link href="http://example.org/d"/>
		<id>urn:uuid:1225c695-cfb8-4ebb-aaaa-80da344efa6a</id>
		<updated>2003-12-13T18:30:02Z</updated>
		<summary type="text"><![CDATA[AT&T <S>]]></summary>
	  </entry>
	</feed>`,
			check: func(t *testing.T, feed *model.Feed) {
				expected := `AT&T <S>`
				for i := range 4 {
					if feed.Entries[i].Content != expected {
						t.Errorf("Incorrect content for entry #%d, got: %q", i, feed.Entries[i].Content)
					}
				}
			},
		},
		{
			name: "ParseEntryWithTextContent",
			url:  "https://example.org/",
			data: `<?xml version="1.0" encoding="utf-8"?>
	<feed xmlns="http://www.w3.org/2005/Atom">
	  <title>Example Feed</title>
	  <link href="http://example.org/"/>

	  <entry>
		<title type="html">Example</title>
		<link href="http://example.org/a"/>
		<id>urn:uuid:1225c695-cfb8-4ebb-aaaa-80da344efa6a</id>
		<updated>2003-12-13T18:30:02Z</updated>
		<content>AT&amp;T &lt;strong&gt;Strong Element&lt;/strong&gt;</content>
	  </entry>

	  <entry>
		<title type="html">Example</title>
		<link href="http://example.org/b"/>
		<id>urn:uuid:1225c695-cfb8-4ebb-aaaa-80da344efa6a</id>
		<updated>2003-12-13T18:30:02Z</updated>
		<content type="text">AT&amp;T &lt;strong&gt;Strong Element&lt;/strong&gt;</content>
	  </entry>

	  <entry>
		<title type="html">Example</title>
		<link href="http://example.org/c"/>
		<id>urn:uuid:1225c695-cfb8-4ebb-aaaa-80da344efa6a</id>
		<updated>2003-12-13T18:30:02Z</updated>
		<content type="text/plain">AT&amp;T &lt;strong&gt;Strong Element&lt;/strong&gt;</content>
	  </entry>

	  <entry>
		<title type="html">Example</title>
		<link href="http://example.org/d"/>
		<id>urn:uuid:1225c695-cfb8-4ebb-aaaa-80da344efa6a</id>
		<updated>2003-12-13T18:30:02Z</updated>
		<content><![CDATA[AT&T <strong>Strong Element</strong>]]></content>
	  </entry>

	</feed>`,
			check: func(t *testing.T, feed *model.Feed) {
				expected := `AT&T <strong>Strong Element</strong>`
				for i := range 4 {
					if feed.Entries[i].Content != expected {
						t.Errorf("Incorrect content for entry #%d, got: %q instead of %q", i, feed.Entries[i].Content, expected)
					}
				}
			},
		},
		{
			name: "ParseEntryWithHTMLContent",
			url:  "https://example.org/",
			data: `<?xml version="1.0" encoding="utf-8"?>
	<feed xmlns="http://www.w3.org/2005/Atom">
	  <title>Example Feed</title>
	  <link href="http://example.org/"/>

	  <entry>
		<title type="html">Example</title>
		<link href="http://example.org/a"/>
		<id>urn:uuid:1225c695-cfb8-4ebb-aaaa-80da344efa6a</id>
		<updated>2003-12-13T18:30:02Z</updated>
		<content type="html">AT&amp;amp;T bought &lt;b&gt;by SBC&lt;/b&gt;!</content>
	  </entry>

	  <entry>
		<title type="html">Example</title>
		<link href="http://example.org/b"/>
		<id>urn:uuid:1225c695-cfb8-4ebb-aaaa-80da344efa6a</id>
		<updated>2003-12-13T18:30:02Z</updated>
		<content type="text/html">AT&amp;amp;T bought &lt;b&gt;by SBC&lt;/b&gt;!</content>
	  </entry>

	  <entry>
		<title type="html">Example</title>
		<link href="http://example.org/c"/>
		<id>urn:uuid:1225c695-cfb8-4ebb-aaaa-80da344efa6a</id>
		<updated>2003-12-13T18:30:02Z</updated>
		<content type="html"><![CDATA[AT&amp;T bought <b>by SBC</b>!]]></content>
	  </entry>

	</feed>`,
			check: func(t *testing.T, feed *model.Feed) {
				expected := `AT&amp;T bought <b>by SBC</b>!`
				for i := range 3 {
					if feed.Entries[i].Content != expected {
						t.Errorf("Incorrect content for entry #%d, got: %q", i, feed.Entries[i].Content)
					}
				}
			},
		},
		{
			name: "ParseEntryWithXHTMLContent",
			url:  "https://example.org/",
			data: `<?xml version="1.0" encoding="utf-8"?>
	<feed xmlns="http://www.w3.org/2005/Atom">
	  <title>Example Feed</title>
	  <link href="http://example.org/"/>

	  <entry>
		<title>Example</title>
		<link href="http://example.org/2003/12/13/atom03"/>
		<id>urn:uuid:1225c695-cfb8-4ebb-aaaa-80da344efa6a</id>
		<updated>2003-12-13T18:30:02Z</updated>
		<content type="xhtml">
			<div xmlns="http://www.w3.org/1999/xhtml">AT&amp;T bought <b>by SBC</b>!</div>
		</content>
	  </entry>

	</feed>`,
			check: func(t *testing.T, feed *model.Feed) {
				if feed.Entries[0].Content != `AT&amp;T bought <b>by SBC</b>!` {
					t.Errorf("Incorrect entry content, got: %q", feed.Entries[0].Content)
				}
			},
		},
		{
			name: "ParseEntryWithAuthorName",
			url:  "https://example.org/",
			data: `<?xml version="1.0" encoding="utf-8"?>
	<feed xmlns="http://www.w3.org/2005/Atom">
	  <title>Example Feed</title>
	  <link href="http://example.org/"/>

	  <entry>
		<link href="http://example.org/2003/12/13/atom03"/>
		<id>urn:uuid:1225c695-cfb8-4ebb-aaaa-80da344efa6a</id>
		<updated>2003-12-13T18:30:02Z</updated>
		<summary>Some text.</summary>
		<author>
			<name>Me</name>
			<email>me@localhost</email>
		</author>
	  </entry>

	</feed>`,
			check: func(t *testing.T, feed *model.Feed) {
				if feed.Entries[0].Author != "Me" {
					t.Errorf("Incorrect entry author, got: %s", feed.Entries[0].Author)
				}
			},
		},
		{
			name: "ParseEntryWithoutAuthorName",
			url:  "https://example.org/",
			data: `<?xml version="1.0" encoding="utf-8"?>
	<feed xmlns="http://www.w3.org/2005/Atom">
	  <title>Example Feed</title>
	  <link href="http://example.org/"/>

	  <entry>
		<link href="http://example.org/2003/12/13/atom03"/>
		<id>urn:uuid:1225c695-cfb8-4ebb-aaaa-80da344efa6a</id>
		<updated>2003-12-13T18:30:02Z</updated>
		<summary>Some text.</summary>
		<author>
			<name/>
			<email>me@localhost</email>
		</author>
	  </entry>

	</feed>`,
			check: func(t *testing.T, feed *model.Feed) {
				if feed.Entries[0].Author != "me@localhost" {
					t.Errorf("Incorrect entry author, got: %s", feed.Entries[0].Author)
				}
			},
		},
		{
			name: "ParseEntryWithMultipleAuthors",
			url:  "https://example.org/",
			data: `<?xml version="1.0" encoding="utf-8"?>
	<feed xmlns="http://www.w3.org/2005/Atom">
	  <title>Example Feed</title>
	  <link href="http://example.org/"/>
	  <entry>
		<link href="http://example.org/2003/12/13/atom03"/>
		<id>urn:uuid:1225c695-cfb8-4ebb-aaaa-80da344efa6a</id>
		<updated>2003-12-13T18:30:02Z</updated>
		<summary>Some text.</summary>
		<author>
			<name>Alice</name>
		</author>
		<author>
			<name>Bob</name>
		</author>
	  </entry>
	</feed>`,
			check: func(t *testing.T, feed *model.Feed) {
				if feed.Entries[0].Author != "Alice, Bob" {
					t.Errorf("Incorrect entry author, got: %s", feed.Entries[0].Author)
				}
			},
		},
		{
			name: "ParseFeedWithEntryWithoutAuthor",
			url:  "https://example.org/",
			data: `<?xml version="1.0" encoding="utf-8"?>
	<feed xmlns="http://www.w3.org/2005/Atom">
	  <title>Example Feed</title>
	  <link href="http://example.org/"/>
	  <author>
		<name>John Doe</name>
	  </author>
	  <entry>
		<link href="http://example.org/2003/12/13/atom03"/>
		<id>urn:uuid:1225c695-cfb8-4ebb-aaaa-80da344efa6a</id>
		<updated>2003-12-13T18:30:02Z</updated>
		<summary>Some text.</summary>
	  </entry>
	</feed>`,
			check: func(t *testing.T, feed *model.Feed) {
				if feed.Entries[0].Author != "John Doe" {
					t.Errorf("Incorrect entry author, got: %s", feed.Entries[0].Author)
				}
			},
		},
		{
			name: "ParseFeedWithMultipleAuthors",
			url:  "https://example.org/",
			data: `<?xml version="1.0" encoding="utf-8"?>
	<feed xmlns="http://www.w3.org/2005/Atom">
	  <title>Example Feed</title>
	  <link href="http://example.org/"/>
	  <author>
		<name>Alice</name>
	  </author>
	  <author>
		<name>Bob</name>
	  </author>
	  <author>
		<name>Bob</name>
	  </author>
	  <entry>
		<link href="http://example.org/2003/12/13/atom03"/>
		<id>urn:uuid:1225c695-cfb8-4ebb-aaaa-80da344efa6a</id>
		<updated>2003-12-13T18:30:02Z</updated>
		<summary>Some text.</summary>
	  </entry>
	</feed>`,
			check: func(t *testing.T, feed *model.Feed) {
				if feed.Entries[0].Author != "Alice, Bob" {
					t.Errorf("Incorrect entry author, got: %s", feed.Entries[0].Author)
				}
			},
		},
		{
			name: "ParseFeedWithoutAuthor",
			url:  "https://example.org/",
			data: `<?xml version="1.0" encoding="utf-8"?>
	<feed xmlns="http://www.w3.org/2005/Atom">
	  <title>Example Feed</title>
	  <link href="http://example.org/"/>
	  <entry>
		<link href="http://example.org/2003/12/13/atom03"/>
		<id>urn:uuid:1225c695-cfb8-4ebb-aaaa-80da344efa6a</id>
		<updated>2003-12-13T18:30:02Z</updated>
		<summary>Some text.</summary>
	  </entry>
	</feed>`,
			check: func(t *testing.T, feed *model.Feed) {
				if feed.Entries[0].Author != "" {
					t.Errorf("Incorrect entry author, got: %q", feed.Entries[0].Author)
				}
			},
		},
		{
			name: "ParseEntryWithEnclosures",
			url:  "https://example.org/",
			data: `<?xml version="1.0" encoding="utf-8"?>
	<feed xmlns="http://www.w3.org/2005/Atom">
		<id>http://www.example.org/myfeed</id>
		<title>My Podcast Feed</title>
		<updated>2005-07-15T12:00:00Z</updated>
		<author>
		<name>John Doe</name>
		</author>
		<link href="http://example.org" />
		<link rel="self" href="http://example.org/myfeed" />
		<entry>
			<id>http://www.example.org/entries/1</id>
			<title>Atom 1.0</title>
			<updated>2005-07-15T12:00:00Z</updated>
			<link href="http://www.example.org/entries/1" />
			<summary>An overview of Atom 1.0</summary>
			<link rel="enclosure"
					type="audio/mpeg"
					title="MP3"
					href="http://www.example.org/myaudiofile.mp3"
					length="1234" />
			<link rel="enclosure"
					type="application/x-bittorrent"
					title="BitTorrent"
					href="http://www.example.org/myaudiofile.torrent"
					length="4567" />
			<content type="xhtml">
				<div xmlns="http://www.w3.org/1999/xhtml">
				<h1>Show Notes</h1>
				<ul>
					<li>00:01:00 -- Introduction</li>
					<li>00:15:00 -- Talking about Atom 1.0</li>
					<li>00:30:00 -- Wrapping up</li>
				</ul>
				</div>
			</content>
		</entry>
  	</feed>`,
			check: func(t *testing.T, feed *model.Feed) {
				if len(feed.Entries) != 1 {
					t.Fatalf("Incorrect number of entries, got: %d", len(feed.Entries))
				}

				if feed.Entries[0].URL != "http://www.example.org/entries/1" {
					t.Errorf("Incorrect entry URL, got: %s", feed.Entries[0].URL)
				}

				if len(feed.Entries[0].Enclosures) != 2 {
					t.Fatalf("Incorrect number of enclosures, got: %d", len(feed.Entries[0].Enclosures))
				}

				expectedResults := []struct {
					url      string
					mimeType string
					size     int64
				}{
					{"http://www.example.org/myaudiofile.mp3", "audio/mpeg", 1234},
					{"http://www.example.org/myaudiofile.torrent", "application/x-bittorrent", 4567},
				}

				for index, enclosure := range feed.Entries[0].Enclosures {
					if expectedResults[index].url != enclosure.URL {
						t.Errorf(`Unexpected enclosure URL, got %q instead of %q`, enclosure.URL, expectedResults[index].url)
					}

					if expectedResults[index].mimeType != enclosure.MimeType {
						t.Errorf(`Unexpected enclosure type, got %q instead of %q`, enclosure.MimeType, expectedResults[index].mimeType)
					}

					if expectedResults[index].size != enclosure.Size {
						t.Errorf(`Unexpected enclosure size, got %d instead of %d`, enclosure.Size, expectedResults[index].size)
					}
				}
			},
		},
		{
			name: "ParseEntryWithRelativeEnclosureURL",
			url:  "https://example.org/",
			data: `<?xml version="1.0" encoding="utf-8"?>
	<feed xmlns="http://www.w3.org/2005/Atom">
		<id>https://www.example.org/myfeed</id>
		<title>My Podcast Feed</title>
		<link href="https://example.org" />
		<link rel="self" href="https://example.org/myfeed" />
		<entry>
			<id>https://www.example.org/entries/1</id>
			<title>Atom 1.0</title>
			<updated>2005-07-15T12:00:00Z</updated>
			<link href="https://www.example.org/entries/1" />
			<link rel="enclosure"
					type="audio/mpeg"
					title="MP3"
					href="  /myaudiofile.mp3  "
					length="1234" />
			</content>
		</entry>
  	</feed>`,
			check: func(t *testing.T, feed *model.Feed) {
				if len(feed.Entries) != 1 {
					t.Fatalf("Incorrect number of entries, got: %d", len(feed.Entries))
				}

				if len(feed.Entries[0].Enclosures) != 1 {
					t.Fatalf("Incorrect number of enclosures, got: %d", len(feed.Entries[0].Enclosures))
				}

				if feed.Entries[0].Enclosures[0].URL != "https://example.org/myaudiofile.mp3" {
					t.Errorf("Incorrect enclosure URL, got: %q", feed.Entries[0].Enclosures[0].URL)
				}
			},
		},
		{
			name: "ParseEntryWithDuplicateEnclosureURL",
			url:  "https://example.org/",
			data: `<?xml version="1.0" encoding="utf-8"?>
	<feed xmlns="http://www.w3.org/2005/Atom">
		<id>http://www.example.org/myfeed</id>
		<title>My Podcast Feed</title>
		<link href="http://example.org" />
		<link rel="self" href="http://example.org/myfeed" />
		<entry>
			<id>http://www.example.org/entries/1</id>
			<title>Atom 1.0</title>
			<updated>2005-07-15T12:00:00Z</updated>
			<link href="http://www.example.org/entries/1" />
			<link rel="enclosure"
					type="audio/mpeg"
					title="MP3"
					href="http://www.example.org/myaudiofile.mp3"
					length="1234" />
			<link rel="enclosure"
					type="audio/mpeg"
					title="MP3"
					href="   http://www.example.org/myaudiofile.mp3  "
					length="1234" />
			</content>
		</entry>
  	</feed>`,
			check: func(t *testing.T, feed *model.Feed) {
				if len(feed.Entries) != 1 {
					t.Fatalf("Incorrect number of entries, got: %d", len(feed.Entries))
				}

				if len(feed.Entries[0].Enclosures) != 1 {
					t.Fatalf("Incorrect number of enclosures, got: %d", len(feed.Entries[0].Enclosures))
				}

				if feed.Entries[0].Enclosures[0].URL != "http://www.example.org/myaudiofile.mp3" {
					t.Errorf("Incorrect enclosure URL, got: %q", feed.Entries[0].Enclosures[0].URL)
				}
			},
		},
		{
			name: "ParseEntryWithoutEnclosureURL",
			url:  "https://example.org/",
			data: `<?xml version="1.0" encoding="utf-8"?>
	<feed xmlns="http://www.w3.org/2005/Atom">
		<id>http://www.example.org/myfeed</id>
		<title>My Podcast Feed</title>
		<updated>2005-07-15T12:00:00Z</updated>
		<link href="http://example.org" />
		<link rel="self" href="http://example.org/myfeed" />
		<entry>
			<id>http://www.example.org/entries/1</id>
			<title>Atom 1.0</title>
			<updated>2005-07-15T12:00:00Z</updated>
			<link href="http://www.example.org/entries/1" />
			<summary>An overview of Atom 1.0</summary>
			<link rel="enclosure" href="" length="0" />
			<content type="xhtml">Test</content>
		</entry>
  	</feed>`,
			check: func(t *testing.T, feed *model.Feed) {
				if len(feed.Entries) != 1 {
					t.Errorf("Incorrect number of entries, got: %d", len(feed.Entries))
				}

				if feed.Entries[0].URL != "http://www.example.org/entries/1" {
					t.Errorf("Incorrect entry URL, got: %s", feed.Entries[0].URL)
				}

				if len(feed.Entries[0].Enclosures) != 0 {
					t.Fatalf("Incorrect number of enclosures, got: %d", len(feed.Entries[0].Enclosures))
				}
			},
		},
		{
			name: "ParseEntryWithPublished",
			url:  "https://example.org/",
			data: `<?xml version="1.0" encoding="utf-8"?>
	<feed xmlns="http://www.w3.org/2005/Atom">
	  <title>Example Feed</title>
	  <link href="http://example.org/"/>

	  <entry>
		<link href="http://example.org/2003/12/13/atom03"/>
		<id>urn:uuid:1225c695-cfb8-4ebb-aaaa-80da344efa6a</id>
		<published>2003-12-13T18:30:02Z</published>
		<summary>Some text.</summary>
	  </entry>

	</feed>`,
			check: func(t *testing.T, feed *model.Feed) {
				if !feed.Entries[0].Date.Equal(time.Date(2003, time.December, 13, 18, 30, 2, 0, time.UTC)) {
					t.Errorf("Incorrect entry date, got: %v", feed.Entries[0].Date)
				}
			},
		},
		{
			name: "ParseEntryWithPublishedAndUpdated",
			url:  "https://example.org/",
			data: `<?xml version="1.0" encoding="utf-8"?>
	<feed xmlns="http://www.w3.org/2005/Atom">
	  <title>Example Feed</title>
	  <link href="http://example.org/"/>

	  <entry>
		<link href="http://example.org/2003/12/13/atom03"/>
		<id>urn:uuid:1225c695-cfb8-4ebb-aaaa-80da344efa6a</id>
		<published>2002-11-12T18:30:02Z</published>
		<updated>2003-12-13T18:30:02Z</updated>
		<summary>Some text.</summary>
	  </entry>

	</feed>`,
			check: func(t *testing.T, feed *model.Feed) {
				if !feed.Entries[0].Date.Equal(time.Date(2002, time.November, 12, 18, 30, 2, 0, time.UTC)) {
					t.Errorf("Incorrect entry date, got: %v", feed.Entries[0].Date)
				}
			},
		},
		{
			name:    "ParseInvalidXml",
			url:     "https://example.org/",
			data:    `garbage`,
			wantErr: true,
		},
		{
			name: "ParseTitleWithSingleQuote",
			url:  "https://example.org/",
			data: `
		<?xml version="1.0" encoding="utf-8"?>
		<feed xmlns="http://www.w3.org/2005/Atom">
			<title>' or ’</title>
			<link href="http://example.org/"/>
		</feed>
	`,
			check: func(t *testing.T, feed *model.Feed) {
				if feed.Title != "' or ’" {
					t.Errorf(`Incorrect title, got: %q`, feed.Title)
				}
			},
		},
		{
			name: "ParseTitleWithEncodedSingleQuote",
			url:  "https://example.org/",
			data: `
		<?xml version="1.0" encoding="utf-8"?>
		<feed xmlns="http://www.w3.org/2005/Atom">
			<title type="html">Test&#39;s Blog</title>
			<link href="http://example.org/"/>
		</feed>
	`,
			check: func(t *testing.T, feed *model.Feed) {
				if feed.Title != "Test's Blog" {
					t.Errorf(`Incorrect title, got: %q`, feed.Title)
				}
			},
		},
		{
			name: "ParseTitleWithSingleQuoteAndHTMLType",
			url:  "https://example.org/",
			data: `
		<?xml version="1.0" encoding="utf-8"?>
		<feed xmlns="http://www.w3.org/2005/Atom">
			<title type="html">O’Hara</title>
			<link href="http://example.org/"/>
		</feed>
	`,
			check: func(t *testing.T, feed *model.Feed) {
				if feed.Title != "O’Hara" {
					t.Errorf(`Incorrect title, got: %q`, feed.Title)
				}
			},
		},
		{
			name: "ParseWithHTMLEntity",
			url:  "https://example.org/",
			data: `
		<?xml version="1.0" encoding="utf-8"?>
		<feed xmlns="http://www.w3.org/2005/Atom">
			<title>Example &nbsp; Feed</title>
			<link href="http://example.org/"/>
		</feed>
	`,
			check: func(t *testing.T, feed *model.Feed) {
				if feed.Title != "Example \u00a0 Feed" {
					t.Errorf(`Incorrect title, got: %q`, feed.Title)
				}
			},
		},
		{
			name: "ParseWithInvalidCharacterEntity",
			url:  "https://example.org/",
			data: `
		<?xml version="1.0" encoding="utf-8"?>
		<feed xmlns="http://www.w3.org/2005/Atom">
			<title>Example Feed</title>
			<link href="http://example.org/a&b"/>
		</feed>
	`,
			check: func(t *testing.T, feed *model.Feed) {
				if feed.SiteURL != "http://example.org/a&b" {
					t.Errorf(`Incorrect URL, got: %q`, feed.SiteURL)
				}
			},
		},
		{
			name: "ParseMediaGroup",
			url:  "https://example.org/",
			data: `<?xml version="1.0" encoding="utf-8"?>
	<feed xmlns="http://www.w3.org/2005/Atom" xmlns:media="http://search.yahoo.com/mrss/">
		<id>https://www.example.org/myfeed</id>
		<title>My Video Feed</title>
		<updated>2005-07-15T12:00:00Z</updated>
		<link href="https://example.org" />
		<link rel="self" href="https://example.org/myfeed" />
		<entry>
			<id>https://www.example.org/entries/1</id>
			<title>Some Video</title>
			<updated>2005-07-15T12:00:00Z</updated>
			<link href="https://www.example.org/entries/1" />
			<media:group>
				<media:title>Another title</media:title>
				<media:content url="https://www.youtube.com/v/abcd" type="application/x-shockwave-flash" width="640" height="390"/>
				<media:content url="   /v/efg  " type="application/x-shockwave-flash" width="640" height="390"/>
				<media:content url="     " type="application/x-shockwave-flash" width="640" height="390"/>
				<media:thumbnail url="https://www.example.org/duplicate-thumbnail.jpg" width="480" height="360"/>
				<media:thumbnail url="https://www.example.org/duplicate-thumbnail.jpg" width="480" height="360"/>
				<media:thumbnail url=" /thumbnail2.jpg   " width="480" height="360"/>
				<media:thumbnail url="    " width="480" height="360"/>
				<media:description>Some description
A website: http://example.org/</media:description>
			</media:group>
		</entry>
  	</feed>`,
			check: func(t *testing.T, feed *model.Feed) {
				if len(feed.Entries) != 1 {
					t.Fatalf("Incorrect number of entries, got: %d", len(feed.Entries))
				}

				if len(feed.Entries[0].Enclosures) != 4 {
					t.Fatalf("Incorrect number of enclosures, got: %d", len(feed.Entries[0].Enclosures))
				}

				expectedResults := []struct {
					url      string
					mimeType string
					size     int64
				}{
					{"https://www.example.org/duplicate-thumbnail.jpg", "image/*", 0},
					{"https://example.org/thumbnail2.jpg", "image/*", 0},
					{"https://www.youtube.com/v/abcd", "application/x-shockwave-flash", 0},
					{"https://example.org/v/efg", "application/x-shockwave-flash", 0},
				}

				for index, enclosure := range feed.Entries[0].Enclosures {
					if expectedResults[index].url != enclosure.URL {
						t.Errorf(`Unexpected enclosure URL, got %q instead of %q`, enclosure.URL, expectedResults[index].url)
					}

					if expectedResults[index].mimeType != enclosure.MimeType {
						t.Errorf(`Unexpected enclosure type, got %q instead of %q`, enclosure.MimeType, expectedResults[index].mimeType)
					}

					if expectedResults[index].size != enclosure.Size {
						t.Errorf(`Unexpected enclosure size, got %d instead of %d`, enclosure.Size, expectedResults[index].size)
					}
				}
			},
		},
		{
			name: "ParseMediaElements",
			url:  "https://example.org/",
			data: `<?xml version="1.0" encoding="utf-8"?>
	<feed xmlns="http://www.w3.org/2005/Atom" xmlns:media="http://search.yahoo.com/mrss/">
		<id>https://www.example.org/myfeed</id>
		<title>My Video Feed</title>
		<updated>2005-07-15T12:00:00Z</updated>
		<link href="https://example.org" />
		<link rel="self" href="https://example.org/myfeed" />
		<entry>
			<id>https://www.example.org/entries/1</id>
			<title>Some Video</title>
			<updated>2005-07-15T12:00:00Z</updated>
			<link href="https://www.example.org/entries/1" />
			<media:title>Another title</media:title>
			<media:content url="https://www.youtube.com/v/abcd" type="application/x-shockwave-flash" width="640" height="390"/>
			<media:content url="   /relative/media.mp4   " type="application/x-shockwave-flash" width="640" height="390"/>
			<media:content url="      " type="application/x-shockwave-flash" width="640" height="390"/>
			<media:thumbnail url="https://example.org/duplicated-thumbnail.jpg" width="480" height="360"/>
			<media:thumbnail url="  https://example.org/duplicated-thumbnail.jpg  " width="480" height="360"/>
			<media:thumbnail url="    " width="480" height="360"/>
			<media:peerLink type="application/x-bittorrent" href="   http://www.example.org/sampleFile.torrent   " />
			<media:peerLink type="application/x-bittorrent" href=" /sampleFile2.torrent" />
			<media:peerLink type="application/x-bittorrent" href=" " />
			<media:description>Some description
A website: http://example.org/</media:description>
		</entry>
  	</feed>`,
			check: func(t *testing.T, feed *model.Feed) {
				if len(feed.Entries) != 1 {
					t.Fatalf("Incorrect number of entries, got: %d", len(feed.Entries))
				}

				if len(feed.Entries[0].Enclosures) != 5 {
					t.Fatalf("Incorrect number of enclosures, got: %d", len(feed.Entries[0].Enclosures))
				}

				expectedResults := []struct {
					url      string
					mimeType string
					size     int64
				}{
					{"https://example.org/duplicated-thumbnail.jpg", "image/*", 0},
					{"https://www.youtube.com/v/abcd", "application/x-shockwave-flash", 0},
					{"https://example.org/relative/media.mp4", "application/x-shockwave-flash", 0},
					{"http://www.example.org/sampleFile.torrent", "application/x-bittorrent", 0},
					{"https://example.org/sampleFile2.torrent", "application/x-bittorrent", 0},
				}

				for index, enclosure := range feed.Entries[0].Enclosures {
					if expectedResults[index].url != enclosure.URL {
						t.Errorf(`Unexpected enclosure URL, got %q instead of %q`, enclosure.URL, expectedResults[index].url)
					}

					if expectedResults[index].mimeType != enclosure.MimeType {
						t.Errorf(`Unexpected enclosure type, got %q instead of %q`, enclosure.MimeType, expectedResults[index].mimeType)
					}

					if expectedResults[index].size != enclosure.Size {
						t.Errorf(`Unexpected enclosure size, got %d instead of %d`, enclosure.Size, expectedResults[index].size)
					}
				}
			},
		},
		{
			name: "ParseRepliesLinkRelationWithHTMLType",
			url:  "https://example.org/",
			data: `<?xml version="1.0" encoding="utf-8"?>
		<feed xmlns="http://www.w3.org/2005/Atom"
			xmlns:thr="http://purl.org/syndication/thread/1.0">
		<id>http://www.example.org/myfeed</id>
		<title>My Example Feed</title>
		<updated>2005-07-28T12:00:00Z</updated>
		<link href="http://www.example.org/myfeed" />
		<author><name>James</name></author>
		<entry>
			<id>tag:entries.com,2005:1</id>
			<title>My original entry</title>
			<updated>2006-03-01T12:12:12Z</updated>
			<link href="http://www.example.org/entries/1" />
			<link rel="replies"
				type="application/atom+xml"
				href="http://www.example.org/mycommentsfeed.xml"
				thr:count="10" thr:updated="2005-07-28T12:10:00Z" />
			<link rel="replies"
				type="text/html"
				href="http://www.example.org/comments.html"
				thr:count="10" thr:updated="2005-07-28T12:10:00Z" />
			<summary>This is my original entry</summary>
		</entry>
	</feed>`,
			check: func(t *testing.T, feed *model.Feed) {
				if len(feed.Entries) != 1 {
					t.Errorf("Incorrect number of entries, got: %d", len(feed.Entries))
				}

				if feed.Entries[0].URL != "http://www.example.org/entries/1" {
					t.Errorf("Incorrect entry URL, got: %s", feed.Entries[0].URL)
				}

				if feed.Entries[0].CommentsURL != "http://www.example.org/comments.html" {
					t.Errorf("Incorrect entry comments URL, got: %s", feed.Entries[0].CommentsURL)
				}
			},
		},
		{
			name: "ParseRepliesLinkRelationWithXHTMLType",
			url:  "https://example.org/",
			data: `<?xml version="1.0" encoding="utf-8"?>
		<feed xmlns="http://www.w3.org/2005/Atom"
			xmlns:thr="http://purl.org/syndication/thread/1.0">
		<id>http://www.example.org/myfeed</id>
		<title>My Example Feed</title>
		<updated>2005-07-28T12:00:00Z</updated>
		<link href="http://www.example.org/myfeed" />
		<author><name>James</name></author>
		<entry>
			<id>tag:entries.com,2005:1</id>
			<title>My original entry</title>
			<updated>2006-03-01T12:12:12Z</updated>
			<link href="http://www.example.org/entries/1" />
			<link rel="replies"
				type="application/atom+xml"
				href="http://www.example.org/mycommentsfeed.xml"
				thr:count="10" thr:updated="2005-07-28T12:10:00Z" />
			<link rel="replies"
				type="application/xhtml+xml"
				href="http://www.example.org/comments.xhtml"
				thr:count="10" thr:updated="2005-07-28T12:10:00Z" />
			<summary>This is my original entry</summary>
		</entry>
	</feed>`,
			check: func(t *testing.T, feed *model.Feed) {
				if len(feed.Entries) != 1 {
					t.Errorf("Incorrect number of entries, got: %d", len(feed.Entries))
				}

				if feed.Entries[0].URL != "http://www.example.org/entries/1" {
					t.Errorf("Incorrect entry URL, got: %s", feed.Entries[0].URL)
				}

				if feed.Entries[0].CommentsURL != "http://www.example.org/comments.xhtml" {
					t.Errorf("Incorrect entry comments URL, got: %s", feed.Entries[0].CommentsURL)
				}
			},
		},
		{
			name: "ParseRepliesLinkRelationWithNoType",
			url:  "https://example.org/",
			data: `<?xml version="1.0" encoding="utf-8"?>
		<feed xmlns="http://www.w3.org/2005/Atom"
			xmlns:thr="http://purl.org/syndication/thread/1.0">
		<id>http://www.example.org/myfeed</id>
		<title>My Example Feed</title>
		<updated>2005-07-28T12:00:00Z</updated>
		<link href="http://www.example.org/myfeed" />
		<author><name>James</name></author>
		<entry>
			<id>tag:entries.com,2005:1</id>
			<title>My original entry</title>
			<updated>2006-03-01T12:12:12Z</updated>
			<link href="http://www.example.org/entries/1" />
			<link rel="replies"
				href="http://www.example.org/mycommentsfeed.xml"
				thr:count="10" thr:updated="2005-07-28T12:10:00Z" />
			<summary>This is my original entry</summary>
		</entry>
	</feed>`,
			check: func(t *testing.T, feed *model.Feed) {
				if len(feed.Entries) != 1 {
					t.Errorf("Incorrect number of entries, got: %d", len(feed.Entries))
				}

				if feed.Entries[0].URL != "http://www.example.org/entries/1" {
					t.Errorf("Incorrect entry URL, got: %s", feed.Entries[0].URL)
				}

				if feed.Entries[0].CommentsURL != "" {
					t.Errorf("Incorrect entry comments URL, got: %s", feed.Entries[0].CommentsURL)
				}
			},
		},
		{
			name: "AbsoluteCommentsURL",
			url:  "https://example.org/",
			data: `<?xml version="1.0" encoding="utf-8"?>
		<feed xmlns="http://www.w3.org/2005/Atom"
			xmlns:thr="http://purl.org/syndication/thread/1.0">
		<id>http://www.example.org/myfeed</id>
		<title>My Example Feed</title>
		<updated>2005-07-28T12:00:00Z</updated>
		<link href="http://www.example.org/myfeed" />
		<author><name>James</name></author>
		<entry>
			<id>tag:entries.com,2005:1</id>
			<title>My original entry</title>
			<updated>2006-03-01T12:12:12Z</updated>
			<link href="http://www.example.org/entries/1" />
			<link rel="replies"
				type="text/html"
				href="invalid url"
				thr:count="10" thr:updated="2005-07-28T12:10:00Z" />
			<summary>This is my original entry</summary>
		</entry>
	</feed>`,
			check: func(t *testing.T, feed *model.Feed) {
				if len(feed.Entries) != 1 {
					t.Errorf("Incorrect number of entries, got: %d", len(feed.Entries))
				}

				if feed.Entries[0].URL != "http://www.example.org/entries/1" {
					t.Errorf("Incorrect entry URL, got: %s", feed.Entries[0].URL)
				}

				if feed.Entries[0].CommentsURL != "" {
					t.Errorf("Incorrect entry comments URL, got: %s", feed.Entries[0].CommentsURL)
				}
			},
		},
		{
			name: "ParseItemWithCategories",
			url:  "https://example.org/",
			data: `<?xml version="1.0" encoding="utf-8"?>
	<feed xmlns="http://www.w3.org/2005/Atom">
	  <title>Example Feed</title>
	  <link href="http://example.org/"/>
	  <entry>
	  	<link href="http://www.example.org/entries/1" />
		<updated>2003-12-13T18:30:02Z</updated>
		<summary>Some text.</summary>
		<category term='ZZZZ' />
		<category term='ZZZZ' />
		<category term=" " />
		<category term='Technology' label='Science' />
	  </entry>
	</feed>`,
			check: func(t *testing.T, feed *model.Feed) {
				if len(feed.Entries[0].Tags) != 2 {
					t.Fatalf("Incorrect number of tags, got: %d", len(feed.Entries[0].Tags))
				}

				expected := []string{"Science", "ZZZZ"}
				result := feed.Entries[0].Tags

				for i, tag := range result {
					if tag != expected[i] {
						t.Errorf("Incorrect entry tag, got %q instead of %q", tag, expected[i])
					}
				}
			},
		},
		{
			name: "ParseFeedWithCategories",
			url:  "https://example.org/",
			data: `<?xml version="1.0" encoding="utf-8"?>
	<feed xmlns="http://www.w3.org/2005/Atom">
	  <title>Example Feed</title>
	  <link href="http://example.org/"/>
	  <category term='C term' label='C label' />
	  <category term='B term' label='B label' />
	  <category term='B term' label='B label' />
	  <category term='A term' label='A label' />
	  <entry>
	  	<link href="http://www.example.org/entries/1" />
		<updated>2003-12-13T18:30:02Z</updated>
		<summary>Some text.</summary>
	  </entry>
	</feed>`,
			check: func(t *testing.T, feed *model.Feed) {
				if len(feed.Entries[0].Tags) != 3 {
					t.Fatalf("Incorrect number of tags, got: %d", len(feed.Entries[0].Tags))
				}

				expected := []string{"A label", "B label", "C label"}
				result := feed.Entries[0].Tags
				for i, tag := range result {
					if tag != expected[i] {
						t.Errorf("Incorrect entry tag, got %q instead of %q", tag, expected[i])
					}
				}
			},
		},
		{
			name: "ParseFeedWithIconURL",
			url:  "https://example.org/",
			data: `<?xml version="1.0" encoding="utf-8"?>
	<feed xmlns="http://www.w3.org/2005/Atom">
		<title>Example Feed</title>
		<link href="http://example.org/"/>
		<icon>http://example.org/icon.png</icon>
	</feed>`,
			check: func(t *testing.T, feed *model.Feed) {
				if feed.IconURL != "http://example.org/icon.png" {
					t.Errorf("Incorrect icon URL, got: %s", feed.IconURL)
				}
			},
		},
		{
			name: "ParseEntryWithIDAsURL",
			url:  "https://example.org/",
			data: `<?xml version="1.0" encoding="utf-8"?>
	<feed xmlns="http://www.w3.org/2005/Atom">
		<title>Example Feed</title>
		<link href="http://example.org/"/>
		<link href="http://example.org/atom" rel="self"/>
		<entry>
			<id>http://www.example.org/entries/1</id>
		</entry>
		<entry>
			<id>mailto:john.doe@example.org</id>
		</entry>
	</feed>`,
			check: func(t *testing.T, feed *model.Feed) {
				if feed.Entries[0].URL != "http://www.example.org/entries/1" {
					t.Errorf("Incorrect entry URL, got: %s", feed.Entries[0].URL)
				}

				if feed.Entries[1].URL != "http://example.org/" {
					t.Errorf("Incorrect entry URL, got: %s", feed.Entries[1].URL)
				}
			},
		},
		{
			name: "ParseFeedWithLanguage",
			url:  "http://example.org/feed.xml",
			data: `<?xml version="1.0" encoding="utf-8"?>
	<feed xmlns="http://www.w3.org/2005/Atom" xml:lang="fr-CA">
		<title>Example Feed</title>
		<link href="http://example.org/"/>
		<updated>2003-12-13T18:30:02Z</updated>
		<id>urn:uuid:60a76c80-d399-11d9-b93C-0003939e0af6</id>
	</feed>`,
			check: func(t *testing.T, feed *model.Feed) {
				if feed.Language != "fr-ca" {
					t.Errorf("Incorrect language, got: %q", feed.Language)
				}
			},
		},
		{
			name: "ParseFeedWithoutLanguage",
			url:  "http://example.org/feed.xml",
			data: `<?xml version="1.0" encoding="utf-8"?>
	<feed xmlns="http://www.w3.org/2005/Atom">
		<title>Example Feed</title>
		<link href="http://example.org/"/>
		<updated>2003-12-13T18:30:02Z</updated>
		<id>urn:uuid:60a76c80-d399-11d9-b93C-0003939e0af6</id>
	</feed>`,
			check: func(t *testing.T, feed *model.Feed) {
				if feed.Language != "" {
					t.Errorf("Expected empty language, got: %q", feed.Language)
				}
			},
		},
		{
			name: "ParseEntryWithLanguage",
			url:  "http://example.org/feed.xml",
			data: `<?xml version="1.0" encoding="utf-8"?>
	<feed xmlns="http://www.w3.org/2005/Atom" xml:lang="en">
		<title>Example Feed</title>
		<link href="http://example.org/"/>
		<updated>2003-12-13T18:30:02Z</updated>
		<id>urn:uuid:60a76c80-d399-11d9-b93C-0003939e0af6</id>
		<entry xml:lang="fr-CA">
			<title>Bonjour</title>
			<link href="http://example.org/2003/12/13/bonjour"/>
			<id>urn:uuid:1225c695-cfb8-4ebb-aaaa-80da344efa6a</id>
			<updated>2003-12-13T18:30:02Z</updated>
		</entry>
	</feed>`,
			check: func(t *testing.T, feed *model.Feed) {
				if len(feed.Entries) != 1 {
					t.Fatalf("Expected 1 entry, got: %d", len(feed.Entries))
				}

				// sss: gofeed does not expose an entry's own xml:lang; entries inherit the feed language.
				if feed.Entries[0].Language != "en" {
					t.Errorf("Incorrect entry language, got: %q", feed.Entries[0].Language)
				}
			},
		},
		{
			name: "ParseEntryWithoutLanguageInheritsFeedLanguage",
			url:  "http://example.org/feed.xml",
			data: `<?xml version="1.0" encoding="utf-8"?>
	<feed xmlns="http://www.w3.org/2005/Atom" xml:lang="en">
		<title>Example Feed</title>
		<link href="http://example.org/"/>
		<updated>2003-12-13T18:30:02Z</updated>
		<id>urn:uuid:60a76c80-d399-11d9-b93C-0003939e0af6</id>
		<entry>
			<title>Hello</title>
			<link href="http://example.org/2003/12/13/hello"/>
			<id>urn:uuid:1225c695-cfb8-4ebb-aaaa-80da344efa6a</id>
			<updated>2003-12-13T18:30:02Z</updated>
		</entry>
	</feed>`,
			check: func(t *testing.T, feed *model.Feed) {
				if len(feed.Entries) != 1 {
					t.Fatalf("Expected 1 entry, got: %d", len(feed.Entries))
				}

				if feed.Entries[0].Language != "en" {
					t.Errorf("Expected entry to inherit feed language, got: %q", feed.Entries[0].Language)
				}
			},
		},
		{
			name: "ParseEntryWithoutAnyLanguage",
			url:  "http://example.org/feed.xml",
			data: `<?xml version="1.0" encoding="utf-8"?>
	<feed xmlns="http://www.w3.org/2005/Atom">
		<title>Example Feed</title>
		<link href="http://example.org/"/>
		<updated>2003-12-13T18:30:02Z</updated>
		<id>urn:uuid:60a76c80-d399-11d9-b93C-0003939e0af6</id>
		<entry>
			<title>Hello</title>
			<link href="http://example.org/2003/12/13/hello"/>
			<id>urn:uuid:1225c695-cfb8-4ebb-aaaa-80da344efa6a</id>
			<updated>2003-12-13T18:30:02Z</updated>
		</entry>
	</feed>`,
			check: func(t *testing.T, feed *model.Feed) {
				if len(feed.Entries) != 1 {
					t.Fatalf("Expected 1 entry, got: %d", len(feed.Entries))
				}

				if feed.Entries[0].Language != "" {
					t.Errorf("Expected empty entry language, got: %q", feed.Entries[0].Language)
				}
			},
		},
		{
			name: "ParseFeedWithForeignLangAttribute",
			url:  "http://example.org/feed.xml",
			data: `<?xml version="1.0" encoding="utf-8"?>
	<feed xmlns="http://www.w3.org/2005/Atom" xmlns:foo="http://example.org/ns" xml:lang="fr-CA" foo:lang="zz">
		<title>Example Feed</title>
		<link href="http://example.org/"/>
		<updated>2003-12-13T18:30:02Z</updated>
		<id>urn:uuid:60a76c80-d399-11d9-b93C-0003939e0af6</id>
		<entry xml:lang="fr-CA" foo:lang="zz">
			<title>Bonjour</title>
			<link href="http://example.org/2003/12/13/bonjour"/>
			<id>urn:uuid:1225c695-cfb8-4ebb-aaaa-80da344efa6a</id>
			<updated>2003-12-13T18:30:02Z</updated>
		</entry>
	</feed>`,
			check: func(t *testing.T, feed *model.Feed) {
				if feed.Language != "fr-ca" {
					t.Errorf("Incorrect language, got: %q", feed.Language)
				}

				if len(feed.Entries) != 1 {
					t.Fatalf("Expected 1 entry, got: %d", len(feed.Entries))
				}

				if feed.Entries[0].Language != "fr-ca" {
					t.Errorf("Incorrect entry language, got: %q", feed.Entries[0].Language)
				}
			},
		},
		{
			name: "ParseFeedWithUnqualifiedLangAttribute",
			url:  "http://example.org/feed.xml",
			data: `<?xml version="1.0" encoding="utf-8"?>
	<feed xmlns="http://www.w3.org/2005/Atom" lang="de">
		<title>Example Feed</title>
		<link href="http://example.org/"/>
		<updated>2003-12-13T18:30:02Z</updated>
		<id>urn:uuid:60a76c80-d399-11d9-b93C-0003939e0af6</id>
	</feed>`,
			check: func(t *testing.T, feed *model.Feed) {
				// sss: gofeed reads lang attributes regardless of namespace.
				if feed.Language != "de" {
					t.Errorf("Expected empty language for unqualified lang attribute, got: %q", feed.Language)
				}
			},
		},
	})
}
