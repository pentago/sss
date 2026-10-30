// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

package model

// Integration represents user integration settings.
type Integration struct {
	UserID                           int64
	InstapaperEnabled                bool
	InstapaperUsername               string
	InstapaperPassword               string
	FeverEnabled                     bool
	FeverUsername                    string
	FeverToken                       string
	GoogleReaderEnabled              bool
	GoogleReaderUsername             string
	GoogleReaderPassword             string
	NotionEnabled                    bool
	NotionToken                      string
	NotionPageID                     string
	TelegramBotEnabled               bool
	TelegramBotToken                 string
	TelegramBotChatID                string
	TelegramBotTopicID               *int64
	TelegramBotDisableWebPagePreview bool
	TelegramBotDisableNotification   bool
	TelegramBotDisableButtons        bool
	LinkwardenEnabled                bool
	LinkwardenURL                    string
	LinkwardenAPIKey                 string
	LinkwardenCollectionID           *int64
	MatrixBotEnabled                 bool
	MatrixBotUser                    string
	MatrixBotPassword                string
	MatrixBotURL                     string
	MatrixBotChatID                  string
	AppriseEnabled                   bool
	AppriseURL                       string
	AppriseServicesURL               string
	WebhookEnabled                   bool
	WebhookURL                       string
	WebhookSecret                    string
	RSSBridgeEnabled                 bool
	RSSBridgeURL                     string
	RSSBridgeToken                   string
	NtfyEnabled                      bool
	NtfyTopic                        string
	NtfyURL                          string
	NtfyAPIToken                     string
	NtfyUsername                     string
	NtfyPassword                     string
	NtfyIconURL                      string
	NtfyInternalLinks                bool
	DiscordEnabled                   bool
	DiscordWebhookLink               string
	SlackEnabled                     bool
	SlackWebhookLink                 string
	PushoverEnabled                  bool
	PushoverUser                     string
	PushoverToken                    string
	PushoverDevice                   string
	PushoverPrefix                   string
	GotifyEnabled                    bool
	GotifyURL                        string
	GotifyToken                      string
	GotifyPriority                   int
}
