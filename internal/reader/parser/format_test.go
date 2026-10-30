// Copyright The Miniflux Authors. Modified by The sss Authors.
// Derived from Miniflux (https://github.com/miniflux/v2).
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"strings"
	"testing"
)

func TestDetectFeedFormat(t *testing.T) {
	tests := []struct {
		name, data, want string
	}{
		// sss: RDF is reported as rss (gofeed parses RDF with its RSS parser).
		{"DetectRDF", `<?xml version="1.0"?><rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#" xmlns="http://my.netscape.com/rdf/simple/0.9/"></rdf:RDF>`, FormatRSS},
		{"DetectRSS", `<?xml version="1.0"?><rss version="2.0"><channel></channel></rss>`, FormatRSS},
		{"DetectAtom10", `<?xml version="1.0" encoding="utf-8"?><feed xmlns="http://www.w3.org/2005/Atom"></feed>`, FormatAtom},
		{"DetectAtom03", `<?xml version="1.0" encoding="utf-8"?><feed version="0.3" xmlns="http://purl.org/atom/ns#" xmlns:dc="http://purl.org/dc/elements/1.1/" xml:lang="en"></feed>`, FormatAtom},
		{"DetectAtomWithISOCharset", `<?xml version="1.0" encoding="ISO-8859-15"?><feed xmlns="http://www.w3.org/2005/Atom"></feed>`, FormatAtom},
		{"DetectJSON", `
	{
		"version" : "https://jsonfeed.org/version/1",
		"title" : "Example"
	}
	`, FormatJSON},
		{"DetectUnknown", `
	<!DOCTYPE html> <html> </html>
	`, FormatUnknown},
		{"DetectJSONWithLargeLeadingWhitespace", strings.Repeat(" ", 10000) + `{
		"version" : "https://jsonfeed.org/version/1",
		"title" : "Example with lots of leading whitespace"
	}`, FormatJSON},
		{"DetectJSONWithMixedWhitespace", strings.Repeat("\n\t  ", 10000) + `{
		"version" : "https://jsonfeed.org/version/1",
		"title" : "Example with mixed whitespace"
	}`, FormatJSON},
		{"DetectOnlyWhitespace", strings.Repeat(" \t\n\r", 10000), FormatUnknown},
		{"DetectJSONSmallerThanBuffer", `{"version":"1"}`, FormatJSON},
		{"DetectJSONWithWhitespaceSmallerThanBuffer", `  {"title":"test"}  `, FormatJSON},
		{"DetectJSONWithUTF8BOM", "\xEF\xBB\xBF" + `{"version":"https://jsonfeed.org/version/1.1","title":"Example"}`, FormatJSON},
	}
	for _, tc := range tests {
		if got := DetectFeedFormat(strings.NewReader(tc.data)); got != tc.want {
			t.Errorf("%s: wrong format detected: %q instead of %q", tc.name, got, tc.want)
		}
	}
}
