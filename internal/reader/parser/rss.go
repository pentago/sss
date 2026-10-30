// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"html"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"
	ext "github.com/mmcdole/gofeed/extensions"
	"github.com/mmcdole/gofeed/rss"

	"sss/internal/crypto"
	"sss/internal/model"
	"sss/internal/reader/sanitizer"
	"sss/internal/urllib"
)

func buildRSS(baseURL string, src *gofeed.Feed, ch *rss.Feed) *model.Feed {
	feed := &model.Feed{
		Title:       html.UnescapeString(strings.TrimSpace(ch.Title)),
		FeedURL:     strings.TrimSpace(baseURL),
		SiteURL:     resolveURL(baseURL, ch.Link),
		Description: strings.TrimSpace(ch.Description),
		Language:    normalizeLanguage(ch.Language),
	}
	if feed.SiteURL == "" {
		feed.SiteURL = feed.FeedURL
	}
	if feed.Language == "" && ch.DublinCoreExt != nil && len(ch.DublinCoreExt.Language) > 0 {
		feed.Language = normalizeLanguage(ch.DublinCoreExt.Language[0])
	}
	for _, l := range ch.Extensions["atom"]["link"] {
		if l.Attrs["rel"] == "self" && strings.TrimSpace(l.Attrs["href"]) != "" {
			feed.FeedURL = resolveURL(feed.FeedURL, l.Attrs["href"])
			break
		}
	}
	if feed.Title == "" {
		feed.Title = feed.SiteURL
	}
	if ttl, err := strconv.Atoi(strings.TrimSpace(ch.TTL)); err == nil {
		feed.TTL = time.Duration(ttl) * time.Minute
	}
	if ch.Image != nil {
		feed.IconURL = resolveURL(feed.SiteURL, ch.Image.URL)
	}

	feedAuthor := rssFeedAuthor(ch)
	feedTags := rssFeedTags(ch)

	// Some feeds reuse one GUID for every item; keep the first stable and
	// disambiguate later collisions with the URL or position.
	seenGUIDs := make(map[string]int)
	for i, item := range ch.Items {
		entry := model.NewEntry()
		entry.Date = rssEntryDate(item, src.Items[i])
		entry.Content = rssEntryContent(item)
		entry.Enclosures = rssEntryEnclosures(item, feed.SiteURL)

		entryURL := rssEntryURL(item)
		entry.URL = resolveURL(feed.SiteURL, entryURL)
		if entry.URL == "" {
			entry.URL = feed.SiteURL
			if len(entry.Enclosures) > 0 {
				entry.URL = entry.Enclosures[0].URL
			}
		}

		title := item.Title
		if item.DublinCoreExt != nil && len(item.DublinCoreExt.Title) > 0 && item.DublinCoreExt.Title[0] != "" {
			title = item.DublinCoreExt.Title[0]
		}
		entry.Title = html.UnescapeString(html.UnescapeString(strings.TrimSpace(title)))
		if entry.Title == "" {
			entry.Title = sanitizer.TruncateHTML(entry.Content, 100)
		}
		if entry.Title == "" {
			entry.Title = entry.URL
		}

		entry.Author = rssEntryAuthor(item)
		if entry.Author == "" {
			entry.Author = feedAuthor
		}

		entry.Language = feed.Language
		if item.DublinCoreExt != nil && len(item.DublinCoreExt.Language) > 0 {
			if lang := normalizeLanguage(item.DublinCoreExt.Language[0]); lang != "" {
				entry.Language = lang
			}
		}

		var guid string
		if item.GUID != nil {
			guid = item.GUID.Value
		}
		switch {
		case guid != "":
			n := seenGUIDs[guid]
			seenGUIDs[guid] = n + 1
			switch {
			case n == 0:
				entry.Hash = crypto.SHA256(guid)
			case entry.URL != "":
				entry.Hash = crypto.SHA256(guid + "|" + entry.URL)
			default:
				entry.Hash = crypto.SHA256(guid + "|" + strconv.Itoa(n))
			}
		case entryURL != "":
			entry.Hash = crypto.SHA256(entryURL)
		default:
			entry.Hash = crypto.SHA256(entry.Title + entry.Content)
		}

		if comments := strings.TrimSpace(item.Comments); urllib.IsAbsoluteURL(comments) {
			entry.CommentsURL = comments
		}

		if item.ITunesExt != nil && item.ITunesExt.Duration != "" {
			if minutes, ok := durationInMinutes(item.ITunesExt.Duration); ok {
				entry.ReadingTime = minutes
			}
		}

		var labels []string
		for _, c := range mediaElements(item.Extensions, "category") {
			labels = append(labels, c.Attrs["label"])
		}
		entry.Tags = sortedUnique(append(rssCategories(item.Categories), labels...)...)
		if len(entry.Tags) == 0 {
			entry.Tags = feedTags
		}

		feed.Entries = append(feed.Entries, entry)
	}
	return feed
}

