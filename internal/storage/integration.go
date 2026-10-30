// Copyright The Miniflux Authors. Modified by The sss Authors.
// Derived from Miniflux (https://github.com/miniflux/v2).
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"sss/internal/model"
)

// integrationColumns lists the integrations columns; integrationFields returns
// pointers to the matching model fields, in the same order.
var integrationColumns = []string{
	"instapaper_enabled", "instapaper_username", "instapaper_password",
	"fever_enabled", "fever_username", "fever_token",
	"googlereader_enabled", "googlereader_username", "googlereader_password",
	"notion_enabled", "notion_token", "notion_page_id",
	"telegram_bot_enabled", "telegram_bot_token", "telegram_bot_chat_id", "telegram_bot_topic_id",
	"telegram_bot_disable_web_page_preview", "telegram_bot_disable_notification", "telegram_bot_disable_buttons",
	"linkwarden_enabled", "linkwarden_url", "linkwarden_api_key", "linkwarden_collection_id",
	"matrix_bot_enabled", "matrix_bot_user", "matrix_bot_password", "matrix_bot_url", "matrix_bot_chat_id",
	"apprise_enabled", "apprise_url", "apprise_services_url",
	"webhook_enabled", "webhook_url", "webhook_secret",
	"rssbridge_enabled", "rssbridge_url", "rssbridge_token",
	"ntfy_enabled", "ntfy_topic", "ntfy_url", "ntfy_api_token", "ntfy_username", "ntfy_password", "ntfy_icon_url", "ntfy_internal_links",
	"discord_enabled", "discord_webhook_link",
	"slack_enabled", "slack_webhook_link",
	"pushover_enabled", "pushover_user", "pushover_token", "pushover_device", "pushover_prefix",
	"gotify_enabled", "gotify_url", "gotify_token", "gotify_priority",
}

func integrationFields(i *model.Integration) []any {
	return []any{
		&i.InstapaperEnabled, &i.InstapaperUsername, &i.InstapaperPassword,
		&i.FeverEnabled, &i.FeverUsername, &i.FeverToken,
		&i.GoogleReaderEnabled, &i.GoogleReaderUsername, &i.GoogleReaderPassword,
		&i.NotionEnabled, &i.NotionToken, &i.NotionPageID,
		&i.TelegramBotEnabled, &i.TelegramBotToken, &i.TelegramBotChatID, &i.TelegramBotTopicID,
		&i.TelegramBotDisableWebPagePreview, &i.TelegramBotDisableNotification, &i.TelegramBotDisableButtons,
		&i.LinkwardenEnabled, &i.LinkwardenURL, &i.LinkwardenAPIKey, &i.LinkwardenCollectionID,
		&i.MatrixBotEnabled, &i.MatrixBotUser, &i.MatrixBotPassword, &i.MatrixBotURL, &i.MatrixBotChatID,
		&i.AppriseEnabled, &i.AppriseURL, &i.AppriseServicesURL,
		&i.WebhookEnabled, &i.WebhookURL, &i.WebhookSecret,
		&i.RSSBridgeEnabled, &i.RSSBridgeURL, &i.RSSBridgeToken,
		&i.NtfyEnabled, &i.NtfyTopic, &i.NtfyURL, &i.NtfyAPIToken, &i.NtfyUsername, &i.NtfyPassword, &i.NtfyIconURL, &i.NtfyInternalLinks,
		&i.DiscordEnabled, &i.DiscordWebhookLink,
		&i.SlackEnabled, &i.SlackWebhookLink,
		&i.PushoverEnabled, &i.PushoverUser, &i.PushoverToken, &i.PushoverDevice, &i.PushoverPrefix,
		&i.GotifyEnabled, &i.GotifyURL, &i.GotifyToken, &i.GotifyPriority,
	}
}

// saveEntryIntegrations are the integrations able to save an entry.
const saveEntryIntegrations = `(instapaper_enabled OR notion_enabled OR linkwarden_enabled OR apprise_enabled OR
	webhook_enabled OR discord_enabled OR slack_enabled)`

// HasDuplicateFeverUsername checks if another user have the same Fever username.
func (s *Storage) HasDuplicateFeverUsername(userID int64, feverUsername string) bool {
	var result bool
	s.db.QueryRow(`SELECT true FROM integrations WHERE user_id != $1 AND fever_username=$2 LIMIT 1`, userID, feverUsername).Scan(&result)
	return result
}

