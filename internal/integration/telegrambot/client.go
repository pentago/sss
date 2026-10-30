// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

package telegrambot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"sss/internal/http/client"
	"sss/internal/version"
)

const (
	defaultClientTimeout = 10 * time.Second
	telegramAPIEndpoint  = "https://api.telegram.org"

	// MarkdownFormatting is the legacy Markdown parse mode.
	MarkdownFormatting = "Markdown"
	// MarkdownV2Formatting is the MarkdownV2 parse mode.
	MarkdownV2Formatting = "MarkdownV2"
	// HTMLFormatting is the HTML parse mode.
	HTMLFormatting = "HTML"
)

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

// Specs: https://core.telegram.org/bots/api#getme
func (c *Client) GetMe() (*User, error) {
	endpointURL, err := url.JoinPath(telegramAPIEndpoint, "/bot"+c.botToken, "/getMe")
	if err != nil {
		return nil, fmt.Errorf("telegram: unable to join base URL and path: %w", err)
	}

	request, err := http.NewRequest(http.MethodGet, endpointURL, nil)
	if err != nil {
		return nil, fmt.Errorf("telegram: unable to create request: %v", err)
	}

	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "sss/"+version.Version)

	httpClient := client.NewClientWithOptions(client.Options{Timeout: defaultClientTimeout})
	response, err := httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("telegram: unable to send request: %v", err)
	}
	defer response.Body.Close()

	var userResponse UserResponse
	if err := json.NewDecoder(response.Body).Decode(&userResponse); err != nil {
		return nil, fmt.Errorf("telegram: unable to decode user response: %w", err)
	}

	if !userResponse.Ok {
		return nil, fmt.Errorf("telegram: unable to send message: %s (error code is %d)", userResponse.Description, userResponse.ErrorCode)
	}

	return &userResponse.Result, nil
}

// Specs: https://core.telegram.org/bots/api#sendmessage
func (c *Client) SendMessage(message *MessageRequest) (*Message, error) {
	endpointURL, err := url.JoinPath(telegramAPIEndpoint, "/bot"+c.botToken, "/sendMessage")
	if err != nil {
		return nil, fmt.Errorf("telegram: unable to join base URL and path: %w", err)
	}

	requestBody, err := json.Marshal(message)
	if err != nil {
		return nil, fmt.Errorf("telegram: unable to encode request body: %v", err)
	}

	request, err := http.NewRequest(http.MethodPost, endpointURL, bytes.NewReader(requestBody))
	if err != nil {
		return nil, fmt.Errorf("telegram: unable to create request: %v", err)
	}

	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "sss/"+version.Version)

	httpClient := client.NewClientWithOptions(client.Options{Timeout: defaultClientTimeout})
	response, err := httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("telegram: unable to send request: %v", err)
	}
	defer response.Body.Close()

	var messageResponse MessageResponse
	if err := json.NewDecoder(response.Body).Decode(&messageResponse); err != nil {
		return nil, fmt.Errorf("telegram: unable to decode discovery response: %w", err)
	}

	if !messageResponse.Ok {
		return nil, fmt.Errorf("telegram: unable to send message: %s (error code is %d)", messageResponse.Description, messageResponse.ErrorCode)
	}

	return &messageResponse.Result, nil
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

// User is a Telegram user or bot.
type User struct {
	ID                      int64  `json:"id"`
	FirstName               string `json:"first_name"`
	LastName                string `json:"last_name"`
	Username                string `json:"username"`
	LanguageCode            string `json:"language_code"`
	IsBot                   bool   `json:"is_bot"`
	IsPremium               bool   `json:"is_premium"`
	CanJoinGroups           bool   `json:"can_join_groups"`
	CanReadAllGroupMessages bool   `json:"can_read_all_group_messages"`
	SupportsInlineQueries   bool   `json:"supports_inline_queries"`
}

// Chat is a Telegram chat.
type Chat struct {
	ID    int64  `json:"id"`
	Type  string `json:"type"`
	Title string `json:"title"`
}

// Message is a sent Telegram message.
type Message struct {
	MessageID       int64 `json:"message_id"`
	From            User  `json:"from"`
	Chat            Chat  `json:"chat"`
	MessageThreadID int64 `json:"message_thread_id"`
	Date            int64 `json:"date"`
}

// BaseResponse holds the common Telegram API response fields.
type BaseResponse struct {
	Ok          bool   `json:"ok"`
	ErrorCode   int    `json:"error_code"`
	Description string `json:"description"`
}

// UserResponse is the getMe response.
type UserResponse struct {
	BaseResponse
	Result User `json:"result"`
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

// MessageResponse is the sendMessage response.
type MessageResponse struct {
	BaseResponse
	Result Message `json:"result"`
}
