// Copyright The Miniflux Authors. Modified by The sss Authors.
// Derived from Miniflux (https://github.com/miniflux/v2).
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"sss/internal/config"
	"sss/internal/version"
)

func TestNewClientWithoutBlockingPrivateNetworks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClientWithOptions(Options{Timeout: 5 * time.Second})
	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestBlockPrivateNetworksBlocksLoopback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClientWithOptions(Options{Timeout: 5 * time.Second, BlockPrivateNetworks: true})
	_, err := client.Get(server.URL)
	if err == nil {
		t.Fatal("Expected an error when connecting to loopback address, got nil")
	}

	if !errors.Is(err, ErrPrivateNetwork) {
		t.Fatalf("Expected ErrPrivateNetwork, got %v", err)
	}
}

func TestRequestBuilderWithJSON(t *testing.T) {
	configureIntegrationAllowPrivateNetworksOption(t)

	var gotMethod, gotContentType, gotUserAgent, gotAuth, gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotContentType = r.Header.Get("Content-Type")
		gotUserAgent = r.Header.Get("User-Agent")
		gotAuth = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	response, err := NewRequestBuilder(server.URL).
		WithMethod(http.MethodPost).
		WithHeader("Authorization", "Bearer secret").
		WithJSON(map[string]string{"hello": "world"}).
		Do()
	if err != nil {
		t.Fatalf("request execution failed: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, response.StatusCode)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("expected method POST, got %s", gotMethod)
	}
	if gotContentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", gotContentType)
	}
	if want := "sss/" + version.Version; gotUserAgent != want {
		t.Errorf("expected User-Agent %q, got %q", want, gotUserAgent)
	}
	if gotAuth != "Bearer secret" {
		t.Errorf("expected Authorization %q, got %q", "Bearer secret", gotAuth)
	}
	if gotBody != `{"hello":"world"}` {
		t.Errorf("expected body %q, got %q", `{"hello":"world"}`, gotBody)
	}
}

func TestRequestBuilderWithInvalidEndpoint(t *testing.T) {
	_, err := NewRequestBuilder("://invalid").WithMethod(http.MethodPost).WithJSON(nil).Do()
	if err == nil {
		t.Fatal("expected an error for an invalid endpoint, got nil")
	}
}

func TestRequestBuilderBlocksPrivateNetworksByDefault(t *testing.T) {
	setOptions(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()

	if _, err := NewRequestBuilder(server.URL).Do(); !errors.Is(err, ErrPrivateNetwork) {
		t.Fatalf("expected ErrPrivateNetwork, got %v", err)
	}
}

func configureIntegrationAllowPrivateNetworksOption(t *testing.T) {
	setOptions(t, "INTEGRATION_ALLOW_PRIVATE_NETWORKS=1")
}

func setOptions(t *testing.T, environ ...string) {
	t.Helper()

	parsedOptions, err := config.Parse(environ)
	if err != nil {
		t.Fatalf("Unable to configure test options: %v", err)
	}

	previousOptions := config.Opts
	config.Opts = parsedOptions
	t.Cleanup(func() {
		config.Opts = previousOptions
	})
}