// HasDuplicateGoogleReaderUsername checks if another user have the same Google Reader username.
func (s *Storage) HasDuplicateGoogleReaderUsername(userID int64, googleReaderUsername string) bool {
	var result bool
	s.db.QueryRow(`SELECT true FROM integrations WHERE user_id != $1 AND googlereader_username=$2 LIMIT 1`, userID, googleReaderUsername).Scan(&result)
	return result
}

// UserByFeverToken returns the user (ID, username, admin flag, timezone) owning the Fever API token.
func (s *Storage) UserByFeverToken(token string) (*model.User, error) {
	var user model.User
	err := s.db.QueryRow(`
		SELECT u.id, u.username, u.is_admin, u.timezone
		FROM users u
		INNER JOIN integrations i ON i.user_id=u.id
		WHERE i.fever_enabled=1 AND lower(i.fever_token)=lower($1)`, token,
	).Scan(&user.ID, &user.Username, &user.IsAdmin, &user.Timezone)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return nil, nil
	case err != nil:
		return nil, fmt.Errorf("storage: unable to fetch user: %w", err)
	}
	return &user, nil
}

// GoogleReaderUserCheckPassword validates the Google Reader hashed password.
func (s *Storage) GoogleReaderUserCheckPassword(username, password string) error {
	var hash string
	err := s.db.QueryRow(`
		SELECT googlereader_password FROM integrations
		WHERE googlereader_enabled=1 AND googlereader_username=$1 AND googlereader_password <> ''`, username,
	).Scan(&hash)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("storage: unable to find this user: %s", username)
	} else if err != nil {
		return fmt.Errorf("storage: unable to fetch user: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return fmt.Errorf("storage: invalid password for %q: %w", username, err)
	}
	return nil
}

// GoogleReaderUserGetIntegration returns the Google Reader fields of the integration of the given username.
func (s *Storage) GoogleReaderUserGetIntegration(username string) (*model.Integration, error) {
	var integration model.Integration
	err := s.db.QueryRow(`
		SELECT user_id, googlereader_enabled, googlereader_username, googlereader_password FROM integrations
		WHERE googlereader_enabled=1 AND googlereader_username=$1 AND googlereader_password <> ''`, username,
	).Scan(&integration.UserID, &integration.GoogleReaderEnabled, &integration.GoogleReaderUsername, &integration.GoogleReaderPassword)
	if errors.Is(err, sql.ErrNoRows) {
		return &integration, fmt.Errorf("storage: unable to find this user: %s", username)
	} else if err != nil {
		return &integration, fmt.Errorf("storage: unable to fetch user: %w", err)
	}
	return &integration, nil
}

// Integration returns user integration settings.
func (s *Storage) Integration(userID int64) (*model.Integration, error) {
	integration := model.Integration{UserID: userID}
	err := s.db.QueryRow(`SELECT `+strings.Join(integrationColumns, ", ")+` FROM integrations WHERE user_id=$1`, userID).
		Scan(integrationFields(&integration)...)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return &integration, fmt.Errorf("storage: unable to fetch integration row: %w", err)
	}
	return &integration, nil
}

// UpdateIntegration saves user integration settings.
func (s *Storage) UpdateIntegration(integration *model.Integration) error {
	assignments := make([]string, len(integrationColumns))
	for i, column := range integrationColumns {
		assignments[i] = fmt.Sprintf("%s=$%d", column, i+1)
	}
	args := append(integrationFields(integration), integration.UserID)

	query := `UPDATE integrations SET ` + strings.Join(assignments, ", ") + fmt.Sprintf(` WHERE user_id=$%d`, len(args))
	if _, err := s.db.Exec(query, args...); err != nil {
		return fmt.Errorf("storage: unable to update integration record: %w", err)
	}
	return nil
}

// HasSaveEntry returns true if the given user can save articles to third-parties.
func (s *Storage) HasSaveEntry(userID int64) (result bool) {
	s.db.QueryRow(`SELECT true FROM integrations WHERE user_id=$1 AND `+saveEntryIntegrations, userID).Scan(&result)
	return result
}
