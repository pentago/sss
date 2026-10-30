// Copyright The Miniflux Authors. Modified by The sss Authors.
// Derived from Miniflux (https://github.com/miniflux/v2).
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	"sss/internal/model"
)

func TestJSON(t *testing.T) {
	runParseTests(t, []parseTest{
		{
			name: "ParseJsonFeedVersion1",
			url:  "https://example.org/feed.json",
			data: `{
		"version": "https://jsonfeed.org/version/1",
		"title": "My Example Feed",
		"icon": "https://micro.blog/jsonfeed/avatar.jpg",
		"favicon": "https://micro.blog/jsonfeed/favicon.png",
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

				if feed.Description != "" {
					t.Errorf("Incorrect description, got: %s", feed.Description)
				}

				if feed.FeedURL != "https://example.org/feed.json" {
					t.Errorf("Incorrect feed URL, got: %s", feed.FeedURL)
				}

				if feed.SiteURL != "https://example.org/" {
					t.Errorf("Incorrect site URL, got: %s", feed.SiteURL)
				}

				if feed.IconURL != "https://micro.blog/jsonfeed/favicon.png" {
					t.Errorf("Incorrect icon URL, got: %s", feed.IconURL)
				}

				if len(feed.Entries) != 2 {
					t.Errorf("Incorrect number of entries, got: %d", len(feed.Entries))
				}

				if feed.Entries[0].Hash != "d4735e3a265e16eee03f59718b9b5d03019c07d8b6c51f90da3a666eec13ab35" {
					t.Errorf("Incorrect entry hash, got: %s", feed.Entries[0].Hash)
				}

				if feed.Entries[0].URL != "https://example.org/second-item" {
					t.Errorf("Incorrect entry URL, got: %s", feed.Entries[0].URL)
				}

				if feed.Entries[0].Title != "This is a second item." {
					t.Errorf(`Incorrect entry title, got: "%s"`, feed.Entries[0].Title)
				}

				if feed.Entries[0].Content != "This is a second item." {
					t.Errorf("Incorrect entry content, got: %s", feed.Entries[0].Content)
				}

				if feed.Entries[1].Hash != "6b86b273ff34fce19d6b804eff5a3f5747ada4eaa22f1d49c01e52ddb7875b4b" {
					t.Errorf("Incorrect entry hash, got: %s", feed.Entries[1].Hash)
				}

				if feed.Entries[1].URL != "https://example.org/initial-post" {
					t.Errorf("Incorrect entry URL, got: %s", feed.Entries[1].URL)
				}

				if feed.Entries[1].Title != "Hello, world!" {
					t.Errorf(`Incorrect entry title, got: "%s"`, feed.Entries[1].Title)
				}

				if feed.Entries[1].Content != "<p>Hello, world!</p>" {
					t.Errorf("Incorrect entry content, got: %s", feed.Entries[1].Content)
				}
			},
		},
		{
			name: "ParseFeedWithDescription",
			url:  "https://example.org/feed.json",
			data: `{
		"version": "https://jsonfeed.org/version/1",
		"title": "My Example Feed",
		"description": "This is a sample feed description.",
		"home_page_url": "https://example.org/",
		"feed_url": "https://example.org/feed.json",
		"items": []
	}`,
			check: func(t *testing.T, feed *model.Feed) {
				if feed.Description != "This is a sample feed description." {
					t.Errorf("Incorrect description, got: %s", feed.Description)
				}
			},
		},
		{
			name: "ParsePodcast",
			url:  "http://therecord.co/feed.json",
			data: `{
		"version": "https://jsonfeed.org/version/1",
		"user_comment": "This is a podcast feed. You can add this feed to your podcast client using the following URL: http://therecord.co/feed.json",
		"title": "The Record",
		"home_page_url": "http://therecord.co/",
		"feed_url": "http://therecord.co/feed.json",
		"items": [
			{
				"id": "http://therecord.co/chris-parrish",
				"title": "Special #1 - Chris Parrish",
				"url": "http://therecord.co/chris-parrish",
				"content_text": "Chris has worked at Adobe and as a founder of Rogue Sheep, which won an Apple Design Award for Postage. Chris’s new company is Aged & Distilled with Guy English — which shipped Napkin, a Mac app for visual collaboration. Chris is also the co-host of The Record. He lives on Bainbridge Island, a quick ferry ride from Seattle.",
				"content_html": "Chris has worked at <a href=\"http://adobe.com/\">Adobe</a> and as a founder of Rogue Sheep, which won an Apple Design Award for Postage. Chris’s new company is Aged & Distilled with Guy English — which shipped <a href=\"http://aged-and-distilled.com/napkin/\">Napkin</a>, a Mac app for visual collaboration. Chris is also the co-host of The Record. He lives on <a href=\"http://www.ci.bainbridge-isl.wa.us/\">Bainbridge Island</a>, a quick ferry ride from Seattle.",
				"summary": "Brent interviews Chris Parrish, co-host of The Record and one-half of Aged & Distilled.",
				"date_published": "2014-05-09T14:04:00-07:00",
				"attachments": [
					{
						"url": "http://therecord.co/downloads/The-Record-sp1e1-ChrisParrish.m4a",
						"mime_type": "audio/x-m4a",
						"size_in_bytes": 89970236,
						"duration_in_seconds": 6629
					}
				]
			}
		]
	}`,
			check: func(t *testing.T, feed *model.Feed) {
				if feed.Title != "The Record" {
					t.Errorf("Incorrect title, got: %s", feed.Title)
				}

				if feed.FeedURL != "http://therecord.co/feed.json" {
					t.Errorf("Incorrect feed URL, got: %s", feed.FeedURL)
				}

				if feed.SiteURL != "http://therecord.co/" {
					t.Errorf("Incorrect site URL, got: %s", feed.SiteURL)
				}

				if len(feed.Entries) != 1 {
					t.Fatalf("Incorrect number of entries, got: %d", len(feed.Entries))
				}

				if feed.Entries[0].Hash != "6b678e57962a1b001e4e873756563cdc08bbd06ca561e764e0baa9a382485797" {
					t.Errorf("Incorrect entry hash, got: %s", feed.Entries[0].Hash)
				}

				if feed.Entries[0].URL != "http://therecord.co/chris-parrish" {
					t.Errorf("Incorrect entry URL, got: %s", feed.Entries[0].URL)
				}

				if feed.Entries[0].Title != "Special #1 - Chris Parrish" {
					t.Errorf(`Incorrect entry title, got: "%s"`, feed.Entries[0].Title)
				}

				if feed.Entries[0].Content != `Chris has worked at <a href="http://adobe.com/">Adobe</a> and as a founder of Rogue Sheep, which won an Apple Design Award for Postage. Chris’s new company is Aged & Distilled with Guy English — which shipped <a href="http://aged-and-distilled.com/napkin/">Napkin</a>, a Mac app for visual collaboration. Chris is also the co-host of The Record. He lives on <a href="http://www.ci.bainbridge-isl.wa.us/">Bainbridge Island</a>, a quick ferry ride from Seattle.` {
					t.Errorf(`Incorrect entry content, got: "%s"`, feed.Entries[0].Content)
				}

				location, _ := time.LoadLocation("America/Vancouver")
				if !feed.Entries[0].Date.Equal(time.Date(2014, time.May, 9, 14, 4, 0, 0, location)) {
					t.Errorf("Incorrect entry date, got: %v", feed.Entries[0].Date)
				}

				if len(feed.Entries[0].Enclosures) != 1 {
					t.Fatalf("Incorrect number of enclosures, got: %d", len(feed.Entries[0].Enclosures))
				}

				if feed.Entries[0].Enclosures[0].URL != "http://therecord.co/downloads/The-Record-sp1e1-ChrisParrish.m4a" {
					t.Errorf("Incorrect enclosure URL, got: %s", feed.Entries[0].Enclosures[0].URL)
				}

				if feed.Entries[0].Enclosures[0].MimeType != "audio/x-m4a" {
					t.Errorf("Incorrect enclosure type, got: %s", feed.Entries[0].Enclosures[0].MimeType)
				}

				if feed.Entries[0].Enclosures[0].Size != 89970236 {
					t.Errorf("Incorrect enclosure length, got: %d", feed.Entries[0].Enclosures[0].Size)
				}
			},
		},
		{
			name: "ParseFeedWithFeedURLWithTrailingSpace",
			url:  "https://example.org/feed.json",
			data: `{
		"version": "https://jsonfeed.org/version/1",
		"title": "My Example Feed",
		"home_page_url": "https://example.org/",
		"feed_url": "https://example.org/feed.json ",
		"items": []
	}`,
			check: func(t *testing.T, feed *model.Feed) {
				if feed.FeedURL != "https://example.org/feed.json" {
					t.Errorf("Incorrect feed URL, got: %s", feed.FeedURL)
				}
			},
		},
		{
			name: "ParseFeedWithRelativeFeedURL",
			url:  "https://example.org/feed.json",
			data: `{
		"version": "https://jsonfeed.org/version/1",
		"title": "My Example Feed",
		"home_page_url": "https://example.org/",
		"feed_url": "/feed.json",
		"items": []
	}`,
			check: func(t *testing.T, feed *model.Feed) {
				if feed.FeedURL != "https://example.org/feed.json" {
					t.Errorf("Incorrect feed URL, got: %s", feed.FeedURL)
				}
			},
		},
		{
			name: "ParseFeedSiteURLWithTrailingSpace",
			url:  "https://example.org/feed.json",
			data: `{
		"version": "https://jsonfeed.org/version/1",
		"title": "My Example Feed",
		"home_page_url": "https://example.org/ ",
		"feed_url": "https://example.org/feed.json",
		"items": []
	}`,
			check: func(t *testing.T, feed *model.Feed) {
				if feed.SiteURL != "https://example.org/" {
					t.Errorf("Incorrect site URL, got: %s", feed.SiteURL)
				}
			},
		},
		{
			name: "ParseFeedWithRelativeSiteURL",
			url:  "https://example.org/feed.json",
			data: `{
		"version": "https://jsonfeed.org/version/1",
		"title": "My Example Feed",
		"home_page_url": "/home ",
		"feed_url": "https://example.org/feed.json",
		"items": []
	}`,
			check: func(t *testing.T, feed *model.Feed) {
				if feed.SiteURL != "https://example.org/home" {
					t.Errorf("Incorrect site URL, got: %s", feed.SiteURL)
				}
			},
		},
		{
			name: "ParseFeedWithoutTitle",
			url:  "https://example.org/feed.json",
			data: `{
		"version": "https://jsonfeed.org/version/1",
		"home_page_url": "https://example.org/",
		"feed_url": "https://example.org/feed.json",
		"items": [
			{
				"id": "2347259",
				"url": "https://example.org/2347259",
				"content_text": "Cats are neat. \n\nhttps://example.org/cats",
				"date_published": "2016-02-09T14:22:00-07:00"
			}
		]
	}`,
			check: func(t *testing.T, feed *model.Feed) {
				if feed.Title != "https://example.org/" {
					t.Errorf("Incorrect title, got: %s", feed.Title)
				}
			},
		},
		{
			name: "ParseFeedWithoutHomePage",
			url:  "https://example.org/feed.json",
			data: `{
		"version": "https://jsonfeed.org/version/1",
		"feed_url": "https://example.org/feed.json",
		"title": "Some test",
		"items": [
			{
				"id": "2347259",
				"url": "https://example.org/2347259",
				"content_text": "Cats are neat. \n\nhttps://example.org/cats",
				"date_published": "2016-02-09T14:22:00-07:00"
			}
		]
	}`,
			check: func(t *testing.T, feed *model.Feed) {
				if feed.SiteURL != "https://example.org/feed.json" {
					t.Errorf("Incorrect title, got: %s", feed.Title)
				}
			},
		},
		{
			name: "ParseFeedWithoutFeedURL",
			url:  "https://example.org/feed.json",
			data: `{
		"version": "https://jsonfeed.org/version/1",
		"title": "Some test",
		"items": [
			{
				"id": "2347259",
				"url": "https://example.org/2347259",
				"content_text": "Cats are neat. \n\nhttps://example.org/cats",
				"date_published": "2016-02-09T14:22:00-07:00"
			}
		]
	}`,
			check: func(t *testing.T, feed *model.Feed) {
				if feed.SiteURL != "https://example.org/feed.json" {
					t.Errorf("Incorrect title, got: %s", feed.Title)
				}
			},
		},
		{
			name: "ParseItemWithoutAttachmentURL",
			url:  "http://therecord.co/feed.json",
			data: `{
		"version": "https://jsonfeed.org/version/1",
		"user_comment": "This is a podcast feed. You can add this feed to your podcast client using the following URL: http://therecord.co/feed.json",
		"title": "The Record",
		"home_page_url": "http://therecord.co/",
		"feed_url": "http://therecord.co/feed.json",
		"items": [
			{
				"id": "http://therecord.co/chris-parrish",
				"title": "Special #1 - Chris Parrish",
				"url": "http://therecord.co/chris-parrish",
				"content_text": "Chris has worked at Adobe and as a founder of Rogue Sheep, which won an Apple Design Award for Postage. Chris’s new company is Aged & Distilled with Guy English — which shipped Napkin, a Mac app for visual collaboration. Chris is also the co-host of The Record. He lives on Bainbridge Island, a quick ferry ride from Seattle.",
				"date_published": "2014-05-09T14:04:00-07:00",
				"attachments": [
					{
						"url": "",
						"mime_type": "audio/x-m4a",
						"size_in_bytes": 0
					}
				]
			}
		]
	}`,
			check: func(t *testing.T, feed *model.Feed) {
				if len(feed.Entries) != 1 {
					t.Fatalf("Incorrect number of entries, got: %d", len(feed.Entries))
				}

				if len(feed.Entries[0].Enclosures) != 0 {
					t.Errorf("Incorrect number of enclosures, got: %d", len(feed.Entries[0].Enclosures))
				}
			},
		},
		{
			name: "ParseItemWithRelativeURL",
			url:  "https://example.org/feed.json",
			data: `{
		"version": "https://jsonfeed.org/version/1",
		"title": "Example",
		"home_page_url": "https://example.org/",
		"feed_url": "https://example.org/feed.json",
		"items": [
			{
				"id": "2347259",
				"url": "something.html",
				"date_published": "2016-02-09T14:22:00-07:00"
			}
		]
	}`,
			check: func(t *testing.T, feed *model.Feed) {
				if feed.Entries[0].URL != "https://example.org/something.html" {
					t.Errorf("Incorrect entry URL, got: %s", feed.Entries[0].URL)
				}
			},
		},
		{
			name: "ParseItemWithExternalURLAndNoURL",
			url:  "https://example.org/feed.json",
			data: `{
		"version": "https://jsonfeed.org/version/1",
		"title": "Example",
		"home_page_url": "https://example.org/",
		"feed_url": "https://example.org/feed.json",
		"items": [
			{
				"id": "1234259",
				"external_url": "some_page.html"
			}
		]
	}`,
			check: func(t *testing.T, feed *model.Feed) {
				if len(feed.Entries) != 1 {
					t.Fatalf("Incorrect number of entries, got: %d", len(feed.Entries))
				}

				if feed.Entries[0].URL != "https://example.org/some_page.html" {
					t.Errorf("Incorrect entry URL, got: %s", feed.Entries[0].URL)
				}
			},
		},
		{
			name: "ParseItemWithExternalURLAndURL",
			url:  "https://example.org/feed.json",
			data: `{
		"version": "https://jsonfeed.org/version/1",
		"title": "Example",
		"home_page_url": "https://example.org/",
		"feed_url": "https://example.org/feed.json",
		"items": [
			{
				"id": "1234259",
				"url": "https://example.org/article",
				"external_url": "https://example.org/another-article"
			}
		]
	}`,
			check: func(t *testing.T, feed *model.Feed) {
				if len(feed.Entries) != 1 {
					t.Fatalf("Incorrect number of entries, got: %d", len(feed.Entries))
				}

				if feed.Entries[0].URL != "https://example.org/article" {
					t.Errorf("Incorrect entry URL, got: %s", feed.Entries[0].URL)
				}
			},
		},
		{
			name: "ParseItemWithLegacyAuthorField",
			url:  "https://example.org/feed.json",
			data: `{
		"version": "https://jsonfeed.org/version/1",
		"user_comment": "This is a microblog feed. You can add this to your feed reader using the following URL: https://example.org/feed.json",
		"title": "Brent Simmons’s Microblog",
		"home_page_url": "https://example.org/",
		"feed_url": "https://example.org/feed.json",
		"author": {
			"name": "Brent Simmons",
			"url": "http://example.org/",
			"avatar": "https://example.org/avatar.png"
		},
		"items": [
			{
				"id": "2347259",
				"url": "https://example.org/2347259",
				"content_text": "Cats are neat. \n\nhttps://example.org/cats",
				"date_published": "2016-02-09T14:22:00-07:00"
			}
		]
	}`,
			check: func(t *testing.T, feed *model.Feed) {
				if len(feed.Entries) != 1 {
					t.Errorf("Incorrect number of entries, got: %d", len(feed.Entries))
				}

				if feed.Entries[0].Author != "Brent Simmons" {
					t.Errorf("Incorrect entry author, got: %s", feed.Entries[0].Author)
				}
			},
		},
		{
			name: "ParseItemWithMultipleAuthorFields",
			url:  "https://example.org/feed.json",
			data: `{
		"version": "https://jsonfeed.org/version/1.1",
		"user_comment": "This is a microblog feed. You can add this to your feed reader using the following URL: https://example.org/feed.json",
		"title": "Brent Simmons’s Microblog",
		"home_page_url": "https://example.org/",
		"feed_url": "https://example.org/feed.json",
		"author": {
			"name": "Deprecated Author Field",
			"url": "http://example.org/",
			"avatar": "https://example.org/avatar.png"
		},
		"authors": [
			{
				"name": "Brent Simmons",
				"url": "http://example.org/",
				"avatar": "https://example.org/avatar.png"
			}
		],
		"items": [
			{
				"id": "2347259",
				"url": "https://example.org/2347259",
				"content_text": "Cats are neat. \n\nhttps://example.org/cats",
				"date_published": "2016-02-09T14:22:00-07:00"
			}
		]
	}`,
			check: func(t *testing.T, feed *model.Feed) {
				if len(feed.Entries) != 1 {
					t.Errorf("Incorrect number of entries, got: %d", len(feed.Entries))
				}

				if feed.Entries[0].Author != "Brent Simmons, Deprecated Author Field" {
					t.Errorf("Incorrect entry author, got: %s", feed.Entries[0].Author)
				}
			},
		},
		{
			name: "ParseItemWithMultipleDuplicateAuthors",
			url:  "https://example.org/feed.json",
			data: `{
		"version": "https://jsonfeed.org/version/1.1",
		"title": "Example",
		"home_page_url": "https://example.org/",
		"feed_url": "https://example.org/feed.json",
		"items": [
			{
				"id": "2347259",
				"url": "https://example.org/2347259",
				"content_text": "Cats are neat. \n\nhttps://example.org/cats",
				"date_published": "2016-02-09T14:22:00-07:00",
				"authors": [
					{
						"name": "Author B",
						"url": "http://example.org/",
						"avatar": "https://example.org/avatar.png"
					},
					{
						"name": "Author A",
						"url": "http://example.org/",
						"avatar": "https://example.org/avatar.png"
					},
					{
						"name": "Author B",
						"url": "http://example.org/",
						"avatar": "https://example.org/avatar.png"
					}
				]
			}
		]
	}`,
			check: func(t *testing.T, feed *model.Feed) {
				if len(feed.Entries) != 1 {
					t.Errorf("Incorrect number of entries, got: %d", len(feed.Entries))
				}

				if feed.Entries[0].Author != "Author A, Author B" {
					t.Errorf("Incorrect entry author, got: %s", feed.Entries[0].Author)
				}
			},
		},
		{
			name: "ParseItemWithAuthorsObject",
			url:  "https://example.org/feed.json",
			data: `{
		"version": "https://jsonfeed.org/version/1.1",
		"title": "Example",
		"home_page_url": "https://example.org/",
		"feed_url": "https://example.org/feed.json",
		"items": [
			{
				"id": "1",
				"title": "Example Item",
				"url": "https://example.org/item",
				"date_published": "2020-01-02T03:04:05Z",
				"authors": {
					"name": "Example Author"
				}
			}
		]
	}`,
			check: func(t *testing.T, feed *model.Feed) {
				if len(feed.Entries) != 1 {
					t.Fatalf("Incorrect number of entries, got: %d", len(feed.Entries))
				}

				if feed.Entries[0].Author != "Example Author" {
					t.Errorf("Incorrect entry author, got: %s", feed.Entries[0].Author)
				}
			},
		},
		{
			name: "ParseItemWithInvalidDate",
			url:  "https://example.org/feed.json",
			data: `{
		"version": "https://jsonfeed.org/version/1",
		"title": "My Example Feed",
		"home_page_url": "https://example.org/",
		"feed_url": "https://example.org/feed.json",
		"items": [
			{
				"id": "2347259",
				"url": "https://example.org/2347259",
				"content_text": "Cats are neat. \n\nhttps://example.org/cats",
				"date_published": "Tomorrow"
			}
		]
	}`,
			check: func(t *testing.T, feed *model.Feed) {
				if len(feed.Entries) != 1 {
					t.Errorf("Incorrect number of entries, got: %d", len(feed.Entries))
				}

				duration := time.Since(feed.Entries[0].Date)
				if duration.Seconds() > 1 {
					t.Errorf("Incorrect entry date, got: %v", feed.Entries[0].Date)
				}
			},
		},
		{
			name: "ParseItemWithMissingTitleUsesSummaryFallback",
			url:  "https://example.org/feed.json",
			data: `{
		"version": "https://jsonfeed.org/version/1",
		"title": "My Example Feed",
		"home_page_url": "https://example.org/",
		"feed_url": "https://example.org/feed.json",
		"items": [
			{
				"summary": "Summary title",
				"content_text": "Content text title",
				"content_html": "<p>HTML title</p>"
			}
		]
	}`,
			check: func(t *testing.T, feed *model.Feed) {
				if len(feed.Entries) != 1 {
					t.Errorf("Incorrect number of entries, got: %d", len(feed.Entries))
				}

				if feed.Entries[0].Title != "Summary title" {
					t.Errorf("Incorrect entry title, got: %s", feed.Entries[0].Title)
				}
			},
		},
		{
			name: "ParseItemWithMissingTitleUsesContentTextFallback",
			url:  "https://example.org/feed.json",
			data: `{
		"version": "https://jsonfeed.org/version/1",
		"title": "My Example Feed",
		"home_page_url": "https://example.org/",
		"feed_url": "https://example.org/feed.json",
		"items": [
			{
				"summary": " ",
				"content_text": "Content text title",
				"content_html": "<p>HTML title</p>"
			}
		]
	}`,
			check: func(t *testing.T, feed *model.Feed) {
				if len(feed.Entries) != 1 {
					t.Errorf("Incorrect number of entries, got: %d", len(feed.Entries))
				}

				if feed.Entries[0].Title != "Content text title" {
					t.Errorf("Incorrect entry title, got: %s", feed.Entries[0].Title)
				}
			},
		},
		{
			name: "ParseItemWithMissingTitleUsesHTMLFallback",
			url:  "https://example.org/feed.json",
			data: `{
		"version": "https://jsonfeed.org/version/1",
		"title": "My Example Feed",
		"home_page_url": "https://example.org/",
		"feed_url": "https://example.org/feed.json",
		"items": [
			{
				"summary": "",
				"content_text": "",
				"content_html": "<p>HTML title</p>"
			}
		]
	}`,
			check: func(t *testing.T, feed *model.Feed) {
				if len(feed.Entries) != 1 {
					t.Errorf("Incorrect number of entries, got: %d", len(feed.Entries))
				}

				if feed.Entries[0].Title != "HTML title" {
					t.Errorf("Incorrect entry title, got: %s", feed.Entries[0].Title)
				}
			},
		},
		{
			name: "ParseItemWithMissingTitleUsesURLFallback",
			url:  "https://example.org/feed.json",
			data: `{
		"version": "https://jsonfeed.org/version/1",
		"title": "My Example Feed",
		"home_page_url": "https://example.org/",
		"feed_url": "https://example.org/feed.json",
		"items": [
			{
				"url": "https://example.org/item"
			}
		]
	}`,
			check: func(t *testing.T, feed *model.Feed) {
				if len(feed.Entries) != 1 {
					t.Errorf("Incorrect number of entries, got: %d", len(feed.Entries))
				}

				if feed.Entries[0].Title != "https://example.org/item" {
					t.Errorf("Incorrect entry title, got: %s", feed.Entries[0].Title)
				}
			},
		},
		{
			name: "ParseItemWithTooLongUnicodeTitle",
			url:  "https://example.org/feed.json",
			data: `{
		"version": "https://jsonfeed.org/version/1",
		"title": "My Example Feed",
		"home_page_url": "https://example.org/",
		"feed_url": "https://example.org/feed.json",
		"items": [
			{
				"title": "I’m riding my electric bike and came across this castle. It’s called “Schloss Richmond”. 🚴‍♂️"
			}
		]
	}`,
			check: func(t *testing.T, feed *model.Feed) {
				if len(feed.Entries) != 1 {
					t.Errorf("Incorrect number of entries, got: %d", len(feed.Entries))
				}

				if len(feed.Entries[0].Title) != 110 {
					t.Errorf("Incorrect entry title, got: %s", feed.Entries[0].Title)
				}

				if len([]rune(feed.Entries[0].Title)) != 93 {
					t.Errorf("Incorrect entry title, got: %s", feed.Entries[0].Title)
				}
			},
		},
		{
			name: "ParseItemTitleWithXMLTags",
			url:  "https://example.org/feed.json",
			data: `{
		"version": "https://jsonfeed.org/version/1",
		"title": "My Example Feed",
		"home_page_url": "https://example.org/",
		"feed_url": "https://example.org/feed.json",
		"items": [
			{
				"title": "</example>"
			}
		]
	}`,
			check: func(t *testing.T, feed *model.Feed) {
				if len(feed.Entries) != 1 {
					t.Errorf("Incorrect number of entries, got: %d", len(feed.Entries))
				}

				if feed.Entries[0].Title != "</example>" {
					t.Errorf("Incorrect entry title, got: %s", feed.Entries[0].Title)
				}
			},
		},
		{
			name: "ParseItemHashPrefersIDOverURL",
			url:  "https://example.org/feed.json",
			data: `{
		"version": "https://jsonfeed.org/version/1",
		"title": "Example",
		"home_page_url": "https://example.org/",
		"feed_url": "https://example.org/feed.json",
		"items": [
			{
				"id": "id-value",
				"url": "https://example.org/article"
			}
		]
	}`,
			check: func(t *testing.T, feed *model.Feed) {
				expected := sha256.Sum256([]byte("id-value"))
				if feed.Entries[0].Hash != hex.EncodeToString(expected[:]) {
					t.Errorf("Incorrect entry hash, got: %s", feed.Entries[0].Hash)
				}
			},
		},
		{
			name: "ParseItemHashUsesURLWhenNoID",
			url:  "https://example.org/feed.json",
			data: `{
		"version": "https://jsonfeed.org/version/1",
		"title": "Example",
		"home_page_url": "https://example.org/",
		"feed_url": "https://example.org/feed.json",
		"items": [
			{
				"url": "https://example.org/article"
			}
		]
	}`,
			check: func(t *testing.T, feed *model.Feed) {
				expected := sha256.Sum256([]byte("https://example.org/article"))
				if feed.Entries[0].Hash != hex.EncodeToString(expected[:]) {
					t.Errorf("Incorrect entry hash, got: %s", feed.Entries[0].Hash)
				}
			},
		},
		{
			name: "ParseItemHashUsesExternalURLFallback",
			url:  "https://example.org/feed.json",
			data: `{
		"version": "https://jsonfeed.org/version/1",
		"title": "Example",
		"home_page_url": "https://example.org/",
		"feed_url": "https://example.org/feed.json",
		"items": [
			{
				"external_url": "https://example.org/external"
			}
		]
	}`,
			check: func(t *testing.T, feed *model.Feed) {
				if len(feed.Entries) != 1 {
					t.Fatalf("Incorrect number of entries, got: %d", len(feed.Entries))
				}

				expected := sha256.Sum256([]byte("https://example.org/external"))
				if feed.Entries[0].Hash != hex.EncodeToString(expected[:]) {
					t.Errorf("Incorrect entry hash, got: %s", feed.Entries[0].Hash)
				}
			},
		},
		{
			name: "ParseItemHashFallsBackToContent",
			url:  "https://example.org/feed.json",
			data: `{
		"version": "https://jsonfeed.org/version/1",
		"title": "Example",
		"home_page_url": "https://example.org/",
		"feed_url": "https://example.org/feed.json",
		"items": [
			{
				"content_text": "Text",
				"content_html": "<p>HTML</p>",
				"summary": "Summary"
			}
		]
	}`,
			check: func(t *testing.T, feed *model.Feed) {
				expected := sha256.Sum256([]byte("Text<p>HTML</p>Summary"))
				if feed.Entries[0].Hash != hex.EncodeToString(expected[:]) {
					t.Errorf("Incorrect entry hash, got: %s", feed.Entries[0].Hash)
				}
			},
		},
		{
			name: "ParseItemTags",
			url:  "https://example.org/feed.json",
			data: `{
		"version": "https://jsonfeed.org/version/1",
		"user_comment": "This is a microblog feed. You can add this to your feed reader using the following URL: https://example.org/feed.json",
		"title": "Brent Simmons’s Microblog",
		"home_page_url": "https://example.org/",
		"feed_url": "https://example.org/feed.json",
		"author": {
			"name": "Brent Simmons",
			"url": "http://example.org/",
			"avatar": "https://example.org/avatar.png"
		},
		"items": [
			{
				"id": "2347259",
				"url": "https://example.org/2347259",
				"content_text": "Cats are neat. \n\nhttps://example.org/cats",
				"date_published": "2016-02-09T14:22:00-07:00",
				"tags": [
					" tag 1",
					" ",
					"tag 2",
					"tag 2",
					"aaa"
				]
			}
		]
	}`,
			check: func(t *testing.T, feed *model.Feed) {
				if len(feed.Entries) != 1 {
					t.Errorf("Incorrect number of entries, got: %d", len(feed.Entries))
				}

				if len(feed.Entries[0].Tags) != 3 {
					t.Errorf("Incorrect number of Tags, got: %d", len(feed.Entries[0].Tags))
				}

				expected := []string{"aaa", "tag 1", "tag 2"}
				for i, tag := range feed.Entries[0].Tags {
					if tag != expected[i] {
						t.Errorf("Incorrect entry tag, got %q instead of %q", tag, expected[i])
					}
				}
			},
		},
		{
			name: "ParseFeedFavicon",
			url:  "https://example.org/feed.json",
			data: `{
		"version": "https://jsonfeed.org/version/1",
		"title": "My Example Feed",
		"favicon": "https://example.org/jsonfeed/favicon.png",
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
				if feed.IconURL != "https://example.org/jsonfeed/favicon.png" {
					t.Errorf("Incorrect icon URL, got: %s", feed.IconURL)
				}
			},
		},
		{
			name: "ParseFeedIcon",
			url:  "https://example.org/feed.json",
			data: `{
		"version": "https://jsonfeed.org/version/1",
		"title": "My Example Feed",
		"icon": "https://example.org/jsonfeed/icon.png",
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
				if feed.IconURL != "https://example.org/jsonfeed/icon.png" {
					t.Errorf("Incorrect icon URL, got: %s", feed.IconURL)
				}
			},
		},
		{
			name: "ParseFeedWithRelativeAttachmentURL",
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
				"url": "https://example.org/second-item",
				"attachments": [
					{
						"url": "   /attachment.mp3  ",
						"mime_type": "audio/mpeg",
						"size_in_bytes": 123456
					}
				]
			}
		]
	}`,
			check: func(t *testing.T, feed *model.Feed) {
				if len(feed.Entries[0].Enclosures) != 1 {
					t.Fatalf("Incorrect number of enclosures, got: %d", len(feed.Entries[0].Enclosures))
				}

				if feed.Entries[0].Enclosures[0].URL != "https://example.org/attachment.mp3" {
					t.Errorf("Incorrect enclosure URL, got: %q", feed.Entries[0].Enclosures[0].URL)
				}
			},
		},
		{
			name:    "ParseInvalidJSON",
			url:     "https://example.org/feed.json",
			data:    `garbage`,
			wantErr: true,
		},
		{
			name: "ParseFeedWithUTF8BOM",
			url:  "https://example.org/feed.json",
			data: "\xEF\xBB\xBF" + `{
		"version": "https://jsonfeed.org/version/1.1",
		"title": "My Example Feed",
		"home_page_url": "https://example.org/",
		"items": []
	}`,
			check: func(t *testing.T, feed *model.Feed) {
				if feed.Title != "My Example Feed" {
					t.Errorf("Incorrect title, got: %s", feed.Title)
				}
			},
		},
		{
			name: "ParseNullJSONFeed",
			url:  "https://example.org/feed.json",
			data: `null`,
			// sss: upstream called json.Parse directly; through ParseFeed (here and upstream) "null" is not a detected feed.
			wantErr: true,
		},
		{
			name: "ParseFeedWithLanguage",
			url:  "https://example.org/feed.json",
			data: `{
		"version": "https://jsonfeed.org/version/1.1",
		"title": "Example",
		"home_page_url": "https://example.org/",
		"feed_url": "https://example.org/feed.json",
		"language": "en-US",
		"items": []
	}`,
			check: func(t *testing.T, feed *model.Feed) {
				if feed.Language != "en-us" {
					t.Errorf("Incorrect language, got: %q", feed.Language)
				}
			},
		},
		{
			name: "ParseFeedWithoutLanguage",
			url:  "https://example.org/feed.json",
			data: `{
		"version": "https://jsonfeed.org/version/1.1",
		"title": "Example",
		"home_page_url": "https://example.org/",
		"feed_url": "https://example.org/feed.json",
		"items": []
	}`,
			check: func(t *testing.T, feed *model.Feed) {
				if feed.Language != "" {
					t.Errorf("Expected empty language, got: %q", feed.Language)
				}
			},
		},
		{
			name: "ParseItemWithLanguage",
			url:  "https://example.org/feed.json",
			data: `{
		"version": "https://jsonfeed.org/version/1.1",
		"title": "Example",
		"home_page_url": "https://example.org/",
		"feed_url": "https://example.org/feed.json",
		"language": "en-US",
		"items": [
			{
				"id": "1",
				"url": "https://example.org/item",
				"content_text": "Bonjour",
				"language": "fr-FR"
			}
		]
	}`,
			check: func(t *testing.T, feed *model.Feed) {
				if len(feed.Entries) != 1 {
					t.Fatalf("Expected 1 entry, got: %d", len(feed.Entries))
				}

				if feed.Entries[0].Language != "fr-fr" {
					t.Errorf("Incorrect entry language, got: %q", feed.Entries[0].Language)
				}
			},
		},
		{
			name: "ParseItemWithoutLanguageInheritsFeedLanguage",
			url:  "https://example.org/feed.json",
			data: `{
		"version": "https://jsonfeed.org/version/1.1",
		"title": "Example",
		"home_page_url": "https://example.org/",
		"feed_url": "https://example.org/feed.json",
		"language": "en-US",
		"items": [
			{
				"id": "1",
				"url": "https://example.org/item",
				"content_text": "Hello"
			}
		]
	}`,
			check: func(t *testing.T, feed *model.Feed) {
				if len(feed.Entries) != 1 {
					t.Fatalf("Expected 1 entry, got: %d", len(feed.Entries))
				}

				if feed.Entries[0].Language != "en-us" {
					t.Errorf("Expected entry to inherit feed language, got: %q", feed.Entries[0].Language)
				}
			},
		},
		{
			name: "ParseItemWithPlainTextContentContainingHTMLCharacters",
			url:  "https://example.org/feed.json",
			data: `{
		"version": "https://jsonfeed.org/version/1.1",
		"title": "Example",
		"home_page_url": "https://example.org/",
		"feed_url": "https://example.org/feed.json",
		"items": [
			{
				"id": "1",
				"url": "https://example.org/1",
				"content_text": "Use <div> literally: a & b < c"
			}
		]
	}`,
			check: func(t *testing.T, feed *model.Feed) {
				// content_text is plain text (JSON Feed 1.1), so it must be HTML-escaped
				// before being stored as HTML, otherwise the sanitizer drops the markup.
				want := "Use &lt;div&gt; literally: a &amp; b &lt; c"
				if feed.Entries[0].Content != want {
					t.Errorf("Incorrect entry content, got: %q, want: %q", feed.Entries[0].Content, want)
				}
			},
		},
	})
}
