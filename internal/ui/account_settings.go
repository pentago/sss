// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"net/http"
	"slices"
	"strconv"
	"strings"

	"sss/internal/config"
	"sss/internal/http/request"
	"sss/internal/http/response"
	"sss/internal/model"
	"sss/internal/timezone"
	"sss/internal/validator"
)

// formInt parses an integer form value; invalid input yields -1 so the validator rejects it.
func formInt(r *http.Request, name string) *int {
	n, err := strconv.Atoi(strings.TrimSpace(r.FormValue(name)))
	if err != nil {
		n = -1
	}
	return &n
}

func formString(r *http.Request, name string) *string {
	s := strings.ReplaceAll(strings.TrimSpace(r.FormValue(name)), "\r\n", "\n")
	return &s
}

func formBool(r *http.Request, name string) *bool {
	b := r.FormValue(name) != ""
	return &b
}

// settingsRequest maps the settings form to a modification request covering every user preference.
func settingsRequest(r *http.Request) *model.UserModificationRequest {
	return &model.UserModificationRequest{
		Theme:                     formString(r, "theme"),
		ThemeLightHour:            formInt(r, "theme_light_hour"),
		ThemeDarkHour:             formInt(r, "theme_dark_hour"),
		Timezone:                  formString(r, "timezone"),
		EntryDirection:            formString(r, "entry_direction"),
		EntryOrder:                formString(r, "entry_order"),
		EntriesPerPage:            formInt(r, "entries_per_page"),
		DefaultHomePage:           formString(r, "default_home_page"),
		CategoriesSortingOrder:    formString(r, "categories_sorting_order"),
		DisplayMode:               formString(r, "display_mode"),
		DefaultReadingSpeed:       formInt(r, "default_reading_speed"),
		CJKReadingSpeed:           formInt(r, "cjk_reading_speed"),
		ShowReadingTime:           formBool(r, "show_reading_time"),
		MarkReadOnView:            formBool(r, "mark_read_on_view"),
		OpenExternalLinksInNewTab: formBool(r, "open_external_links_in_new_tab"),
		AlwaysOpenExternalLinks:   formBool(r, "always_open_external_links"),
		BlockFilterEntryRules:     formString(r, "block_filter_entry_rules"),
		KeepFilterEntryRules:      formString(r, "keep_filter_entry_rules"),
		Stylesheet:                formString(r, "custom_css"),
		CustomJS:                  formString(r, "custom_js"),
		ExternalFontHosts:         formString(r, "external_font_hosts"),
		KeyboardShortcuts:         formBool(r, "keyboard_shortcuts"),
		EntrySwipe:                formBool(r, "entry_swipe"),
		GestureNav:                formString(r, "gesture_nav"),
		MarkReadOnScroll:          formBool(r, "mark_read_on_scroll"),
		ServiceWorker:             formBool(r, "service_worker"),
		GlobalUserAgent:           formString(r, "global_user_agent"),
		GlobalCrawler:             formBool(r, "global_crawler"),
	}
}

func (h *handler) renderSettings(w http.ResponseWriter, r *http.Request, form *model.User, errorMessage string) {
	data := map[string]any{
		"title":             "Settings",
		"menu":              "settings",
		"form":              form,
		"themes":            model.Themes(),
		"timezones":         slices.Collect(timezone.AvailableTimezones()),
		"homePages":         model.HomePages(),
		"categoriesSorting": model.CategoriesSortingOptions(),
		"maxEntriesPerPage": model.MaxEntryLimit,
		"defaultUserAgent":  config.Opts.HTTPClientUserAgent,
	}
	if errorMessage != "" {
		data["errorMessage"] = errorMessage
	}
	h.renderAccount(w, r, "account_settings", data)
}

func (h *handler) showSettings(w http.ResponseWriter, r *http.Request) {
	h.renderSettings(w, r, currentUser(r), "")
}

func (h *handler) updateSettings(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	changes := settingsRequest(r)
	form := *user
	err := validator.ValidateUserModification(h.store, user.ID, changes)
	changes.Patch(&form)
	if err != nil {
		h.renderSettings(w, r, &form, err.Error())
		return
	}
	if err := h.store.UpdateUser(&form); err != nil {
		response.HTMLServerError(w, r, err)
		return
	}
	sess := request.WebSession(r)
	sess.SetUser(&form)
	sess.SetSuccessMessage("Preferences saved.")
	response.HTMLRedirect(w, r, route("/settings"))
}
