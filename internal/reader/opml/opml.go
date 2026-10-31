// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

// Package opml parses and serializes OPML subscription lists, including
// the sss per-feed settings namespace.
package opml

import (
	"bytes"
	"cmp"
	"encoding/xml"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html/charset"

	"sss/internal/model"
)

const sssNamespace = "https://github.com/pentago/sss/opml"

type document struct {
	XMLName      xml.Name  `xml:"opml"`
	Version      string    `xml:"version,attr"`
	SSSNamespace string    `xml:"xmlns:sss,attr,omitempty"`
	Header       header    `xml:"head"`
	Outlines     []outline `xml:"body>outline"`
}

type header struct {
	Title       string `xml:"title,omitempty"`
	DateCreated string `xml:"dateCreated,omitempty"`
}

type outline struct {
	Title       string    `xml:"title,attr,omitempty"`
	Text        string    `xml:"text,attr"`
	FeedURL     string    `xml:"xmlUrl,attr,omitempty"`
	SiteURL     string    `xml:"htmlUrl,attr,omitempty"`
	Description string    `xml:"description,attr,omitempty"`
	Type        string    `xml:"type,attr,omitempty"`
	Outlines    []outline `xml:"outline,omitempty"`

	ScraperRules                string `xml:"sss:scraperRules,attr,omitempty"`
	RewriteRules                string `xml:"sss:rewriteRules,attr,omitempty"`
	UrlRewriteRules             string `xml:"sss:urlRewriteRules,attr,omitempty"`
	BlocklistRules              string `xml:"sss:blocklistRules,attr,omitempty"`
	KeeplistRules               string `xml:"sss:keeplistRules,attr,omitempty"`
	BlockFilterEntryRules       string `xml:"sss:blockFilterEntryRules,attr,omitempty"`
	KeepFilterEntryRules        string `xml:"sss:keepFilterEntryRules,attr,omitempty"`
	UserAgent                   string `xml:"sss:userAgent,attr,omitempty"`
	Crawler                     bool   `xml:"sss:crawler,attr,omitempty"`
	IgnoreHTTPCache             bool   `xml:"sss:ignoreHTTPCache,attr,omitempty"`
	FetchViaProxy               bool   `xml:"sss:fetchViaProxy,attr,omitempty"`
	Disabled                    bool   `xml:"sss:disabled,attr,omitempty"`
	NoMediaPlayer               bool   `xml:"sss:noMediaPlayer,attr,omitempty"`
	HideGlobally                bool   `xml:"sss:hideGlobally,attr,omitempty"`
	AllowSelfSignedCertificates bool   `xml:"sss:allowSelfSignedCertificates,attr,omitempty"`
	DisableHTTP2                bool   `xml:"sss:disableHTTP2,attr,omitempty"`
	IgnoreEntryUpdates          bool   `xml:"sss:ignoreEntryUpdates,attr,omitempty"`
}

// UnmarshalXML decodes the standard attributes, then the namespaced sss settings
// (encoding/xml does not map prefixed tag names on decode).
func (o *outline) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	type plain outline
	if err := d.DecodeElement((*plain)(o), &start); err != nil {
		return err
	}
	for _, attr := range start.Attr {
		if attr.Name.Space != sssNamespace {
			continue
		}
		v := attr.Value
		var target *bool
		switch attr.Name.Local {
		case "scraperRules":
			o.ScraperRules = v
		case "rewriteRules":
			o.RewriteRules = v
		case "urlRewriteRules":
			o.UrlRewriteRules = v
		case "blocklistRules":
			o.BlocklistRules = v
		case "keeplistRules":
			o.KeeplistRules = v
		case "blockFilterEntryRules":
			o.BlockFilterEntryRules = v
		case "keepFilterEntryRules":
			o.KeepFilterEntryRules = v
		case "userAgent":
			o.UserAgent = v
		case "crawler":
			target = &o.Crawler
		case "ignoreHTTPCache":
			target = &o.IgnoreHTTPCache
		case "fetchViaProxy":
			target = &o.FetchViaProxy
		case "disabled":
			target = &o.Disabled
		case "noMediaPlayer":
			target = &o.NoMediaPlayer
		case "hideGlobally":
			target = &o.HideGlobally
		case "allowSelfSignedCertificates":
			target = &o.AllowSelfSignedCertificates
		case "disableHTTP2":
			target = &o.DisableHTTP2
		case "ignoreEntryUpdates":
			target = &o.IgnoreEntryUpdates
		}
		if target != nil {
			b, err := strconv.ParseBool(v)
			if err != nil {
				return fmt.Errorf("opml: invalid sss attribute %q: %w", attr.Name.Local, err)
			}
			*target = b
		}
	}
	return nil
}

