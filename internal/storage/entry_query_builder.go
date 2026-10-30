// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"sss/internal/model"
	"sss/internal/timezone"
)

// EntryQueryBuilder builds a SQL query to fetch entries.
type EntryQueryBuilder struct {
	store           *Storage
	args            []any
	conditions      []string
	sortExpressions []string
	limit           int
	offset          int
	search          bool
	fetchEnclosures bool
	excludeContent  bool
}

// NewEntryQueryBuilder returns a new EntryQueryBuilder.
func (s *Storage) NewEntryQueryBuilder(userID int64) *EntryQueryBuilder {
	return &EntryQueryBuilder{
		store:      s,
		args:       []any{userID},
		conditions: []string{"e.user_id = $1"},
	}
}

// NewAnonymousQueryBuilder returns a new EntryQueryBuilder suitable for anonymous users.
func (s *Storage) NewAnonymousQueryBuilder() *EntryQueryBuilder {
	return &EntryQueryBuilder{store: s}
}

// nextArg appends value to the arguments and returns its placeholder.
func (e *EntryQueryBuilder) nextArg(value any) string {
	e.args = append(e.args, value)
	return "$" + strconv.Itoa(len(e.args))
}

// WithEnclosures fetches enclosures for each entry.
func (e *EntryQueryBuilder) WithEnclosures() *EntryQueryBuilder {
	e.fetchEnclosures = true
	return e
}

// WithoutContent excludes the content column from the query results,
// replacing it with an empty string.
func (e *EntryQueryBuilder) WithoutContent() *EntryQueryBuilder {
	e.excludeContent = true
	return e
}

// WithSearchQuery adds a full-text search condition (web-search syntax:
// quoted phrases, "or", -exclusion) and sorts by relevance, then recency.
func (e *EntryQueryBuilder) WithSearchQuery(query string) *EntryQueryBuilder {
	if match := ftsQuery(query); match != "" {
		e.search = true
		e.conditions = append(e.conditions, "entries_fts MATCH "+e.nextArg(match))
		// bm25() is lower for better matches; recency adds 0.0000001 per second of age.
		e.sortExpressions = append(e.sortExpressions, "bm25(entries_fts, 2.0, 1.0) + (unixepoch() - e.published_at) * 0.0000001 ASC")
	}
	return e
}

// WithStarred adds starred filter.
func (e *EntryQueryBuilder) WithStarred(starred bool) *EntryQueryBuilder {
	if starred {
		e.conditions = append(e.conditions, "e.starred = 1")
	} else {
		e.conditions = append(e.conditions, "e.starred = 0")
	}
	return e
}

// BeforeChangedDate adds a condition < changed_at
func (e *EntryQueryBuilder) BeforeChangedDate(date time.Time) *EntryQueryBuilder {
	e.conditions = append(e.conditions, "e.changed_at < "+e.nextArg(date))
	return e
}

// AfterChangedDate adds a condition > changed_at
func (e *EntryQueryBuilder) AfterChangedDate(date time.Time) *EntryQueryBuilder {
	e.conditions = append(e.conditions, "e.changed_at > "+e.nextArg(date))
	return e
}

// BeforePublishedDate adds a condition < published_at
func (e *EntryQueryBuilder) BeforePublishedDate(date time.Time) *EntryQueryBuilder {
	e.conditions = append(e.conditions, "e.published_at < "+e.nextArg(date))
	return e
}

// AfterPublishedDate adds a condition > published_at
func (e *EntryQueryBuilder) AfterPublishedDate(date time.Time) *EntryQueryBuilder {
	e.conditions = append(e.conditions, "e.published_at > "+e.nextArg(date))
	return e
}

// BeforeEntryID adds a condition < entryID.
func (e *EntryQueryBuilder) BeforeEntryID(entryID int64) *EntryQueryBuilder {
	if entryID != 0 {
		e.conditions = append(e.conditions, "e.id < "+e.nextArg(entryID))
	}
	return e
}

// AfterEntryID adds a condition > entryID.
func (e *EntryQueryBuilder) AfterEntryID(entryID int64) *EntryQueryBuilder {
	if entryID != 0 {
		e.conditions = append(e.conditions, "e.id > "+e.nextArg(entryID))
	}
	return e
}

// WithEntryIDs filter by entry IDs.
func (e *EntryQueryBuilder) WithEntryIDs(entryIDs ...int64) *EntryQueryBuilder {
	switch {
	case len(entryIDs) == 1:
		e.conditions = append(e.conditions, "e.id = "+e.nextArg(entryIDs[0]))
	case len(entryIDs) > 1:
		e.conditions = append(e.conditions, "e.id IN (SELECT value FROM json_each("+e.nextArg(jsonArray(entryIDs))+"))")
	}
	return e
}

