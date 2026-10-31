// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"cmp"
	"html"
	"strconv"
	"strings"
	"time"

	"github.com/mmcdole/gofeed/atom"

	"sss/internal/crypto"
	"sss/internal/model"
	"sss/internal/reader/sanitizer"
	"sss/internal/urllib"
)

func buildAtom(baseURL string, af *atom.Feed) *model.Feed {
	baseURL = strings.TrimSpace(baseURL)
	feed := &model.Feed{
		FeedURL:     baseURL,
		SiteURL:     baseURL,
		Title:       strings.TrimSpace(af.Title),
		Description: strings.TrimSpace(af.Subtitle),
		Language:    normalizeLanguage(af.Language),
	}
	if u := resolveURL(baseURL, firstAtomLink(af.Links, "self")); u != "" {
		feed.FeedURL = u
	}
	if u := resolveURL(baseURL, originalAtomLink(af.Links)); u != "" {
		feed.SiteURL = u
	}
	if feed.Title == "" {
		feed.Title = feed.SiteURL
	}
	for _, icon := range []string{af.Icon, af.Logo} {
		if icon = resolveURL(feed.SiteURL, icon); icon != "" {
			feed.IconURL = icon
			break
		}
	}

	atom03 := af.Version == "0.3"
	feedAuthors := atomPersonNames(af.Authors)
	feedTags := atomCategoryNames(af.Categories)

	for _, ae := range af.Entries {
		entry := model.NewEntry()

		link := originalAtomLink(ae.Links)
		entry.URL = resolveURL(feed.SiteURL, link)
		if entry.URL == "" {
			entry.URL = feed.SiteURL
			if urllib.IsAbsoluteURL(ae.ID) {
				entry.URL = ae.ID
			}
		}

		if ae.Content != nil {
			entry.Content = strings.TrimSpace(ae.Content.Value)
			// Atom 0.3 content is plain text unless declared text/html.
			if atom03 && ae.Content.Type != "text/html" {
				entry.Content = html.EscapeString(entry.Content)
			}
		}
		if entry.Content == "" {
			entry.Content = strings.TrimSpace(ae.Summary)
			// ponytail: gofeed drops the summary type, so Atom 0.3 summaries are
			// always treated as text/plain (the 0.3 default); keep the type if it matters.
			if atom03 {
				entry.Content = html.EscapeString(entry.Content)
			}
		}
		if entry.Content == "" {
			entry.Content = firstMediaDescription(ae.Extensions)
		}

		// sss: gofeed drops the CDATA marker upstream uses to decide whether to
		// entity-decode a title, so titles are always decoded once more.
		entry.Title = html.UnescapeString(strings.TrimSpace(ae.Title))
		if entry.Title == "" {
			entry.Title = cmp.Or(sanitizer.TruncateHTML(entry.Content, 100), entry.URL)
		}

		// sss: gofeed does not expose an entry's own xml:lang, so entries inherit the feed language.
		entry.Language = feed.Language

		authors := atomPersonNames(ae.Authors)
		if len(authors) == 0 {
			authors = feedAuthors
		}
		entry.Author = strings.Join(authors, ", ")

		switch {
		case ae.PublishedParsed != nil:
			entry.Date = *ae.PublishedParsed
		case ae.UpdatedParsed != nil:
			entry.Date = *ae.UpdatedParsed
		default:
			entry.Date = time.Now()
		}

		entry.Tags = atomCategoryNames(ae.Categories)
		if len(entry.Tags) == 0 {
			entry.Tags = feedTags
		}

		for _, l := range ae.Links {
			if strings.EqualFold(l.Rel, "replies") && (strings.EqualFold(l.Type, "text/html") || strings.EqualFold(l.Type, "application/xhtml+xml")) {
				if u := strings.TrimSpace(l.Href); urllib.IsAbsoluteURL(u) {
					entry.CommentsURL = u
				}
				break
			}
		}

		for _, v := range []string{ae.ID, link} {
			if v != "" {
				entry.Hash = crypto.SHA256(v)
				break
			}
		}

		set := newEnclosureSet(feed.SiteURL)
		set.addMediaThumbnails(ae.Extensions)
		for _, l := range ae.Links {
			if strings.EqualFold(l.Rel, "enclosure") {
				size, _ := strconv.ParseInt(l.Length, 10, 64)
				set.add(l.Href, l.Type, size)
			}
		}
		set.addMediaContents(ae.Extensions)
		entry.Enclosures = set.list

		feed.Entries = append(feed.Entries, entry)
	}
	return feed
}

// originalAtomLink returns the alternate link, or the first untyped/HTML link without a rel.
func originalAtomLink(links []*atom.Link) string {
	for _, l := range links {
		if strings.EqualFold(l.Rel, "alternate") {
			return strings.TrimSpace(l.Href)
		}
		if l.Rel == "" && (l.Type == "" || l.Type == "text/html") {
			return strings.TrimSpace(l.Href)
		}
	}
	return ""
}

func firstAtomLink(links []*atom.Link, rel string) string {
	for _, l := range links {
		if strings.EqualFold(l.Rel, rel) {
			return strings.TrimSpace(l.Href)
		}
	}
	return ""
}

func atomPersonNames(persons []*atom.Person) []string {
	names := make([]string, 0, len(persons))
	for _, p := range persons {
		name := cmp.Or(strings.TrimSpace(p.Name), p.Email)
		names = append(names, name)
	}
	return sortedUnique(names...)
}

func atomCategoryNames(categories []*atom.Category) []string {
	names := make([]string, 0, len(categories))
	for _, c := range categories {
		name := cmp.Or(strings.TrimSpace(c.Label), c.Term)
		names = append(names, name)
	}
	return sortedUnique(names...)
}
