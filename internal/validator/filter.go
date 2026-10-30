// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

package validator

import (
	"fmt"
	"slices"
	"strings"
)

// IsValidFilterRules validates entry filter rules ("FieldName=RegEx" per line); filterType is "block" or "keep".
func IsValidFilterRules(filterEntryRules string, filterType string) error {
	fieldNames := []string{"EntryTitle", "EntryURL", "EntryCommentsURL", "EntryContent", "EntryAuthor", "EntryTag", "EntryDate"}
	kind := strings.ToUpper(filterType[:1]) + filterType[1:]

	for i, rule := range strings.Split(filterEntryRules, "\n") {
		idx := slices.IndexFunc(fieldNames, func(fieldName string) bool { return strings.HasPrefix(rule, fieldName) })
		if idx == -1 {
			return fmt.Errorf("Invalid %s rule: rule #%d is missing a valid field name (Options: '%s')", kind, i+1, strings.Join(fieldNames, "', '"))
		}
		fieldRegEx, _ := strings.CutPrefix(rule, fieldNames[idx])

		fieldRegEx, found := strings.CutPrefix(fieldRegEx, "=")
		if !found {
			return fmt.Errorf("Invalid %s rule: rule #%d's pattern is required to be separated by a '='", kind, i+1)
		}

		if fieldRegEx == "" {
			return fmt.Errorf("Invalid %s rule: rule #%d's pattern is not provided", kind, i+1)
		}

		if !IsValidRegex(fieldRegEx) {
			return fmt.Errorf("Invalid %s rule: rule #%d's pattern is not a valid regex", kind, i+1)
		}
	}
	return nil
}
