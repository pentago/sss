// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

package matrixbot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"sss/internal/config"
	"sss/internal/crypto"
	"sss/internal/http/client"
	"sss/internal/version"
)

const defaultClientTimeout = 10 * time.Second

// Client is a Matrix API client.
type Client struct {
	matrixBaseURL string
}

// NewClient returns a Matrix client.
func NewClient(matrixBaseURL string) *Client {
	return &Client{matrixBaseURL: matrixBaseURL}
}

// Specs: https://spec.matrix.org/v1.8/client-server-api/#getwell-knownmatrixclient
func (c *Client) DiscoverEndpoints() (*DiscoveryEndpointResponse, error) {
	endpointURL, err := url.JoinPath(c.matrixBaseURL, "/.well-known/matrix/client")
	if err != nil {
		return nil, fmt.Errorf("matrix: unable to join base URL and path: %w", err)
	}

	request, err := http.NewRequest(http.MethodGet, endpointURL, nil)
	if err != nil {
		return nil, fmt.Errorf("matrix: unable to create request: %v", err)
	}

	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "sss/"+version.Version)

	httpClient := client.NewClientWithOptions(client.Options{Timeout: defaultClientTimeout, BlockPrivateNetworks: !config.Opts.IntegrationAllowPrivateNetworks})
	response, err := httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("matrix: unable to send request: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode >= 400 {
		return nil, fmt.Errorf("matrix: unexpected response from %s status code is %d", endpointURL, response.StatusCode)
	}

	var discoveryEndpointResponse DiscoveryEndpointResponse
	if err := json.NewDecoder(response.Body).Decode(&discoveryEndpointResponse); err != nil {
		return nil, fmt.Errorf("matrix: unable to decode discovery response: %w", err)
	}

	return &discoveryEndpointResponse, nil
}

// Specs https://spec.matrix.org/v1.8/client-server-api/#post_matrixclientv3login
func (c *Client) Login(homeServerURL, matrixUsername, matrixPassword string) (*LoginResponse, error) {
	endpointURL, err := url.JoinPath(homeServerURL, "/_matrix/client/v3/login")
	if err != nil {
		return nil, fmt.Errorf("matrix: unable to join base URL and path: %w", err)
	}

	loginRequest := LoginRequest{
		Type: "m.login.password",
		Identifier: UserIdentifier{
			Type: "m.id.user",
			User: matrixUsername,
		},
		Password: matrixPassword,
	}

	requestBody, err := json.Marshal(loginRequest)
	if err != nil {
		return nil, fmt.Errorf("matrix: unable to encode request body: %v", err)
	}

	request, err := http.NewRequest(http.MethodPost, endpointURL, bytes.NewReader(requestBody))
	if err != nil {
		return nil, fmt.Errorf("matrix: unable to create request: %v", err)
	}

	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "sss/"+version.Version)

	httpClient := client.NewClientWithOptions(client.Options{Timeout: defaultClientTimeout, BlockPrivateNetworks: !config.Opts.IntegrationAllowPrivateNetworks})
	response, err := httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("matrix: unable to send request: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode >= 400 {
		return nil, fmt.Errorf("matrix: unexpected response from %s status code is %d", endpointURL, response.StatusCode)
	}

	var loginResponse LoginResponse
	if err := json.NewDecoder(response.Body).Decode(&loginResponse); err != nil {
		return nil, fmt.Errorf("matrix: unable to decode login response: %w", err)
	}

	return &loginResponse, nil
}

// Specs https://spec.matrix.org/v1.8/client-server-api/#put_matrixclientv3roomsroomidsendeventtypetxnid
func (c *Client) SendFormattedTextMessage(homeServerURL, accessToken, roomID, textMessage, formattedMessage string) (*RoomEventResponse, error) {
	txnID := crypto.GenerateRandomStringHex(10)
	endpointURL, err := url.JoinPath(homeServerURL, "/_matrix/client/v3/rooms/", roomID, "/send/m.room.message/", txnID)
	if err != nil {
		return nil, fmt.Errorf("matrix: unable to join base URL and path: %w", err)
	}

	messageEvent := TextMessageEventRequest{
		MsgType:       "m.text",
		Body:          textMessage,
		Format:        "org.matrix.custom.html",
		FormattedBody: formattedMessage,
	}

	requestBody, err := json.Marshal(messageEvent)
	if err != nil {
		return nil, fmt.Errorf("matrix: unable to encode request body: %v", err)
	}

	request, err := http.NewRequest(http.MethodPut, endpointURL, bytes.NewReader(requestBody))
	if err != nil {
		return nil, fmt.Errorf("matrix: unable to create request: %v", err)
	}

	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "sss/"+version.Version)
	request.Header.Set("Authorization", "Bearer "+accessToken)

	httpClient := client.NewClientWithOptions(client.Options{Timeout: defaultClientTimeout, BlockPrivateNetworks: !config.Opts.IntegrationAllowPrivateNetworks})
	response, err := httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("matrix: unable to send request: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode >= 400 {
		return nil, fmt.Errorf("matrix: unexpected response from %s status code is %d", endpointURL, response.StatusCode)
	}

	var eventResponse RoomEventResponse
	if err := json.NewDecoder(response.Body).Decode(&eventResponse); err != nil {
		return nil, fmt.Errorf("matrix: unable to decode event response: %w", err)
	}

	return &eventResponse, nil
}

// HomeServerInformation is the m.homeserver discovery entry.
type HomeServerInformation struct {
	BaseURL string `json:"base_url"`
}

// IdentityServerInformation is the m.identity_server discovery entry.
type IdentityServerInformation struct {
	BaseURL string `json:"base_url"`
}

// DiscoveryEndpointResponse is the .well-known/matrix/client response.
type DiscoveryEndpointResponse struct {
	HomeServerInformation     HomeServerInformation     `json:"m.homeserver"`
	IdentityServerInformation IdentityServerInformation `json:"m.identity_server"`
}

// UserIdentifier identifies the user in a login request.
type UserIdentifier struct {
	Type string `json:"type"`
	User string `json:"user"`
}

// LoginRequest is a password login request body.
type LoginRequest struct {
	Type       string         `json:"type"`
	Identifier UserIdentifier `json:"identifier"`
	Password   string         `json:"password"`
}

// LoginResponse is the login response holding the access token.
type LoginResponse struct {
	UserID      string `json:"user_id"`
	AccessToken string `json:"access_token"`
	DeviceID    string `json:"device_id"`
	HomeServer  string `json:"home_server"`
}

// TextMessageEventRequest is an m.room.message event body.
type TextMessageEventRequest struct {
	MsgType       string `json:"msgtype"`
	Body          string `json:"body"`
	Format        string `json:"format"`
	FormattedBody string `json:"formatted_body"`
}

// RoomEventResponse is the response to sending a room event.
type RoomEventResponse struct {
	EventID string `json:"event_id"`
}
