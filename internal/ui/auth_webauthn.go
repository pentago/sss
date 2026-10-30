// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"

	"sss/internal/config"
	"sss/internal/crypto"
	"sss/internal/http/request"
	"sss/internal/http/response"
	"sss/internal/model"
)

type webAuthnUser struct {
	user        *model.User
	handle      []byte
	credentials []model.WebAuthnCredential
}

func (u webAuthnUser) WebAuthnID() []byte          { return u.handle }
func (u webAuthnUser) WebAuthnName() string        { return u.user.Username }
func (u webAuthnUser) WebAuthnDisplayName() string { return u.user.Username }
func (u webAuthnUser) WebAuthnCredentials() []webauthn.Credential {
	creds := make([]webauthn.Credential, len(u.credentials))
	for i, c := range u.credentials {
		creds[i] = c.Credential
	}
	return creds
}

func newWebAuthn() (*webauthn.WebAuthn, error) {
	baseURL, err := url.Parse(config.Opts.BaseURL)
	if err != nil {
		return nil, err
	}
	return webauthn.New(&webauthn.Config{
		RPDisplayName: "sss",
		RPID:          baseURL.Hostname(),
		RPOrigins:     []string{config.Opts.RootURL},
	})
}

func (h *handler) webAuthnRegisterBegin(w http.ResponseWriter, r *http.Request) {
	web, err := newWebAuthn()
	if err != nil {
		response.JSONServerError(w, r, err)
		return
	}
	user := currentUser(r)
	credentials, err := h.store.WebAuthnCredentialsByUserID(user.ID)
	if err != nil {
		response.JSONServerError(w, r, err)
		return
	}
	exclusions := make([]protocol.CredentialDescriptor, len(credentials))
	for i, c := range credentials {
		exclusions[i] = c.Credential.Descriptor()
	}
	options, session, err := web.BeginRegistration(
		webAuthnUser{user: user, handle: crypto.GenerateRandomBytes(32)},
		webauthn.WithExclusions(exclusions),
		webauthn.WithResidentKeyRequirement(protocol.ResidentKeyRequirementRequired),
	)
	if err != nil {
		response.JSONServerError(w, r, err)
		return
	}
	request.WebSession(r).SetWebAuthn(session)
	response.JSON(w, r, options)
}

func (h *handler) webAuthnRegisterFinish(w http.ResponseWriter, r *http.Request) {
	web, err := newWebAuthn()
	if err != nil {
		response.JSONServerError(w, r, err)
		return
	}
	user := currentUser(r)
	session := request.WebSession(r).ConsumeWebAuthnSession()
	if session == nil {
		response.JSONBadRequest(w, r, errors.New("missing webauthn session data"))
		return
	}
	credential, err := web.FinishRegistration(webAuthnUser{user: user, handle: session.UserID}, *session, r)
	if err != nil {
		response.JSONBadRequest(w, r, err)
		return
	}
	if err := h.store.AddWebAuthnCredential(user.ID, session.UserID, credential); err != nil {
		response.JSONServerError(w, r, err)
		return
	}
	response.JSON(w, r, map[string]string{"redirect": route("/webauthn/%s/rename", hex.EncodeToString(session.UserID))})
}

func (h *handler) webAuthnLoginBegin(w http.ResponseWriter, r *http.Request) {
	web, err := newWebAuthn()
	if err != nil {
		response.JSONServerError(w, r, err)
		return
	}
	assertion, session, err := web.BeginDiscoverableLogin()
	if err != nil {
		response.JSONServerError(w, r, err)
		return
	}
	request.WebSession(r).SetWebAuthn(session)
	response.JSON(w, r, assertion)
}

