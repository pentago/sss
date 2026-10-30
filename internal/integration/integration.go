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
	if userIntegrations.InstapaperEnabled {
		slog.Debug("Sending entry to Instapaper",
			slog.Int64("user_id", userIntegrations.UserID),
			slog.Int64("entry_id", entry.ID),
			slog.String("entry_url", entry.URL),
		)

		client := instapaper.NewClient(userIntegrations.InstapaperUsername, userIntegrations.InstapaperPassword)
		if err := client.AddURL(entry.URL, entry.Title); err != nil {
			slog.Error("Unable to send entry to Instapaper",
				slog.Int64("user_id", userIntegrations.UserID),
				slog.Int64("entry_id", entry.ID),
				slog.String("entry_url", entry.URL),
				slog.Any("error", err),
			)
		}
	}

	if userIntegrations.NotionEnabled {
		slog.Debug("Sending entry to Notion",
			slog.Int64("user_id", userIntegrations.UserID),
			slog.Int64("entry_id", entry.ID),
			slog.String("entry_url", entry.URL),
		)

		client := notion.NewClient(
			userIntegrations.NotionToken,
			userIntegrations.NotionPageID,
		)
		if err := client.UpdateDocument(entry.URL, entry.Title); err != nil {
			slog.Error("Unable to send entry to Notion",
				slog.Int64("user_id", userIntegrations.UserID),
				slog.Int64("entry_id", entry.ID),
				slog.String("entry_url", entry.URL),
				slog.Any("error", err),
			)
		}
	}

	if userIntegrations.LinkwardenEnabled {
		attrs := []any{
			slog.Int64("user_id", userIntegrations.UserID),
			slog.Int64("entry_id", entry.ID),
			slog.String("entry_url", entry.URL),
		}

		if userIntegrations.LinkwardenCollectionID != nil {
			attrs = append(attrs, slog.Int64("collection_id", *userIntegrations.LinkwardenCollectionID))
		}

		slog.Debug("Sending entry to linkwarden", attrs...)

		client := linkwarden.NewClient(
			userIntegrations.LinkwardenURL,
			userIntegrations.LinkwardenAPIKey,
			userIntegrations.LinkwardenCollectionID,
		)
		if err := client.CreateBookmark(entry.URL, entry.Title); err != nil {
			attrs = append(attrs, slog.Any("error", err))
			slog.Error("Unable to send entry to Linkwarden", attrs...)
		}
	}

	if userIntegrations.WebhookEnabled {
		var webhookURL string
		if entry.Feed != nil && entry.Feed.WebhookURL != "" {
			webhookURL = entry.Feed.WebhookURL
		} else {
			webhookURL = userIntegrations.WebhookURL
		}

		slog.Debug("Sending entry to Webhook",
			slog.Int64("user_id", userIntegrations.UserID),
			slog.Int64("entry_id", entry.ID),
			slog.String("entry_url", entry.URL),
			slog.String("webhook_url", webhookURL),
		)

		webhookClient := webhook.NewClient(webhookURL, userIntegrations.WebhookSecret)
		if err := webhookClient.SendSaveEntryWebhookEvent(entry); err != nil {
			slog.Error("Unable to send entry to Webhook",
				slog.Int64("user_id", userIntegrations.UserID),
				slog.Int64("entry_id", entry.ID),
				slog.String("entry_url", entry.URL),
				slog.String("webhook_url", webhookURL),
				slog.Any("error", err),
			)
		}
	}

}

