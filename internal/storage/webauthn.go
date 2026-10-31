// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"fmt"

	"github.com/go-webauthn/webauthn/webauthn"

	"sss/internal/model"
)

// AddWebAuthnCredential stores a new passkey of the user.
func (s *Storage) AddWebAuthnCredential(userID int64, handle []byte, credential *webauthn.Credential) error {
	_, err := s.db.Exec(`
		INSERT INTO webauthn_credentials
			(handle, cred_id, user_id, public_key, attestation_type, aaguid, sign_count, clone_warning, backup_eligible, backup_state)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		handle,
		credential.ID,
		userID,
		credential.PublicKey,
		credential.AttestationType,
		credential.Authenticator.AAGUID,
		credential.Authenticator.SignCount,
		credential.Authenticator.CloneWarning,
		credential.Flags.BackupEligible,
		credential.Flags.BackupState,
	)
	return err
}

// webAuthnDest returns scan destinations for: cred_id, public_key,
// attestation_type, aaguid, sign_count, clone_warning, name, added_on,
// last_seen_on, backup_eligible, backup_state.
func webAuthnDest(c *model.WebAuthnCredential) []any {
	return []any{
		&c.Credential.ID,
		&c.Credential.PublicKey,
		&c.Credential.AttestationType,
		&c.Credential.Authenticator.AAGUID,
		&c.Credential.Authenticator.SignCount,
		&c.Credential.Authenticator.CloneWarning,
		&c.Name,
		nullUnixTime{&c.AddedOn},
		nullUnixTime{&c.LastSeenOn},
		&c.Credential.Flags.BackupEligible,
		&c.Credential.Flags.BackupState,
	}
}

const webAuthnColumns = `cred_id, public_key, attestation_type, aaguid, sign_count, clone_warning, name, added_on, last_seen_on, backup_eligible, backup_state`

// WebAuthnCredentialByHandle returns the user ID and credential for the given handle.
func (s *Storage) WebAuthnCredentialByHandle(handle []byte) (int64, *model.WebAuthnCredential, error) {
	var credential model.WebAuthnCredential
	var userID int64

	dest := append([]any{&userID}, webAuthnDest(&credential)...)
	if err := s.db.QueryRow(`SELECT user_id, `+webAuthnColumns+` FROM webauthn_credentials WHERE handle=$1`, handle).Scan(dest...); err != nil {
		return 0, nil, err
	}

	credential.Handle = handle
	return userID, &credential, nil
}

// WebAuthnCredentialsByUserID returns all passkeys of the user.
func (s *Storage) WebAuthnCredentialsByUserID(userID int64) ([]model.WebAuthnCredential, error) {
	rows, err := s.db.Query(`SELECT handle, `+webAuthnColumns+` FROM webauthn_credentials WHERE user_id=$1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var creds []model.WebAuthnCredential
	for rows.Next() {
		var cred model.WebAuthnCredential
		if err := rows.Scan(append([]any{&cred.Handle}, webAuthnDest(&cred)...)...); err != nil {
			return nil, err
		}
		creds = append(creds, cred)
	}
	return creds, rows.Err()
}

// WebAuthnSaveLogin writes back the per-assertion fields (sign count, clone warning, backup state, BE) the WebAuthn spec requires after every successful login.
func (s *Storage) WebAuthnSaveLogin(handle []byte, credential *webauthn.Credential) error {
	_, err := s.db.Exec(`
		UPDATE webauthn_credentials
		SET last_seen_on=unixepoch(), sign_count=$1, clone_warning=$2, backup_eligible=$3, backup_state=$4
		WHERE handle=$5`,
		credential.Authenticator.SignCount,
		credential.Authenticator.CloneWarning,
		credential.Flags.BackupEligible,
		credential.Flags.BackupState,
		handle,
	)
	if err != nil {
		return fmt.Errorf("storage: unable to update webauthn credential after login: %w", err)
	}
	return nil
}

// WebAuthnUpdateName renames a passkey of the user.
func (s *Storage) WebAuthnUpdateName(userID int64, handle []byte, name string) error {
	if _, err := s.db.Exec(`UPDATE webauthn_credentials SET name=$1 WHERE handle=$2 AND user_id=$3`, name, handle, userID); err != nil {
		return fmt.Errorf("storage: unable to update name for webauthn credential: %w", err)
	}
	return nil
}

// DeleteCredentialByHandle removes a passkey of the user.
func (s *Storage) DeleteCredentialByHandle(userID int64, handle []byte) error {
	_, err := s.db.Exec(`DELETE FROM webauthn_credentials WHERE user_id=$1 AND handle=$2`, userID, handle)
	return err
}

// DeleteAllWebAuthnCredentialsByUserID removes all passkeys of the user.
func (s *Storage) DeleteAllWebAuthnCredentialsByUserID(userID int64) error {
	_, err := s.db.Exec(`DELETE FROM webauthn_credentials WHERE user_id=$1`, userID)
	return err
}
