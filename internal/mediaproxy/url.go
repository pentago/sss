// Copyright The Miniflux Authors. Modified by The sss Authors.
// Derived from Miniflux (https://github.com/miniflux/v2).
// SPDX-License-Identifier: Apache-2.0

// Package mediaproxy signs media URLs and rewrites HTML so remote media is served through /proxy.
package mediaproxy

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/url"

	"sss/internal/config"
	"sss/internal/model"
)

// ProxifyRelativeURL returns the signed proxy path (with base path) for mediaURL.
func ProxifyRelativeURL(mediaURL string) string {
	if mediaURL == "" {
		return ""
	}

	if customProxyURL := config.Opts.MediaProxyCustomURL; customProxyURL != nil {
		return proxifyURLWithCustomProxy(mediaURL, customProxyURL)
	}

	mediaURLBytes := []byte(mediaURL)

	mac := hmac.New(sha256.New, config.Opts.MediaProxyPrivateKey)
	mac.Write(mediaURLBytes)
	digest := mac.Sum(nil)

	// Preserve the configured base path so proxied URLs still work when Miniflux is served from a subfolder.
	return fmt.Sprintf("%s/proxy/%s/%s", config.Opts.BasePath, base64.URLEncoding.EncodeToString(digest), base64.URLEncoding.EncodeToString(mediaURLBytes))
}

// ProxifyAbsoluteURL returns the signed absolute proxy URL for mediaURL.
func ProxifyAbsoluteURL(mediaURL string) string {
	if mediaURL == "" {
		return ""
	}

	if customProxyURL := config.Opts.MediaProxyCustomURL; customProxyURL != nil {
		return proxifyURLWithCustomProxy(mediaURL, customProxyURL)
	}

	// Note that the proxyified URL is relative to the root URL.
	proxifiedUrl := ProxifyRelativeURL(mediaURL)
	absoluteURL, err := url.JoinPath(config.Opts.RootURL, proxifiedUrl)
	if err != nil {
		return mediaURL
	}

	return absoluteURL
}

func proxifyURLWithCustomProxy(mediaURL string, customProxyURL *url.URL) string {
	if customProxyURL == nil {
		return mediaURL
	}

	absoluteURL := customProxyURL.JoinPath(base64.URLEncoding.EncodeToString([]byte(mediaURL)))
	return absoluteURL.String()
}

// ProxifyEnclosureURLs rewrites enclosure URLs to the absolute proxy URL when the media proxy settings require it.
func ProxifyEnclosureURLs(enclosures model.EnclosureList) {
	for _, enclosure := range enclosures {
		if ShouldProxifyURLWithMimeType(enclosure.URL, enclosure.MimeType, config.Opts.MediaProxyMode, config.Opts.MediaProxyResourceTypes) {
			enclosure.URL = ProxifyAbsoluteURL(enclosure.URL)
		}
	}
}