// WithFeedID filter by feed ID.
func (e *EntryQueryBuilder) WithFeedID(feedID int64) *EntryQueryBuilder {
	if feedID > 0 {
		e.conditions = append(e.conditions, "e.feed_id = "+e.nextArg(feedID))
	}
	return e
}

// WithCategoryID filter by category ID.
func (e *EntryQueryBuilder) WithCategoryID(categoryID int64) *EntryQueryBuilder {
	if categoryID > 0 {
		e.conditions = append(e.conditions, "f.category_id = "+e.nextArg(categoryID))
	}
	return e
}

// WithStatuses filter by a list of entry statuses.
func (e *EntryQueryBuilder) WithStatuses(statuses ...string) *EntryQueryBuilder {
	switch {
	case len(statuses) == 1:
		e.conditions = append(e.conditions, "e.status = "+e.nextArg(statuses[0]))
	case len(statuses) > 1:
		e.conditions = append(e.conditions, "e.status IN (SELECT value FROM json_each("+e.nextArg(jsonArray(statuses))+"))")
	}
	return e
}

// WithTags filters entries having all the given tags (case-insensitive).
func (e *EntryQueryBuilder) WithTags(tags ...string) *EntryQueryBuilder {
	for _, tag := range tags {
		e.conditions = append(e.conditions, tagCondition(e.nextArg(tag)))
	}
	return e
}

func tagCondition(placeholder string) string {
	return "EXISTS (SELECT 1 FROM json_each(e.tags) WHERE lower(json_each.value) = lower(" + placeholder + "))"
}

// WithoutStatus set the entry status that should not be returned.
func (e *EntryQueryBuilder) WithoutStatus(status string) *EntryQueryBuilder {
	if status != "" {
		e.conditions = append(e.conditions, "e.status <> "+e.nextArg(status))
	}
	return e
}

// WithShareCode set the entry share code.
func (e *EntryQueryBuilder) WithShareCode(shareCode string) *EntryQueryBuilder {
	e.conditions = append(e.conditions, "e.share_code = "+e.nextArg(shareCode))
	return e
}

// WithShareCodeNotEmpty adds a filter for non-empty share code.
func (e *EntryQueryBuilder) WithShareCodeNotEmpty() *EntryQueryBuilder {
	e.conditions = append(e.conditions, "e.share_code <> ''")
	return e
}

// WithSorting add a sort expression. column is an entries column, "category_title" or "category_id".
func (e *EntryQueryBuilder) WithSorting(column, direction string) *EntryQueryBuilder {
	if dir := sortDirection(direction); dir != "" {
		e.sortExpressions = append(e.sortExpressions, entrySortColumn(column)+" "+dir)
	}
	return e
}

func entrySortColumn(column string) string {
	switch column {
	case "category_title":
		return "c.title"
	case "category_id":
		return "f.category_id"
	}
	return "e." + quoteIdent(column)
}

// WithLimit sets the limit. A non-positive limit is clamped to
// model.MaxEntryLimit so callers cannot request an unbounded result set.
func (e *EntryQueryBuilder) WithLimit(limit int) *EntryQueryBuilder {
	return e.WithLimitAndMaximum(limit, model.MaxEntryLimit)
}

// WithLimitAndMaximum sets the limit, capped at the given maximum.
// A non-positive limit is clamped to the maximum.
func (e *EntryQueryBuilder) WithLimitAndMaximum(limit, maximum int) *EntryQueryBuilder {
	if limit <= 0 || limit > maximum {
		limit = maximum
	}
	e.limit = limit
	return e
}

// WithOffset set the offset.
func (e *EntryQueryBuilder) WithOffset(offset int) *EntryQueryBuilder {
	if offset > 0 {
		e.offset = offset
	}
	return e
}

// WithGloballyVisible filters out entries of feeds or categories hidden globally.
func (e *EntryQueryBuilder) WithGloballyVisible() *EntryQueryBuilder {
	e.conditions = append(e.conditions, "c.hide_globally = 0", "f.hide_globally = 0")
	return e
}

// CountEntries count the number of entries that match the condition.
func (e *EntryQueryBuilder) CountEntries() (count int, err error) {
	query := `SELECT count(*) ` + e.buildFrom() + ` WHERE ` + e.buildCondition()
	if err := e.store.db.QueryRow(query, e.args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("storage: unable to count entries: %w", err)
	}
	return count, nil
}

// GetEntry returns a single entry that match the condition.
func (e *EntryQueryBuilder) GetEntry() (*model.Entry, error) {
	e.limit = 1
	entries, err := e.GetEntries()
	if err != nil {
		return nil, err
	}
	if len(entries) != 1 {
		return nil, nil
	}

	entries[0].Enclosures, err = e.store.EnclosuresByEntryID(entries[0].ID)
	if err != nil {
		return nil, err
	}
	return entries[0], nil
}

