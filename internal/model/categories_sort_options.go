// Copyright The Miniflux Authors. Modified by The sss Authors.
// Derived from Miniflux (https://github.com/miniflux/v2).
// SPDX-License-Identifier: Apache-2.0

package model

// CategoriesSortingOptions returns the available category sorting orders and their labels.
func CategoriesSortingOptions() map[string]string {
	return map[string]string{
		"unread_count": "Unread count",
		"alphabetical": "Alphabetical",
	}
}
