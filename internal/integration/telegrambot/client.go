// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

package telegrambot

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"sss/internal/http/client"
)

const telegramAPIEndpoint = "https://api.telegram.org"

// HTMLFormatting is the HTML parse mode.
const HTMLFormatting = "HTML"

// Client is a Telegram Bot API API client.
type Client struct {
	botToken string
	chatID   string
}

// NewClient returns a Telegram Bot API client.
func NewClient(botToken, chatID string) *Client {
	return &Client{
		botToken: botToken,
		chatID:   chatID,
	}
}

// Specs: https://core.telegram.org/bots/api#sendmessage
func (c *Client) SendMessage(message *MessageRequest) error {
	endpointURL, err := url.JoinPath(telegramAPIEndpoint, "/bot"+c.botToken, "/sendMessage")
	if err != nil {
		return fmt.Errorf("telegram: unable to join base URL and path: %w", err)
	}

	response, err := client.NewRequestBuilder(endpointURL).
		WithMethod(http.MethodPost).
		WithHeader("Accept", "application/json").
		WithJSON(message).
		Do()
	if err != nil {
		return fmt.Errorf("telegram: unable to send request: %v", err)
	}
	defer response.Body.Close()

	var baseResponse BaseResponse
	if err := json.NewDecoder(response.Body).Decode(&baseResponse); err != nil {
		return fmt.Errorf("telegram: unable to decode discovery response: %w", err)
	}

	if !baseResponse.Ok {
		return fmt.Errorf("telegram: unable to send message: %s (error code is %d)", baseResponse.Description, baseResponse.ErrorCode)
	}

	return nil
}

// InlineKeyboard is a Telegram inline keyboard markup.
type InlineKeyboard struct {
	InlineKeyboard []InlineKeyboardRow `json:"inline_keyboard"`
}

// InlineKeyboardRow is one row of inline keyboard buttons.
type InlineKeyboardRow []*InlineKeyboardButton

// InlineKeyboardButton is an inline keyboard URL button.
type InlineKeyboardButton struct {
	Text string `json:"text"`
	URL  string `json:"url,omitempty"`
}

// BaseResponse holds the common Telegram API response fields.
type BaseResponse struct {
	Ok          bool   `json:"ok"`
	ErrorCode   int    `json:"error_code"`
	Description string `json:"description"`
}

// MessageRequest is a sendMessage request body.
type MessageRequest struct {
	ChatID                string          `json:"chat_id"`
	MessageThreadID       int64           `json:"message_thread_id,omitempty"`
	Text                  string          `json:"text"`
	ParseMode             string          `json:"parse_mode,omitempty"`
	DisableWebPagePreview bool            `json:"disable_web_page_preview"`
	DisableNotification   bool            `json:"disable_notification"`
	ReplyMarkup           *InlineKeyboard `json:"reply_markup,omitempty"`
}
