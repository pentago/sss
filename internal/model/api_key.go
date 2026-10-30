// Copyright The Miniflux Authors. Modified by The sss Authors.
// Derived from Miniflux (https://github.com/miniflux/v2).
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"time"
)

// APIKey represents an application API key.
// We need to use a pointer for LastUsedAt,
// as the value obtained from the database might sometimes be nil.
type APIKey struct {
	ID          int64      `json:"id"`
	UserID      int64      `json:"user_id"`
	Token       string     `json:"token"`
	Description string     `json:"description"`
	LastUsedAt  *time.Time `json:"last_used_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

// APIKeys represents a collection of API Key.
type APIKeys []APIKey

// APIKeyCreationRequest represents the request to create a new API Key.
type APIKeyCreationRequest struct {
	Description string `json:"description"`
}
