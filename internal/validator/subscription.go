// Copyright The Miniflux Authors. Modified by The sss Authors.
// Derived from Miniflux (https://github.com/miniflux/v2).
// SPDX-License-Identifier: Apache-2.0

package validator

import (
	"errors"

	"sss/internal/model"
	"sss/internal/urllib"
)

// ValidateSubscriptionDiscovery validates subscription discovery requests.
func ValidateSubscriptionDiscovery(request *model.SubscriptionDiscoveryRequest) error {
	if !urllib.IsAbsoluteURL(request.URL) {
		return errInvalidSiteURL
	}

	if request.ProxyURL != "" && !urllib.IsValidProxyURL(request.ProxyURL) {
		return errors.New("Invalid proxy URL.")
	}

	return nil
}
