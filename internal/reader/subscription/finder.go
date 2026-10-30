// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

// Package subscription discovers feeds from a website URL.
package subscription

import (
	"bytes"
	"fmt"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"sss/internal/config"
	"sss/internal/integration/rssbridge"
	"sss/internal/model"
	"sss/internal/reader/encoding"
	"sss/internal/reader/fetcher"
	"sss/internal/reader/parser"
	"sss/internal/urllib"
)

// Subscription is a discovered feed.
type Subscription struct {
	Title string `json:"title"`
	URL   string `json:"url"`
	Type  string `json:"type"`
}

// NewSubscription returns a Subscription.
func NewSubscription(title, url, kind string) *Subscription {
	return &Subscription{Title: title, URL: url, Type: kind}
}

// Subscriptions is a list of discovered feeds.
type Subscriptions []*Subscription

// SubscriptionFinder discovers feeds; when the URL is itself a feed it keeps the downloaded body.
type SubscriptionFinder struct {
	requestBuilder   *fetcher.RequestBuilder
	feedDownloaded   bool
	feedResponseInfo *model.FeedCreationRequestFromSubscriptionDiscovery
}

// NewSubscriptionFinder returns a finder using requestBuilder for every request.
func NewSubscriptionFinder(requestBuilder *fetcher.RequestBuilder) *SubscriptionFinder {
	return &SubscriptionFinder{requestBuilder: requestBuilder}
}

// IsFeedAlreadyDownloaded reports whether the website URL was a feed.
func (f *SubscriptionFinder) IsFeedAlreadyDownloaded() bool {
	return f.feedDownloaded
}

// FeedResponseInfo returns the downloaded document and its cache validators.
func (f *SubscriptionFinder) FeedResponseInfo() *model.FeedCreationRequestFromSubscriptionDiscovery {
	return f.feedResponseInfo
}

// FindSubscriptions discovers feeds for websiteURL. rssBridgeURL is empty when RSS-Bridge is not configured.
func (f *SubscriptionFinder) FindSubscriptions(websiteURL, rssBridgeURL, rssBridgeToken string) (Subscriptions, error) {
	responseHandler := fetcher.NewResponseHandler(f.requestBuilder.ExecuteRequest(websiteURL))
	defer responseHandler.Close()

	if err := responseHandler.LocalizedError(); err != nil {
		return nil, err
	}
	body, err := responseHandler.ReadBody(config.Opts.HTTPClientMaxBodySize)
	if err != nil {
		return nil, err
	}

	f.feedResponseInfo = &model.FeedCreationRequestFromSubscriptionDiscovery{
		Content:      bytes.NewReader(body),
		ETag:         responseHandler.ETag(),
		LastModified: responseHandler.LastModified(),
	}

	// 1) The URL is already a feed.
	if format := parser.DetectFeedFormat(f.feedResponseInfo.Content); format != parser.FormatUnknown {
		f.feedDownloaded = true
		return Subscriptions{NewSubscription(responseHandler.EffectiveURL(), responseHandler.EffectiveURL(), format)}, nil
	}

	r, err := encoding.NewCharsetReader(bytes.NewReader(body), responseHandler.ContentType())
	if err != nil {
		return nil, err
	}
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		return nil, fmt.Errorf("subscription: unable to parse HTML document: %w", err)
	}
	baseURL := websiteURL
	if href, ok := doc.FindMatcher(goquery.Single("head base")).Attr("href"); ok && urllib.IsAbsoluteURL(strings.TrimSpace(href)) {
		baseURL = strings.TrimSpace(href)
	}

	// 2) Canonical URL.
	if href, ok := doc.FindMatcher(goquery.Single("head link[rel='canonical' i]")).Attr("href"); ok && strings.TrimSpace(href) != "" {
		if u, err := urllib.ResolveToAbsoluteURL(baseURL, strings.TrimSpace(href)); err == nil {
			websiteURL = u
		}
	}

	// 3-7) YouTube, GitHub, HTML <link>, RSS-Bridge, well-known URLs.
	if s := findSubscriptionsFromYouTube(websiteURL); len(s) > 0 {
		return s, nil
	}
	if s := findSubscriptionsFromGitHub(websiteURL); len(s) > 0 {
		return s, nil
	}
	if s := findSubscriptionsFromWebPage(baseURL, doc); len(s) > 0 {
		return s, nil
	}
	if rssBridgeURL != "" {
		bridges, err := rssbridge.DetectBridges(rssBridgeURL, rssBridgeToken, websiteURL)
		if err != nil {
			return nil, fmt.Errorf("subscription: unable to detect feeds with RSS-Bridge: %w", err)
		}
		if len(bridges) > 0 {
			subscriptions := make(Subscriptions, 0, len(bridges))
			for _, b := range bridges {
				subscriptions = append(subscriptions, NewSubscription(b.BridgeMeta.Name, b.URL, parser.FormatAtom))
			}
			return subscriptions, nil
		}
	}
	return f.findSubscriptionsFromWellKnownURLs(websiteURL), nil
}

