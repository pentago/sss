// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

package apprise

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"sss/internal/config"
	"sss/internal/http/client"
	"sss/internal/model"
	"sss/internal/urllib"
	"sss/internal/version"
)

const defaultClientTimeout = 10 * time.Second

// Client is a Apprise API client.
type Client struct {
	servicesURL string
	baseURL     string
}

// NewClient returns a Apprise client.
func NewClient(serviceURL, baseURL string) *Client {
	return &Client{servicesURL: serviceURL, baseURL: baseURL}
}

// SendNotification sends one Apprise notification per entry.
func (c *Client) SendNotification(feed *model.Feed, entries model.Entries) error {
	if c.baseURL == "" || c.servicesURL == "" {
		return errors.New("apprise: missing base URL or services URL")
	}

	for _, entry := range entries {
		message := "[" + entry.Title + "]" + "(" + entry.URL + ")" + "\n\n"
		apiEndpoint, err := urllib.JoinBaseURLAndPath(c.baseURL, "/notify")
		if err != nil {
			return fmt.Errorf(`apprise: invalid API endpoint: %v`, err)
		}

		requestBody, err := json.Marshal(map[string]any{
			"urls":  c.servicesURL,
			"body":  message,
			"title": feed.Title,
		})
		if err != nil {
			return fmt.Errorf("apprise: unable to encode request body: %v", err)
		}

		request, err := http.NewRequest(http.MethodPost, apiEndpoint, bytes.NewReader(requestBody))
		if err != nil {
			return fmt.Errorf("apprise: unable to create request: %v", err)
		}

		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("User-Agent", "sss/"+version.Version)

		slog.Debug("Sending Apprise notification",
			slog.String("apprise_url", c.baseURL),
			slog.String("services_url", c.servicesURL),
			slog.String("title", feed.Title),
			slog.String("body", message),
			slog.String("entry_url", entry.URL),
		)

		httpClient := client.NewClientWithOptions(client.Options{Timeout: defaultClientTimeout, BlockPrivateNetworks: !config.Opts.IntegrationAllowPrivateNetworks})
		response, err := httpClient.Do(request)
		if err != nil {
			return fmt.Errorf("apprise: unable to send request: %v", err)
		}
		response.Body.Close()

		if response.StatusCode >= 400 {
			return fmt.Errorf("apprise: unable to send a notification: url=%s status=%d", apiEndpoint, response.StatusCode)
		}
	}

	return nil
}
