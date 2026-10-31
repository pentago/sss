// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"strings"
)

// Enclosure represents an attachment.
type Enclosure struct {
	ID               int64  `json:"id"`
	UserID           int64  `json:"user_id"`
	EntryID          int64  `json:"entry_id"`
	URL              string `json:"url"`
	MimeType         string `json:"mime_type"`
	Size             int64  `json:"size"`
	MediaProgression int64  `json:"media_progression"`
}

// EnclosureUpdateRequest represents the request to update an enclosure.
type EnclosureUpdateRequest struct {
	MediaProgression int64 `json:"media_progression"`
}

// IsAudio reports whether the enclosure is an audio file.
func (e *Enclosure) IsAudio() bool {
	return strings.HasPrefix(strings.ToLower(e.MimeType), "audio/")
}

// IsVideo reports whether the enclosure is a video file.
func (e *Enclosure) IsVideo() bool {
	return strings.HasPrefix(strings.ToLower(e.MimeType), "video/")
}

// IsImage reports whether the enclosure is an image.
func (e *Enclosure) IsImage() bool {
	mimeType := strings.ToLower(e.MimeType)
	if strings.HasPrefix(mimeType, "image/") {
		return true
	}
	mediaURL := strings.ToLower(e.URL)
	return strings.HasSuffix(mediaURL, ".jpg") || strings.HasSuffix(mediaURL, ".jpeg") || strings.HasSuffix(mediaURL, ".png") || strings.HasSuffix(mediaURL, ".gif")
}

// EnclosureList represents a list of attachments.
type EnclosureList []*Enclosure
