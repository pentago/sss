// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

package validator

import (
	"testing"

	"sss/internal/config"
	"sss/internal/model"
)

func TestValidateUsername(t *testing.T) {
	for username, wantErr := range map[string]bool{
		"userone": false, "user.name": false, "user@example.com": false, "john_doe": false,
		"john-doe": false, "User123": false, "invalid username": true, "user/path": true, "user🙂": true,
	} {
		if err := validateUsername(username); (err != nil) != wantErr {
			t.Errorf("%q: got %v, wantErr %v", username, err, wantErr)
		}
	}
}

func TestValidatePassword(t *testing.T) {
	for password, wantErr := range map[string]bool{"secret": false, "longpass": false, "short": true, "": true} {
		if err := validatePassword(password); (err != nil) != wantErr {
			t.Errorf("%q: got %v, wantErr %v", password, err, wantErr)
		}
	}
}

func TestValidateUserModification(t *testing.T) {
	opts, err := config.Parse(nil)
	if err != nil {
		t.Fatal(err)
	}
	config.Opts = opts
	t.Cleanup(func() { config.Opts = nil })

	tests := []struct {
		name    string
		req     model.UserModificationRequest
		wantErr bool
	}{
		{"theme system", model.UserModificationRequest{Theme: new("system_sans_serif")}, false},
		{"theme time", model.UserModificationRequest{Theme: new("time_serif")}, false},
		{"theme none", model.UserModificationRequest{Theme: new("none")}, false},
		{"bare scheme", model.UserModificationRequest{Theme: new("dark")}, true},
		{"unknown theme", model.UserModificationRequest{Theme: new("none_serif")}, true},
		{"hours bounds", model.UserModificationRequest{ThemeLightHour: new(0), ThemeDarkHour: new(23)}, false},
		{"light hour too big", model.UserModificationRequest{ThemeLightHour: new(24)}, true},
		{"dark hour negative", model.UserModificationRequest{ThemeDarkHour: new(-1)}, true},
		{"timezone", model.UserModificationRequest{Timezone: new("UTC")}, false},
		{"invalid timezone", model.UserModificationRequest{Timezone: new("Invalid/Zone")}, true},
		{"direction", model.UserModificationRequest{EntryDirection: new("sideways")}, true},
		{"entry order", model.UserModificationRequest{EntryOrder: new("created_at")}, false},
		{"entry order not in enum", model.UserModificationRequest{EntryOrder: new("title")}, true},
		{"entries per page max", model.UserModificationRequest{EntriesPerPage: new(model.MaxEntryLimit)}, false},
		{"entries per page zero", model.UserModificationRequest{EntriesPerPage: new(0)}, true},
		{"entries per page too big", model.UserModificationRequest{EntriesPerPage: new(model.MaxEntryLimit + 1)}, true},
		{"categories order", model.UserModificationRequest{CategoriesSortingOrder: new("popularity")}, true},
		{"display mode", model.UserModificationRequest{DisplayMode: new("windowed")}, true},
		{"gesture nav", model.UserModificationRequest{GestureNav: new("tap")}, false},
		{"invalid gesture nav", model.UserModificationRequest{GestureNav: new("pinch")}, true},
		{"reading speed", model.UserModificationRequest{CJKReadingSpeed: new(0)}, true},
		{"home page", model.UserModificationRequest{DefaultHomePage: new("dashboard")}, true},
		{"clear filter rules", model.UserModificationRequest{BlockFilterEntryRules: new(""), KeepFilterEntryRules: new("")}, false},
		{"invalid filter rule", model.UserModificationRequest{BlockFilterEntryRules: new("EntryTitle=[")}, true},
		{"font hosts", model.UserModificationRequest{ExternalFontHosts: new("fonts.example.org")}, false},
		{"invalid font hosts", model.UserModificationRequest{ExternalFontHosts: new("not a/domain")}, true},
		{"short password", model.UserModificationRequest{Password: new("short")}, true},
		{"password", model.UserModificationRequest{Password: new("longenough")}, false},
	}
	for _, tc := range tests {
		if err := ValidateUserModification(nil, 0, &tc.req); (err != nil) != tc.wantErr {
			t.Errorf("%s: got %v, wantErr %v", tc.name, err, tc.wantErr)
		}
	}

	config.Opts.DisableLocalAuth = true
	if err := ValidateUserModification(nil, 0, &model.UserModificationRequest{Password: new("newpassword")}); err == nil {
		t.Error("expected password modification to be rejected when local auth is disabled")
	}
}
