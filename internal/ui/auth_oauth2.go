// Copyright The Miniflux Authors. Modified by The sss Authors.
// Derived from Miniflux (https://github.com/miniflux/v2).
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"sss/internal/config"
	"sss/internal/crypto"
	"sss/internal/http/request"
	"sss/internal/http/response"
	"sss/internal/model"
)

// oauth2Provider is the single provider configured by OAUTH2_PROVIDER (google or oidc).
type oauth2Provider struct {
	name   string
	config *oauth2.Config
	oidc   *oidc.Provider // nil for google
}

type oauth2Profile struct {
	id       string
	username string
	groups   []string
}

// newOAuth2Provider returns the configured provider when name matches OAUTH2_PROVIDER.
// ponytail: OIDC discovery runs per request like upstream; cache the provider if it shows up in latency.
func newOAuth2Provider(ctx context.Context, name string) (*oauth2Provider, error) {
	if name == "" || name != config.Opts.OAuth2Provider {
		return nil, fmt.Errorf("ui: oauth2 provider %q not configured", name)
	}
	p := &oauth2Provider{name: name, config: &oauth2.Config{
		ClientID:     config.Opts.OAuth2ClientID,
		ClientSecret: config.Opts.OAuth2ClientSecret,
		RedirectURL:  config.Opts.OAuth2RedirectURL,
	}}
	switch name {
	case "google":
		p.config.Scopes = []string{"email"}
		p.config.Endpoint = oauth2.Endpoint{
			AuthURL:  "https://accounts.google.com/o/oauth2/v2/auth",
			TokenURL: "https://oauth2.googleapis.com/token",
		}
	case "oidc":
		provider, err := oidc.NewProvider(ctx, config.Opts.OAuth2OIDCDiscoveryEndpoint)
		if err != nil {
			return nil, fmt.Errorf("ui: oidc discovery: %w", err)
		}
		p.oidc = provider
		p.config.Scopes = slices.Concat([]string{oidc.ScopeOpenID, "profile", "email"}, config.Opts.OAuth2AdditionalScopes)
		p.config.Endpoint = provider.Endpoint()
	default:
		return nil, fmt.Errorf("ui: unsupported oauth2 provider %q", name)
	}
	return p, nil
}

// userKey is the users column holding this provider's subject.
func (p *oauth2Provider) userKey() string {
	if p.name == "google" {
		return "google_id"
	}
	return "openid_connect_id"
}

// userField points at the user's stored subject for this provider.
func (p *oauth2Provider) userField(u *model.User) *string {
	if p.name == "google" {
		return &u.GoogleID
	}
	return &u.OpenIDConnectID
}

// authCodeURL returns the authorization URL plus the state and PKCE verifier to remember.
func (p *oauth2Provider) authCodeURL() (url, state, verifier string) {
	verifier = crypto.GenerateRandomStringHex(32)
	state = crypto.GenerateRandomStringHex(24)
	sum := sha256.Sum256([]byte(verifier))
	url = p.config.AuthCodeURL(state,
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
		oauth2.SetAuthURLParam("code_challenge", base64.RawURLEncoding.EncodeToString(sum[:])),
	)
	return url, state, verifier
}

func (p *oauth2Provider) profile(ctx context.Context, code, verifier string) (*oauth2Profile, error) {
	token, err := p.config.Exchange(ctx, code, oauth2.SetAuthURLParam("code_verifier", verifier))
	if err != nil {
		return nil, fmt.Errorf("ui: oauth2 token exchange: %w", err)
	}
	if p.oidc == nil {
		return googleProfile(ctx, p.config.Client(ctx, token))
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, errors.New("ui: oidc: no id_token in token response")
	}
	idToken, err := p.oidc.Verifier(&oidc.Config{ClientID: p.config.ClientID}).Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("ui: oidc: verify id token: %w", err)
	}
	info, err := p.oidc.UserInfo(ctx, oauth2.StaticTokenSource(token))
	if err != nil {
		return nil, fmt.Errorf("ui: oidc: user info: %w", err)
	}
	if idToken.Subject != info.Subject {
		return nil, errors.New("ui: oidc: id token subject does not match userinfo subject")
	}
	var claims map[string]any
	if err := info.Claims(&claims); err != nil {
		return nil, fmt.Errorf("ui: oidc: parse claims: %w", err)
	}
	return oidcProfile(info.Subject, claims, config.Opts.OAuth2GroupClaim)
}

func googleProfile(ctx context.Context, client *http.Client) (*oauth2Profile, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://www.googleapis.com/oauth2/v3/userinfo", nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ui: google user info: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ui: google user info: status %d", resp.StatusCode)
	}
	var v struct{ Sub, Email string }
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		return nil, fmt.Errorf("ui: google user info: %w", err)
	}
	return &oauth2Profile{id: v.Sub, username: v.Email}, nil
}

// oidcProfile builds a profile from userinfo claims: username is the first non-empty of
// preferred_username, email, name, profile; groups come from groupClaim (string or string array).
func oidcProfile(subject string, claims map[string]any, groupClaim string) (*oauth2Profile, error) {
	p := &oauth2Profile{id: subject}
	for _, key := range []string{"preferred_username", "email", "name", "profile"} {
		if s, _ := claims[key].(string); s != "" {
			p.username = s
			break
		}
	}
	if groupClaim != "" {
		switch v := claims[groupClaim].(type) {
		case string:
			p.groups = []string{v}
		case []any:
			for _, g := range v {
				s, ok := g.(string)
				if !ok {
					return nil, fmt.Errorf("ui: oidc: claim %q must only contain strings", groupClaim)
				}
				p.groups = append(p.groups, s)
			}
		}
	}
	if p.username == "" {
		return nil, errors.New("ui: oidc: username is empty")
	}
	return p, nil
}

