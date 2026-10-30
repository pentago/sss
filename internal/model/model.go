// Copyright The Miniflux Authors. Modified by The sss Authors.
// Derived from Miniflux (https://github.com/miniflux/v2).
// SPDX-License-Identifier: Apache-2.0

package model

// Number is a numeric type accepted by OptionalNumber.
type Number interface {
	int | int64 | float64
}

// OptionalNumber returns a pointer to value, or nil when value is not positive.
func OptionalNumber[T Number](value T) *T {
	if value > 0 {
		return &value
	}
	return nil
}

// OptionalString returns a pointer to value, or nil when value is empty.
func OptionalString(value string) *string {
	if value != "" {
		return &value
	}
	return nil
}
