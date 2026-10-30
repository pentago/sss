// Copyright The Miniflux Authors. Modified by The sss Authors.
// Derived from Miniflux (https://github.com/miniflux/v2).
// SPDX-License-Identifier: Apache-2.0

package validator

import (
	"testing"

	"sss/internal/model"
)

func TestValidateSubscriptionDiscovery(t *testing.T) {
	tests := []struct {
		name    string
		req     *model.SubscriptionDiscoveryRequest
		wantErr bool
	}{
		{
			name:    "valid site url",
			req:     &model.SubscriptionDiscoveryRequest{URL: "https://example.org"},
			wantErr: false,
		},
		{
			name:    "invalid site url",
			req:     &model.SubscriptionDiscoveryRequest{URL: "example.org"},
			wantErr: true,
		},
		{
			name:    "invalid proxy url",
			req:     &model.SubscriptionDiscoveryRequest{URL: "https://example.org", ProxyURL: "example.org"},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateSubscriptionDiscovery(tc.req); (err != nil) != tc.wantErr {
				t.Fatalf("expected error %v, got %v", tc.wantErr, err)
			}
		})
	}
}
