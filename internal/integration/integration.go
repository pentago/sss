// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

package integration

import (
	"log/slog"

	"sss/internal/integration/apprise"
	"sss/internal/integration/discord"
	"sss/internal/integration/gotify"
	"sss/internal/integration/instapaper"
	"sss/internal/integration/linkwarden"
	"sss/internal/integration/matrixbot"
	"sss/internal/integration/notion"
	"sss/internal/integration/ntfy"
	"sss/internal/integration/pushover"
	"sss/internal/integration/slack"
	"sss/internal/integration/telegrambot"
	"sss/internal/integration/webhook"
	"sss/internal/model"
)

// SendEntry sends the entry to third-party providers when the user click on "Save".
func SendEntry(entry *model.Entry, userIntegrations *model.Integration) {
	log := slog.With(
		slog.Int64("user_id", userIntegrations.UserID),
		slog.Int64("entry_id", entry.ID),
		slog.String("entry_url", entry.URL),
	)

	if userIntegrations.InstapaperEnabled {
		log.Debug("Sending entry to Instapaper")

		client := instapaper.NewClient(userIntegrations.InstapaperUsername, userIntegrations.InstapaperPassword)
		if err := client.AddURL(entry.URL, entry.Title); err != nil {
			log.Error("Unable to send entry to Instapaper", "error", err)
		}
	}

	if userIntegrations.NotionEnabled {
		log.Debug("Sending entry to Notion")

		client := notion.NewClient(
			userIntegrations.NotionToken,
			userIntegrations.NotionPageID,
		)
		if err := client.UpdateDocument(entry.URL, entry.Title); err != nil {
			log.Error("Unable to send entry to Notion", "error", err)
		}
	}

	if userIntegrations.LinkwardenEnabled {
		if userIntegrations.LinkwardenCollectionID != nil {
			log = log.With(slog.Int64("collection_id", *userIntegrations.LinkwardenCollectionID))
		}
		log.Debug("Sending entry to linkwarden")

		client := linkwarden.NewClient(
			userIntegrations.LinkwardenURL,
			userIntegrations.LinkwardenAPIKey,
			userIntegrations.LinkwardenCollectionID,
		)
		if err := client.CreateBookmark(entry.URL, entry.Title); err != nil {
			log.Error("Unable to send entry to Linkwarden", "error", err)
		}
	}

	if userIntegrations.WebhookEnabled {
		var webhookURL string
		if entry.Feed != nil && entry.Feed.WebhookURL != "" {
			webhookURL = entry.Feed.WebhookURL
		} else {
			webhookURL = userIntegrations.WebhookURL
		}

		log = log.With(slog.String("webhook_url", webhookURL))
		log.Debug("Sending entry to Webhook")

		webhookClient := webhook.NewClient(webhookURL, userIntegrations.WebhookSecret)
		if err := webhookClient.SendSaveEntryWebhookEvent(entry); err != nil {
			log.Error("Unable to send entry to Webhook", "error", err)
		}
	}
}

