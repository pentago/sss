// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"sss/internal/crypto"
	"sss/internal/model"
)

var dummyBcryptHash = []byte("$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy")

const userColumns = `id, username, is_admin, theme, timezone, entry_direction, entry_order,
	stylesheet, custom_js, external_font_hosts, google_id, openid_connect_id, entries_per_page,
	gesture_nav, last_login_at, display_mode, default_reading_speed, cjk_reading_speed,
	default_home_page, categories_sorting_order, block_filter_entry_rules, keep_filter_entry_rules,
	mark_read_on_view, always_open_external_links, open_external_links_in_new_tab,
	keyboard_shortcuts, show_reading_time, entry_swipe, mark_read_on_scroll, service_worker,
	theme_light_hour, theme_dark_hour, 'en_US'`

// userDest returns the scan destinations matching userColumns.
func userDest(u *model.User) []any {
	return []any{
		&u.ID, &u.Username, &u.IsAdmin, &u.Theme, &u.Timezone, &u.EntryDirection, &u.EntryOrder,
		&u.Stylesheet, &u.CustomJS, &u.ExternalFontHosts, &u.GoogleID, &u.OpenIDConnectID, &u.EntriesPerPage,
		&u.GestureNav, nullUnixTime{&u.LastLoginAt}, &u.DisplayMode, &u.DefaultReadingSpeed, &u.CJKReadingSpeed,
		&u.DefaultHomePage, &u.CategoriesSortingOrder, &u.BlockFilterEntryRules, &u.KeepFilterEntryRules,
		&u.MarkReadOnView, &u.AlwaysOpenExternalLinks, &u.OpenExternalLinksInNewTab,
		&u.KeyboardShortcuts, &u.ShowReadingTime, &u.EntrySwipe, &u.MarkReadOnScroll, &u.ServiceWorker,
		&u.ThemeLightHour, &u.ThemeDarkHour, &u.Language,
	}
}

// SetLastLogin sets the user's last login timestamp to the current time.
func (s *Storage) SetLastLogin(userID int64) error {
	if _, err := s.db.Exec(`UPDATE users SET last_login_at=unixepoch() WHERE id=$1`, userID); err != nil {
		return fmt.Errorf("storage: unable to update last login date: %w", err)
	}
	return nil
}

// UserExists returns true if a user with the given username exists.
func (s *Storage) UserExists(username string) bool {
	var result bool
	s.db.QueryRow(`SELECT true FROM users WHERE username=lower($1) LIMIT 1`, username).Scan(&result)
	return result
}

// AnotherUserExists returns true if a user other than userID has the given username.
func (s *Storage) AnotherUserExists(userID int64, username string) bool {
	var result bool
	s.db.QueryRow(`SELECT true FROM users WHERE id != $1 AND username=lower($2) LIMIT 1`, userID, username).Scan(&result)
	return result
}

// CreateUser creates a new user with its default category and integration row.
func (s *Storage) CreateUser(request *model.UserCreationRequest) (*model.User, error) {
	var hashedPassword string
	if request.Password != "" {
		var err error
		if hashedPassword, err = crypto.HashPassword(request.Password); err != nil {
			return nil, err
		}
	}

	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("storage: unable to start transaction: %w", err)
	}

	var user model.User
	err = tx.QueryRow(`
		INSERT INTO users (username, password, is_admin, google_id, openid_connect_id)
		VALUES (lower($1), $2, $3, $4, $5)
		RETURNING `+userColumns,
		request.Username, hashedPassword, request.IsAdmin, request.GoogleID, request.OpenIDConnectID,
	).Scan(userDest(&user)...)
	if err != nil {
		return nil, rollback(tx, fmt.Errorf("storage: unable to create user: %w", err))
	}

	if _, err := tx.Exec(`INSERT INTO categories (user_id, title) VALUES ($1, 'All')`, user.ID); err != nil {
		return nil, rollback(tx, fmt.Errorf("storage: unable to create user default category: %w", err))
	}

	if _, err := tx.Exec(`INSERT INTO integrations (user_id) VALUES ($1)`, user.ID); err != nil {
		return nil, rollback(tx, fmt.Errorf("storage: unable to create integration row: %w", err))
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("storage: unable to commit transaction: %w", err)
	}
	return &user, nil
}

