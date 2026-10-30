// Copyright The Miniflux Authors. Modified by The sss Authors.
// Derived from Miniflux (https://github.com/miniflux/v2).
// SPDX-License-Identifier: Apache-2.0

package model

// HomePages returns the list of available home pages.
func HomePages() map[string]string {
	return map[string]string{
		"unread":     "Unread",
		"starred":    "Starred",
		"history":    "History",
		"feeds":      "Feeds",
		"categories": "Categories",
	}
}
