// Copyright The Miniflux Authors. Modified by The sss Authors.
// Derived from Miniflux (https://github.com/miniflux/v2).
// SPDX-License-Identifier: Apache-2.0

package validator

import (
	"errors"

	"sss/internal/model"
)

var (
	errTitleRequired         = errors.New("The title is mandatory.")
	errCategoryAlreadyExists = errors.New("This category already exists.")
)

// ValidateCategoryCreation validates category creation.
func ValidateCategoryCreation(store Store, userID int64, request *model.CategoryCreationRequest) error {
	if request.Title == "" {
		return errTitleRequired
	}

	if store.CategoryTitleExists(userID, request.Title) {
		return errCategoryAlreadyExists
	}

	return nil
}

// ValidateCategoryModification validates category modification.
func ValidateCategoryModification(store Store, userID, categoryID int64, request *model.CategoryModificationRequest) error {
	if request.Title != nil {
		if *request.Title == "" {
			return errTitleRequired
		}

		if store.AnotherCategoryExists(userID, categoryID, *request.Title) {
			return errCategoryAlreadyExists
		}
	}

	return nil
}
