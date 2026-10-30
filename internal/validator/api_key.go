// Copyright The Miniflux Authors. Modified by The sss Authors.
// Derived from Miniflux (https://github.com/miniflux/v2).
// SPDX-License-Identifier: Apache-2.0

package validator

import (
	"errors"

	"sss/internal/model"
)

// ValidateAPIKeyCreation ensures API key creation requests include a description and are unique per user.
func ValidateAPIKeyCreation(store Store, userID int64, request *model.APIKeyCreationRequest) error {
	if request.Description == "" {
		return errors.New("All fields are mandatory.")
	}

	if store.APIKeyExists(userID, request.Description) {
		return errors.New("This API Key already exists.")
	}

	return nil
}