// GetEntries returns a list of entries that match the condition.
func (e *EntryQueryBuilder) GetEntries() (model.Entries, error) {
	entries, _, err := e.fetchEntries(false)
	return entries, err
}

// GetEntriesWithCount returns a list of entries and the total count of matching
// rows, ignoring limit and offset.
func (e *EntryQueryBuilder) GetEntriesWithCount() (model.Entries, int, error) {
	// FTS5 bm25() (search ranking) cannot be combined with a window function.
	withWindowCount := !e.search
	entries, total, err := e.fetchEntries(withWindowCount)
	if err != nil {
		return nil, 0, err
	}

	// The window count is unavailable when the offset is past the last row.
	if !withWindowCount || (len(entries) == 0 && e.offset > 0) {
		if total, err = e.CountEntries(); err != nil {
			return nil, 0, err
		}
	}
	return entries, total, nil
}

func (e *EntryQueryBuilder) fetchEntries(withCount bool) (model.Entries, int, error) {
	countColumn := ""
	if withCount {
		countColumn = "count(*) OVER(),"
	}
	contentColumn := "e.content"
	if e.excludeContent {
		contentColumn = "''"
	}

	query := `
		SELECT
			` + countColumn + `
			e.id,
			e.user_id,
			e.feed_id,
			e.hash,
			e.published_at,
			e.title,
			e.url,
			e.comments_url,
			e.author,
			e.share_code,
			` + contentColumn + `,
			e.status,
			e.starred,
			e.reading_time,
			e.created_at,
			e.changed_at,
			e.tags,
			e.language,
			f.title,
			f.feed_url,
			f.site_url,
			f.description,
			f.language,
			f.checked_at,
			f.category_id,
			c.title,
			c.hide_globally,
			f.scraper_rules,
			f.rewrite_rules,
			f.crawler,
			f.user_agent,
			f.cookie,
			f.hide_globally,
			f.no_media_player,
			f.webhook_url,
			fi.icon_id,
			i.external_id,
			u.timezone
		` + e.buildFrom() + `
		LEFT JOIN feed_icons fi ON fi.feed_id=f.id
		LEFT JOIN icons i ON i.id=fi.icon_id
		INNER JOIN users u ON u.id=e.user_id
		WHERE ` + e.buildCondition() + e.buildSorting()

	rows, err := e.store.db.Query(query, e.args...)
	if err != nil {
		return nil, 0, fmt.Errorf("storage: unable to get entries: %w", err)
	}
	defer rows.Close()

	entries := make(model.Entries, 0, max(e.limit, 0))
	entryMap := make(map[int64]*model.Entry, max(e.limit, 0))
	var totalCount int

	for rows.Next() {
		var iconID sql.NullInt64
		var externalIconID sql.NullString
		var tz string

		entry := model.NewEntry()
		dest := []any{
			&entry.ID,
			&entry.UserID,
			&entry.FeedID,
			&entry.Hash,
			unixTime{&entry.Date},
			&entry.Title,
			&entry.URL,
			&entry.CommentsURL,
			&entry.Author,
			&entry.ShareCode,
			&entry.Content,
			&entry.Status,
			&entry.Starred,
			&entry.ReadingTime,
			unixTime{&entry.CreatedAt},
			unixTime{&entry.ChangedAt},
			jsonStrings{&entry.Tags},
			&entry.Language,
			&entry.Feed.Title,
			&entry.Feed.FeedURL,
			&entry.Feed.SiteURL,
			&entry.Feed.Description,
			&entry.Feed.Language,
			unixTime{&entry.Feed.CheckedAt},
			&entry.Feed.Category.ID,
			&entry.Feed.Category.Title,
			&entry.Feed.Category.HideGlobally,
			&entry.Feed.ScraperRules,
			&entry.Feed.RewriteRules,
			&entry.Feed.Crawler,
			&entry.Feed.UserAgent,
			&entry.Feed.Cookie,
			&entry.Feed.HideGlobally,
			&entry.Feed.NoMediaPlayer,
			&entry.Feed.WebhookURL,
			&iconID,
			&externalIconID,
			&tz,
		}
		if withCount {
			dest = append([]any{&totalCount}, dest...)
		}

		if err := rows.Scan(dest...); err != nil {
			return nil, 0, fmt.Errorf("storage: unable to fetch entry row: %w", err)
		}

		if iconID.Valid && externalIconID.String != "" {
			entry.Feed.Icon.IconID = iconID.Int64
			entry.Feed.Icon.ExternalIconID = externalIconID.String
		}

		// Timestamps carry the user timezone (API output).
		entry.Date = timezone.Convert(tz, entry.Date)
		entry.CreatedAt = timezone.Convert(tz, entry.CreatedAt)
		entry.ChangedAt = timezone.Convert(tz, entry.ChangedAt)
		entry.Feed.CheckedAt = timezone.Convert(tz, entry.Feed.CheckedAt)

		entry.Feed.ID = entry.FeedID
		entry.Feed.UserID = entry.UserID
		entry.Feed.Icon.FeedID = entry.FeedID
		entry.Feed.Category.UserID = entry.UserID

		entries = append(entries, entry)
		entryMap[entry.ID] = entry
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("storage: unable to get entries: %w", err)
	}

	if e.fetchEnclosures && len(entries) > 0 {
		entryIDs := make([]int64, 0, len(entries))
		for _, entry := range entries {
			entryIDs = append(entryIDs, entry.ID)
		}
		enclosures, err := e.store.EnclosuresByEntryIDs(entryIDs)
		if err != nil {
			return nil, 0, err
		}
		for entryID, list := range enclosures {
			entryMap[entryID].Enclosures = list
		}
	}

	return entries, totalCount, nil
}

