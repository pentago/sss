// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

// Package icon finds and downloads website icons.
package icon

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"mime"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"sss/internal/config"
	"sss/internal/crypto"
	"sss/internal/model"
	"sss/internal/reader/encoding"
	"sss/internal/reader/fetcher"
	"sss/internal/urllib"
)

const maxIconDimension = 4096

// FindIcon looks for the icon of feed: the feed's own icon URL, then <link> icons
// of the site (and its root), then /favicon.ico. It returns nil, nil when none is found.
func FindIcon(feed *model.Feed) (*model.Icon, error) {
	requestBuilder := fetcher.NewRequestBuilder().
		WithUserAgent(feed.UserAgent, config.Opts.HTTPClientUserAgent).
		WithCookie(feed.Cookie, feed.FeedURL).
		WithTimeout(config.Opts.HTTPClientTimeout).
		WithCustomFeedProxyURL(feed.ProxyURL).
		WithCustomApplicationProxyURL(config.Opts.HTTPClientProxy).
		UseCustomApplicationProxyURL(feed.FetchViaProxy).
		IgnoreTLSErrors(feed.AllowSelfSignedCertificates).
		DisableHTTP2(feed.DisableHTTP2)
	return (&finder{requestBuilder}).find(feed.SiteURL, feed.IconURL)
}

type finder struct {
	requestBuilder *fetcher.RequestBuilder
}

func (f *finder) find(websiteURL, feedIconURL string) (*model.Icon, error) {
	if feedIconURL != "" {
		if icon, err := f.download(feedIconURL); err == nil {
			return icon, nil
		}
	}

	// The site URL may be a subdirectory with its own icon references.
	urls := []string{websiteURL}
	if root := urllib.RootURL(websiteURL); root != websiteURL {
		urls = append(urls, root)
	}
	for _, documentURL := range urls {
		if icon, err := f.fromHTMLDocument(documentURL); err == nil && icon != nil {
			return icon, nil
		}
	}

	iconURL, err := urllib.JoinBaseURLAndPath(urllib.RootURL(websiteURL), "favicon.ico")
	if err != nil {
		return nil, fmt.Errorf("icon: unable to join root URL and path: %w", err)
	}
	return f.download(iconURL)
}

func (f *finder) fromHTMLDocument(documentURL string) (*model.Icon, error) {
	rh := fetcher.NewResponseHandler(f.requestBuilder.ExecuteRequest(documentURL))
	defer rh.Close()
	if err := rh.LocalizedError(); err != nil {
		return nil, err
	}
	r, err := encoding.NewCharsetReader(rh.Body(config.Opts.HTTPClientMaxBodySize), rh.ContentType())
	if err != nil {
		return nil, err
	}
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		return nil, fmt.Errorf("icon: unable to read document: %w", err)
	}

	sel := doc.Find(`link[rel='icon' i][href], link[rel='shortcut icon' i][href], link[rel='icon shortcut' i][href], link[rel='apple-touch-icon'][href]`)
	for _, s := range sel.EachIter() {
		href := strings.TrimSpace(s.AttrOr("href", ""))
		if href == "" {
			continue
		}
		if strings.HasPrefix(href, "data:") {
			return parseImageDataURL(href)
		}
		iconURL, err := urllib.ResolveToAbsoluteURL(documentURL, href)
		if err != nil {
			continue
		}
		if icon, err := f.download(iconURL); err == nil {
			return icon, nil
		}
	}
	return nil, nil
}

func (f *finder) download(iconURL string) (*model.Icon, error) {
	rh := fetcher.NewResponseHandler(f.requestBuilder.ExecuteRequest(iconURL))
	defer rh.Close()
	if err := rh.LocalizedError(); err != nil {
		return nil, fmt.Errorf("icon: unable to download website icon: %w", err)
	}
	body, err := rh.ReadBody(config.Opts.HTTPClientMaxBodySize)
	if err != nil {
		return nil, fmt.Errorf("icon: %w", err)
	}
	return newIcon(rh.ContentType(), body)
}

// newIcon validates that body is an image of acceptable dimensions.
func newIcon(contentType string, body []byte) (*model.Icon, error) {
	mimeType, _, _ := mime.ParseMediaType(contentType)
	if !strings.HasPrefix(mimeType, "image/") {
		// Many servers send icons as application/octet-stream.
		mimeType, _, _ = mime.ParseMediaType(http.DetectContentType(body))
	}
	if !strings.HasPrefix(mimeType, "image/") {
		return nil, fmt.Errorf("icon: unexpected content type %q", contentType)
	}
	// Reject decodable bitmaps that are absurdly large; formats the stdlib can't decode (ico, svg, webp) pass.
	if cfg, _, err := image.DecodeConfig(bytes.NewReader(body)); err == nil {
		if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > maxIconDimension || cfg.Height > maxIconDimension {
			return nil, errors.New("icon: invalid image dimensions")
		}
	}
	return &model.Icon{Hash: crypto.HashFromBytes(body), MimeType: mimeType, Content: body}, nil
}

var dataURLRegexp = regexp.MustCompile(`^data:(image/[^;,]+)(?:;(base64|utf8))?,(.+)$`)

// parseImageDataURL decodes data:image/...[;base64|;utf8],<data>.
func parseImageDataURL(value string) (*model.Icon, error) {
	m := dataURLRegexp.FindStringSubmatch(value)
	if m == nil {
		return nil, fmt.Errorf("icon: invalid data URL %q", value)
	}
	var blob []byte
	switch m[2] {
	case "base64":
		var err error
		if blob, err = base64.StdEncoding.DecodeString(m[3]); err != nil {
			return nil, fmt.Errorf("icon: invalid data URL: %w", err)
		}
	case "":
		s, err := url.QueryUnescape(m[3])
		if err != nil {
			return nil, fmt.Errorf("icon: unable to decode data URL: %w", err)
		}
		blob = []byte(s)
	case "utf8":
		blob = []byte(m[3])
	}
	return &model.Icon{Hash: crypto.HashFromBytes(blob), Content: blob, MimeType: m[1]}, nil
}
