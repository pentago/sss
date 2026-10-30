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

// Html5MimeType will modify the actual MimeType to allow direct playback from HTML5 player for some kind of MimeType
func (e *Enclosure) Html5MimeType() string {
	if e.MimeType == "video/m4v" {
		return "video/x-m4v"
	}
	return e.MimeType
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

// FindMediaPlayerEnclosure returns the first enclosure that can be played by a media player.
func (el EnclosureList) FindMediaPlayerEnclosure() *Enclosure {
	for _, enclosure := range el {
		if enclosure.URL != "" {
			if enclosure.IsAudio() || enclosure.IsVideo() {
				return enclosure
			}
		}
	}

	return nil
}

// ContainsAudioOrVideo reports whether any enclosure is audio or video.
func (el EnclosureList) ContainsAudioOrVideo() bool {
	for _, enclosure := range el {
		if enclosure.IsAudio() || enclosure.IsVideo() {
			return true
		}
	}
	return false
}