// GetEntryIDs returns a list of entry IDs that match the condition.
func (e *EntryQueryBuilder) GetEntryIDs() ([]int64, error) {
	query := `SELECT e.id ` + e.buildFrom() + ` WHERE ` + e.buildCondition() + e.buildSorting()
	rows, err := e.store.db.Query(query, e.args...)
	if err != nil {
		return nil, fmt.Errorf("storage: unable to get entries: %w", err)
	}
	defer rows.Close()

	var entryIDs []int64
	for rows.Next() {
		var entryID int64
		if err := rows.Scan(&entryID); err != nil {
			return nil, fmt.Errorf("storage: unable to fetch entry row: %w", err)
		}
		entryIDs = append(entryIDs, entryID)
	}
	return entryIDs, rows.Err()
}

// GetEntryIDsWithCount returns a list of entry IDs and the total count of
// matching rows (ignoring limit/offset).
func (e *EntryQueryBuilder) GetEntryIDsWithCount() ([]int64, int, error) {
	total, err := e.CountEntries()
	if err != nil {
		return nil, 0, err
	}
	entryIDs, err := e.GetEntryIDs()
	if err != nil {
		return nil, 0, err
	}
	return entryIDs, total, nil
}

func (e *EntryQueryBuilder) buildFrom() string {
	from := `FROM entries e
		INNER JOIN feeds f ON f.id=e.feed_id
		INNER JOIN categories c ON c.id=f.category_id`
	if e.search {
		from += ` INNER JOIN entries_fts ON entries_fts.rowid=e.id`
	}
	return from
}

func (e *EntryQueryBuilder) buildCondition() string {
	if len(e.conditions) == 0 {
		return "1"
	}
	return strings.Join(e.conditions, " AND ")
}

func (e *EntryQueryBuilder) buildSorting() string {
	var parts string
	if len(e.sortExpressions) > 0 {
		parts += " ORDER BY " + strings.Join(e.sortExpressions, ", ")
	}
	if e.limit > 0 {
		parts += " LIMIT " + strconv.Itoa(e.limit)
	}
	if e.offset > 0 {
		if e.limit <= 0 {
			parts += " LIMIT -1"
		}
		parts += " OFFSET " + strconv.Itoa(e.offset)
	}
	return parts
}

// ftsQuery converts a web-search style query (like PostgreSQL's
// websearch_to_tsquery) into a safe FTS5 MATCH expression: every word or
// "quoted phrase" becomes an FTS5 string, "or" between terms becomes OR and a
// leading "-" excludes the term. It returns "" when nothing is searchable.
func ftsQuery(query string) string {
	var parts []string
	or := false
	for query != "" {
		query = strings.TrimLeft(query, " \t\r\n")
		if query == "" {
			break
		}

		negate := false
		if query[0] == '-' && len(query) > 1 {
			negate = true
			query = query[1:]
		}

		var term string
		if query[0] == '"' {
			end := strings.IndexByte(query[1:], '"')
			if end < 0 {
				term, query = query[1:], ""
			} else {
				term, query = query[1:end+1], query[end+2:]
			}
		} else {
			end := strings.IndexAny(query, " \t\r\n")
			if end < 0 {
				end = len(query)
			}
			term, query = query[:end], query[end:]
			if !negate && strings.EqualFold(term, "or") {
				or = len(parts) > 0
				continue
			}
		}

		if strings.TrimSpace(term) == "" {
			continue
		}
		quoted := `"` + strings.ReplaceAll(term, `"`, `""`) + `"`
		switch {
		case negate && len(parts) == 0:
			// FTS5 NOT is binary; a leading exclusion cannot be expressed.
		case negate:
			parts = append(parts, "NOT "+quoted)
		case or:
			parts = append(parts, "OR "+quoted)
		default:
			parts = append(parts, quoted)
		}
		or = false
	}
	return strings.Join(parts, " ")
}
