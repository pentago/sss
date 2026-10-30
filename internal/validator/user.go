// Copyright The Miniflux Authors. Modified by The sss Authors.
// Derived from Miniflux (https://github.com/miniflux/v2).
// SPDX-License-Identifier: Apache-2.0

package validator

import (
	"errors"
	"strings"
	"unicode"

	"sss/internal/config"
	"sss/internal/model"
	"sss/internal/timezone"
)

var (
	errUserMandatoryFields = errors.New("The username is mandatory.")
	errUserAlreadyExists   = errors.New("This user already exists.")
)

// ValidateUserCreationWithPassword validates user creation with a password.
func ValidateUserCreationWithPassword(store Store, request *model.UserCreationRequest) error {
	if request.Username == "" {
		return errUserMandatoryFields
	}

	if store.UserExists(request.Username) {
		return errUserAlreadyExists
	}

	if err := validateUsername(request.Username); err != nil {
		return err
	}

	return validatePassword(request.Password)
}

// ValidateUserModification validates user modifications.
func ValidateUserModification(store Store, userID int64, changes *model.UserModificationRequest) error {
	if changes.Username != nil {
		if *changes.Username == "" {
			return errUserMandatoryFields
		} else if store.AnotherUserExists(userID, *changes.Username) {
			return errUserAlreadyExists
		}
	}

	if changes.Password != nil {
		if config.Opts.DisableLocalAuth {
			return errors.New("Local authentication is disabled. Passwords cannot be changed.")
		}

		if err := validatePassword(*changes.Password); err != nil {
			return err
		}
	}

	if changes.Theme != nil {
		if _, found := model.Themes()[*changes.Theme]; !found {
			return errors.New("Invalid theme.")
		}
	}

	if changes.ThemeLightHour != nil {
		if err := validateHour(*changes.ThemeLightHour); err != nil {
			return err
		}
	}

	if changes.ThemeDarkHour != nil {
		if err := validateHour(*changes.ThemeDarkHour); err != nil {
			return err
		}
	}

	if changes.Timezone != nil && !timezone.IsValid(*changes.Timezone) {
		return errors.New("Invalid timezone.")
	}

	if changes.EntryDirection != nil {
		if err := ValidateDirection(*changes.EntryDirection); err != nil {
			return errors.New("Invalid entry direction.")
		}
	}

	// Only the values of the entry_sorting_order column are accepted.
	if changes.EntryOrder != nil && *changes.EntryOrder != "published_at" && *changes.EntryOrder != "created_at" {
		return errors.New("Invalid entry order.")
	}

	if changes.EntriesPerPage != nil && (*changes.EntriesPerPage < 1 || *changes.EntriesPerPage > model.MaxEntryLimit) {
		return errors.New("The number of entries per page is not valid.")
	}

	if changes.CategoriesSortingOrder != nil {
		if _, found := model.CategoriesSortingOptions()[*changes.CategoriesSortingOrder]; !found {
			return errors.New("Invalid categories sorting order.")
		}
	}

	if changes.DisplayMode != nil {
		switch *changes.DisplayMode {
		case "fullscreen", "standalone", "minimal-ui", "browser":
		default:
			return errors.New("Invalid web app display mode.")
		}
	}

	if changes.GestureNav != nil {
		switch *changes.GestureNav {
		case "none", "tap", "swipe":
		default:
			return errors.New("Invalid gesture navigation.")
		}
	}

	if (changes.DefaultReadingSpeed != nil && *changes.DefaultReadingSpeed <= 0) || (changes.CJKReadingSpeed != nil && *changes.CJKReadingSpeed <= 0) {
		return errors.New("The reading speeds must be positive integers.")
	}

	if changes.DefaultHomePage != nil {
		if _, found := model.HomePages()[*changes.DefaultHomePage]; !found {
			return errors.New("Invalid default homepage.")
		}
	}

	if changes.BlockFilterEntryRules != nil && *changes.BlockFilterEntryRules != "" {
		if err := IsValidFilterRules(*changes.BlockFilterEntryRules, "block"); err != nil {
			return err
		}
	}

	if changes.KeepFilterEntryRules != nil && *changes.KeepFilterEntryRules != "" {
		if err := IsValidFilterRules(*changes.KeepFilterEntryRules, "keep"); err != nil {
			return err
		}
	}

	if changes.ExternalFontHosts != nil && !IsValidDomainList(*changes.ExternalFontHosts) {
		return errors.New("Invalid domain list. Please provide a space separated list of domains.")
	}

	return nil
}

func validateHour(hour int) error {
	if hour < 0 || hour > 23 {
		return errors.New("The theme hours must be between 0 and 23.")
	}
	return nil
}

func validatePassword(password string) error {
	if len(password) < 6 {
		return errors.New("The password must have at least 6 characters.")
	}
	return nil
}

// validateUsername returns an error if the username contains a character
// that isn't a letter, a digit, or one of "_-@.".
//
// Note: this validation should not be applied to previously created usernames,
// and cannot be applied to Google/OIDC accounts creation because the email
// address is used for the username field.
func validateUsername(username string) error {
	if strings.ContainsFunc(username, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r) && !strings.ContainsRune("_-@.", r)
	}) {
		return errors.New("The username is invalid. Only letters, digits and _ - @ . are allowed.")
	}
	return nil
}