func rssCategories(categories []*rss.Category) []string {
	out := make([]string, 0, len(categories))
	for _, c := range categories {
		out = append(out, c.Value)
	}
	return out
}

func rssFeedAuthor(ch *rss.Feed) string {
	var author string
	if ch.ITunesExt != nil {
		author = ch.ITunesExt.Author
	}
	if author == "" {
		author = firstValue(googlePlay(ch.Extensions, "author"))
	}
	if author == "" && ch.ITunesExt != nil && ch.ITunesExt.Owner != nil {
		author = strings.TrimSpace(ch.ITunesExt.Owner.Name)
		if author == "" {
			author = strings.TrimSpace(ch.ITunesExt.Owner.Email)
		}
	}
	if author == "" {
		author = ch.ManagingEditor
	}
	if author == "" {
		author = ch.WebMaster
	}
	return sanitizer.StripTags(author)
}

func rssFeedTags(ch *rss.Feed) []string {
	tags := rssCategories(ch.Categories)
	if ch.ITunesExt != nil {
		for _, c := range ch.ITunesExt.Categories {
			for ; c != nil; c = c.Subcategory {
				tags = append(tags, c.Text)
			}
		}
	}
	for _, c := range googlePlay(ch.Extensions, "category") {
		tags = append(tags, c.Attrs["text"])
	}
	return sortedUnique(tags...)
}

func rssEntryDate(item *rss.Item, it *gofeed.Item) time.Time {
	// dc:date wins over pubDate; gofeed exposes a parsed dc:date as UpdatedParsed.
	if item.DublinCoreExt != nil && len(item.DublinCoreExt.Date) > 0 {
		if it.UpdatedParsed != nil {
			return *it.UpdatedParsed
		}
		return time.Now()
	}
	if item.PubDateParsed != nil {
		return *item.PubDateParsed
	}
	return time.Now()
}