// UpdateUser updates a user. The password is changed only when user.Password is not empty.
func (s *Storage) UpdateUser(user *model.User) error {
	user.ExternalFontHosts = strings.TrimSpace(user.ExternalFontHosts)

	var hashedPassword string
	if user.Password != "" {
		var err error
		if hashedPassword, err = crypto.HashPassword(user.Password); err != nil {
			return err
		}
	}

	_, err := s.db.Exec(`
		UPDATE users SET
			username=lower($1),
			is_admin=$2,
			theme=$3,
			timezone=$4,
			entry_direction=$5,
			entry_order=$6,
			stylesheet=$7,
			custom_js=$8,
			external_font_hosts=$9,
			google_id=$10,
			openid_connect_id=$11,
			entries_per_page=$12,
			gesture_nav=$13,
			display_mode=$14,
			default_reading_speed=$15,
			cjk_reading_speed=$16,
			default_home_page=$17,
			categories_sorting_order=$18,
			block_filter_entry_rules=$19,
			keep_filter_entry_rules=$20,
			mark_read_on_view=$21,
			always_open_external_links=$22,
			open_external_links_in_new_tab=$23,
			keyboard_shortcuts=$24,
			show_reading_time=$25,
			entry_swipe=$26,
			mark_read_on_scroll=$27,
			service_worker=$28,
			theme_light_hour=$29,
			theme_dark_hour=$30,
			password=CASE WHEN $31 = '' THEN password ELSE $31 END
		WHERE id=$32`,
		user.Username, user.IsAdmin, user.Theme, user.Timezone, user.EntryDirection, user.EntryOrder,
		user.Stylesheet, user.CustomJS, user.ExternalFontHosts, user.GoogleID, user.OpenIDConnectID,
		user.EntriesPerPage, user.GestureNav, user.DisplayMode, user.DefaultReadingSpeed, user.CJKReadingSpeed,
		user.DefaultHomePage, user.CategoriesSortingOrder, user.BlockFilterEntryRules, user.KeepFilterEntryRules,
		user.MarkReadOnView, user.AlwaysOpenExternalLinks, user.OpenExternalLinksInNewTab,
		user.KeyboardShortcuts, user.ShowReadingTime, user.EntrySwipe, user.MarkReadOnScroll, user.ServiceWorker,
		user.ThemeLightHour, user.ThemeDarkHour, hashedPassword, user.ID,
	)
	if err != nil {
		return fmt.Errorf("storage: unable to update user: %w", err)
	}
	return nil
}

// UserByID finds a user by the ID.
func (s *Storage) UserByID(userID int64) (*model.User, error) {
	return s.fetchUser(`SELECT `+userColumns+` FROM users WHERE id=$1`, userID)
}

// UserByUsername finds a user by the username.
func (s *Storage) UserByUsername(username string) (*model.User, error) {
	return s.fetchUser(`SELECT `+userColumns+` FROM users WHERE username=lower($1)`, username)
}

// UserByField returns the user matching the given column name and value.
func (s *Storage) UserByField(field, value string) (*model.User, error) {
	return s.fetchUser(`SELECT `+userColumns+` FROM users WHERE `+quoteIdent(field)+`=$1`, value)
}

// AnotherUserWithFieldExists returns true if a user other than userID has the given value in the given column.
func (s *Storage) AnotherUserWithFieldExists(userID int64, field, value string) bool {
	var result bool
	s.db.QueryRow(`SELECT true FROM users WHERE id <> $1 AND `+quoteIdent(field)+`=$2 LIMIT 1`, userID, value).Scan(&result)
	return result
}

// UserByAPIKey returns the user associated with the given API key.
func (s *Storage) UserByAPIKey(token string) (*model.User, error) {
	return s.fetchUser(`SELECT `+userColumns+` FROM users WHERE id=(SELECT user_id FROM api_keys WHERE token=$1)`, token)
}

func (s *Storage) fetchUser(query string, args ...any) (*model.User, error) {
	var user model.User
	err := s.db.QueryRow(query, args...).Scan(userDest(&user)...)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return nil, nil
	case err != nil:
		return nil, fmt.Errorf("storage: unable to fetch user: %w", err)
	}
	return &user, nil
}

// RemoveUser deletes a user and all related data.
func (s *Storage) RemoveUser(userID int64) error {
	if _, err := s.db.Exec(`DELETE FROM users WHERE id=$1`, userID); err != nil {
		return fmt.Errorf("storage: unable to remove user #%d: %w", userID, err)
	}
	return nil
}

// Users returns all users.
func (s *Storage) Users() (model.Users, error) {
	rows, err := s.db.Query(`SELECT ` + userColumns + ` FROM users ORDER BY username ASC`)
	if err != nil {
		return nil, fmt.Errorf("storage: unable to fetch users: %w", err)
	}
	defer rows.Close()

	var users model.Users
	for rows.Next() {
		var user model.User
		if err := rows.Scan(userDest(&user)...); err != nil {
			return nil, fmt.Errorf("storage: unable to fetch users row: %w", err)
		}
		users = append(users, &user)
	}
	return users, rows.Err()
}

// CheckPassword returns nil if the given password matches the user's stored hash.
func (s *Storage) CheckPassword(username, password string) error {
	var hash string
	username = strings.ToLower(username)

	err := s.db.QueryRow(`SELECT password FROM users WHERE username=$1`, username).Scan(&hash)
	if errors.Is(err, sql.ErrNoRows) {
		// Dummy comparison so response timing doesn't reveal whether the user exists.
		_ = bcrypt.CompareHashAndPassword(dummyBcryptHash, []byte(password))
		return fmt.Errorf("storage: unable to find this user: %s", username)
	} else if err != nil {
		return fmt.Errorf("storage: unable to fetch user: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return fmt.Errorf("storage: invalid password for %q: %w", username, err)
	}
	return nil
}

// HasPassword returns true if the given user exists and has a non-empty password.
func (s *Storage) HasPassword(userID int64) (bool, error) {
	var result bool
	err := s.db.QueryRow(`SELECT true FROM users WHERE id=$1 AND password <> '' LIMIT 1`, userID).Scan(&result)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf("storage: unable to execute query: %w", err)
	}
	return result, nil
}
