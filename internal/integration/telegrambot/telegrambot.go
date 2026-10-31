// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

package telegrambot

import (
	"fmt"
	"log/slog"
	"strconv"

	"sss/internal/config"
	"sss/internal/model"
	"sss/internal/urllib"
)

// PushEntry sends one entry to a Telegram chat, optionally with link buttons.
func PushEntry(feed *model.Feed, entry *model.Entry, botToken, chatID string, topicID *int64, disableWebPagePreview, disableNotification bool, disableButtons bool) error {
	formattedText := fmt.Sprintf(
		`<b>%s</b> - <a href=%q>%s</a>`,
		feed.Title,
		entry.URL,
		entry.Title,
	)

	message := &MessageRequest{
		ChatID:                chatID,
		Text:                  formattedText,
		ParseMode:             HTMLFormatting,
		DisableWebPagePreview: disableWebPagePreview,
		DisableNotification:   disableNotification,
	}

	if topicID != nil {
		message.MessageThreadID = *topicID
	}

	if !disableButtons {
		var markupRow []*InlineKeyboardButton

		baseURL := config.Opts.BaseURL
		entryPath := "/unread/entry/" + strconv.FormatInt(entry.ID, 10)

		sssEntryURL, err := urllib.JoinBaseURLAndPath(baseURL, entryPath)
		if err != nil {
			slog.Error("Unable to create entry URL", slog.Any("error", err))
		} else {
			sssEntryURLButton := InlineKeyboardButton{Text: "Go to sss", URL: sssEntryURL}
			markupRow = append(markupRow, &sssEntryURLButton)
		}

		articleURLButton := InlineKeyboardButton{Text: "Go to article", URL: entry.URL}
		markupRow = append(markupRow, &articleURLButton)

		if entry.CommentsURL != "" {
			commentURLButton := InlineKeyboardButton{Text: "Comments", URL: entry.CommentsURL}
			markupRow = append(markupRow, &commentURLButton)
		}

		message.ReplyMarkup = &InlineKeyboard{}
		message.ReplyMarkup.InlineKeyboard = append(message.ReplyMarkup.InlineKeyboard, markupRow)
	}

	client := NewClient(botToken, chatID)
	return client.SendMessage(message)
}
