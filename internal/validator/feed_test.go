// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

package validator

import (
	"testing"

	"sss/internal/model"
)

func TestValidateFeedModificationProxyURL(t *testing.T) {
	tests := []struct {
		name     string
		proxyURL string
		wantErr  bool
	}{
		{
			name:     "empty proxy URL",
			proxyURL: "",
			wantErr:  false,
		},
		{
			name:     "valid proxy URL",
			proxyURL: "http://127.0.0.1:3128",
			wantErr:  false,
		},
		{
			name:     "invalid proxy URL",
			proxyURL: "example.org",
			wantErr:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			request := &model.FeedModificationRequest{ProxyURL: &tc.proxyURL}
			if err := ValidateFeedModification(nil, 0, 0, request); (err != nil) != tc.wantErr {
				t.Fatalf("expected error %v, got %v", tc.wantErr, err)
			}
		})
	}
}

// fakeStore reports no duplicates and every category as existing.
type fakeStore struct{ Store }

func (fakeStore) FeedURLExists(int64, string) bool               { return false }
func (fakeStore) AnotherFeedURLExists(int64, int64, string) bool { return false }
func (fakeStore) CategoryIDExists(int64, int64) (bool, error)    { return true, nil }

func TestValidateFeedRules(t *testing.T) {
	tests := []struct {
		name                         string
		scraper, rewrite, urlRewrite string
		wantErr                      bool
	}{
		{name: "valid", scraper: "article", rewrite: "add_youtube_video", urlRewrite: `rewrite("^(.*)$"|"$1")`},
		{name: "invalid scraper rules", scraper: "a[", wantErr: true},
		{name: "invalid rewrite rules", rewrite: "no_such_rule", wantErr: true},
		{name: "invalid URL rewrite rules", urlRewrite: "garbage", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			creation := &model.FeedCreationRequest{FeedURL: "https://example.org/feed", CategoryID: 1,
				ScraperRules: tc.scraper, RewriteRules: tc.rewrite, UrlRewriteRules: tc.urlRewrite}
			if err := ValidateFeedCreation(fakeStore{}, 1, creation); (err != nil) != tc.wantErr {
				t.Errorf("creation: expected error %v, got %v", tc.wantErr, err)
			}
			modification := &model.FeedModificationRequest{ScraperRules: model.OptionalString(tc.scraper),
				RewriteRules: model.OptionalString(tc.rewrite), UrlRewriteRules: model.OptionalString(tc.urlRewrite)}
			if err := ValidateFeedModification(fakeStore{}, 1, 1, modification); (err != nil) != tc.wantErr {
				t.Errorf("modification: expected error %v, got %v", tc.wantErr, err)
			}
		})
	}
}