func findSubscriptionsFromWebPage(baseURL string, doc *goquery.Document) Subscriptions {
	var subscriptions Subscriptions
	seen := make(map[string]bool)
	doc.Find("link[type]").Each(func(_ int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		var format string
		switch t, _ := s.Attr("type"); t {
		case "application/rss+xml":
			format = parser.FormatRSS
		case "application/atom+xml":
			format = parser.FormatAtom
		case "application/feed+json":
			format = parser.FormatJSON
		case "application/json":
			// Skip WordPress REST API endpoints.
			if strings.Contains(href, "/wp-json/") {
				return
			}
			format = parser.FormatJSON
		default:
			return
		}
		if href == "" {
			return
		}
		u, err := urllib.ResolveToAbsoluteURL(baseURL, href)
		if err != nil || seen[u] {
			return
		}
		seen[u] = true
		title, _ := s.Attr("title")
		if title == "" {
			title = u
		}
		subscriptions = append(subscriptions, NewSubscription(title, u, format))
	})
	return subscriptions
}

func (f *SubscriptionFinder) findSubscriptionsFromWellKnownURLs(websiteURL string) Subscriptions {
	knownURLs := []struct{ path, format string }{
		{"atom.xml", parser.FormatAtom},
		{"feed.atom", parser.FormatAtom},
		{"feed.xml", parser.FormatAtom},
		{"feed/", parser.FormatAtom},
		{"index.rss", parser.FormatRSS},
		{"index.xml", parser.FormatRSS},
		{"rss.xml", parser.FormatRSS},
		{"rss/", parser.FormatRSS},
		{"rss/feed.xml", parser.FormatRSS},
	}

	root := urllib.RootURL(websiteURL)
	baseURLs := []string{root}
	if dir, _ := urllib.ResolveToAbsoluteURL(websiteURL, "./"); dir != root {
		baseURLs = append(baseURLs, dir)
	}

	var subscriptions Subscriptions
	for _, baseURL := range baseURLs {
		for _, known := range knownURLs {
			fullURL, err := urllib.ResolveToAbsoluteURL(baseURL, known.path)
			if err != nil {
				continue
			}
			// Sites often redirect unknown URLs to the home page: ignore redirects.
			rh := fetcher.NewResponseHandler(f.requestBuilder.Clone().WithoutRedirects().ExecuteRequest(fullURL))
			err = rh.LocalizedError()
			rh.Close()
			if err != nil || rh.IsRedirect() {
				continue
			}
			subscriptions = append(subscriptions, NewSubscription(fullURL, fullURL, known.format))
		}
	}
	return subscriptions
}

func findSubscriptionsFromYouTube(websiteURL string) Subscriptions {
	u, err := url.Parse(websiteURL)
	if err != nil || !strings.HasSuffix(u.Host, "youtube.com") {
		return nil
	}
	if _, baseID, found := strings.Cut(u.Path, "channel/UC"); found {
		return Subscriptions{
			NewSubscription("Channel", "https://www.youtube.com/feeds/videos.xml?channel_id=UC"+baseID, parser.FormatAtom),
			NewSubscription("Videos", "https://www.youtube.com/feeds/videos.xml?playlist_id=UULF"+baseID, parser.FormatAtom),
			NewSubscription("Short videos", "https://www.youtube.com/feeds/videos.xml?playlist_id=UUSH"+baseID, parser.FormatAtom),
			NewSubscription("Live streams", "https://www.youtube.com/feeds/videos.xml?playlist_id=UULV"+baseID, parser.FormatAtom),
		}
	}
	if strings.HasPrefix(u.Path, "/watch") || strings.HasPrefix(u.Path, "/playlist") {
		if id := u.Query().Get("list"); id != "" {
			return Subscriptions{NewSubscription(u.String(), "https://www.youtube.com/feeds/videos.xml?playlist_id="+id, parser.FormatAtom)}
		}
	}
	return nil
}

// findSubscriptionsFromGitHub returns the Atom feeds GitHub exposes for profiles and repositories.
func findSubscriptionsFromGitHub(websiteURL string) Subscriptions {
	u, err := url.Parse(websiteURL)
	if err != nil || (u.Host != "github.com" && u.Host != "www.github.com") {
		return nil
	}
	p := strings.Trim(u.Path, "/")
	if p == "" {
		return nil
	}
	switch segments := strings.SplitN(p, "/", 3); len(segments) {
	case 1:
		return Subscriptions{NewSubscription(segments[0], "https://github.com/"+segments[0]+".atom", parser.FormatAtom)}
	case 2:
		repo := "https://github.com/" + segments[0] + "/" + segments[1] + "/"
		return Subscriptions{
			NewSubscription("Commits", repo+"commits.atom", parser.FormatAtom),
			NewSubscription("Releases", repo+"releases.atom", parser.FormatAtom),
			NewSubscription("Tags", repo+"tags.atom", parser.FormatAtom),
		}
	}
	return nil
}