// PushEntries pushes a list of entries to activated third-party providers during feed refreshes.
func PushEntries(feed *model.Feed, entries model.Entries, userIntegrations *model.Integration) {
	log := slog.With(
		slog.Int64("user_id", userIntegrations.UserID),
		slog.Int("nb_entries", len(entries)),
		slog.Int64("feed_id", feed.ID),
	)

	if userIntegrations.MatrixBotEnabled {
		log.Debug("Sending new entries to Matrix")

		err := matrixbot.PushEntries(
			feed,
			entries,
			userIntegrations.MatrixBotURL,
			userIntegrations.MatrixBotUser,
			userIntegrations.MatrixBotPassword,
			userIntegrations.MatrixBotChatID,
		)
		if err != nil {
			log.Error("Unable to send new entries to Matrix", "error", err)
		}
	}
	if userIntegrations.WebhookEnabled {
		var webhookURL string
		if feed.WebhookURL != "" {
			webhookURL = feed.WebhookURL
		} else {
			webhookURL = userIntegrations.WebhookURL
		}

		log = log.With(slog.String("webhook_url", webhookURL))
		log.Debug("Sending new entries to Webhook")

		webhookClient := webhook.NewClient(webhookURL, userIntegrations.WebhookSecret)
		if err := webhookClient.SendNewEntriesWebhookEvent(feed, entries); err != nil {
			log.Warn("Unable to send new entries to Webhook", "error", err)
		}
	}

	if userIntegrations.NtfyEnabled && feed.NtfyEnabled {
		ntfyTopic := feed.NtfyTopic
		if ntfyTopic == "" {
			ntfyTopic = userIntegrations.NtfyTopic
		}
		log = log.With(slog.String("topic", ntfyTopic))
		log.Debug("Sending new entries to Ntfy")

		client := ntfy.NewClient(
			userIntegrations.NtfyURL,
			ntfyTopic,
			userIntegrations.NtfyAPIToken,
			userIntegrations.NtfyUsername,
			userIntegrations.NtfyPassword,
			userIntegrations.NtfyIconURL,
			userIntegrations.NtfyInternalLinks,
			feed.NtfyPriority,
		)

		if err := client.SendMessages(feed, entries); err != nil {
			log.Warn("Unable to send new entries to Ntfy", "error", err)
		}
	}

	if userIntegrations.AppriseEnabled {
		log.Debug("Sending new entries to Apprise")

		appriseServiceURLs := userIntegrations.AppriseServicesURL
		if feed.AppriseServiceURLs != "" {
			appriseServiceURLs = feed.AppriseServiceURLs
		}

		client := apprise.NewClient(
			appriseServiceURLs,
			userIntegrations.AppriseURL,
		)

		if err := client.SendNotification(feed, entries); err != nil {
			log.Warn("Unable to send new entries to Apprise", "error", err)
		}
	}

	if userIntegrations.DiscordEnabled {
		log.Debug("Sending new entries to Discord")

		client := discord.NewClient(
			userIntegrations.DiscordWebhookLink,
		)

		if err := client.SendDiscordMsg(feed, entries); err != nil {
			log.Warn("Unable to send new entries to Discord", "error", err)
		}
	}

	if userIntegrations.SlackEnabled {
		log.Debug("Sending new entries to Slack")

		client := slack.NewClient(
			userIntegrations.SlackWebhookLink,
		)

		if err := client.SendSlackMsg(feed, entries); err != nil {
			log.Warn("Unable to send new entries to Slack", "error", err)
		}
	}

	if userIntegrations.PushoverEnabled && feed.PushoverEnabled {
		log.Debug("Sending new entries to Pushover")

		client := pushover.NewClient(
			userIntegrations.PushoverUser,
			userIntegrations.PushoverToken,
			feed.PushoverPriority,
			userIntegrations.PushoverDevice,
			userIntegrations.PushoverPrefix,
		)

		if err := client.SendMessages(feed, entries); err != nil {
			log.Warn("Unable to send new entries to Pushover", "error", err)
		}
	}

	if userIntegrations.GotifyEnabled {
		log.Debug("Sending new entries to Gotify")

		client := gotify.NewClient(
			userIntegrations.GotifyURL,
			userIntegrations.GotifyToken,
			userIntegrations.GotifyPriority,
		)

		if err := client.SendMessages(feed, entries); err != nil {
			log.Warn("Unable to send new entries to Gotify", "error", err)
		}
	}

	// Integrations that only support sending individual entries
	if userIntegrations.TelegramBotEnabled {
		for _, entry := range entries {
			entryLog := log.With(
				slog.Int64("entry_id", entry.ID),
				slog.String("entry_url", entry.URL),
			)
			entryLog.Debug("Sending a new entry to Telegram")

			if err := telegrambot.PushEntry(
				feed,
				entry,
				userIntegrations.TelegramBotToken,
				userIntegrations.TelegramBotChatID,
				userIntegrations.TelegramBotTopicID,
				userIntegrations.TelegramBotDisableWebPagePreview,
				userIntegrations.TelegramBotDisableNotification,
				userIntegrations.TelegramBotDisableButtons,
			); err != nil {
				entryLog.Error("Unable to send entry to Telegram", "error", err)
			}
		}
	}
}