// Parse reads an OPML document and returns one feed per subscription outline.
// Each feed has Category set (Title empty for top-level outlines).
func Parse(r io.Reader) ([]*model.Feed, error) {
	var doc document
	decoder := xml.NewDecoder(r)
	decoder.Entity = xml.HTMLEntity
	decoder.Strict = false
	decoder.CharsetReader = charset.NewReaderLabel
	if err := decoder.Decode(&doc); err != nil {
		return nil, fmt.Errorf("opml: unable to parse document: %w", err)
	}
	return feedsFromOutlines(doc.Outlines, ""), nil
}

func feedsFromOutlines(outlines []outline, category string) []*model.Feed {
	var feeds []*model.Feed
	for _, o := range outlines {
		if strings.TrimSpace(o.FeedURL) == "" {
			// Nested categories are flattened into the nearest named parent.
			feeds = append(feeds, feedsFromOutlines(o.Outlines, cmp.Or(o.Title, o.Text, o.SiteURL))...)
			continue
		}
		feeds = append(feeds, &model.Feed{
			Title:                       cmp.Or(o.Title, o.Text, o.SiteURL, o.FeedURL),
			FeedURL:                     o.FeedURL,
			SiteURL:                     cmp.Or(o.SiteURL, o.FeedURL),
			Description:                 o.Description,
			Category:                    &model.Category{Title: category},
			ScraperRules:                o.ScraperRules,
			RewriteRules:                o.RewriteRules,
			UrlRewriteRules:             o.UrlRewriteRules,
			BlocklistRules:              o.BlocklistRules,
			KeeplistRules:               o.KeeplistRules,
			BlockFilterEntryRules:       o.BlockFilterEntryRules,
			KeepFilterEntryRules:        o.KeepFilterEntryRules,
			UserAgent:                   o.UserAgent,
			Crawler:                     o.Crawler,
			IgnoreHTTPCache:             o.IgnoreHTTPCache,
			FetchViaProxy:               o.FetchViaProxy,
			Disabled:                    o.Disabled,
			NoMediaPlayer:               o.NoMediaPlayer,
			HideGlobally:                o.HideGlobally,
			AllowSelfSignedCertificates: o.AllowSelfSignedCertificates,
			DisableHTTP2:                o.DisableHTTP2,
			IgnoreEntryUpdates:          o.IgnoreEntryUpdates,
		})
	}
	return feeds
}

// Serialize returns an OPML 2.0 document with feeds grouped by category title (sorted).
func Serialize(feeds []*model.Feed) (string, error) {
	doc := document{
		Version:      "2.0",
		SSSNamespace: sssNamespace,
		Header:       header{Title: "sss", DateCreated: time.Now().Format("Mon, 02 Jan 2006 15:04:05 MST")},
	}

	groups := make(map[string][]outline)
	for _, f := range feeds {
		category := ""
		if f.Category != nil {
			category = f.Category.Title
		}
		groups[category] = append(groups[category], outline{
			Title:                       f.Title,
			Text:                        f.Title,
			FeedURL:                     f.FeedURL,
			SiteURL:                     f.SiteURL,
			Description:                 f.Description,
			Type:                        "rss",
			ScraperRules:                f.ScraperRules,
			RewriteRules:                f.RewriteRules,
			UrlRewriteRules:             f.UrlRewriteRules,
			BlocklistRules:              f.BlocklistRules,
			KeeplistRules:               f.KeeplistRules,
			BlockFilterEntryRules:       f.BlockFilterEntryRules,
			KeepFilterEntryRules:        f.KeepFilterEntryRules,
			UserAgent:                   f.UserAgent,
			Crawler:                     f.Crawler,
			IgnoreHTTPCache:             f.IgnoreHTTPCache,
			FetchViaProxy:               f.FetchViaProxy,
			Disabled:                    f.Disabled,
			NoMediaPlayer:               f.NoMediaPlayer,
			HideGlobally:                f.HideGlobally,
			AllowSelfSignedCertificates: f.AllowSelfSignedCertificates,
			DisableHTTP2:                f.DisableHTTP2,
			IgnoreEntryUpdates:          f.IgnoreEntryUpdates,
		})
	}
	categories := make([]string, 0, len(groups))
	for c := range groups {
		categories = append(categories, c)
	}
	slices.Sort(categories)
	for _, c := range categories {
		doc.Outlines = append(doc.Outlines, outline{Text: c, Outlines: groups[c]})
	}

	var b bytes.Buffer
	b.WriteString(xml.Header)
	encoder := xml.NewEncoder(&b)
	encoder.Indent("", "    ")
	if err := encoder.Encode(doc); err != nil {
		return "", fmt.Errorf("opml: unable to serialize document: %w", err)
	}
	return b.String(), nil
}