func (h *handler) webAuthnLoginFinish(w http.ResponseWriter, r *http.Request) {
	web, err := newWebAuthn()
	if err != nil {
		response.JSONServerError(w, r, err)
		return
	}
	parsed, err := protocol.ParseCredentialRequestResponseBody(r.Body)
	if err != nil {
		response.JSONBadRequest(w, r, err)
		return
	}
	session := request.WebSession(r).ConsumeWebAuthnSession()
	if session == nil {
		response.JSONBadRequest(w, r, errors.New("missing webauthn session data"))
		return
	}

	var user *model.User
	var credential *model.WebAuthnCredential
	userByHandle := func(_, handle []byte) (webauthn.User, error) {
		userID, cred, err := h.store.WebAuthnCredentialByHandle(handle)
		if err != nil {
			return nil, err
		}
		if cred == nil {
			return nil, fmt.Errorf("no user found for handle %x", handle)
		}
		u, err := h.store.UserByID(userID)
		if err != nil {
			return nil, err
		}
		if u == nil {
			return nil, fmt.Errorf("no user found for handle %x", handle)
		}
		// Credentials stored before backup_eligible was tracked trust the assertion once.
		if !cred.BackupEligibleKnown {
			cred.Credential.Flags.BackupEligible = parsed.Response.AuthenticatorData.Flags.HasBackupEligible()
		}
		user, credential = u, cred
		return webAuthnUser{user: u, handle: handle, credentials: []model.WebAuthnCredential{*cred}}, nil
	}
	validated, err := web.ValidateDiscoverableLogin(userByHandle, *session, parsed)
	if err != nil {
		slog.Warn("WebAuthn login failed", slog.String("client_ip", request.ClientIP(r)), slog.Any("error", err))
		response.JSONUnauthorized(w, r)
		return
	}
	if err := h.store.WebAuthnSaveLogin(credential.Handle, validated); err != nil {
		slog.Warn("WebAuthn: unable to persist credential state", slog.Int64("user_id", user.ID), slog.Any("error", err))
	}
	if err := h.store.SetLastLogin(user.ID); err != nil {
		slog.Warn("Unable to update last login date", slog.Int64("user_id", user.ID), slog.Any("error", err))
	}
	if err := authenticateWebSession(w, r, h.store, user); err != nil {
		response.JSONServerError(w, r, err)
		return
	}
	slog.Info("User authenticated successfully with webauthn", slog.Int64("user_id", user.ID), slog.String("username", user.Username))
	response.JSON(w, r, map[string]string{"redirect": loginRedirect(r.URL.Query().Get("redirect_url"), user)})
}

// ownedCredential decodes the {credentialHandle} path value and checks it belongs to the current user.
func (h *handler) ownedCredential(w http.ResponseWriter, r *http.Request) ([]byte, *model.WebAuthnCredential, bool) {
	handle, err := hex.DecodeString(r.PathValue("credentialHandle"))
	if err != nil {
		response.HTMLNotFound(w, r)
		return nil, nil, false
	}
	userID, credential, err := h.store.WebAuthnCredentialByHandle(handle)
	if err != nil {
		response.HTMLServerError(w, r, err)
		return nil, nil, false
	}
	if credential == nil || userID != request.UserID(r) {
		response.HTMLNotFound(w, r)
		return nil, nil, false
	}
	return handle, credential, true
}

func (h *handler) webAuthnRename(w http.ResponseWriter, r *http.Request) {
	if _, credential, ok := h.ownedCredential(w, r); ok {
		h.renderAccount(w, r, "auth_webauthn_rename", map[string]any{"title": "Rename passkey", "menu": "settings", "credential": credential, "name": credential.Name})
	}
}

func (h *handler) webAuthnSave(w http.ResponseWriter, r *http.Request) {
	handle, credential, ok := h.ownedCredential(w, r)
	if !ok {
		return
	}
	name := r.FormValue("name")
	if name == "" {
		h.renderAccount(w, r, "auth_webauthn_rename", map[string]any{"title": "Rename passkey", "menu": "settings", "credential": credential, "errorMessage": "The name is required."})
		return
	}
	if _, err := h.store.WebAuthnUpdateName(request.UserID(r), handle, name); err != nil {
		response.HTMLServerError(w, r, err)
		return
	}
	response.HTMLRedirect(w, r, route("/authentication"))
}

func (h *handler) webAuthnDelete(w http.ResponseWriter, r *http.Request) {
	handle, _, ok := h.ownedCredential(w, r)
	if !ok {
		return
	}
	if err := h.store.DeleteCredentialByHandle(request.UserID(r), handle); err != nil {
		response.HTMLServerError(w, r, err)
		return
	}
	response.HTMLRedirect(w, r, route("/authentication"))
}

func (h *handler) webAuthnDeleteAll(w http.ResponseWriter, r *http.Request) {
	if err := h.store.DeleteAllWebAuthnCredentialsByUserID(request.UserID(r)); err != nil {
		response.HTMLServerError(w, r, err)
		return
	}
	response.HTMLRedirect(w, r, route("/authentication"))
}
