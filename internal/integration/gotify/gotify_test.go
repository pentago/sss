// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

package gotify

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"sss/internal/config"
	"sss/internal/model"
)

func TestSendMessagesWithoutPriorityOrExtras(t *testing.T) {
	configureIntegrationAllowPrivateNetworksOption(t)

	var receivedBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read body: %v", err)
		}

		if err := json.Unmarshal(body, &receivedBody); err != nil {
			t.Fatalf("failed to unmarshal request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":2,"message":"plain message"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token", 0)
	err := client.SendMessages(&model.Feed{}, model.Entries{{Title: "plain message"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedBody["message"] != "plain message" {
		t.Errorf("expected message %q, got %v", "plain message", receivedBody["message"])
	}
	if _, exists := receivedBody["title"]; exists {
		t.Errorf("expected title to be omitted, got %v", receivedBody["title"])
	}
	if _, exists := receivedBody["priority"]; exists {
		t.Errorf("expected priority to be omitted, got %v", receivedBody["priority"])
	}
	if _, exists := receivedBody["extras"]; exists {
		t.Errorf("expected extras to be omitted, got %v", receivedBody["extras"])
	}
}

func TestSendMessagesEndpointFormats(t *testing.T) {
	configureIntegrationAllowPrivateNetworksOption(t)

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if r.URL.Path != "/message" {
			t.Errorf("expected path /message, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":1}`))
	}))
	defer server.Close()

	testURLs := []string{
		server.URL,
		server.URL + "/",
		server.URL + "/message",
		server.URL + "/message/",
	}

	for _, u := range testURLs {
		client := NewClient(u, "test-token", 0)
		if err := client.SendMessages(&model.Feed{Title: "title"}, model.Entries{{Title: "msg", URL: "https://example.com"}}); err != nil {
			t.Errorf("failed for URL %q: %v", u, err)
		}
	}

	if requestCount != len(testURLs) {
		t.Errorf("expected %d requests, got %d", len(testURLs), requestCount)
	}
}

func TestSendMessagesServerErrors(t *testing.T) {
	configureIntegrationAllowPrivateNetworksOption(t)

	tests := []struct {
		name        string
		statusCode  int
		response    string
		errContains string
	}{
		{
			name:        "unauthorized 401",
			statusCode:  http.StatusUnauthorized,
			response:    `{"error":"Unauthorized","errorCode":401}`,
			errContains: "status=401",
		},
		{
			name:        "bad request 400",
			statusCode:  http.StatusBadRequest,
			response:    `{"error":"Bad Request","errorCode":400}`,
			errContains: "status=400",
		},
		{
			name:        "internal server error 500",
			statusCode:  http.StatusInternalServerError,
			response:    `{"error":"Internal Error","errorCode":500}`,
			errContains: "status=500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.response))
			}))
			defer server.Close()

			client := NewClient(server.URL, "token", 0)
			err := client.SendMessages(&model.Feed{Title: "title"}, model.Entries{{Title: "msg", URL: "https://example.com"}})
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.errContains) {
				t.Fatalf("expected error containing %q, got %q", tt.errContains, err.Error())
			}
		})
	}
}

func TestSendMessages(t *testing.T) {
	configureIntegrationAllowPrivateNetworksOption(t)

	var receivedToken string
	var receivedContentType string
	var recordedRequests []map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedToken = r.Header.Get("X-Gotify-Key")
		receivedContentType = r.Header.Get("Content-Type")

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read body: %v", err)
		}

		var parsed map[string]any
		if err := json.Unmarshal(body, &parsed); err != nil {
			t.Fatalf("failed to unmarshal request body: %v", err)
		}

		recordedRequests = append(recordedRequests, parsed)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":1}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "token", 6)
	feed := &model.Feed{Title: "Feed Title"}
	entries := model.Entries{
		{
			Title: "First Article",
			URL:   "https://example.com/first",
		},
		{
			Title: "",
			URL:   "https://example.com/second",
		},
	}

	err := client.SendMessages(feed, entries)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(recordedRequests) != 2 {
		t.Fatalf("expected 2 recorded requests, got %d", len(recordedRequests))
	}

	if receivedToken != "token" {
		t.Errorf("expected token %q, got %q", "token", receivedToken)
	}
	if receivedContentType != "application/json" {
		t.Errorf("expected content type %q, got %q", "application/json", receivedContentType)
	}

	req1 := recordedRequests[0]
	if req1["title"] != "Feed Title" {
		t.Errorf("req1: expected title %q, got %v", "Feed Title", req1["title"])
	}
	if req1["message"] != "First Article" {
		t.Errorf("req1: expected message %q, got %v", "First Article", req1["message"])
	}
	if int(req1["priority"].(float64)) != 6 {
		t.Errorf("req1: expected priority 6, got %v", req1["priority"])
	}
	extra1 := req1["extras"].(map[string]any)["client::notification"].(map[string]any)["click"].(map[string]any)
	if extra1["url"] != "https://example.com/first" {
		t.Errorf("req1: expected click url %q, got %v", "https://example.com/first", extra1["url"])
	}

	req2 := recordedRequests[1]
	if req2["title"] != "Feed Title" {
		t.Errorf("req2: expected title %q, got %v", "Feed Title", req2["title"])
	}
	if req2["message"] != "https://example.com/second" {
		t.Errorf("req2: expected fallback message %q, got %v", "https://example.com/second", req2["message"])
	}
	extra2 := req2["extras"].(map[string]any)["client::notification"].(map[string]any)["click"].(map[string]any)
	if extra2["url"] != "https://example.com/second" {
		t.Errorf("req2: expected click url %q, got %v", "https://example.com/second", extra2["url"])
	}
}

func TestSendMessagesMissingConfigOrEmptyEntries(t *testing.T) {
	configureIntegrationAllowPrivateNetworksOption(t)

	t.Run("missing base URL returns error", func(t *testing.T) {
		client := NewClient("", "token", 0)
		err := client.SendMessages(&model.Feed{}, model.Entries{{Title: "Test"}})
		if err == nil {
			t.Fatal("expected error for missing base URL, got nil")
		}
		if !strings.Contains(err.Error(), "gotify: missing base URL or token") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("missing token returns error", func(t *testing.T) {
		client := NewClient("https://gotify.example.com", "", 0)
		err := client.SendMessages(&model.Feed{}, model.Entries{{Title: "Test"}})
		if err == nil {
			t.Fatal("expected error for missing token, got nil")
		}
		if !strings.Contains(err.Error(), "gotify: missing base URL or token") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("skip entries with empty title and empty url", func(t *testing.T) {
		requestCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestCount++
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"id":1}`))
		}))
		defer server.Close()

		client := NewClient(server.URL, "token", 0)
		feed := &model.Feed{Title: "Feed"}
		entries := model.Entries{
			{Title: "", URL: ""},
			{Title: "Valid Title", URL: "https://example.com/item"},
		}

		if err := client.SendMessages(feed, entries); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if requestCount != 1 {
			t.Errorf("expected 1 request for valid entry, got %d", requestCount)
		}
	})
}

func TestNewClientPriorityClamping(t *testing.T) {
	client := NewClient("https://gotify.example.com", "token", -5)
	if client.priority != 0 {
		t.Errorf("expected negative priority clamped to 0, got %d", client.priority)
	}
}

func configureIntegrationAllowPrivateNetworksOption(t *testing.T) {
	t.Helper()

	parsedOptions, err := config.Parse([]string{"INTEGRATION_ALLOW_PRIVATE_NETWORKS=1"})
	if err != nil {
		t.Fatalf("Unable to configure test options: %v", err)
	}

	previousOptions := config.Opts
	config.Opts = parsedOptions
	t.Cleanup(func() {
		config.Opts = previousOptions
	})
}
