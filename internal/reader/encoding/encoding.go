// Copyright The Miniflux Authors. Modified by The sss Authors.
// Derived from Miniflux (https://github.com/miniflux/v2).
// SPDX-License-Identifier: Apache-2.0

// Package encoding converts documents to UTF-8.
package encoding

import (
	"bytes"
	"fmt"
	"io"
	"unicode/utf8"

	"golang.org/x/net/html/charset"
)

// NewCharsetReader returns a reader converting r to UTF-8 using the
// Content-Type header or the document's <meta charset>. Valid UTF-8 is returned as-is.
func NewCharsetReader(r io.Reader, contentType string) (io.Reader, error) {
	buffer, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("encoding: unable to read input: %w", err)
	}
	if utf8.Valid(buffer) {
		return bytes.NewReader(buffer), nil
	}
	return charset.NewReader(bytes.NewReader(buffer), contentType)
}
