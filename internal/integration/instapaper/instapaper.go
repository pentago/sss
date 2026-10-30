// Copyright The Miniflux Authors. Modified by The sss Authors.
// Derived from Miniflux (https://github.com/miniflux/v2).
// SPDX-License-Identifier: Apache-2.0

package instapaper

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"sss/internal/http/client"
	"sss/internal/version"
)

const defaultClientTimeout = 10 * time.Second

// Client is a Instapaper API client.
type Client struct {
	username string
	password string
}

// NewClient returns a Instapaper client.
func NewClient(username, password string) *Client {
	return &Client{username: username, password: password}
}

// AddURL saves the URL to Instapaper.
func (c *Client) AddURL(entryURL, entryTitle string) error {
	if c.username == "" || c.password == "" {
		return errors.New("instapaper: missing username or password")
	}

	values := url.Values{}
	values.Add("url", entryURL)
	values.Add("title", entryTitle)

	apiEndpoint := "https://www.instapaper.com/api/add?" + values.Encode()
	request, err := http.NewRequest(http.MethodGet, apiEndpoint, nil)
	if err != nil {
		return fmt.Errorf("instapaper: unable to create request: %v", err)
	}

	request.SetBasicAuth(c.username, c.password)
	request.Header.Set("User-Agent", "sss/"+version.Version)

	httpClient := client.NewClientWithOptions(client.Options{Timeout: defaultClientTimeout})
	response, err := httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("instapaper: unable to send request: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusCreated {
		return fmt.Errorf("instapaper: unable to add URL: url=%s status=%d", apiEndpoint, response.StatusCode)
	}

	return nil
}
