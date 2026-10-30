// Copyright The Miniflux Authors. Modified by The sss Authors.
// Derived from Miniflux (https://github.com/miniflux/v2).
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"sss/internal/model"
)

const webSessionColumns = `id, secret_hash, user_id, created_at, user_agent, ip, state`

// anonymousWebSessionLifetime is how long a web session that never signed in is kept.
const anonymousWebSessionLifetime = 24 * time.Hour

// CreateWebSession persists a new web session built via model.NewWebSession.
func (s *Storage) CreateWebSession(session *model.WebSession) error {
	if session == nil {
		return errors.New("storage: web session is nil")
	}

	state, err := session.MarshalState()
	if err != nil {
		return fmt.Errorf("storage: unable to serialize web session state: %w", err)
	}

	err = s.db.QueryRow(`
		INSERT INTO web_sessions (id, secret_hash, user_agent, ip, state)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING created_at`,
		session.ID, session.SecretHash, session.UserAgent,
		sql.NullString{String: session.IP, Valid: session.IP != ""}, string(state),
	).Scan(unixTime{&session.CreatedAt})
	if err != nil {
		return fmt.Errorf("storage: unable to create web session: %w", err)
	}
	return nil
}

// WebSessionsByUserID returns web sessions for the given user.
func (s *Storage) WebSessionsByUserID(userID int64) ([]model.WebSession, error) {
	rows, err := s.db.Query(`SELECT `+webSessionColumns+` FROM web_sessions WHERE user_id=$1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("storage: unable to fetch web sessions: %w", err)
	}
	defer rows.Close()

	var sessions []model.WebSession
	for rows.Next() {
		session, err := scanWebSession(rows)
		if err != nil {
			return nil, fmt.Errorf("storage: unable to fetch web session row: %w", err)
		}
		sessions = append(sessions, *session)
	}
	return sessions, rows.Err()
}

// WebSessionByID returns the web session identified by id, or nil if not found.
func (s *Storage) WebSessionByID(sessionID string) (*model.WebSession, error) {
	if sessionID == "" {
		return nil, nil
	}

	session, err := scanWebSession(s.db.QueryRow(`SELECT `+webSessionColumns+` FROM web_sessions WHERE id=$1`, sessionID))
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return nil, nil
	case err != nil:
		return nil, fmt.Errorf("storage: unable to fetch web session: %w", err)
	}
	return session, nil
}

// RotateWebSession persists a session whose identity has been rotated via
// (*model.WebSession).Rotate(), updating the row previously keyed by oldID.
func (s *Storage) RotateWebSession(oldID string, session *model.WebSession) error {
	if session == nil {
		return errors.New("storage: web session is nil")
	}
	if oldID == "" || session.ID == "" {
		return errors.New("storage: web session ID cannot be empty")
	}

	state, err := session.MarshalState()
	if err != nil {
		return fmt.Errorf("storage: unable to serialize web session state: %w", err)
	}

	err = s.db.QueryRow(`
		UPDATE web_sessions SET id=$2, secret_hash=$3, user_id=$4, state=$5, created_at=unixepoch()
		WHERE id=$1
		RETURNING created_at`,
		oldID, session.ID, session.SecretHash, session.NullUserID(), string(state),
	).Scan(unixTime{&session.CreatedAt})
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return errors.New("storage: nothing has been updated")
	case err != nil:
		return fmt.Errorf("storage: unable to rotate web session: %w", err)
	}
	return nil
}

// UpdateWebSession updates the mutable fields of a web session.
func (s *Storage) UpdateWebSession(session *model.WebSession) error {
	if session == nil {
		return errors.New("storage: web session is nil")
	}
	if session.ID == "" {
		return errors.New("storage: web session ID cannot be empty")
	}

	state, err := session.MarshalState()
	if err != nil {
		return fmt.Errorf("storage: unable to serialize web session state: %w", err)
	}

	result, err := s.db.Exec(`UPDATE web_sessions SET user_id=$2, state=$3 WHERE id=$1`, session.ID, session.NullUserID(), string(state))
	if err != nil {
		return fmt.Errorf("storage: unable to update web session: %w", err)
	}
	if count, _ := result.RowsAffected(); count != 1 {
		return errors.New("storage: nothing has been updated")
	}
	return nil
}

// RemoveUserWebSession removes a web session for the given user if present.
func (s *Storage) RemoveUserWebSession(userID int64, sessionID string) error {
	if _, err := s.db.Exec(`DELETE FROM web_sessions WHERE user_id=$1 AND id=$2`, userID, sessionID); err != nil {
		return fmt.Errorf("storage: unable to remove this web session: %w", err)
	}
	return nil
}

// RemoveUserWebSessionsExcept removes all web sessions of the given user except
// the one identified by keepSessionID. An empty keepSessionID removes them all.
func (s *Storage) RemoveUserWebSessionsExcept(userID int64, keepSessionID string) error {
	if _, err := s.db.Exec(`DELETE FROM web_sessions WHERE user_id=$1 AND id<>$2`, userID, keepSessionID); err != nil {
		return fmt.Errorf("storage: unable to remove web sessions: %w", err)
	}
	return nil
}

// CleanOldWebSessions removes web sessions older than the specified interval (24h minimum),
// and anonymous web sessions older than 24 hours.
func (s *Storage) CleanOldWebSessions(interval time.Duration) (int64, error) {
	days := max(int64(interval/(24*time.Hour)), 1)
	result, err := s.db.Exec(`
		DELETE FROM web_sessions
		WHERE created_at < unixepoch() - $1 OR (user_id IS NULL AND created_at < unixepoch() - $2)`,
		days*86400, int64(anonymousWebSessionLifetime/time.Second),
	)
	if err != nil {
		return 0, fmt.Errorf("storage: unable to clean old web sessions: %w", err)
	}
	return result.RowsAffected()
}

// FlushAllSessions removes all sessions from the database.
func (s *Storage) FlushAllSessions() error {
	if _, err := s.db.Exec(`DELETE FROM web_sessions`); err != nil {
		return fmt.Errorf("storage: unable to delete all web sessions: %w", err)
	}
	return nil
}

func scanWebSession(row interface{ Scan(...any) error }) (*model.WebSession, error) {
	var session model.WebSession
	var userID sql.NullInt64
	var ip sql.NullString
	var state string

	if err := row.Scan(&session.ID, &session.SecretHash, &userID, unixTime{&session.CreatedAt}, &session.UserAgent, &ip, &state); err != nil {
		return nil, err
	}

	session.ScanUserID(userID)
	session.IP = ip.String
	if err := session.UnmarshalState([]byte(state)); err != nil {
		return nil, err
	}
	return &session, nil
}
