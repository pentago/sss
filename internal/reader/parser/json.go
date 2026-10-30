// Copyright The Miniflux Authors. Modified by The sss Authors.
// Derived from Miniflux (https://github.com/miniflux/v2).
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"html"
	"io"
	"strings"
	"time"

	"sss/internal/crypto"
	"sss/internal/model"
	"sss/internal/reader/sanitizer"
)

// JSON Feed is decoded with encoding/json rather than gofeed, which rejects
// a UTF-8 BOM and single-object "authors" values found in real feeds.
type jsonFeed struct {
	Title       string      `json:"title"`
	HomePageURL string      `json:"home_page_url"`
	FeedURL     string      `json:"feed_url"`
	Description string      `json:"description"`
	IconURL     string      `json:"icon"`
	FaviconURL  string      `json:"favicon"`
	Authors     jsonAuthors `json:"authors"`
	Author      jsonAuthor  `json:"author"`
	Language    string      `json:"language"`
	Items       []jsonItem  `json:"items"`
}

type jsonAuthor struct {
	Name string `json:"name"`
}

// jsonAuthors accepts both an array and a single object; anything else is ignored.
type jsonAuthors []jsonAuthor

func (a *jsonAuthors) UnmarshalJSON(data []byte) error {
	var authors []jsonAuthor
	if err := json.Unmarshal(data, &authors); err == nil {
		*a = authors
		return nil
	}
	var author jsonAuthor
	if err := json.Unmarshal(data, &author); err == nil {
		*a = jsonAuthors{author}
	}
	return nil
}

type jsonItem struct {
	ID            string           `json:"id"`
	URL           string           `json:"url"`
	ExternalURL   string           `json:"external_url"`
	Title         string           `json:"title"`
	ContentHTML   string           `json:"content_html"`
	ContentText   string           `json:"content_text"`
	Summary       string           `json:"summary"`
	DatePublished string           `json:"date_published"`
	DateModified  string           `json:"date_modified"`
	Language      string           `json:"language"`
	Authors       jsonAuthors      `json:"authors"`
	Author        jsonAuthor       `json:"author"`
	Tags          []string         `json:"tags"`
	Attachments   []jsonAttachment `json:"attachments"`
}

type jsonAttachment struct {
	URL      string `json:"url"`
	MimeType string `json:"mime_type"`
	Size     int64  `json:"size_in_bytes"`
}

func parseJSON(baseURL string, r io.Reader) (*model.Feed, error) {
	br := bufio.NewReader(r)
	if prefix, err := br.Peek(3); err == nil && bytes.Equal(prefix, []byte("\xEF\xBB\xBF")) {
		br.Discard(3)
	}
	var jf jsonFeed
	if err := json.NewDecoder(br).Decode(&jf); err != nil {
		return nil, errors.New("parser: json: " + err.Error())
	}

	baseURL = strings.TrimSpace(baseURL)
	feed := &model.Feed{
		Title:       strings.TrimSpace(jf.Title),
		FeedURL:     strings.TrimSpace(jf.FeedURL),
		SiteURL:     strings.TrimSpace(jf.HomePageURL),
		Description: strings.TrimSpace(jf.Description),
		Language:    normalizeLanguage(jf.Language),
	}
	if feed.FeedURL == "" {
		feed.FeedURL = baseURL
	}
	if feed.SiteURL == "" {
		feed.SiteURL = feed.FeedURL
	}
	feed.FeedURL = resolveURL(baseURL, feed.FeedURL)
	feed.SiteURL = resolveURL(baseURL, feed.SiteURL)
	if feed.Title == "" {
		feed.Title = feed.SiteURL
	}
	for _, icon := range []string{jf.FaviconURL, jf.IconURL} {
		if icon = resolveURL(feed.SiteURL, icon); icon != "" {
			feed.IconURL = icon
			break
		}
	}

	for _, item := range jf.Items {
		entry := model.NewEntry()

		entry.Language = normalizeLanguage(item.Language)
		if entry.Language == "" {
			entry.Language = feed.Language
		}

		for _, u := range []string{item.URL, item.ExternalURL} {
			if u = resolveURL(feed.SiteURL, u); u != "" {
				entry.URL = u
				break
			}
		}

		entry.Title = strings.TrimSpace(item.Title)
		for _, v := range []string{item.Summary, item.ContentText, item.ContentHTML} {
			if entry.Title != "" {
				break
			}
			entry.Title = sanitizer.TruncateHTML(v, 100)
		}
		if entry.Title == "" {
			entry.Title = entry.URL
		}

		if v := strings.TrimSpace(item.ContentHTML); v != "" {
			entry.Content = v
		} else if v := strings.TrimSpace(item.ContentText); v != "" {
			entry.Content = html.EscapeString(v)
		} else if v := strings.TrimSpace(item.Summary); v != "" {
			entry.Content = html.EscapeString(v)
		}

		for _, v := range []string{item.DatePublished, item.DateModified} {
			if t, err := time.Parse(time.RFC3339, strings.TrimSpace(v)); err == nil {
				entry.Date = t
				break
			}
		}
		if entry.Date.IsZero() {
			entry.Date = time.Now()
		}

		var names []string
		for _, authors := range []jsonAuthors{jf.Authors, item.Authors, {item.Author, jf.Author}} {
			for _, a := range authors {
				names = append(names, a.Name)
			}
		}
		entry.Author = strings.Join(sortedUnique(names...), ", ")

		set := newEnclosureSet(feed.SiteURL)
		for _, a := range item.Attachments {
			set.add(a.URL, a.MimeType, a.Size)
		}
		entry.Enclosures = set.list

		entry.Tags = sortedUnique(item.Tags...)

		for _, v := range []string{item.ID, item.URL, item.ExternalURL, item.ContentText + item.ContentHTML + item.Summary} {
			if v = strings.TrimSpace(v); v != "" {
				entry.Hash = crypto.SHA256(v)
				break
			}
		}

		feed.Entries = append(feed.Entries, entry)
	}
	return feed, nil
}
