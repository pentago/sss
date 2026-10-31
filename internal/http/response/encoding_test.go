// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

package response

import "testing"

func TestGzipOK(t *testing.T) {
	t.Parallel()

	tests := []struct {
		acceptEncoding string
		want           bool
	}{
		{"", false},
		{"identity", false},
		{"deflate", false},
		{"br, identity", false},
		{"gzip", true},
		{"  gzip  ", true},
		{"gzip, deflate", true},
		{"deflate, gzip", true},
		{"gzip;q=0", false},
		{"gzip;q=0.0", false},
		{"gzip;q=0.5", true},
		{"deflate, gzip;q=1.0", true},
	}

	for _, test := range tests {
		t.Run(test.acceptEncoding, func(t *testing.T) {
			t.Parallel()

			if got := gzipOK(test.acceptEncoding); got != test.want {
				t.Errorf("gzipOK(%q) = %v, want %v", test.acceptEncoding, got, test.want)
			}
		})
	}
}
