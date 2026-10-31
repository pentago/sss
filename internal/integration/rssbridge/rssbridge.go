// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

// Package rssbridge queries an RSS-Bridge instance for feeds matching a website.
package rssbridge

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"sss/internal/http/client"
)

const defaultClientTimeout = 30 * time.Second

// Bridge is a feed proposed by RSS-Bridge.
type Bridge struct {
	URL        string     `json:"url"`
	BridgeMeta BridgeMeta `json:"bridgeMeta"`
}

// BridgeMeta describes the bridge producing a feed.
type BridgeMeta struct {
	Name string `json:"name"`
}

// DetectBridges asks RSS-Bridge (action=findfeed) for feeds of websiteURL.
func DetectBridges(rssBridgeURL, rssBridgeToken, websiteURL string) ([]*Bridge, error) {
	endpointURL, err := url.Parse(rssBridgeURL)
	if err != nil {
		return nil, fmt.Errorf("rssbridge: unable to parse bridge URL: %w", err)
	}
	values := endpointURL.Query()
	if rssBridgeToken != "" {
		values.Add("token", rssBridgeToken)
	}
	values.Add("action", "findfeed")
	values.Add("format", "atom")
	values.Add("url", websiteURL)
	endpointURL.RawQuery = values.Encode()

	response, err := client.NewRequestBuilder(endpointURL.String()).
		WithTimeout(defaultClientTimeout).
		Do()
	if err != nil {
		return nil, fmt.Errorf("rssbridge: unable to execute request: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if response.StatusCode >= 400 {
		return nil, fmt.Errorf("rssbridge: unexpected status code %d", response.StatusCode)
	}

	var bridges []*Bridge
	if err := json.NewDecoder(response.Body).Decode(&bridges); err != nil {
		return nil, fmt.Errorf("rssbridge: unable to decode bridge response: %w", err)
	}
	for _, bridge := range bridges {
		if rest, ok := strings.CutPrefix(bridge.URL, "./"); ok {
			bridge.URL = rssBridgeURL + rest
		}
		if rssBridgeToken != "" {
			bridge.URL += "&token=" + rssBridgeToken
		}
	}
	return bridges, nil
}
