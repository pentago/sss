// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"encoding/hex"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
)

// WebAuthnCredential is a passkey registered by a user.
type WebAuthnCredential struct {
	Credential webauthn.Credential
	Name       string
	AddedOn    *time.Time
	LastSeenOn *time.Time
	Handle     []byte

	// False for rows predating the backup_eligible column; the login handler backfills from the assertion on first use.
	BackupEligibleKnown bool
}

// HandleEncoded returns the hex-encoded user handle.
func (s WebAuthnCredential) HandleEncoded() string {
	return hex.EncodeToString(s.Handle)
}
