// Copyright The Miniflux Authors. Modified by The sss Authors.
// Derived from Miniflux (https://github.com/miniflux/v2).
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"errors"
	"fmt"

	"sss/internal/crypto"
	"sss/internal/model"
)

// ErrAPIKeyNotFound is returned when deleting an API key that doesn't exist.
var ErrAPIKeyNotFound = errors.New("storage: API Key not found")

const apiKeyColumns = `id, user_id, token, description, last_used_at, created_at`

func apiKeyDest(k *model.APIKey) []any {
	return []any{&k.ID, &k.UserID, &k.Token, &k.Description, nullUnixTime{&k.LastUsedAt}, unixTime{&k.CreatedAt}}
}

// APIKeyExists checks if an API Key with the same description exists.
func (s *Storage) APIKeyExists(userID int64, description string) bool {
	var result bool
	s.db.QueryRow(`SELECT true FROM api_keys WHERE user_id=$1 AND lower(description)=lower($2) LIMIT 1`, userID, description).Scan(&result)
	return result
}

// SetAPIKeyUsedTimestamp updates the last used date of an API Key.
func (s *Storage) SetAPIKeyUsedTimestamp(userID int64, token string) error {
	if _, err := s.db.Exec(`UPDATE api_keys SET last_used_at=unixepoch() WHERE user_id=$1 AND token=$2`, userID, token); err != nil {
		return fmt.Errorf("storage: unable to update last used date for API key: %w", err)
	}
	return nil
}

// APIKeys returns all API Keys that belongs to the given user.
func (s *Storage) APIKeys(userID int64) (model.APIKeys, error) {
	rows, err := s.db.Query(`SELECT `+apiKeyColumns+` FROM api_keys WHERE user_id=$1 ORDER BY description ASC`, userID)
	if err != nil {
		return nil, fmt.Errorf("storage: unable to fetch API Keys: %w", err)
	}
	defer rows.Close()

	apiKeys := make(model.APIKeys, 0)
	for rows.Next() {
		var apiKey model.APIKey
		if err := rows.Scan(apiKeyDest(&apiKey)...); err != nil {
			return nil, fmt.Errorf("storage: unable to fetch API Key row: %w", err)
		}
		apiKeys = append(apiKeys, apiKey)
	}
	return apiKeys, rows.Err()
}

// CreateAPIKey inserts a new API key.
func (s *Storage) CreateAPIKey(userID int64, description string) (*model.APIKey, error) {
	var apiKey model.APIKey
	err := s.db.QueryRow(`
		INSERT INTO api_keys (user_id, token, description) VALUES ($1, $2, $3)
		RETURNING `+apiKeyColumns,
		userID, crypto.GenerateRandomStringHex(32), description,
	).Scan(apiKeyDest(&apiKey)...)
	if err != nil {
		return nil, fmt.Errorf("storage: unable to create API Key: %w", err)
	}
	return &apiKey, nil
}

// DeleteAPIKey deletes an API Key.
func (s *Storage) DeleteAPIKey(userID, keyID int64) error {
	result, err := s.db.Exec(`DELETE FROM api_keys WHERE id=$1 AND user_id=$2`, keyID, userID)
	if err != nil {
		return fmt.Errorf("storage: unable to delete this API Key: %w", err)
	}
	if count, _ := result.RowsAffected(); count == 0 {
		return ErrAPIKeyNotFound
	}
	return nil
}
