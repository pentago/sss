// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

package request

import (
	"context"
	"net/http"
	"testing"

	"sss/internal/model"
)

func newRequestWithWebSession(session *model.WebSession) *http.Request {
	r, _ := http.NewRequest("GET", "http://example.org", nil)
	ctx := context.WithValue(r.Context(), WebSessionContextKey, session)
	return r.WithContext(ctx)
}

func TestCtxValue(t *testing.T) {
	newRequest := func(value any) *http.Request {
		r, _ := http.NewRequest("GET", "http://example.org", nil)
		if value != nil {
			r = r.WithContext(context.WithValue(r.Context(), ClientIPContextKey, value))
		}
		return r
	}

	scenarios := []struct {
		name     string
		value    any
		expected any
	}{
		{"string value", "IP", "IP"},
		{"string with invalid type", 0, ""},
		{"string when unset", nil, ""},
		{"bool value", true, true},
		{"bool with invalid type", "invalid", false},
		{"bool when unset", nil, false},
		{"int64 value", int64(1234), int64(1234)},
		{"int64 with invalid type", "invalid", int64(0)},
		{"int64 when unset", nil, int64(0)},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			var result any
			switch expected := scenario.expected.(type) {
			case bool:
				result = ctxValue[bool](newRequest(scenario.value), ClientIPContextKey)
				if result != expected {
					t.Errorf(`Unexpected context value, got %v instead of %v`, result, expected)
				}
			case int64:
				result = ctxValue[int64](newRequest(scenario.value), ClientIPContextKey)
				if result != expected {
					t.Errorf(`Unexpected context value, got %d instead of %d`, result, expected)
				}
			default:
				result = ctxValue[string](newRequest(scenario.value), ClientIPContextKey)
				if result != expected {
					t.Errorf(`Unexpected context value, got %q instead of %q`, result, expected)
				}
			}
		})
	}
}

func TestIsAdmin(t *testing.T) {
	r, _ := http.NewRequest("GET", "http://example.org", nil)

	result := IsAdminUser(r)
	expected := false

	if result != expected {
		t.Errorf(`Unexpected context value, got %v instead of %v`, result, expected)
	}

	ctx := r.Context()
	ctx = context.WithValue(ctx, IsAdminUserContextKey, true)
	r = r.WithContext(ctx)

	result = IsAdminUser(r)
	expected = true

	if result != expected {
		t.Errorf(`Unexpected context value, got %v instead of %v`, result, expected)
	}
}

func TestIsAuthenticated(t *testing.T) {
	r, _ := http.NewRequest("GET", "http://example.org", nil)

	result := IsAuthenticated(r)
	expected := false

	if result != expected {
		t.Errorf(`Unexpected context value, got %v instead of %v`, result, expected)
	}

	ctx := r.Context()
	ctx = context.WithValue(ctx, IsAuthenticatedContextKey, true)
	r = r.WithContext(ctx)

	result = IsAuthenticated(r)
	expected = true

	if result != expected {
		t.Errorf(`Unexpected context value, got %v instead of %v`, result, expected)
	}

	session := &model.WebSession{}
	session.SetUser(&model.User{ID: 42})
	r = newRequestWithWebSession(session)

	result = IsAuthenticated(r)
	if !result {
		t.Errorf("Unexpected context value, got %v instead of true", result)
	}
}

func TestUserID(t *testing.T) {
	r, _ := http.NewRequest("GET", "http://example.org", nil)

	result := UserID(r)
	expected := int64(0)

	if result != expected {
		t.Errorf(`Unexpected context value, got %v instead of %v`, result, expected)
	}

	ctx := r.Context()
	ctx = context.WithValue(ctx, UserIDContextKey, int64(123))
	r = r.WithContext(ctx)

	result = UserID(r)
	expected = int64(123)

	if result != expected {
		t.Errorf(`Unexpected context value, got %v instead of %v`, result, expected)
	}

	session := &model.WebSession{}
	session.SetUser(&model.User{ID: 456})
	r = newRequestWithWebSession(session)

	result = UserID(r)
	expected = int64(456)

	if result != expected {
		t.Errorf(`Unexpected context value, got %v instead of %v`, result, expected)
	}
}

func TestUserName(t *testing.T) {
	r, _ := http.NewRequest("GET", "http://example.org", nil)

	result := UserName(r)
	expected := "unknown"

	if result != expected {
		t.Errorf(`Unexpected context value, got %q instead of %q`, result, expected)
	}

	ctx := r.Context()
	ctx = context.WithValue(ctx, UserNameContextKey, "jane")
	r = r.WithContext(ctx)

	result = UserName(r)
	expected = "jane"

	if result != expected {
		t.Errorf(`Unexpected context value, got %q instead of %q`, result, expected)
	}
}

func TestUserTimezone(t *testing.T) {
	r, _ := http.NewRequest("GET", "http://example.org", nil)

	result := UserTimezone(r)
	expected := "UTC"

	if result != expected {
		t.Errorf(`Unexpected context value, got %q instead of %q`, result, expected)
	}

	ctx := r.Context()
	ctx = context.WithValue(ctx, UserTimezoneContextKey, "Europe/Paris")
	r = r.WithContext(ctx)

	result = UserTimezone(r)
	expected = "Europe/Paris"

	if result != expected {
		t.Errorf(`Unexpected context value, got %q instead of %q`, result, expected)
	}
}

func TestWebSession(t *testing.T) {
	r, _ := http.NewRequest("GET", "http://example.org", nil)

	if result := WebSession(r); result != nil {
		t.Fatalf("Unexpected context value, got %v instead of nil", result)
	}

	session := &model.WebSession{ID: "session-id"}
	ctx := r.Context()
	ctx = context.WithValue(ctx, WebSessionContextKey, session)
	r = r.WithContext(ctx)

	result := WebSession(r)
	if result == nil || result.ID != "session-id" {
		t.Fatalf("Unexpected context value, got %#v instead of session-id", result)
	}
}

func TestClientIP(t *testing.T) {
	r, _ := http.NewRequest("GET", "http://example.org", nil)

	result := ClientIP(r)
	expected := ""

	if result != expected {
		t.Errorf(`Unexpected context value, got %q instead of %q`, result, expected)
	}

	ctx := r.Context()
	ctx = context.WithValue(ctx, ClientIPContextKey, "127.0.0.1")
	r = r.WithContext(ctx)

	result = ClientIP(r)
	expected = "127.0.0.1"

	if result != expected {
		t.Errorf(`Unexpected context value, got %q instead of %q`, result, expected)
	}
}

func TestGoogleReaderToken(t *testing.T) {
	r, _ := http.NewRequest("GET", "http://example.org", nil)

	result := GoogleReaderToken(r)
	expected := ""

	if result != expected {
		t.Errorf(`Unexpected context value, got %q instead of %q`, result, expected)
	}

	ctx := r.Context()
	ctx = context.WithValue(ctx, GoogleReaderTokenKey, "token")
	r = r.WithContext(ctx)

	result = GoogleReaderToken(r)
	expected = "token"

	if result != expected {
		t.Errorf(`Unexpected context value, got %q instead of %q`, result, expected)
	}
}
