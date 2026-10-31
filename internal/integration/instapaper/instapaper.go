// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

package instapaper

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"sss/internal/http/client"
)

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
	response, err := client.NewRequestBuilder(apiEndpoint).
		WithBasicAuth(c.username, c.password).
		Do()
	if err != nil {
		return fmt.Errorf("instapaper: unable to send request: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusCreated {
		return fmt.Errorf("instapaper: unable to add URL: url=%s status=%d", apiEndpoint, response.StatusCode)
	}

	return nil
}