func rssEntryContent(item *rss.Item) string {
	values := []string{item.Content, item.Description, firstValue(googlePlay(item.Extensions, "description"))}
	if item.ITunesExt != nil {
		values = append(values, item.ITunesExt.Summary, item.ITunesExt.Subtitle)
	}
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func rssEntryURL(item *rss.Item) string {
	if l := firstExtValue(item.Extensions, "feedburner", "origLink"); l != "" {
		return l
	}
	if l := strings.TrimSpace(item.Link); l != "" {
		return l
	}
	for _, l := range item.Extensions["atom"]["link"] {
		if href := strings.TrimSpace(l.Attrs["href"]); href != "" && (strings.EqualFold(l.Attrs["rel"], "alternate") || l.Attrs["rel"] == "") {
			return href
		}
	}
	// A guid is a permalink unless isPermaLink="false".
	if item.GUID != nil && (item.GUID.IsPermalink == "" || item.GUID.IsPermalink == "true") {
		return strings.TrimSpace(item.GUID.Value)
	}
	return ""
}

func rssEntryAuthor(item *rss.Item) string {
	author := firstValue(googlePlay(item.Extensions, "author"))
	if author == "" && item.ITunesExt != nil {
		author = item.ITunesExt.Author
	}
	if author == "" && item.DublinCoreExt != nil && len(item.DublinCoreExt.Creator) > 0 {
		author = item.DublinCoreExt.Creator[0]
	}
	if author == "" {
		author = atomPersonName(item.Extensions)
	}
	if author == "" {
		author = item.Author
	}
	return sanitizer.StripTags(author)
}

// atomPersonName returns the name (or email) of the first atom:author extension.
func atomPersonName(exts ext.Extensions) string {
	for _, a := range exts["atom"]["author"] {
		for _, field := range []string{"name", "email"} {
			for _, c := range a.Children[field] {
				if v := strings.TrimSpace(c.Value); v != "" {
					return v
				}
			}
		}
	}
	return ""
}

func rssEntryEnclosures(item *rss.Item, siteURL string) model.EnclosureList {
	set := newEnclosureSet(siteURL)
	set.addMediaThumbnails(item.Extensions)
	origEnclosure := firstExtValue(item.Extensions, "feedburner", "origEnclosureLink")
	for _, e := range item.Enclosures {
		u := e.URL
		if origEnclosure != "" && strings.HasSuffix(u, path.Base(origEnclosure)) {
			u = origEnclosure
		}
		size, _ := strconv.ParseInt(strings.TrimSpace(e.Length), 10, 64)
		set.add(u, e.Type, size)
	}
	set.addMediaContents(item.Extensions)
	return set.list
}

// durationInMinutes parses an iTunes duration ("SS", "MM:SS" or "HH:MM:SS").
func durationInMinutes(raw string) (int, bool) {
	parts := strings.Split(raw, ":")
	if len(parts) > 3 {
		return 0, false
	}
	seconds := 0
	for _, p := range parts {
		v, err := strconv.Atoi(p)
		if err != nil {
			return 0, false
		}
		seconds = seconds*60 + v
	}
	return seconds / 60, true
}

func buildRDF(baseURL string, src *gofeed.Feed, ch *rss.Feed) *model.Feed {
	feed := &model.Feed{
		Title:       sanitizer.StripTags(ch.Title),
		FeedURL:     strings.TrimSpace(baseURL),
		SiteURL:     strings.TrimSpace(ch.Link),
		Description: strings.TrimSpace(ch.Description),
	}
	if feed.Title == "" {
		feed.Title = baseURL
	}
	feed.SiteURL = resolveURL(feed.FeedURL, feed.SiteURL)
	if feed.SiteURL == "" {
		feed.SiteURL = feed.FeedURL
	}
	var channelCreator string
	if dc := ch.DublinCoreExt; dc != nil {
		if len(dc.Language) > 0 {
			feed.Language = normalizeLanguage(dc.Language[0])
		}
		if len(dc.Creator) > 0 {
			channelCreator = dc.Creator[0]
		}
	}

	for i, item := range ch.Items {
		entry := model.NewEntry()
		itemLink := strings.TrimSpace(item.Link)
		entry.URL = feed.SiteURL
		if itemLink != "" {
			entry.URL = resolveURL(feed.SiteURL, itemLink)
		}

		titles := []string{item.Title}
		if item.DublinCoreExt != nil {
			titles = append(titles, item.DublinCoreExt.Title...)
		}
		for _, title := range titles {
			if title = strings.TrimSpace(title); title != "" {
				entry.Title = html.UnescapeString(title)
				break
			}
		}
		if entry.Title == "" {
			entry.Title = entry.URL
		}

		entry.Content = item.Content
		if entry.Content == "" {
			entry.Content = item.Description
		}

		hashValue := itemLink
		if hashValue == "" {
			hashValue = item.Title + item.Description
		}
		entry.Hash = crypto.SHA256(hashValue)

		entry.Date = time.Now()
		if it := src.Items[i]; it.PublishedParsed != nil {
			entry.Date = *it.PublishedParsed
		}

		entry.Author = sanitizer.StripTags(channelCreator)
		entry.Language = feed.Language
		if dc := item.DublinCoreExt; dc != nil {
			if len(dc.Creator) > 0 && dc.Creator[0] != "" {
				entry.Author = sanitizer.StripTags(dc.Creator[0])
			}
			if len(dc.Language) > 0 {
				if lang := normalizeLanguage(dc.Language[0]); lang != "" {
					entry.Language = lang
				}
			}
		}

		feed.Entries = append(feed.Entries, entry)
	}
	return feed
}

// googlePlay returns Google Play podcast elements named name.
// ponytail: gofeed keys unknown namespaces by document prefix, so only the
// common "googleplay" and "gplay" prefixes are recognized.
func googlePlay(exts ext.Extensions, name string) []ext.Extension {
	return append(slices.Clone(exts["googleplay"][name]), exts["gplay"][name]...)
}

func firstValue(elems []ext.Extension) string {
	for _, e := range elems {
		if v := strings.TrimSpace(e.Value); v != "" {
			return v
		}
	}
	return ""
}
