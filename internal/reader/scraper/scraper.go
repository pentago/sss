// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

// Package scraper extracts article content from web pages using CSS selector rules or readability.
package scraper

import (
	"fmt"
	"io"
	"strings"

	"sss/internal/config"
	"sss/internal/reader/encoding"
	"sss/internal/reader/fetcher"
	"sss/internal/reader/readability"
	"sss/internal/urllib"

	"github.com/PuerkitoBio/goquery"
	"github.com/andybalholm/cascadia"
)

// ScrapeWebsite fetches pageURL and extracts its content with rules (custom or predefined) or readability.
// baseURL is the document base URL to resolve relative links against.
func ScrapeWebsite(requestBuilder *fetcher.RequestBuilder, pageURL, rules string) (baseURL string, extractedContent string, err error) {
	responseHandler := fetcher.NewResponseHandler(requestBuilder.ExecuteRequest(pageURL))
	defer responseHandler.Close()

	if err := responseHandler.LocalizedError(); err != nil {
		return "", "", err
	}

	if !isAllowedContentType(responseHandler.ContentType()) {
		return "", "", fmt.Errorf("scraper: this resource is not a HTML document (%s)", responseHandler.ContentType())
	}

	// The entry URL could redirect somewhere else.
	sameSite := urllib.Domain(pageURL) == urllib.Domain(responseHandler.EffectiveURL())
	pageURL = responseHandler.EffectiveURL()

	if rules == "" {
		rules = predefinedRules[urllib.DomainWithoutWWW(pageURL)]
	}

	htmlDocumentReader, err := encoding.NewCharsetReader(
		responseHandler.Body(config.Opts.HTTPClientMaxBodySize),
		responseHandler.ContentType(),
	)

	if err != nil {
		return "", "", fmt.Errorf("scraper: unable to read HTML document with charset reader: %v", err)
	}

	// Custom rules are ignored when the entry URL redirects to another domain.
	if sameSite && rules != "" {
		baseURL, extractedContent, err = findContentUsingCustomRules(htmlDocumentReader, rules)
	} else {
		baseURL, extractedContent, err = readability.ExtractContent(htmlDocumentReader)
	}
	if err != nil {
		return "", "", fmt.Errorf("scraper: unable to extract content: %w", err)
	}

	if baseURL == "" {
		baseURL = pageURL
	}

	return baseURL, extractedContent, nil
}

func findContentUsingCustomRules(page io.Reader, rules string) (baseURL string, extractedContent string, err error) {
	document, err := goquery.NewDocumentFromReader(page)
	if err != nil {
		return "", "", err
	}

	if hrefValue, exists := document.FindMatcher(goquery.Single("head base")).Attr("href"); exists {
		hrefValue = strings.TrimSpace(hrefValue)
		if urllib.IsAbsoluteURL(hrefValue) {
			baseURL = hrefValue
		}
	}

	var buf strings.Builder
	document.Find(rules).Each(func(i int, s *goquery.Selection) {
		if content, err := goquery.OuterHtml(s); err == nil {
			buf.WriteString(content)
		}
	})

	return baseURL, buf.String(), nil
}

func isAllowedContentType(contentType string) bool {
	contentType = strings.ToLower(contentType)
	return strings.HasPrefix(contentType, "text/html") ||
		strings.HasPrefix(contentType, "application/xhtml+xml")
}

// IsValidScraperRules reports whether rules is empty or a valid CSS selector group.
func IsValidScraperRules(rules string) bool {
	if rules == "" {
		return true
	}
	_, err := cascadia.ParseGroup(rules)
	return err == nil
}
