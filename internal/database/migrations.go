// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

package database

// migrations is the ordered schema history; PRAGMA user_version stores how
// many have been applied. Append new migrations, never edit applied ones.
//
// Conventions: timestamps are INTEGER unix seconds UTC, booleans are 0/1,
// tags are a JSON array of strings.
var migrations = []string{
	`
CREATE TABLE users (
	id INTEGER PRIMARY KEY,
	username TEXT NOT NULL UNIQUE,
	password TEXT NOT NULL DEFAULT '',
	is_admin INTEGER NOT NULL DEFAULT 0,
	timezone TEXT NOT NULL DEFAULT 'UTC',
	theme TEXT NOT NULL DEFAULT 'dark_serif',
	last_login_at INTEGER,
	entry_direction TEXT NOT NULL DEFAULT 'asc' CHECK (entry_direction IN ('asc', 'desc')),
	entry_order TEXT NOT NULL DEFAULT 'published_at' CHECK (entry_order IN ('published_at', 'created_at')),
	entries_per_page INTEGER NOT NULL DEFAULT 100,
	keyboard_shortcuts INTEGER NOT NULL DEFAULT 0,
	show_reading_time INTEGER NOT NULL DEFAULT 1,
	entry_swipe INTEGER NOT NULL DEFAULT 0,
	gesture_nav TEXT NOT NULL DEFAULT 'none',
	stylesheet TEXT NOT NULL DEFAULT '',
	custom_js TEXT NOT NULL DEFAULT '',
	external_font_hosts TEXT NOT NULL DEFAULT '',
	google_id TEXT NOT NULL DEFAULT '',
	openid_connect_id TEXT NOT NULL DEFAULT '',
	display_mode TEXT NOT NULL DEFAULT 'standalone' CHECK (display_mode IN ('fullscreen', 'standalone', 'minimal-ui', 'browser')),
	default_reading_speed INTEGER NOT NULL DEFAULT 265,
	cjk_reading_speed INTEGER NOT NULL DEFAULT 500,
	default_home_page TEXT NOT NULL DEFAULT 'unread',
	categories_sorting_order TEXT NOT NULL DEFAULT 'unread_count',
	mark_read_on_view INTEGER NOT NULL DEFAULT 1,
	block_filter_entry_rules TEXT NOT NULL DEFAULT '',
	keep_filter_entry_rules TEXT NOT NULL DEFAULT '',
	always_open_external_links INTEGER NOT NULL DEFAULT 0,
	open_external_links_in_new_tab INTEGER NOT NULL DEFAULT 1,
	mark_read_on_scroll INTEGER NOT NULL DEFAULT 0,
	service_worker INTEGER NOT NULL DEFAULT 0,
	theme_light_hour INTEGER NOT NULL DEFAULT 7,
	theme_dark_hour INTEGER NOT NULL DEFAULT 19
);
CREATE UNIQUE INDEX users_google_id_idx ON users(google_id) WHERE google_id <> '';
CREATE UNIQUE INDEX users_openid_connect_id_idx ON users(openid_connect_id) WHERE openid_connect_id <> '';

CREATE TABLE web_sessions (
	id TEXT PRIMARY KEY,
	secret_hash BLOB NOT NULL,
	user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
	created_at INTEGER NOT NULL DEFAULT (unixepoch()),
	user_agent TEXT NOT NULL DEFAULT '',
	ip TEXT,
	state TEXT NOT NULL DEFAULT '{}' CHECK (json_type(state) = 'object')
);
CREATE INDEX web_sessions_user_id_idx ON web_sessions(user_id) WHERE user_id IS NOT NULL;
CREATE INDEX web_sessions_created_at_idx ON web_sessions(created_at);

CREATE TABLE api_keys (
	id INTEGER PRIMARY KEY,
	user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	token TEXT NOT NULL UNIQUE,
	description TEXT NOT NULL,
	last_used_at INTEGER,
	created_at INTEGER NOT NULL DEFAULT (unixepoch()),
	UNIQUE (user_id, description)
);

CREATE TABLE categories (
	id INTEGER PRIMARY KEY,
	user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	title TEXT NOT NULL,
	hide_globally INTEGER NOT NULL DEFAULT 0,
	UNIQUE (user_id, title)
);

CREATE TABLE feeds (
	id INTEGER PRIMARY KEY,
	user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	category_id INTEGER NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
	title TEXT NOT NULL,
	feed_url TEXT NOT NULL,
	site_url TEXT NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	language TEXT NOT NULL DEFAULT '',
	checked_at INTEGER NOT NULL DEFAULT (unixepoch()),
	next_check_at INTEGER NOT NULL DEFAULT (unixepoch()),
	etag_header TEXT NOT NULL DEFAULT '',
	last_modified_header TEXT NOT NULL DEFAULT '',
	parsing_error_msg TEXT NOT NULL DEFAULT '',
	parsing_error_count INTEGER NOT NULL DEFAULT 0,
	scraper_rules TEXT NOT NULL DEFAULT '',
	rewrite_rules TEXT NOT NULL DEFAULT '',
	url_rewrite_rules TEXT NOT NULL DEFAULT '',
	blocklist_rules TEXT NOT NULL DEFAULT '',
	keeplist_rules TEXT NOT NULL DEFAULT '',
	block_filter_entry_rules TEXT NOT NULL DEFAULT '',
	keep_filter_entry_rules TEXT NOT NULL DEFAULT '',
	crawler INTEGER NOT NULL DEFAULT 0,
	user_agent TEXT NOT NULL DEFAULT '',
	cookie TEXT NOT NULL DEFAULT '',
	username TEXT NOT NULL DEFAULT '',
	password TEXT NOT NULL DEFAULT '',
	disabled INTEGER NOT NULL DEFAULT 0,
	ignore_http_cache INTEGER NOT NULL DEFAULT 0,
	allow_self_signed_certificates INTEGER NOT NULL DEFAULT 0,
	fetch_via_proxy INTEGER NOT NULL DEFAULT 0,
	hide_globally INTEGER NOT NULL DEFAULT 0,
	no_media_player INTEGER NOT NULL DEFAULT 0,
	disable_http2 INTEGER NOT NULL DEFAULT 0,
	ignore_entry_updates INTEGER NOT NULL DEFAULT 0,
	apprise_service_urls TEXT NOT NULL DEFAULT '',
	webhook_url TEXT NOT NULL DEFAULT '',
	ntfy_enabled INTEGER NOT NULL DEFAULT 0,
	ntfy_priority INTEGER NOT NULL DEFAULT 3,
	ntfy_topic TEXT NOT NULL DEFAULT '',
	pushover_enabled INTEGER NOT NULL DEFAULT 0,
	pushover_priority INTEGER NOT NULL DEFAULT 0,
	proxy_url TEXT NOT NULL DEFAULT '',
	UNIQUE (user_id, feed_url)
);
CREATE INDEX feeds_user_category_idx ON feeds(user_id, category_id);
CREATE INDEX feeds_category_idx ON feeds(category_id);
CREATE INDEX feeds_next_check_at_idx ON feeds(next_check_at);

CREATE TABLE entries (
	id INTEGER PRIMARY KEY,
	user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	feed_id INTEGER NOT NULL REFERENCES feeds(id) ON DELETE CASCADE,
	hash TEXT NOT NULL,
	published_at INTEGER NOT NULL,
	created_at INTEGER NOT NULL DEFAULT (unixepoch()),
	changed_at INTEGER NOT NULL DEFAULT (unixepoch()),
	title TEXT NOT NULL,
	url TEXT NOT NULL,
	comments_url TEXT NOT NULL DEFAULT '',
	author TEXT NOT NULL DEFAULT '',
	content TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'unread' CHECK (status IN ('unread', 'read')),
	starred INTEGER NOT NULL DEFAULT 0,
	share_code TEXT NOT NULL DEFAULT '',
	reading_time INTEGER NOT NULL DEFAULT 0,
	tags TEXT NOT NULL DEFAULT '[]',
	language TEXT NOT NULL DEFAULT '',
	UNIQUE (feed_id, hash)
);
CREATE UNIQUE INDEX entries_share_code_idx ON entries(share_code) WHERE share_code <> '';
CREATE INDEX entries_user_feed_idx ON entries(user_id, feed_id);
CREATE INDEX entries_feed_id_status_hash_idx ON entries(feed_id, status, hash);
CREATE INDEX entries_user_id_status_starred_idx ON entries(user_id, status, starred);
CREATE INDEX entries_user_status_feed_idx ON entries(user_id, status, feed_id);
CREATE INDEX entries_user_status_published_idx ON entries(user_id, status, published_at);
CREATE INDEX entries_user_status_created_idx ON entries(user_id, status, created_at);
CREATE INDEX entries_user_status_changed_published_idx ON entries(user_id, status, changed_at, published_at);
CREATE INDEX entries_status_created_idx ON entries(status, created_at);

CREATE VIRTUAL TABLE entries_fts USING fts5(
	title, content,
	content='entries', content_rowid='id',
	tokenize='porter unicode61 remove_diacritics 2'
);
CREATE TRIGGER entries_fts_insert AFTER INSERT ON entries BEGIN
	INSERT INTO entries_fts(rowid, title, content) VALUES (new.id, new.title, new.content);
END;
CREATE TRIGGER entries_fts_delete AFTER DELETE ON entries BEGIN
	INSERT INTO entries_fts(entries_fts, rowid, title, content) VALUES ('delete', old.id, old.title, old.content);
END;
CREATE TRIGGER entries_fts_update AFTER UPDATE OF title, content ON entries BEGIN
	INSERT INTO entries_fts(entries_fts, rowid, title, content) VALUES ('delete', old.id, old.title, old.content);
	INSERT INTO entries_fts(rowid, title, content) VALUES (new.id, new.title, new.content);
END;

CREATE TABLE entry_tombstones (
	feed_id INTEGER NOT NULL REFERENCES feeds(id) ON DELETE CASCADE,
	hash TEXT NOT NULL CHECK (hash <> ''),
	deleted_at INTEGER NOT NULL DEFAULT (unixepoch()),
	PRIMARY KEY (feed_id, hash)
) WITHOUT ROWID;

CREATE TABLE enclosures (
	id INTEGER PRIMARY KEY,
	user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	entry_id INTEGER NOT NULL REFERENCES entries(id) ON DELETE CASCADE,
	url TEXT NOT NULL,
	size INTEGER NOT NULL DEFAULT 0,
	mime_type TEXT NOT NULL DEFAULT '',
	media_progression INTEGER NOT NULL DEFAULT 0,
	UNIQUE (entry_id, url)
);
CREATE INDEX enclosures_user_idx ON enclosures(user_id);

CREATE TABLE icons (
	id INTEGER PRIMARY KEY,
	hash TEXT NOT NULL UNIQUE,
	mime_type TEXT NOT NULL,
	content BLOB NOT NULL,
	external_id TEXT NOT NULL DEFAULT ''
);
CREATE UNIQUE INDEX icons_external_id_idx ON icons(external_id) WHERE external_id <> '';

CREATE TABLE feed_icons (
	feed_id INTEGER NOT NULL REFERENCES feeds(id) ON DELETE CASCADE,
	icon_id INTEGER NOT NULL REFERENCES icons(id) ON DELETE CASCADE,
	PRIMARY KEY (feed_id, icon_id)
) WITHOUT ROWID;
CREATE INDEX feed_icons_icon_idx ON feed_icons(icon_id);

CREATE TABLE integrations (
	user_id INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
	instapaper_enabled INTEGER NOT NULL DEFAULT 0,
	instapaper_username TEXT NOT NULL DEFAULT '',
	instapaper_password TEXT NOT NULL DEFAULT '',
	fever_enabled INTEGER NOT NULL DEFAULT 0,
	fever_username TEXT NOT NULL DEFAULT '',
	fever_token TEXT NOT NULL DEFAULT '',
	googlereader_enabled INTEGER NOT NULL DEFAULT 0,
	googlereader_username TEXT NOT NULL DEFAULT '',
	googlereader_password TEXT NOT NULL DEFAULT '',
	notion_enabled INTEGER NOT NULL DEFAULT 0,
	notion_token TEXT NOT NULL DEFAULT '',
	notion_page_id TEXT NOT NULL DEFAULT '',
	telegram_bot_enabled INTEGER NOT NULL DEFAULT 0,
	telegram_bot_token TEXT NOT NULL DEFAULT '',
	telegram_bot_chat_id TEXT NOT NULL DEFAULT '',
	telegram_bot_topic_id INTEGER,
	telegram_bot_disable_web_page_preview INTEGER NOT NULL DEFAULT 0,
	telegram_bot_disable_notification INTEGER NOT NULL DEFAULT 0,
	telegram_bot_disable_buttons INTEGER NOT NULL DEFAULT 0,
	linkwarden_enabled INTEGER NOT NULL DEFAULT 0,
	linkwarden_url TEXT NOT NULL DEFAULT '',
	linkwarden_api_key TEXT NOT NULL DEFAULT '',
	linkwarden_collection_id INTEGER,
	matrix_bot_enabled INTEGER NOT NULL DEFAULT 0,
	matrix_bot_user TEXT NOT NULL DEFAULT '',
	matrix_bot_password TEXT NOT NULL DEFAULT '',
	matrix_bot_url TEXT NOT NULL DEFAULT '',
	matrix_bot_chat_id TEXT NOT NULL DEFAULT '',
	apprise_enabled INTEGER NOT NULL DEFAULT 0,
	apprise_url TEXT NOT NULL DEFAULT '',
	apprise_services_url TEXT NOT NULL DEFAULT '',
	webhook_enabled INTEGER NOT NULL DEFAULT 0,
	webhook_url TEXT NOT NULL DEFAULT '',
	webhook_secret TEXT NOT NULL DEFAULT '',
	rssbridge_enabled INTEGER NOT NULL DEFAULT 0,
	rssbridge_url TEXT NOT NULL DEFAULT '',
	rssbridge_token TEXT NOT NULL DEFAULT '',
	ntfy_enabled INTEGER NOT NULL DEFAULT 0,
	ntfy_topic TEXT NOT NULL DEFAULT '',
	ntfy_url TEXT NOT NULL DEFAULT '',
	ntfy_api_token TEXT NOT NULL DEFAULT '',
	ntfy_username TEXT NOT NULL DEFAULT '',
	ntfy_password TEXT NOT NULL DEFAULT '',
	ntfy_icon_url TEXT NOT NULL DEFAULT '',
	ntfy_internal_links INTEGER NOT NULL DEFAULT 0,
	discord_enabled INTEGER NOT NULL DEFAULT 0,
	discord_webhook_link TEXT NOT NULL DEFAULT '',
	slack_enabled INTEGER NOT NULL DEFAULT 0,
	slack_webhook_link TEXT NOT NULL DEFAULT '',
	pushover_enabled INTEGER NOT NULL DEFAULT 0,
	pushover_user TEXT NOT NULL DEFAULT '',
	pushover_token TEXT NOT NULL DEFAULT '',
	pushover_device TEXT NOT NULL DEFAULT '',
	pushover_prefix TEXT NOT NULL DEFAULT '',
	gotify_enabled INTEGER NOT NULL DEFAULT 0,
	gotify_url TEXT NOT NULL DEFAULT '',
	gotify_token TEXT NOT NULL DEFAULT '',
	gotify_priority INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE webauthn_credentials (
	handle BLOB PRIMARY KEY,
	cred_id BLOB NOT NULL UNIQUE,
	user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	public_key BLOB NOT NULL,
	attestation_type TEXT NOT NULL,
	aaguid BLOB,
	sign_count INTEGER,
	clone_warning INTEGER,
	name TEXT NOT NULL DEFAULT '',
	added_on INTEGER DEFAULT (unixepoch()),
	last_seen_on INTEGER DEFAULT (unixepoch()),
	backup_eligible INTEGER,
	backup_state INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX webauthn_credentials_user_idx ON webauthn_credentials(user_id);

CREATE TABLE acme_cache (
	key TEXT PRIMARY KEY,
	data BLOB NOT NULL,
	updated_at INTEGER NOT NULL
);
`,
}
