// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"testing"
	"time"
	_ "time/tzdata"

	"sss/internal/config"
)

func TestFeedScheduleNextCheck(t *testing.T) {
	tests := []struct {
		name         string
		env          []string
		weeklyCount  int
		refreshDelay time.Duration
		want         time.Duration
	}{
		{"round robin default", nil, 0, 0, 60 * time.Minute},
		{"round robin refresh delay above min", nil, 0, 90 * time.Minute, 90 * time.Minute},
		{"round robin refresh delay below min", nil, 0, 30 * time.Minute, 60 * time.Minute},
		{"round robin refresh delay above max", nil, 0, 48 * time.Hour, 24 * time.Hour},
		{"round robin min interval", []string{"SCHEDULER_ROUND_ROBIN_MIN_INTERVAL=500"}, 0, 0, 500 * time.Minute},
		{"entry frequency zero weekly count", []string{"POLLING_SCHEDULER=entry_frequency", "SCHEDULER_ENTRY_FREQUENCY_MAX_INTERVAL=600"}, 0, 0, 600 * time.Minute},
		{"entry frequency capped at max", []string{"POLLING_SCHEDULER=entry_frequency", "SCHEDULER_ENTRY_FREQUENCY_MAX_INTERVAL=600"}, 1, 0, 600 * time.Minute},
		{"entry frequency raised to min", []string{"POLLING_SCHEDULER=entry_frequency", "SCHEDULER_ENTRY_FREQUENCY_MIN_INTERVAL=30"}, 10080, 0, 30 * time.Minute},
		{"entry frequency weekly count", []string{"POLLING_SCHEDULER=entry_frequency"}, 7 * 24, 0, time.Hour},
		{"entry frequency factor", []string{"POLLING_SCHEDULER=entry_frequency", "SCHEDULER_ENTRY_FREQUENCY_FACTOR=2"}, 7 * 24, 0, 30 * time.Minute},
		{"entry frequency small TTL", []string{"POLLING_SCHEDULER=entry_frequency", "SCHEDULER_ENTRY_FREQUENCY_MIN_INTERVAL=30"}, 10080, time.Minute, 30 * time.Minute},
		{"entry frequency large TTL", []string{"POLLING_SCHEDULER=entry_frequency", "SCHEDULER_ENTRY_FREQUENCY_MIN_INTERVAL=30"}, 10080, 120 * time.Minute, 120 * time.Minute},
		{"entry frequency TTL capped at max", []string{"POLLING_SCHEDULER=entry_frequency", "SCHEDULER_ENTRY_FREQUENCY_MAX_INTERVAL=600"}, 10080, 48 * time.Hour, 600 * time.Minute},
	}
	t.Cleanup(func() { config.Opts = nil })
	for _, tc := range tests {
		opts, err := config.Parse(tc.env)
		if err != nil {
			t.Fatal(err)
		}
		config.Opts = opts

		before := time.Now()
		feed := &Feed{}
		if got := feed.ScheduleNextCheck(tc.weeklyCount, tc.refreshDelay); got != tc.want {
			t.Errorf("%s: interval %v, want %v", tc.name, got, tc.want)
		}
		if feed.NextCheckAt.Before(before.Add(tc.want)) || feed.NextCheckAt.After(time.Now().Add(tc.want)) {
			t.Errorf("%s: next check at %v not now+%v", tc.name, feed.NextCheckAt, tc.want)
		}
	}
}

func TestFeedErrorCounter(t *testing.T) {
	feed := &Feed{}
	feed.WithErrorMessage("boom")
	feed.WithErrorMessage("boom again")
	if feed.ParsingErrorCount != 2 || feed.ParsingErrorMsg != "boom again" {
		t.Errorf("got %d %q", feed.ParsingErrorCount, feed.ParsingErrorMsg)
	}
	feed.ResetErrorCounter()
	if feed.ParsingErrorCount != 0 || feed.ParsingErrorMsg != "" {
		t.Errorf("reset: got %d %q", feed.ParsingErrorCount, feed.ParsingErrorMsg)
	}
}

func TestFeedModificationRequestPatch(t *testing.T) {
	feed := &Feed{FeedURL: "https://a/feed", Title: "A", Category: &Category{ID: 1}, Crawler: true}
	(&FeedModificationRequest{
		FeedURL:    new(""),
		Title:      new("B"),
		CategoryID: new(int64(0)),
		Crawler:    new(false),
		ProxyURL:   new("http://proxy"),
	}).Patch(feed)
	if feed.FeedURL != "https://a/feed" || feed.Title != "B" || feed.Category.ID != 1 || feed.Crawler || feed.ProxyURL != "http://proxy" {
		t.Errorf("unexpected feed %+v", feed)
	}
	(&FeedModificationRequest{CategoryID: new(int64(3))}).Patch(feed)
	if feed.Category.ID != 3 {
		t.Errorf("category not patched: %d", feed.Category.ID)
	}
}

func TestUserModificationRequestPatch(t *testing.T) {
	user := &User{Theme: ThemeDefault, ThemeLightHour: 7, ThemeDarkHour: 19, KeyboardShortcuts: true}
	(&UserModificationRequest{
		Theme:            new("time_serif"),
		ThemeLightHour:   new(6),
		MarkReadOnScroll: new(true),
		ServiceWorker:    new(true),
	}).Patch(user)
	if user.Theme != "time_serif" || user.ThemeLightHour != 6 || user.ThemeDarkHour != 19 || !user.MarkReadOnScroll || !user.ServiceWorker || !user.KeyboardShortcuts {
		t.Errorf("unexpected user %+v", user)
	}
}

