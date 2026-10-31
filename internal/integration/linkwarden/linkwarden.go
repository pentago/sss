// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

package linkwarden

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	"sss/internal/http/client"
	"sss/internal/urllib"
)

// Client is a Linkwarden API client.
type Client struct {
	baseURL      string
	apiKey       string
	collectionID *int64
}

type linkwardenCollection struct {
	ID *int64 `json:"id"`
}

type linkwardenRequest struct {
	URL        string                `json:"url"`
	Name       string                `json:"name"`
	Collection *linkwardenCollection `json:"collection,omitempty"`
}

// NewClient returns a Linkwarden client.
func NewClient(baseURL, apiKey string, collectionID *int64) *Client {
	return &Client{baseURL: baseURL, apiKey: apiKey, collectionID: collectionID}
}

// CreateBookmark saves the entry URL as a Linkwarden link.
func (c *Client) CreateBookmark(entryURL, entryTitle string) error {
	if c.baseURL == "" || c.apiKey == "" {
		return errors.New("linkwarden: missing base URL or API key")
	}

	apiEndpoint, err := urllib.JoinBaseURLAndPath(c.baseURL, "/api/v1/links")
	if err != nil {
		return fmt.Errorf(`linkwarden: invalid API endpoint: %v`, err)
	}

	payload := linkwardenRequest{
		URL:  entryURL,
		Name: entryTitle,
	}

	if c.collectionID != nil {
		payload.Collection = &linkwardenCollection{ID: c.collectionID}
	}

	response, err := client.NewRequestBuilder(apiEndpoint).
		WithMethod(http.MethodPost).
		WithJSON(payload).
		WithHeader("Authorization", "Bearer "+c.apiKey).
		Do()
	if err != nil {
		return fmt.Errorf("linkwarden: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode >= 400 {
		responseBody, err := io.ReadAll(response.Body)
		if err != nil {
			return fmt.Errorf("linkwarden: unable to read response body: %v", err)
		}

		return fmt.Errorf("linkwarden: unable to create link: status=%d body=%s", response.StatusCode, string(responseBody))
	}

	return nil
}
