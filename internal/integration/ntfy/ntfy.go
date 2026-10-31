// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

package ntfy

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"

	"sss/internal/config"
	"sss/internal/http/client"
	"sss/internal/model"
)

const defaultNtfyURL = "https://ntfy.sh"

// Client is a ntfy API client.
type Client struct {
	ntfyURL, ntfyTopic, ntfyApiToken, ntfyUsername, ntfyPassword, ntfyIconURL string
	ntfyInternalLinks                                                         bool
	ntfyPriority                                                              int
}

// NewClient returns a ntfy client.
func NewClient(ntfyURL, ntfyTopic, ntfyApiToken, ntfyUsername, ntfyPassword, ntfyIconURL string, ntfyInternalLinks bool, ntfyPriority int) *Client {
	if ntfyURL == "" {
		ntfyURL = defaultNtfyURL
	}
	return &Client{
		ntfyURL:           ntfyURL,
		ntfyTopic:         ntfyTopic,
		ntfyApiToken:      ntfyApiToken,
		ntfyUsername:      ntfyUsername,
		ntfyPassword:      ntfyPassword,
		ntfyIconURL:       ntfyIconURL,
		ntfyInternalLinks: ntfyInternalLinks,
		ntfyPriority:      ntfyPriority,
	}
}

// SendMessages sends one ntfy notification per entry.
func (c *Client) SendMessages(feed *model.Feed, entries model.Entries) error {
	for _, entry := range entries {
		ntfyMessage := &ntfyMessage{
			Topic:    c.ntfyTopic,
			Message:  entry.Title,
			Title:    feed.Title,
			Priority: c.ntfyPriority,
			Click:    entry.URL,
		}

		if c.ntfyIconURL != "" {
			ntfyMessage.Icon = c.ntfyIconURL
		}

		if c.ntfyInternalLinks {
			url, err := url.Parse(config.Opts.BaseURL)
			if err != nil {
				slog.Error("Unable to parse base URL", slog.Any("error", err))
			} else {
				ntfyMessage.Click = fmt.Sprintf("%s%s%d", url, "/unread/entry/", entry.ID)
			}
		}

		slog.Debug("Sending Ntfy message",
			slog.String("url", c.ntfyURL),
			slog.String("topic", c.ntfyTopic),
			slog.Int("priority", ntfyMessage.Priority),
			slog.String("message", ntfyMessage.Message),
			slog.String("entry_url", ntfyMessage.Click),
		)

		if err := c.makeRequest(ntfyMessage); err != nil {
			return err
		}
	}

	return nil
}

func (c *Client) makeRequest(payload any) error {
	request := client.NewRequestBuilder(c.ntfyURL).
		WithMethod(http.MethodPost).
		WithJSON(payload)

	// See https://docs.ntfy.sh/publish/#access-tokens
	if c.ntfyApiToken != "" {
		request = request.WithHeader("Authorization", "Bearer "+c.ntfyApiToken)
	}

	// See https://docs.ntfy.sh/publish/#username-password
	if c.ntfyUsername != "" && c.ntfyPassword != "" {
		request = request.WithBasicAuth(c.ntfyUsername, c.ntfyPassword)
	}

	response, err := request.Do()
	if err != nil {
		return fmt.Errorf("ntfy: unable to send request: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode >= 400 {
		return fmt.Errorf("ntfy: incorrect response status code %d for url %s", response.StatusCode, c.ntfyURL)
	}

	return nil
}

// See https://docs.ntfy.sh/publish/#publish-as-json
type ntfyMessage struct {
	Topic    string `json:"topic"`
	Message  string `json:"message"`
	Title    string `json:"title"`
	Priority int    `json:"priority,omitempty"`
	Icon     string `json:"icon,omitempty"` // https://docs.ntfy.sh/publish/#icons
	Click    string `json:"click,omitempty"`
}
