// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

package scraper

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"sss/internal/config"
	"sss/internal/reader/fetcher"
	"sss/internal/urllib"

	"github.com/andybalholm/cascadia"
)

func TestPredefinedRulesAreValidSelectors(t *testing.T) {
	for domain, rules := range predefinedRules {
		if _, err := cascadia.Compile(rules); err != nil {
			t.Errorf("Predefined scraper rules for %s are not a valid CSS selector: %v", domain, err)
		}
	}
}

func TestPredefinedRuleLookup(t *testing.T) {
	if predefinedRules[urllib.DomainWithoutWWW("https://www.linux.com/")] == "" {
		t.Error("Unable to find rule for linux.com (www prefix should be stripped)")
	}
	if predefinedRules[urllib.DomainWithoutWWW("https://example.org/")] != "" {
		t.Error("A rule not defined should not return anything")
	}
}

func TestWhitelistedContentTypes(t *testing.T) {
	scenarios := map[string]bool{
		"text/html":                            true,
		"TeXt/hTmL":                            true,
		"application/xhtml+xml":                true,
		"text/html; charset=utf-8":             true,
		"application/xhtml+xml; charset=utf-8": true,
		"text/css":                             false,
		"application/javascript":               false,
		"image/png":                            false,
		"application/pdf":                      false,
	}

	for inputValue, expectedResult := range scenarios {
		actualResult := isAllowedContentType(inputValue)
		if actualResult != expectedResult {
			t.Errorf(`Unexpected result for content type whitelist, got "%v" instead of "%v"`, actualResult, expectedResult)
		}
	}
}

func TestSelectorRules(t *testing.T) {
	var ruleTestCases = map[string]string{
		"img.html":    "article > img",
		"iframe.html": "article > iframe",
		"p.html":      "article > p",
	}

	for filename, rule := range ruleTestCases {
		html, err := os.ReadFile("testdata/" + filename)
		if err != nil {
			t.Fatalf(`Unable to read file %q: %v`, filename, err)
		}

		_, actualResult, err := findContentUsingCustomRules(bytes.NewReader(html), rule)
		if err != nil {
			t.Fatalf(`Scraping error for %q - %q: %v`, filename, rule, err)
		}

		expectedResult, err := os.ReadFile("testdata/" + filename + "-result")
		if err != nil {
			t.Fatalf(`Unable to read file %q: %v`, filename, err)
		}

		if actualResult != strings.TrimSpace(string(expectedResult)) {
			t.Errorf(`Unexpected result for %q, got %q instead of %q`, rule, actualResult, expectedResult)
		}
	}
}

func TestParseBaseURLWithCustomRules(t *testing.T) {
	html := `<html><head><base href="https://example.com/"></head><body><img src="image.jpg"></body></html>`
	baseURL, _, err := findContentUsingCustomRules(strings.NewReader(html), "img")
	if err != nil {
		t.Fatalf(`Scraping error: %v`, err)
	}

	if baseURL != "https://example.com/" {
		t.Errorf(`Unexpected base URL, got %q instead of "https://example.com/"`, baseURL)
	}
}

func TestParseMultipleBaseURLWithCustomRules(t *testing.T) {
	html := `<html><head><base href="https://example.com/"><base href="https://example.org/"/></head><body><img src="image.jpg"></body></html>`
	baseURL, _, err := findContentUsingCustomRules(strings.NewReader(html), "img")
	if err != nil {
		t.Fatalf(`Scraping error: %v`, err)
	}

	if baseURL != "https://example.com/" {
		t.Errorf(`Unexpected base URL, got %q instead of "https://example.com/"`, baseURL)
	}
}

func TestParseRelativeBaseURLWithCustomRules(t *testing.T) {
	html := `<html><head><base href="/test"></head><body><img src="image.jpg"></body></html>`
	baseURL, _, err := findContentUsingCustomRules(strings.NewReader(html), "img")
	if err != nil {
		t.Fatalf(`Scraping error: %v`, err)
	}

	if baseURL != "" {
		t.Errorf(`Unexpected base URL, got %q`, baseURL)
	}
}

func TestParseEmptyBaseURLWithCustomRules(t *testing.T) {
	html := `<html><head><base href=" "></head><body><img src="image.jpg"></body></html>`
	baseURL, _, err := findContentUsingCustomRules(strings.NewReader(html), "img")
	if err != nil {
		t.Fatalf(`Scraping error: %v`, err)
	}

	if baseURL != "" {
		t.Errorf(`Unexpected base URL, got %q instead of ""`, baseURL)
	}
}

func TestParseMissingBaseURLWithCustomRules(t *testing.T) {
	html := `<html><head></head><body><img src="image.jpg"></body></html>`
	baseURL, _, err := findContentUsingCustomRules(strings.NewReader(html), "img")
	if err != nil {
		t.Fatalf(`Scraping error: %v`, err)
	}

	if baseURL != "" {
		t.Errorf(`Unexpected base URL, got %q instead of ""`, baseURL)
	}
}

func TestScrapeWebsite(t *testing.T) {
	var err error
	if config.Opts, err = config.Parse([]string{"FETCHER_ALLOW_PRIVATE_NETWORKS=1"}); err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`<html><body><nav>Menu</nav><article><p>Hello</p></article></body></html>`))
	}))
	defer server.Close()

	baseURL, content, err := ScrapeWebsite(fetcher.NewRequestBuilder(), server.URL+"/post", "article p")
	if err != nil {
		t.Fatal(err)
	}
	if baseURL != server.URL+"/post" {
		t.Errorf("unexpected base URL %q", baseURL)
	}
	if content != "<p>Hello</p>" {
		t.Errorf("unexpected content %q", content)
	}
}

func TestScrapeWebsiteRejectsNonHTML(t *testing.T) {
	var err error
	if config.Opts, err = config.Parse([]string{"FETCHER_ALLOW_PRIVATE_NETWORKS=1"}); err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
	}))
	defer server.Close()

	if _, _, err := ScrapeWebsite(fetcher.NewRequestBuilder(), server.URL, "article"); err == nil {
		t.Error("expected an error for a non-HTML document")
	}
}

func TestIsValidScraperRules(t *testing.T) {
	for rules, valid := range map[string]bool{"": true, "article > p, .content": true, "div[": false, "<p>": false} {
		if got := IsValidScraperRules(rules); got != valid {
			t.Errorf("IsValidScraperRules(%q) = %v, want %v", rules, got, valid)
		}
	}
}