func (h *handler) oauth2Redirect(w http.ResponseWriter, r *http.Request) {
	provider, err := newOAuth2Provider(r.Context(), r.PathValue("provider"))
	if err != nil {
		slog.Error("Unable to initialize OAuth2 provider", slog.Any("error", err))
		response.HTMLRedirect(w, r, config.Opts.BasePath+"/")
		return
	}
	url, state, verifier := provider.authCodeURL()
	request.WebSession(r).StartOAuth2Flow(state, verifier)
	response.HTMLRedirect(w, r, url)
}

func (h *handler) oauth2Callback(w http.ResponseWriter, r *http.Request) {
	home := config.Opts.BasePath + "/"
	sess := request.WebSession(r)
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	expected := sess.OAuth2State()
	verifier := sess.OAuth2CodeVerifier()
	sess.ClearOAuth2Flow()
	if code == "" || expected == "" || subtle.ConstantTimeCompare([]byte(state), []byte(expected)) == 0 {
		slog.Warn("Invalid OAuth2 callback: missing code or bad state")
		response.HTMLRedirect(w, r, home)
		return
	}

	provider, err := newOAuth2Provider(r.Context(), r.PathValue("provider"))
	if err != nil {
		slog.Error("Unable to initialize OAuth2 provider", slog.Any("error", err))
		response.HTMLRedirect(w, r, home)
		return
	}
	profile, err := provider.profile(r.Context(), code, verifier)
	if err != nil {
		slog.Warn("Unable to get OAuth2 profile", slog.Any("error", err))
		response.HTMLRedirect(w, r, home)
		return
	}

	authPage := config.Opts.BasePath + "/authentication"
	if request.IsAuthenticated(r) {
		user, err := h.store.UserByID(request.UserID(r))
		if err != nil {
			response.HTMLServerError(w, r, err)
			return
		}
		field := provider.userField(user)
		if h.store.AnotherUserWithFieldExists(user.ID, provider.userKey(), profile.id) || (*field != "" && *field != profile.id) {
			sess.SetErrorMessage("This account is already linked to another user.")
			response.HTMLRedirect(w, r, authPage)
			return
		}
		*field = profile.id
		if err := h.store.UpdateUser(user); err != nil {
			response.HTMLServerError(w, r, err)
			return
		}
		sess.SetSuccessMessage("Your external account is now linked.")
		response.HTMLRedirect(w, r, authPage)
		return
	}

	user, err := h.store.UserByField(provider.userKey(), profile.id)
	if err != nil {
		response.HTMLServerError(w, r, err)
		return
	}
	adminMapping := provider.name == "oidc" && config.Opts.OAuth2GroupClaim != "" && config.Opts.OAuth2AdminGroup != ""
	isAdmin := adminMapping && slices.Contains(profile.groups, config.Opts.OAuth2AdminGroup)

	if user == nil {
		if !config.Opts.OAuth2UserCreation {
			response.HTMLForbidden(w, r)
			return
		}
		if h.store.UserExists(profile.username) {
			response.HTMLBadRequest(w, r, errors.New("This user already exists."))
			return
		}
		req := &model.UserCreationRequest{Username: profile.username, IsAdmin: isAdmin}
		if provider.name == "google" {
			req.GoogleID = profile.id
		} else {
			req.OpenIDConnectID = profile.id
		}
		if user, err = h.store.CreateUser(req); err != nil {
			response.HTMLServerError(w, r, err)
			return
		}
	} else if adminMapping && user.IsAdmin != isAdmin {
		user.IsAdmin = isAdmin
		if err := h.store.UpdateUser(user); err != nil {
			response.HTMLServerError(w, r, err)
			return
		}
	}

	slog.Info("User authenticated successfully using OAuth2",
		slog.String("client_ip", request.ClientIP(r)),
		slog.Int64("user_id", user.ID),
		slog.String("username", user.Username),
	)
	h.login(w, r, user, "")
}

func (h *handler) oauth2Unlink(w http.ResponseWriter, r *http.Request) {
	authPage := config.Opts.BasePath + "/authentication"
	if config.Opts.DisableLocalAuth {
		response.HTMLRedirect(w, r, authPage)
		return
	}
	provider, err := newOAuth2Provider(r.Context(), r.PathValue("provider"))
	if err != nil {
		slog.Error("Unable to initialize OAuth2 provider", slog.Any("error", err))
		response.HTMLRedirect(w, r, authPage)
		return
	}
	user, err := h.store.UserByID(request.UserID(r))
	if err != nil {
		response.HTMLServerError(w, r, err)
		return
	}
	hasPassword, err := h.store.HasPassword(user.ID)
	if err != nil {
		response.HTMLServerError(w, r, err)
		return
	}
	sess := request.WebSession(r)
	if !hasPassword {
		sess.SetErrorMessage("You must define a password before unlinking your account, otherwise you won't be able to log in.")
		response.HTMLRedirect(w, r, authPage)
		return
	}
	*provider.userField(user) = ""
	if err := h.store.UpdateUser(user); err != nil {
		response.HTMLServerError(w, r, err)
		return
	}
	sess.SetSuccessMessage("Your external account is now unlinked.")
	response.HTMLRedirect(w, r, authPage)
}
