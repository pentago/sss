// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

package gotify

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"sss/internal/http/client"
	"sss/internal/model"
	"sss/internal/urllib"
)

// Client is a Gotify API client.
type Client struct {
	baseURL  string
	token    string
	priority int
}

// NewClient returns a Gotify client.
func NewClient(baseURL, token string, priority int) *Client {
	if priority < 0 {
		priority = 0
	}

	return &Client{
		baseURL:  baseURL,
		token:    token,
		priority: priority,
	}
}

// Payload represents a Gotify push notification message payload.
type Payload struct {
	Title    string         `json:"title,omitempty"`
	Message  string         `json:"message"`
	Priority int            `json:"priority,omitempty"`
	Extras   map[string]any `json:"extras,omitempty"`
}

// SendMessages sends one Gotify notification per entry.
func (c *Client) SendMessages(feed *model.Feed, entries model.Entries) error {
	if c.baseURL == "" || c.token == "" {
		return errors.New("gotify: missing base URL or token")
	}

	apiEndpoint := strings.TrimRight(c.baseURL, "/")
	if !strings.HasSuffix(apiEndpoint, "/message") {
		var err error
		apiEndpoint, err = urllib.JoinBaseURLAndPath(c.baseURL, "/message")
		if err != nil {
			return fmt.Errorf("gotify: invalid API endpoint: %w", err)
		}
	}

	title := feed.Title

	for _, entry := range entries {
		message := entry.Title
		if message == "" {
			message = entry.URL
		}

		if message == "" {
			continue
		}

		slog.Debug("Sending Gotify notification",
			slog.String("url", c.baseURL),
			slog.String("title", title),
			slog.String("entry_title", entry.Title),
			slog.String("entry_url", entry.URL),
		)

		extras := make(map[string]any)
		if entry.URL != "" {
			extras["client::notification"] = map[string]any{
				"click": map[string]any{
					"url": entry.URL,
				},
			}
		}

		msg := &Payload{
			Title:    title,
			Message:  message,
			Priority: c.priority,
			Extras:   extras,
		}

		response, err := client.NewRequestBuilder(apiEndpoint).
			WithMethod(http.MethodPost).
			WithHeader("X-Gotify-Key", c.token).
			WithJSON(msg).
			Do()
		if err != nil {
			return fmt.Errorf("gotify: %w", err)
		}
		defer response.Body.Close()

		if response.StatusCode >= 400 {
			return fmt.Errorf("gotify: unable to send message: url=%s status=%d", apiEndpoint, response.StatusCode)
		}
	}

	return nil
}