func TestUserThemeAt(t *testing.T) {
	at := func(hour int) time.Time { return time.Date(2026, 1, 15, hour, 30, 0, 0, time.UTC) }
	tests := []struct {
		user User
		now  time.Time
		want string
	}{
		{User{Theme: "dark_sans_serif"}, at(12), SchemeDark},
		{User{Theme: "system_serif"}, at(12), "system"},
		{User{Theme: ThemeNone}, at(12), ThemeNone},
		{User{Theme: "time_sans_serif", Timezone: "UTC", ThemeLightHour: 7, ThemeDarkHour: 19}, at(7), SchemeLight},
		{User{Theme: "time_sans_serif", Timezone: "UTC", ThemeLightHour: 7, ThemeDarkHour: 19}, at(18), SchemeLight},
		{User{Theme: "time_sans_serif", Timezone: "UTC", ThemeLightHour: 7, ThemeDarkHour: 19}, at(19), SchemeDark},
		{User{Theme: "time_sans_serif", Timezone: "UTC", ThemeLightHour: 7, ThemeDarkHour: 19}, at(3), SchemeDark},
		// 20:30 UTC is 05:30 in Tokyo.
		{User{Theme: "time_sans_serif", Timezone: "Asia/Tokyo", ThemeLightHour: 7, ThemeDarkHour: 19}, at(20), SchemeDark},
		{User{Theme: "time_sans_serif", Timezone: "Asia/Tokyo", ThemeLightHour: 7, ThemeDarkHour: 19}, at(0), SchemeLight},
		// Light period wrapping midnight.
		{User{Theme: "time_sans_serif", Timezone: "UTC", ThemeLightHour: 22, ThemeDarkHour: 4}, at(23), SchemeLight},
		{User{Theme: "time_sans_serif", Timezone: "UTC", ThemeLightHour: 22, ThemeDarkHour: 4}, at(2), SchemeLight},
		{User{Theme: "time_sans_serif", Timezone: "UTC", ThemeLightHour: 22, ThemeDarkHour: 4}, at(12), SchemeDark},
		{User{Theme: "time_sans_serif", Timezone: "UTC", ThemeLightHour: 5, ThemeDarkHour: 5}, at(5), SchemeDark},
	}
	for i, tc := range tests {
		if got, _ := tc.user.ThemeAt(tc.now); got != tc.want {
			t.Errorf("%d: got %q, want %q", i, got, tc.want)
		}
	}
}

func TestEnclosure(t *testing.T) {
	tests := []struct {
		enclosure           Enclosure
		audio, video, image bool
	}{
		{Enclosure{MimeType: "audio/MPEG"}, true, false, false},
		{Enclosure{MimeType: "video/m4v"}, false, true, false},
		{Enclosure{MimeType: "image/png"}, false, false, true},
		{Enclosure{URL: "https://a/b.JPG", MimeType: "application/octet-stream"}, false, false, true},
	}
	for i, tc := range tests {
		e := tc.enclosure
		if e.IsAudio() != tc.audio || e.IsVideo() != tc.video || e.IsImage() != tc.image {
			t.Errorf("%d: unexpected result for %+v", i, e)
		}
	}
}

func TestEntryShouldMarkAsReadOnView(t *testing.T) {
	user := &User{MarkReadOnView: true}
	if !(&Entry{Status: EntryStatusUnread}).ShouldMarkAsReadOnView(user) {
		t.Error("unread entry should be marked as read")
	}
	if (&Entry{Status: EntryStatusRead}).ShouldMarkAsReadOnView(user) {
		t.Error("read entry should not be marked again")
	}
	if (&Entry{Status: EntryStatusUnread}).ShouldMarkAsReadOnView(&User{}) {
		t.Error("setting disabled")
	}
}

func TestWebSession(t *testing.T) {
	session, secret := NewWebSession("agent", "127.0.0.1")
	if session.ID == "" || !session.VerifySecret(secret) || session.VerifySecret("wrong") || session.VerifySecret("") {
		t.Fatal("unexpected new session state")
	}
	if session.IsAuthenticated() || session.Theme() != ThemeDefault {
		t.Errorf("fresh session: authenticated=%v theme=%q", session.IsAuthenticated(), session.Theme())
	}

	oldID, newSecret := session.Rotate()
	if oldID == session.ID || session.VerifySecret(secret) || !session.VerifySecret(newSecret) {
		t.Error("rotate must change the identity and secret")
	}

	session.SetUser(&User{ID: 42, Theme: "dark_serif"})
	session.SetErrorMessage("oops")
	if id, ok := session.UserID(); !ok || id != 42 || session.Theme() != "dark_serif" || !session.IsDirty() {
		t.Errorf("after SetUser: id=%d ok=%v theme=%q", id, ok, session.Theme())
	}

	data, err := session.MarshalState()
	if err != nil {
		t.Fatal(err)
	}
	restored := &WebSession{}
	if err := restored.UnmarshalState(data); err != nil {
		t.Fatal(err)
	}
	restored.ScanUserID(session.NullUserID())
	if success, failure := restored.ConsumeMessages(); success != "" || failure != "oops" {
		t.Errorf("messages: %q %q", success, failure)
	}
	if success, failure := restored.ConsumeMessages(); success != "" || failure != "" {
		t.Error("messages must be consumed once")
	}
	if id, _ := restored.UserID(); id != 42 || restored.Theme() != "dark_serif" {
		t.Errorf("restored: id=%d theme=%q", id, restored.Theme())
	}

	restored.ClearUser()
	if restored.IsAuthenticated() || restored.NullUserID().Valid {
		t.Error("ClearUser must unbind the user")
	}
}
