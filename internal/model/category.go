// Copyright The Miniflux Authors. Modified by The sss Authors.
// Derived from Miniflux (https://github.com/miniflux/v2).
// SPDX-License-Identifier: Apache-2.0

package model

import "fmt"

// Category represents a feed category.
type Category struct {
	ID           int64  `json:"id"`
	Title        string `json:"title"`
	UserID       int64  `json:"user_id"`
	HideGlobally bool   `json:"hide_globally"`
	// Pointers are needed to avoid breaking /v1/categories?counts=true
	FeedCount   *int `json:"feed_count,omitempty"`
	TotalUnread *int `json:"total_unread,omitempty"`
}

// String returns a debug representation of the category.
func (c *Category) String() string {
	return fmt.Sprintf("ID=%d, UserID=%d, Title=%s", c.ID, c.UserID, c.Title)
}

// CategoryCreationRequest represents the request to create a category.
type CategoryCreationRequest struct {
	Title        string `json:"title"`
	HideGlobally bool   `json:"hide_globally"`
}

// CategoryModificationRequest represents the request to update a category.
type CategoryModificationRequest struct {
	Title        *string `json:"title"`
	HideGlobally *bool   `json:"hide_globally"`
}

// Patch updates the category with the modified values.
func (c *CategoryModificationRequest) Patch(category *Category) {
	if c.Title != nil {
		category.Title = *c.Title
	}

	if c.HideGlobally != nil {
		category.HideGlobally = *c.HideGlobally
	}
}

// Categories represents a list of categories.
type Categories []Category
