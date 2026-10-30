// Copyright The Miniflux Authors. Modified by The sss Authors.
// Derived from Miniflux (https://github.com/miniflux/v2).
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"crypto/md5"
	"encoding/hex"
	"net/http"
	"strconv"

	"sss/internal/config"
	"sss/internal/crypto"
	"sss/internal/http/request"
	"sss/internal/http/response"
	"sss/internal/model"
)

// feverToken is the Fever API key: md5("username:password"), as Fever clients compute it.
func feverToken(username, password string) string {
	sum := md5.Sum([]byte(username + ":" + password))
	return hex.EncodeToString(sum[:])
}

func optionalInt64(value string) *int64 {
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return nil
	}
	return &n
}

// mergeIntegrationForm copies the submitted form into integ. Secrets derived from passwords
// (Fever token, Google Reader hash, webhook secret) are handled by updateIntegrations.
func mergeIntegrationForm(r *http.Request, i *model.Integration) {
	v := r.FormValue
	on := func(name string) bool { return v(name) != "" }
	gotifyPriority, _ := strconv.Atoi(v("gotify_priority"))

	i.FeverEnabled = on("fever_enabled")
	i.FeverUsername = v("fever_username")
	i.GoogleReaderEnabled = on("googlereader_enabled")
	i.GoogleReaderUsername = v("googlereader_username")
	i.AppriseEnabled = on("apprise_enabled")
	i.AppriseURL = v("apprise_url")
	i.AppriseServicesURL = v("apprise_services_url")
	i.DiscordEnabled = on("discord_enabled")
	i.DiscordWebhookLink = v("discord_webhook_link")
	i.GotifyEnabled = on("gotify_enabled")
	i.GotifyURL = v("gotify_url")
	i.GotifyToken = v("gotify_token")
	i.GotifyPriority = gotifyPriority
	i.InstapaperEnabled = on("instapaper_enabled")
	i.InstapaperUsername = v("instapaper_username")
	i.InstapaperPassword = v("instapaper_password")
	i.LinkwardenEnabled = on("linkwarden_enabled")
	i.LinkwardenURL = v("linkwarden_url")
	i.LinkwardenAPIKey = v("linkwarden_api_key")
	i.LinkwardenCollectionID = optionalInt64(v("linkwarden_collection_id"))
	i.MatrixBotEnabled = on("matrix_bot_enabled")
	i.MatrixBotUser = v("matrix_bot_user")
	i.MatrixBotPassword = v("matrix_bot_password")
	i.MatrixBotURL = v("matrix_bot_url")
	i.MatrixBotChatID = v("matrix_bot_chat_id")
	i.NotionEnabled = on("notion_enabled")
	i.NotionToken = v("notion_token")
	i.NotionPageID = v("notion_page_id")
	i.NtfyEnabled = on("ntfy_enabled")
	i.NtfyTopic = v("ntfy_topic")
	i.NtfyURL = v("ntfy_url")
	i.NtfyAPIToken = v("ntfy_api_token")
	i.NtfyUsername = v("ntfy_username")
	i.NtfyPassword = v("ntfy_password")
	i.NtfyIconURL = v("ntfy_icon_url")
	i.NtfyInternalLinks = on("ntfy_internal_links")
	i.PushoverEnabled = on("pushover_enabled")
	i.PushoverUser = v("pushover_user")
	i.PushoverToken = v("pushover_token")
	i.PushoverDevice = v("pushover_device")
	i.PushoverPrefix = v("pushover_prefix")
	i.RSSBridgeEnabled = on("rssbridge_enabled")
	i.RSSBridgeURL = v("rssbridge_url")
	i.RSSBridgeToken = v("rssbridge_token")
	i.SlackEnabled = on("slack_enabled")
	i.SlackWebhookLink = v("slack_webhook_link")
	i.TelegramBotEnabled = on("telegram_bot_enabled")
	i.TelegramBotToken = v("telegram_bot_token")
	i.TelegramBotChatID = v("telegram_bot_chat_id")
	i.TelegramBotTopicID = optionalInt64(v("telegram_bot_topic_id"))
	i.TelegramBotDisableWebPagePreview = on("telegram_bot_disable_web_page_preview")
	i.TelegramBotDisableNotification = on("telegram_bot_disable_notification")
	i.TelegramBotDisableButtons = on("telegram_bot_disable_buttons")
	i.WebhookEnabled = on("webhook_enabled")
	i.WebhookURL = v("webhook_url")
}

func (h *handler) showIntegrations(w http.ResponseWriter, r *http.Request) {
	integration, err := h.store.Integration(request.UserID(r))
	if err != nil {
		response.HTMLServerError(w, r, err)
		return
	}
	h.renderAccount(w, r, "account_integrations", map[string]any{
		"title":       "Integrations",
		"menu":        "settings",
		"integration": integration,
		"rootURL":     config.Opts.BaseURL,
		"disableAPI":  config.Opts.DisableAPI,
	})
}

func (h *handler) updateIntegrations(w http.ResponseWriter, r *http.Request) {
	sess := request.WebSession(r)
	userID := request.UserID(r)
	page := route("/integrations")
	integration, err := h.store.Integration(userID)
	if err != nil {
		response.HTMLServerError(w, r, err)
		return
	}
	mergeIntegrationForm(r, integration)

	if integration.FeverUsername != "" && h.store.HasDuplicateFeverUsername(userID, integration.FeverUsername) {
		sess.SetErrorMessage("This Fever username is already used by another user.")
		response.HTMLRedirect(w, r, page)
		return
	}
	if integration.FeverEnabled {
		if password := r.FormValue("fever_password"); password != "" {
			integration.FeverToken = feverToken(integration.FeverUsername, password)
		}
	} else {
		integration.FeverToken = ""
	}

	grPassword := r.FormValue("googlereader_password")
	if integration.GoogleReaderEnabled && (integration.GoogleReaderUsername == "" || (grPassword == "" && integration.GoogleReaderPassword == "")) {
		sess.SetErrorMessage("The Google Reader username and password are required.")
		response.HTMLRedirect(w, r, page)
		return
	}
	if integration.GoogleReaderUsername != "" && h.store.HasDuplicateGoogleReaderUsername(userID, integration.GoogleReaderUsername) {
		sess.SetErrorMessage("This Google Reader username is already used by another user.")
		response.HTMLRedirect(w, r, page)
		return
	}
	if !integration.GoogleReaderEnabled {
		integration.GoogleReaderPassword = ""
	} else if grPassword != "" {
		if integration.GoogleReaderPassword, err = crypto.HashPassword(grPassword); err != nil {
			response.HTMLServerError(w, r, err)
			return
		}
	}

	switch {
	case !integration.WebhookEnabled || integration.WebhookURL == "":
		integration.WebhookEnabled = false
		integration.WebhookURL = ""
		integration.WebhookSecret = ""
	case integration.WebhookSecret == "":
		integration.WebhookSecret = crypto.GenerateRandomStringHex(32)
	}

	if err := h.store.UpdateIntegration(integration); err != nil {
		response.HTMLServerError(w, r, err)
		return
	}
	sess.SetSuccessMessage("Preferences saved.")
	response.HTMLRedirect(w, r, page)
}