// PushEntries pushes a list of entries to activated third-party providers during feed refreshes.
func PushEntries(feed *model.Feed, entries model.Entries, userIntegrations *model.Integration) {
	if userIntegrations.MatrixBotEnabled {
		slog.Debug("Sending new entries to Matrix",
			slog.Int64("user_id", userIntegrations.UserID),
			slog.Int("nb_entries", len(entries)),
			slog.Int64("feed_id", feed.ID),
		)

		err := matrixbot.PushEntries(
			feed,
			entries,
			userIntegrations.MatrixBotURL,
			userIntegrations.MatrixBotUser,
			userIntegrations.MatrixBotPassword,
			userIntegrations.MatrixBotChatID,
		)
		if err != nil {
			slog.Error("Unable to send new entries to Matrix",
				slog.Int64("user_id", userIntegrations.UserID),
				slog.Int("nb_entries", len(entries)),
				slog.Int64("feed_id", feed.ID),
				slog.Any("error", err),
			)
		}
	}
	if userIntegrations.WebhookEnabled {
		var webhookURL string
		if feed.WebhookURL != "" {
			webhookURL = feed.WebhookURL
		} else {
			webhookURL = userIntegrations.WebhookURL
		}

		slog.Debug("Sending new entries to Webhook",
			slog.Int64("user_id", userIntegrations.UserID),
			slog.Int("nb_entries", len(entries)),
			slog.Int64("feed_id", feed.ID),
			slog.String("webhook_url", webhookURL),
		)

		webhookClient := webhook.NewClient(webhookURL, userIntegrations.WebhookSecret)
		if err := webhookClient.SendNewEntriesWebhookEvent(feed, entries); err != nil {
			slog.Warn("Unable to send new entries to Webhook",
				slog.Int64("user_id", userIntegrations.UserID),
				slog.Int("nb_entries", len(entries)),
				slog.Int64("feed_id", feed.ID),
				slog.String("webhook_url", webhookURL),
				slog.Any("error", err),
			)
		}
	}

	if userIntegrations.NtfyEnabled && feed.NtfyEnabled {
		ntfyTopic := feed.NtfyTopic
		if ntfyTopic == "" {
			ntfyTopic = userIntegrations.NtfyTopic
		}
		slog.Debug("Sending new entries to Ntfy",
			slog.Int64("user_id", userIntegrations.UserID),
			slog.Int("nb_entries", len(entries)),
			slog.Int64("feed_id", feed.ID),
			slog.String("topic", ntfyTopic),
		)

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
			slog.Warn("Unable to send new entries to Ntfy", slog.Any("error", err))
		}
	}

	if userIntegrations.AppriseEnabled {
		slog.Debug("Sending new entries to Apprise",
			slog.Int64("user_id", userIntegrations.UserID),
			slog.Int("nb_entries", len(entries)),
			slog.Int64("feed_id", feed.ID),
		)

		appriseServiceURLs := userIntegrations.AppriseServicesURL
		if feed.AppriseServiceURLs != "" {
			appriseServiceURLs = feed.AppriseServiceURLs
		}

		client := apprise.NewClient(
			appriseServiceURLs,
			userIntegrations.AppriseURL,
		)

		if err := client.SendNotification(feed, entries); err != nil {
			slog.Warn("Unable to send new entries to Apprise", slog.Any("error", err))
		}
	}

	if userIntegrations.DiscordEnabled {
		slog.Debug("Sending new entries to Discord",
			slog.Int64("user_id", userIntegrations.UserID),
			slog.Int("nb_entries", len(entries)),
			slog.Int64("feed_id", feed.ID),
		)

		client := discord.NewClient(
			userIntegrations.DiscordWebhookLink,
		)

		if err := client.SendDiscordMsg(feed, entries); err != nil {
			slog.Warn("Unable to send new entries to Discord", slog.Any("error", err))
		}
	}

	if userIntegrations.SlackEnabled {
		slog.Debug("Sending new entries to Slack",
			slog.Int64("user_id", userIntegrations.UserID),
			slog.Int("nb_entries", len(entries)),
			slog.Int64("feed_id", feed.ID),
		)

		client := slack.NewClient(
			userIntegrations.SlackWebhookLink,
		)

		if err := client.SendSlackMsg(feed, entries); err != nil {
			slog.Warn("Unable to send new entries to Slack", slog.Any("error", err))
		}
	}

	if userIntegrations.PushoverEnabled && feed.PushoverEnabled {
		slog.Debug("Sending new entries to Pushover",
			slog.Int64("user_id", userIntegrations.UserID),
			slog.Int("nb_entries", len(entries)),
			slog.Int64("feed_id", feed.ID),
		)

		client := pushover.NewClient(
			userIntegrations.PushoverUser,
			userIntegrations.PushoverToken,
			feed.PushoverPriority,
			userIntegrations.PushoverDevice,
			userIntegrations.PushoverPrefix,
		)

		if err := client.SendMessages(feed, entries); err != nil {
			slog.Warn("Unable to send new entries to Pushover", slog.Any("error", err))
		}
	}

	if userIntegrations.GotifyEnabled {
		slog.Debug("Sending new entries to Gotify",
			slog.Int64("user_id", userIntegrations.UserID),
			slog.Int("nb_entries", len(entries)),
			slog.Int64("feed_id", feed.ID),
		)

		client := gotify.NewClient(
			userIntegrations.GotifyURL,
			userIntegrations.GotifyToken,
			userIntegrations.GotifyPriority,
		)

		if err := client.SendMessages(feed, entries); err != nil {
			slog.Warn("Unable to send new entries to Gotify", slog.Any("error", err))
		}
	}

	// Integrations that only support sending individual entries
	if userIntegrations.TelegramBotEnabled {
		for _, entry := range entries {
			slog.Debug("Sending a new entry to Telegram",
				slog.Int64("user_id", userIntegrations.UserID),
				slog.Int64("entry_id", entry.ID),
				slog.String("entry_url", entry.URL),
			)

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
				slog.Error("Unable to send entry to Telegram",
					slog.Int64("user_id", userIntegrations.UserID),
					slog.Int64("entry_id", entry.ID),
					slog.String("entry_url", entry.URL),
					slog.Any("error", err),
				)
			}
		}
	}

}
