// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"io"

	"github.com/mmcdole/gofeed"
)

// Feed formats reported by discovery.
const (
	FormatRSS     = "rss"
	FormatAtom    = "atom"
	FormatJSON    = "json"
	FormatUnknown = "unknown"
)

// DetectFeedFormat guesses the feed format of r and rewinds it. RDF is reported as rss.
func DetectFeedFormat(r io.ReadSeeker) string {
	r.Seek(0, io.SeekStart)
	defer r.Seek(0, io.SeekStart)
	switch gofeed.DetectFeedType(r) {
	case gofeed.FeedTypeRSS:
		return FormatRSS
	case gofeed.FeedTypeAtom:
		return FormatAtom
	case gofeed.FeedTypeJSON:
		return FormatJSON
	}
	return FormatUnknown
}
