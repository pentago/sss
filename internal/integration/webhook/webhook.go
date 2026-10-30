// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"sss/internal/crypto"
	"sss/internal/http/client"
	"sss/internal/model"
)

const (
	// NewEntriesEventType is the event type for newly fetched entries.
	NewEntriesEventType = "new_entries"
	// SaveEntryEventType is the event type for an entry saved by the user.
	SaveEntryEventType = "save_entry"
)

// Client is a webhook API client.
type Client struct {
	webhookURL    string
	webhookSecret string
}

// NewClient returns a webhook client.
func NewClient(webhookURL, webhookSecret string) *Client {
	return &Client{webhookURL, webhookSecret}
}

// SendSaveEntryWebhookEvent posts a signed save_entry event for the entry.
func (c *Client) SendSaveEntryWebhookEvent(entry *model.Entry) error {
	return c.makeRequest(SaveEntryEventType, &WebhookSaveEntryEvent{
		EventType: SaveEntryEventType,
		Entry: &WebhookEntry{
			ID:          entry.ID,
			UserID:      entry.UserID,
			FeedID:      entry.FeedID,
			Status:      entry.Status,
			Hash:        entry.Hash,
			Title:       entry.Title,
			URL:         entry.URL,
			CommentsURL: entry.CommentsURL,
			Date:        entry.Date,
			CreatedAt:   entry.CreatedAt,
			ChangedAt:   entry.ChangedAt,
			Content:     entry.Content,
			Author:      entry.Author,
			ShareCode:   entry.ShareCode,
			Starred:     entry.Starred,
			ReadingTime: entry.ReadingTime,
			Enclosures:  entry.Enclosures,
			Tags:        entry.Tags,
			Feed: &WebhookFeed{
				ID:         entry.Feed.ID,
				UserID:     entry.Feed.UserID,
				CategoryID: entry.Feed.Category.ID,
				Category:   &WebhookCategory{ID: entry.Feed.Category.ID, Title: entry.Feed.Category.Title},
				FeedURL:    entry.Feed.FeedURL,
				SiteURL:    entry.Feed.SiteURL,
				Title:      entry.Feed.Title,
				CheckedAt:  entry.Feed.CheckedAt,
			},
		},
	})
}

// SendNewEntriesWebhookEvent posts a signed new_entries event; no-op without entries.
func (c *Client) SendNewEntriesWebhookEvent(feed *model.Feed, entries model.Entries) error {
	if len(entries) == 0 {
		return nil
	}

	webhookEntries := make([]*WebhookEntry, 0, len(entries))
	for _, entry := range entries {
		webhookEntries = append(webhookEntries, &WebhookEntry{
			ID:          entry.ID,
			UserID:      entry.UserID,
			FeedID:      entry.FeedID,
			Status:      entry.Status,
			Hash:        entry.Hash,
			Title:       entry.Title,
			URL:         entry.URL,
			CommentsURL: entry.CommentsURL,
			Date:        entry.Date,
			CreatedAt:   entry.CreatedAt,
			ChangedAt:   entry.ChangedAt,
			Content:     entry.Content,
			Author:      entry.Author,
			ShareCode:   entry.ShareCode,
			Starred:     entry.Starred,
			ReadingTime: entry.ReadingTime,
			Enclosures:  entry.Enclosures,
			Tags:        entry.Tags,
		})
	}
	return c.makeRequest(NewEntriesEventType, &WebhookNewEntriesEvent{
		EventType: NewEntriesEventType,
		Feed: &WebhookFeed{
			ID:         feed.ID,
			UserID:     feed.UserID,
			CategoryID: feed.Category.ID,
			Category:   &WebhookCategory{ID: feed.Category.ID, Title: feed.Category.Title},
			FeedURL:    feed.FeedURL,
			SiteURL:    feed.SiteURL,
			Title:      feed.Title,
			CheckedAt:  feed.CheckedAt,
		},
		Entries: webhookEntries,
	})
}

func (c *Client) makeRequest(eventType string, payload any) error {
	if c.webhookURL == "" {
		return errors.New(`webhook: missing webhook URL`)
	}

	requestBody, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("webhook: unable to encode request body: %v", err)
	}

	response, err := client.NewRequestBuilder(c.webhookURL).
		WithMethod(http.MethodPost).
		WithJSONBody(requestBody).
		WithHeader("X-Sss-Signature", crypto.GenerateSHA256Hmac(c.webhookSecret, requestBody)).
		WithHeader("X-Sss-Event-Type", eventType).
		Do()
	if err != nil {
		return fmt.Errorf("webhook: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode >= 400 {
		return fmt.Errorf("webhook: incorrect response status code %d for url %s", response.StatusCode, c.webhookURL)
	}

	return nil
}

// WebhookFeed is the feed as serialized in webhook payloads.
type WebhookFeed struct {
	ID         int64            `json:"id"`
	UserID     int64            `json:"user_id"`
	CategoryID int64            `json:"category_id"`
	Category   *WebhookCategory `json:"category,omitempty"`
	FeedURL    string           `json:"feed_url"`
	SiteURL    string           `json:"site_url"`
	Title      string           `json:"title"`
	CheckedAt  time.Time        `json:"checked_at"`
}

// WebhookCategory is the category as serialized in webhook payloads.
type WebhookCategory struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
}

// WebhookEntry is the entry as serialized in webhook payloads.
type WebhookEntry struct {
	ID          int64               `json:"id"`
	UserID      int64               `json:"user_id"`
	FeedID      int64               `json:"feed_id"`
	Status      string              `json:"status"`
	Hash        string              `json:"hash"`
	Title       string              `json:"title"`
	URL         string              `json:"url"`
	CommentsURL string              `json:"comments_url"`
	Date        time.Time           `json:"published_at"`
	CreatedAt   time.Time           `json:"created_at"`
	ChangedAt   time.Time           `json:"changed_at"`
	Content     string              `json:"content"`
	Author      string              `json:"author"`
	ShareCode   string              `json:"share_code"`
	Starred     bool                `json:"starred"`
	ReadingTime int                 `json:"reading_time"`
	Enclosures  model.EnclosureList `json:"enclosures"`
	Tags        []string            `json:"tags"`
	Feed        *WebhookFeed        `json:"feed,omitempty"`
}

// WebhookNewEntriesEvent is the new_entries event payload.
type WebhookNewEntriesEvent struct {
	EventType string          `json:"event_type"`
	Feed      *WebhookFeed    `json:"feed"`
	Entries   []*WebhookEntry `json:"entries"`
}

// WebhookSaveEntryEvent is the save_entry event payload.
type WebhookSaveEntryEvent struct {
	EventType string        `json:"event_type"`
	Entry     *WebhookEntry `json:"entry"`
}
