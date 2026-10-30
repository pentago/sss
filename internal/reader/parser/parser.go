// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

// Package parser parses RSS, Atom, RDF and JSON feeds into normalized model.Feed values.
package parser

import (
	"errors"
	"io"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/mmcdole/gofeed"
	"github.com/mmcdole/gofeed/atom"
	ext "github.com/mmcdole/gofeed/extensions"
	"github.com/mmcdole/gofeed/rss"

	"sss/internal/model"
	"sss/internal/urllib"
)

var textLinkRegex = regexp.MustCompile(`(?mi)(\bhttps?://[^\s]+)`)

// ErrFeedFormatNotDetected is returned when the document is not a known feed format.
var ErrFeedFormatNotDetected = errors.New("parser: unable to detect feed format")

// ParseFeed parses r and returns a normalized feed; baseURL is the URL the feed was fetched from.
//
// XML feeds go through gofeed: the universal feed supplies parsed dates, the
// format-specific original feed supplies everything the translator drops.
// JSON feeds are decoded directly (see json.go).
func ParseFeed(baseURL string, r io.ReadSeeker) (*model.Feed, error) {
	switch DetectFeedFormat(r) {
	case FormatJSON:
		return parseJSON(baseURL, r)
	case FormatUnknown:
		return nil, ErrFeedFormatNotDetected
	}

	p := gofeed.NewParser()
	p.KeepOriginalFeed = true
	src, err := p.Parse(r)
	if err != nil {
		if errors.Is(err, gofeed.ErrFeedTypeNotDetected) {
			return nil, ErrFeedFormatNotDetected
		}
		return nil, errors.New("parser: " + err.Error())
	}

	switch o := src.OriginalFeed().(type) {
	case *rss.Feed:
		if o.Version == "1.0" || o.Version == "0.9" {
			return buildRDF(baseURL, src, o), nil
		}
		return buildRSS(baseURL, src, o), nil
	case *atom.Feed:
		return buildAtom(baseURL, o), nil
	}
	return nil, ErrFeedFormatNotDetected
}

// resolveURL trims u and resolves it against base; unresolvable URLs are returned trimmed.
func resolveURL(base, u string) string {
	u = strings.TrimSpace(u)
	// gofeed re-serializes href/src/url attributes through url.Parse, which
	// turns trailing spaces into "%20"; undo that so they trim like text.
	for strings.HasSuffix(u, "%20") {
		u = u[:len(u)-3]
	}
	if u == "" {
		return ""
	}
	if abs, err := urllib.ResolveToAbsoluteURL(base, u); err == nil {
		return abs
	}
	return u
}

// enclosureSet collects enclosures, skipping empty and duplicate URLs.
type enclosureSet struct {
	siteURL string
	list    model.EnclosureList
	seen    map[string]bool
}

func newEnclosureSet(siteURL string) *enclosureSet {
	return &enclosureSet{siteURL: siteURL, list: make(model.EnclosureList, 0), seen: make(map[string]bool)}
}

func (s *enclosureSet) add(rawURL, mimeType string, size int64) {
	u := resolveURL(s.siteURL, rawURL)
	if u == "" || s.seen[u] {
		return
	}
	s.seen[u] = true
	s.list = append(s.list, &model.Enclosure{URL: u, MimeType: mimeType, Size: size})
}

// addMediaThumbnails adds Media RSS thumbnails; upstream lists them before regular enclosures.
func (s *enclosureSet) addMediaThumbnails(exts ext.Extensions) {
	for _, t := range mediaElements(exts, "thumbnail") {
		s.add(t.Attrs["url"], "image/*", 0)
	}
}

// addMediaContents adds Media RSS contents then peer links; upstream lists them after regular enclosures.
func (s *enclosureSet) addMediaContents(exts ext.Extensions) {
	for _, c := range mediaElements(exts, "content") {
		mimeType := c.Attrs["type"]
		if mimeType == "" {
			switch c.Attrs["medium"] {
			case "image", "video", "audio":
				mimeType = c.Attrs["medium"] + "/*"
			default:
				mimeType = "application/octet-stream"
			}
		}
		size, _ := strconv.ParseInt(c.Attrs["fileSize"], 10, 64)
		s.add(c.Attrs["url"], mimeType, size)
	}
	for _, p := range mediaElements(exts, "peerLink") {
		mimeType := p.Attrs["type"]
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
		s.add(p.Attrs["href"], mimeType, 0)
	}
}

// mediaElements returns Media RSS elements named name, including those inside <media:group>.
func mediaElements(exts ext.Extensions, name string) []ext.Extension {
	media := exts["media"]
	out := slices.Clone(media[name])
	for _, g := range media["group"] {
		out = append(out, g.Children[name]...)
	}
	return out
}

// firstMediaDescription returns the first non-empty media:description as HTML;
// plain-text descriptions get their URLs linked and newlines turned into <br>.
func firstMediaDescription(exts ext.Extensions) string {
	for _, e := range mediaElements(exts, "description") {
		if e.Value == "" {
			continue
		}
		if e.Attrs["type"] == "html" {
			return e.Value
		}
		return strings.ReplaceAll(textLinkRegex.ReplaceAllString(e.Value, `<a href="${1}">${1}</a>`), "\n", "<br>")
	}
	return ""
}

// firstExtValue returns the first non-empty value of prefix:name.
func firstExtValue(exts ext.Extensions, prefix, name string) string {
	return firstValue(exts[prefix][name])
}

// sortedUnique trims values and returns them sorted without empties or duplicates.
func sortedUnique(values ...string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" {
			if i, found := slices.BinarySearch(out, v); !found {
				out = slices.Insert(out, i, v)
			}
		}
	}
	return out
}

// normalizeLanguage turns a feed language tag into a lower-case, hyphenated
// value usable as an HTML lang attribute, or "" if it is not a plausible tag.
func normalizeLanguage(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 50 {
		return ""
	}
	b := []byte(s)
	for i, c := range b {
		switch {
		case c >= 'A' && c <= 'Z':
			b[i] = c + 'a' - 'A'
		case c == '_':
			b[i] = '-'
		case (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-':
		default:
			return ""
		}
	}
	return string(b)
}
