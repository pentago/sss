// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

package response

import (
	"strconv"
	"strings"
)

// gzipOK reports whether the client accepts gzip: listed either bare or with a
// non-zero q value. A client that never lists gzip gets an identity response,
// which is always valid per HTTP semantics.
func gzipOK(acceptEncoding string) bool {
	for enc := range strings.SplitSeq(acceptEncoding, ",") {
		enc = strings.TrimSpace(enc)
		if enc != gzipEncoding && !strings.HasPrefix(enc, gzipEncoding+";") {
			continue
		}

		if qi := strings.IndexByte(enc, ';'); qi > -1 {
			qstr := strings.TrimSpace(strings.TrimPrefix(enc[qi:], ";"))
			qstr = strings.TrimPrefix(qstr, "q=")
			if q, err := strconv.ParseFloat(qstr, 64); err != nil || q == 0 {
				return false
			}
		}

		return true
	}
	return false
}
