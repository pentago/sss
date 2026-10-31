package storage

import (
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"

	"sss/internal/config"
	"sss/internal/crypto"
	"sss/internal/database"
	"sss/internal/model"
)

func newTestStorage(t *testing.T) *Storage {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	return New(db)
}

func mustCreateUser(t *testing.T, s *Storage, username string) *model.User {
	t.Helper()
	user, err := s.CreateUser(&model.UserCreationRequest{Username: username, Password: "secret123"})
	if err != nil {
		t.Fatal(err)
	}
	return user
}

func mustCreateFeed(t *testing.T, s *Storage, userID, categoryID int64, feedURL string, entries ...*model.Entry) *model.Feed {
	t.Helper()
	feed := &model.Feed{UserID: userID, FeedURL: feedURL, SiteURL: feedURL, Title: feedURL, Category: &model.Category{ID: categoryID}, Entries: entries}
	if err := s.CreateFeed(feed); err != nil {
		t.Fatal(err)
	}
	return feed
}

func newEntry(hash, title, content string, published time.Time, tags ...string) *model.Entry {
	return &model.Entry{Hash: hash, Title: title, Content: content, URL: "https://example.org/" + hash, Date: published, Tags: tags}
}

func entryIDs(t *testing.T, b *EntryQueryBuilder) []int64 {
	t.Helper()
	entries, err := b.GetEntries()
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]int64, 0, len(entries))
	for _, e := range entries {
		ids = append(ids, e.ID)
	}
	return ids
}

func TestUsers(t *testing.T) {
	s := newTestStorage(t)
	user := mustCreateUser(t, s, "Alice")

	if user.Username != "alice" || user.Theme != model.ThemeDefault || user.KeyboardShortcuts || user.EntrySwipe ||
		user.GestureNav != "none" || user.MarkReadOnScroll || user.ServiceWorker || user.ThemeLightHour != 7 ||
		user.ThemeDarkHour != 19 || user.EntriesPerPage != 100 || !user.MarkReadOnView || user.LastLoginAt != nil {
		t.Fatalf("unexpected defaults: %+v", user)
	}
	if !user.IsAdmin {
		t.Fatal("the first user must be an admin so a password-less install can be administered")
	}
	if !s.UserExists("ALICE") || s.AnotherUserExists(user.ID, "alice") {
		t.Fatal("UserExists/AnotherUserExists mismatch")
	}
	if category, err := s.FirstCategory(user.ID); err != nil || category.Title != "All" {
		t.Fatalf("default category: %v %v", category, err)
	}

	if err := s.CheckPassword("ALICE", "secret123"); err != nil {
		t.Fatal(err)
	}
	if s.CheckPassword("alice", "wrong") == nil || s.CheckPassword("bob", "secret123") == nil {
		t.Fatal("CheckPassword accepted invalid credentials")
	}

	user.Theme = "dark_serif"
	user.ThemeDarkHour = 21
	if err := s.UpdateUser(user); err != nil {
		t.Fatal(err)
	}
	if err := s.CheckPassword("alice", "secret123"); err != nil {
		t.Fatal("password changed by an update without password:", err)
	}
	user.Password = "newsecret"
	if err := s.UpdateUser(user); err != nil {
		t.Fatal(err)
	}
	if err := s.CheckPassword("alice", "newsecret"); err != nil {
		t.Fatal(err)
	}

	if err := s.SetLastLogin(user.ID); err != nil {
		t.Fatal(err)
	}
	got, err := s.UserByUsername("Alice")
	if err != nil || got == nil {
		t.Fatal(got, err)
	}
	if got.Theme != "dark_serif" || got.ThemeDarkHour != 21 || got.LastLoginAt == nil || time.Since(*got.LastLoginAt) > time.Minute {
		t.Fatalf("unexpected user after update: %+v", got)
	}
	if ok, err := s.HasPassword(user.ID); err != nil || !ok {
		t.Fatal("HasPassword", ok, err)
	}

	oidc, err := s.CreateUser(&model.UserCreationRequest{Username: "bob", OpenIDConnectID: "sub-1"})
	if err != nil {
		t.Fatal(err)
	}
	if oidc.IsAdmin {
		t.Fatal("only the first user is an admin by default")
	}
	if ok, _ := s.HasPassword(oidc.ID); ok {
		t.Fatal("OIDC user has a password")
	}
	if u, err := s.UserByField("openid_connect_id", "sub-1"); err != nil || u == nil || u.ID != oidc.ID {
		t.Fatal("UserByField", u, err)
	}
	if !s.AnotherUserWithFieldExists(user.ID, "openid_connect_id", "sub-1") {
		t.Fatal("AnotherUserWithFieldExists")
	}

	if users, err := s.Users(); err != nil || len(users) != 2 || users[0].Username != "alice" {
		t.Fatal("Users", users, err)
	}
	if err := s.RemoveUser(oidc.ID); err != nil {
		t.Fatal(err)
	}
	if u, err := s.UserByID(oidc.ID); err != nil || u != nil {
		t.Fatal("removed user still exists", u, err)
	}
}

func TestAPIKeys(t *testing.T) {
	s := newTestStorage(t)
	user := mustCreateUser(t, s, "alice")

	key, err := s.CreateAPIKey(user.ID, "My Key")
	if err != nil {
		t.Fatal(err)
	}
	if len(key.Token) != 64 || key.LastUsedAt != nil || key.CreatedAt.IsZero() {
		t.Fatalf("unexpected key: %+v", key)
	}
	if !s.APIKeyExists(user.ID, "my key") {
		t.Fatal("APIKeyExists should be case-insensitive")
	}
	if u, err := s.UserByAPIKey(key.Token); err != nil || u == nil || u.ID != user.ID {
		t.Fatal("UserByAPIKey", u, err)
	}
	if u, err := s.UserByAPIKey("nope"); err != nil || u != nil {
		t.Fatal("UserByAPIKey with unknown token", u, err)
	}
	if err := s.SetAPIKeyUsedTimestamp(user.ID, key.Token); err != nil {
		t.Fatal(err)
	}
	if keys, err := s.APIKeys(user.ID); err != nil || len(keys) != 1 || keys[0].LastUsedAt == nil {
		t.Fatal("APIKeys", keys, err)
	}
	if err := s.DeleteAPIKey(user.ID, key.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteAPIKey(user.ID, key.ID); err != ErrAPIKeyNotFound {
		t.Fatal("expected ErrAPIKeyNotFound, got", err)
	}
}

func TestFeedEntriesDedupAndRefresh(t *testing.T) {
	s := newTestStorage(t)
	user := mustCreateUser(t, s, "alice")
	category, _ := s.FirstCategory(user.ID)
	now := time.Now().Truncate(time.Second)

	enclosure := &model.Enclosure{URL: " https://example.org/a.mp3 ", MimeType: "audio/mpeg", Size: 10}
	first := newEntry("h1", "First", "alpha", now.Add(-time.Hour), "Go")
	first.Enclosures = model.EnclosureList{enclosure}
	feed := mustCreateFeed(t, s, user.ID, category.ID, "https://example.org/feed",
		first,
		newEntry("h2", "Second", "beta", now),
		newEntry("h2", "Duplicate", "gamma", now),
	)

	count, err := s.NewEntryQueryBuilder(user.ID).WithFeedID(feed.ID).CountEntries()
	if err != nil || count != 2 {
		t.Fatalf("expected 2 entries, got %d (%v)", count, err)
	}

	entry, err := s.NewEntryQueryBuilder(user.ID).WithEntryIDs(first.ID).GetEntry()
	if err != nil || entry == nil {
		t.Fatal(entry, err)
	}
	if entry.Status != model.EntryStatusUnread || !entry.Date.Equal(first.Date) || entry.Feed.Title != feed.Title ||
		entry.Feed.Category.Title != "All" || !slices.Equal(entry.Tags, []string{"Go"}) || len(entry.Enclosures) != 1 ||
		entry.Enclosures[0].URL != "https://example.org/a.mp3" {
		t.Fatalf("unexpected entry: %+v", entry)
	}

	// Mark the first entry read; a refresh must keep its status and changed_at.
	if err := s.SetEntriesStatus(user.ID, []int64{first.ID}, model.EntryStatusRead); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`UPDATE entries SET changed_at = 1000 WHERE id = $1`, first.ID); err != nil {
		t.Fatal(err)
	}

	updated := newEntry("h1", "First updated", "omega", now)
	newEntries, err := s.RefreshFeedEntries(user.ID, feed.ID, model.Entries{updated, newEntry("h3", "Third", "delta", now)}, false)
	if err != nil || len(newEntries) != 1 || newEntries[0].Hash != "h3" {
		t.Fatalf("unexpected new entries %v (%v)", newEntries, err)
	}
	if ids := entryIDs(t, s.NewEntryQueryBuilder(user.ID).WithSearchQuery("omega")); len(ids) != 0 {
		t.Fatal("entry updated although updateExistingEntries is false")
	}

	if _, err := s.RefreshFeedEntries(user.ID, feed.ID, model.Entries{updated}, true); err != nil {
		t.Fatal(err)
	}
	entry, _ = s.NewEntryQueryBuilder(user.ID).WithEntryIDs(first.ID).GetEntry()
	if entry.Title != "First updated" || entry.Status != model.EntryStatusRead || entry.ChangedAt.Unix() != 1000 ||
		!entry.Date.Equal(first.Date) || len(entry.Enclosures) != 0 || len(entry.Tags) != 0 {
		t.Fatalf("unexpected refreshed entry: %+v", entry)
	}
	// Full-text index follows updates.
	if ids := entryIDs(t, s.NewEntryQueryBuilder(user.ID).WithSearchQuery("omega")); !slices.Equal(ids, []int64{first.ID}) {
		t.Fatal("search after update", ids)
	}
	if ids := entryIDs(t, s.NewEntryQueryBuilder(user.ID).WithSearchQuery("alpha")); len(ids) != 0 {
		t.Fatal("stale content still indexed", ids)
	}

	entry.Title, entry.Content = "Rewritten", "epsilon"
	if err := s.UpdateEntryTitleAndContent(entry); err != nil {
		t.Fatal(err)
	}
	if ids := entryIDs(t, s.NewEntryQueryBuilder(user.ID).WithSearchQuery("epsilon")); !slices.Equal(ids, []int64{first.ID}) {
		t.Fatal("search after UpdateEntryTitleAndContent", ids)
	}

	created, err := s.InsertEntryForFeed(user.ID, feed.ID, newEntry("h3", "Again", "", now))
	if err != nil || created {
		t.Fatal("InsertEntryForFeed with existing hash", created, err)
	}
	if s.IsNewEntry(feed.ID, "h3") || !s.IsNewEntry(feed.ID, "h4") {
		t.Fatal("IsNewEntry")
	}

	if err := s.RemoveFeed(user.ID, feed.ID); err != nil {
		t.Fatal(err)
	}
	var ftsRows int
	s.db.QueryRow(`SELECT count(*) FROM entries_fts WHERE entries_fts MATCH 'delta'`).Scan(&ftsRows)
	if ftsRows != 0 {
		t.Fatal("FTS rows left after cascade delete")
	}
}

func TestEntryQueryBuilder(t *testing.T) {
	s := newTestStorage(t)
	user := mustCreateUser(t, s, "alice")
	other := mustCreateUser(t, s, "bob")
	all, _ := s.FirstCategory(user.ID)
	news, err := s.CreateCategory(user.ID, &model.CategoryCreationRequest{Title: "News", HideGlobally: true})
	if err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	e1 := newEntry("1", "Running fast", "The quick brown fox", base, "Tech", "Go")
	e2 := newEntry("2", "Cooking", "Slow food recipes", base.Add(time.Hour), "food")
	e3 := newEntry("3", "Hidden news", "Breaking quick news", base.Add(2*time.Hour), "tech")
	feedA := mustCreateFeed(t, s, user.ID, all.ID, "https://a.example/feed", e1, e2)
	feedB := mustCreateFeed(t, s, user.ID, news.ID, "https://b.example/feed", e3)
	otherCategory, _ := s.FirstCategory(other.ID)
	mustCreateFeed(t, s, other.ID, otherCategory.ID, "https://a.example/feed", newEntry("1", "Other", "quick", base))

	if err := s.SetEntriesStarredState(user.ID, []int64{e2.ID}, true); err != nil {
		t.Fatal(err)
	}
	if err := s.SetEntriesStatus(user.ID, []int64{e1.ID}, model.EntryStatusRead); err != nil {
		t.Fatal(err)
	}

	q := func() *EntryQueryBuilder { return s.NewEntryQueryBuilder(user.ID).WithSorting("published_at", "asc") }
	tests := []struct {
		name string
		b    *EntryQueryBuilder
		want []int64
	}{
		{"all", q(), []int64{e1.ID, e2.ID, e3.ID}},
		{"desc", s.NewEntryQueryBuilder(user.ID).WithSorting("published_at", "DESC"), []int64{e3.ID, e2.ID, e1.ID}},
		{"status", q().WithStatuses(model.EntryStatusUnread), []int64{e2.ID, e3.ID}},
		{"statuses", q().WithStatuses(model.EntryStatusUnread, model.EntryStatusRead), []int64{e1.ID, e2.ID, e3.ID}},
		{"without status", q().WithoutStatus(model.EntryStatusUnread), []int64{e1.ID}},
		{"starred", q().WithStarred(true), []int64{e2.ID}},
		{"not starred", q().WithStarred(false), []int64{e1.ID, e3.ID}},
		{"feed", q().WithFeedID(feedB.ID), []int64{e3.ID}},
		{"category", q().WithCategoryID(all.ID), []int64{e1.ID, e2.ID}},
		{"globally visible", q().WithGloballyVisible(), []int64{e1.ID, e2.ID}},
		{"entry ids", q().WithEntryIDs(e1.ID, e3.ID), []int64{e1.ID, e3.ID}},
		{"before", q().BeforePublishedDate(base.Add(time.Hour)), []int64{e1.ID}},
		{"after", q().AfterPublishedDate(base), []int64{e2.ID, e3.ID}},
		{"after id", q().AfterEntryID(e1.ID), []int64{e2.ID, e3.ID}},
		{"before id", q().BeforeEntryID(e3.ID), []int64{e1.ID, e2.ID}},
		{"tags case-insensitive", q().WithTags("TECH"), []int64{e1.ID, e3.ID}},
		{"all tags", q().WithTags("tech", "go"), []int64{e1.ID}},
		{"limit offset", q().WithLimit(1).WithOffset(1), []int64{e2.ID}},
		{"offset only", q().WithOffset(2), []int64{e3.ID}},
		{"category title sort", s.NewEntryQueryBuilder(user.ID).WithSorting("category_title", "desc").WithSorting("id", "asc"), []int64{e3.ID, e1.ID, e2.ID}},
		{"search", q().WithSearchQuery("quick"), []int64{e1.ID, e3.ID}},
		{"search stemming", q().WithSearchQuery("run"), []int64{e1.ID}},
		{"search phrase", q().WithSearchQuery(`"brown fox"`), []int64{e1.ID}},
		{"search phrase miss", q().WithSearchQuery(`"fox brown"`), nil},
		{"search and", q().WithSearchQuery("quick news"), []int64{e3.ID}},
		{"search or", q().WithSearchQuery("fox or recipes"), []int64{e1.ID, e2.ID}},
		{"search not", q().WithSearchQuery("quick -news"), []int64{e1.ID}},
		{"search hostile syntax is literal", q().WithSearchQuery(`quick" OR NEAR(* "^`), []int64{e1.ID, e3.ID}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := entryIDs(t, tt.b); !slices.Equal(got, tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}

	// Relevance ranking: an older title match outranks a newer content match, both ways round.
	for _, query := range []string{"recipes or fast", "fast or recipes"} {
		if got := entryIDs(t, s.NewEntryQueryBuilder(user.ID).WithSearchQuery(query)); !slices.Equal(got, []int64{e1.ID, e2.ID}) {
			t.Fatal("search ranking", query, got)
		}
	}

	entries, total, err := q().WithLimit(2).GetEntriesWithCount()
	if err != nil || len(entries) != 2 || total != 3 {
		t.Fatal("GetEntriesWithCount", len(entries), total, err)
	}
	if _, total, err := q().WithLimit(2).WithOffset(10).GetEntriesWithCount(); err != nil || total != 3 {
		t.Fatal("GetEntriesWithCount past the end", total, err)
	}
	ids, total, err := q().WithStatuses(model.EntryStatusUnread).WithLimit(1).GetEntryIDsWithCount()
	if err != nil || !slices.Equal(ids, []int64{e2.ID}) || total != 2 {
		t.Fatal("GetEntryIDsWithCount", ids, total, err)
	}
	searchEntries, total, err := s.NewEntryQueryBuilder(user.ID).WithSearchQuery("quick").WithLimit(1).GetEntriesWithCount()
	if err != nil || len(searchEntries) != 1 || total != 2 {
		t.Fatal("GetEntriesWithCount with search", len(searchEntries), total, err)
	}
	ids, total, err = s.NewEntryQueryBuilder(user.ID).WithSearchQuery("quick").WithLimit(1).GetEntryIDsWithCount()
	if err != nil || len(ids) != 1 || total != 2 {
		t.Fatal("GetEntryIDsWithCount with search", ids, total, err)
	}
	if entries, _ := q().WithoutContent().GetEntries(); entries[0].Content != "" {
		t.Fatal("WithoutContent returned content")
	}

	code, err := s.EntryShareCode(user.ID, e2.ID)
	if err != nil || code == "" {
		t.Fatal(code, err)
	}
	if again, _ := s.EntryShareCode(user.ID, e2.ID); again != code {
		t.Fatal("share code changed")
	}
	if shared, err := s.NewAnonymousQueryBuilder().WithShareCode(code).GetEntry(); err != nil || shared == nil || shared.ID != e2.ID {
		t.Fatal("anonymous share lookup", shared, err)
	}

	prev, next, err := s.NewEntryPaginationBuilder(user.ID, e2.ID, "published_at", "asc").Entries()
	if err != nil || prev == nil || next == nil || prev.ID != e1.ID || next.ID != e3.ID {
		t.Fatal("pagination", prev, next, err)
	}
	prev, next, err = s.NewEntryPaginationBuilder(user.ID, e2.ID, "published_at", "desc").WithGloballyVisible().Entries()
	if err != nil || prev != nil || next == nil || next.ID != e1.ID {
		t.Fatal("pagination desc visible", prev, next, err)
	}
	prev, next, err = s.NewEntryPaginationBuilder(user.ID, e3.ID, "published_at", "asc").WithSearchQuery("quick").Entries()
	if err != nil || prev == nil || prev.ID != e1.ID || next != nil {
		t.Fatal("pagination search", prev, next, err)
	}

	categories, err := s.CategoriesWithFeedCount(user.ID, "unread_count")
	if err != nil || len(categories) != 2 || categories[0].Title != "All" || *categories[0].FeedCount != 1 || *categories[0].TotalUnread != 1 {
		t.Fatal("CategoriesWithFeedCount", categories, err)
	}

	feeds, err := s.FeedsWithCounters(user.ID)
	if err != nil || len(feeds) != 2 {
		t.Fatal(feeds, err)
	}
	for _, f := range feeds {
		if f.ID == feedA.ID && (f.ReadCount != 1 || f.UnreadCount != 1) {
			t.Fatalf("feed counters: %+v", f)
		}
	}
}

func TestMarkAsRead(t *testing.T) {
	s := newTestStorage(t)
	user := mustCreateUser(t, s, "alice")
	all, _ := s.FirstCategory(user.ID)
	hidden, _ := s.CreateCategory(user.ID, &model.CategoryCreationRequest{Title: "Hidden", HideGlobally: true})
	now := time.Now()

	a1, a2 := newEntry("1", "a1", "", now.Add(-2*time.Hour)), newEntry("2", "a2", "", now)
	h1 := newEntry("3", "h1", "", now.Add(-2*time.Hour))
	feedA := mustCreateFeed(t, s, user.ID, all.ID, "https://a.example/", a1, a2)
	mustCreateFeed(t, s, user.ID, hidden.ID, "https://h.example/", h1)

	unread := func() []int64 {
		return entryIDs(t, s.NewEntryQueryBuilder(user.ID).WithStatuses(model.EntryStatusUnread).WithSorting("id", "asc"))
	}

	if err := s.MarkFeedAsRead(user.ID, feedA.ID, now.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if got := unread(); !slices.Equal(got, []int64{a2.ID, h1.ID}) {
		t.Fatal("MarkFeedAsRead", got)
	}
	s.SetEntriesStatus(user.ID, []int64{a1.ID, a2.ID, h1.ID}, model.EntryStatusUnread)

	if err := s.MarkGloballyVisibleFeedsAsRead(user.ID); err != nil {
		t.Fatal(err)
	}
	if got := unread(); !slices.Equal(got, []int64{h1.ID}) {
		t.Fatal("MarkGloballyVisibleFeedsAsRead", got)
	}

	s.SetEntriesStatus(user.ID, []int64{a1.ID, a2.ID}, model.EntryStatusUnread)
	if err := s.MarkCategoryAsRead(user.ID, hidden.ID, now); err != nil {
		t.Fatal(err)
	}
	if got := unread(); !slices.Equal(got, []int64{a1.ID, a2.ID}) {
		t.Fatal("MarkCategoryAsRead", got)
	}

	if err := s.MarkAllAsRead(user.ID); err != nil {
		t.Fatal(err)
	}
	if got := unread(); len(got) != 0 {
		t.Fatal("MarkAllAsRead", got)
	}

	if err := s.ToggleStarred(user.ID, a1.ID); err != nil {
		t.Fatal(err)
	}
	if got := entryIDs(t, s.NewEntryQueryBuilder(user.ID).WithStarred(true)); !slices.Equal(got, []int64{a1.ID}) {
		t.Fatal("ToggleStarred", got)
	}
}

func TestArchiveAndFlushHistory(t *testing.T) {
	s := newTestStorage(t)
	user := mustCreateUser(t, s, "alice")
	all, _ := s.FirstCategory(user.ID)
	now := time.Now()

	oldRead, oldStarred, oldUnread, recentRead := newEntry("1", "", "", now), newEntry("2", "", "", now), newEntry("3", "", "", now), newEntry("4", "", "", now)
	feed := mustCreateFeed(t, s, user.ID, all.ID, "https://a.example/", oldRead, oldStarred, oldUnread, recentRead)
	s.SetEntriesStatus(user.ID, []int64{oldRead.ID, oldStarred.ID, recentRead.ID}, model.EntryStatusRead)
	s.SetEntriesStarredState(user.ID, []int64{oldStarred.ID}, true)
	s.db.Exec(`UPDATE entries SET created_at = unixepoch() - 10 * 86400 WHERE id IN ($1, $2, $3)`, oldRead.ID, oldStarred.ID, oldUnread.ID)

	count, err := s.ArchiveEntries(model.EntryStatusRead, 5*24*time.Hour, 100)
	if err != nil || count != 1 {
		t.Fatal("ArchiveEntries", count, err)
	}
	remaining := entryIDs(t, s.NewEntryQueryBuilder(user.ID).WithSorting("id", "asc"))
	if !slices.Equal(remaining, []int64{oldStarred.ID, oldUnread.ID, recentRead.ID}) {
		t.Fatal("remaining after archive", remaining)
	}

	// The archived entry is tombstoned and is not re-ingested.
	if s.IsNewEntry(feed.ID, "1") {
		t.Fatal("archived entry reported as new")
	}
	newEntries, err := s.RefreshFeedEntries(user.ID, feed.ID, model.Entries{newEntry("1", "", "", now)}, false)
	if err != nil || len(newEntries) != 0 {
		t.Fatal("tombstoned entry re-ingested", newEntries, err)
	}
	if _, err := s.InsertEntryForFeed(user.ID, feed.ID, newEntry("1", "", "", now)); err != ErrEntryTombstoned {
		t.Fatal("expected ErrEntryTombstoned, got", err)
	}

	if err := s.FlushHistory(user.ID); err != nil {
		t.Fatal(err)
	}
	remaining = entryIDs(t, s.NewEntryQueryBuilder(user.ID).WithSorting("id", "asc"))
	if !slices.Equal(remaining, []int64{oldStarred.ID, oldUnread.ID}) || s.IsNewEntry(feed.ID, "4") {
		t.Fatal("FlushHistory", remaining)
	}
}

func TestWebSessions(t *testing.T) {
	s := newTestStorage(t)
	user := mustCreateUser(t, s, "alice")

	session, secret := model.NewWebSession("agent", "127.0.0.1")
	if err := s.CreateWebSession(session); err != nil {
		t.Fatal(err)
	}
	got, err := s.WebSessionByID(session.ID)
	if err != nil || got == nil || !got.VerifySecret(secret) || got.IsAuthenticated() || got.IP != "127.0.0.1" || got.CreatedAt.IsZero() {
		t.Fatalf("WebSessionByID: %+v %v", got, err)
	}

	oldID, newSecret := got.Rotate()
	got.SetUser(user)
	got.SetSuccessMessage("hello")
	if err := s.RotateWebSession(oldID, got); err != nil {
		t.Fatal(err)
	}
	if old, _ := s.WebSessionByID(oldID); old != nil {
		t.Fatal("old session ID still valid")
	}
	got, _ = s.WebSessionByID(got.ID)
	if id, ok := got.UserID(); !ok || id != user.ID || !got.VerifySecret(newSecret) {
		t.Fatalf("rotated session: %+v", got)
	}
	if msg, _ := got.ConsumeMessages(); msg != "hello" {
		t.Fatal("state not persisted", msg)
	}
	if err := s.UpdateWebSession(got); err != nil {
		t.Fatal(err)
	}

	anonymous, _ := model.NewWebSession("", "")
	s.CreateWebSession(anonymous)
	s.db.Exec(`UPDATE web_sessions SET created_at = unixepoch() - 2 * 86400`)
	if n, err := s.CleanOldWebSessions(30 * 24 * time.Hour); err != nil || n != 1 {
		t.Fatal("CleanOldWebSessions", n, err)
	}
	if sessions, err := s.WebSessionsByUserID(user.ID); err != nil || len(sessions) != 1 {
		t.Fatal("WebSessionsByUserID", sessions, err)
	}
	if err := s.RemoveUserWebSessionsExcept(user.ID, ""); err != nil {
		t.Fatal(err)
	}
	if sessions, _ := s.WebSessionsByUserID(user.ID); len(sessions) != 0 {
		t.Fatal("sessions left", sessions)
	}
}

func TestIntegrations(t *testing.T) {
	var err error
	if config.Opts, err = config.Parse(nil); err != nil {
		t.Fatal(err)
	}
	s := newTestStorage(t)
	user := mustCreateUser(t, s, "alice")
	other := mustCreateUser(t, s, "bob")

	integration, err := s.Integration(user.ID)
	if err != nil || integration.FeverEnabled || integration.TelegramBotTopicID != nil {
		t.Fatalf("default integration: %+v %v", integration, err)
	}
	if s.HasSaveEntry(user.ID) {
		t.Fatal("HasSaveEntry without integrations")
	}

	topic := int64(42)
	integration.FeverEnabled, integration.FeverUsername, integration.FeverToken = true, "fever", "ABCDEF"
	integration.GoogleReaderEnabled, integration.GoogleReaderUsername = true, "reader"
	integration.GoogleReaderPassword = mustHash(t, "gr-pass")
	integration.TelegramBotTopicID = &topic
	integration.SlackEnabled = true
	if err := s.UpdateIntegration(integration); err != nil {
		t.Fatal(err)
	}

	got, _ := s.Integration(user.ID)
	if !got.FeverEnabled || got.TelegramBotTopicID == nil || *got.TelegramBotTopicID != 42 || !got.SlackEnabled {
		t.Fatalf("integration round trip: %+v", got)
	}
	if !s.HasSaveEntry(user.ID) || s.HasSaveEntry(other.ID) {
		t.Fatal("HasSaveEntry")
	}
	if u, err := s.UserByFeverToken("abcdef"); err != nil || u == nil || u.ID != user.ID || u.Timezone != "UTC" {
		t.Fatal("UserByFeverToken", u, err)
	}
	if u, err := s.UserByFeverToken("other"); err != nil || u != nil {
		t.Fatal("UserByFeverToken unknown", u, err)
	}
	if !s.HasDuplicateFeverUsername(other.ID, "fever") || s.HasDuplicateFeverUsername(user.ID, "fever") {
		t.Fatal("HasDuplicateFeverUsername")
	}
	if !s.HasDuplicateGoogleReaderUsername(other.ID, "reader") {
		t.Fatal("HasDuplicateGoogleReaderUsername")
	}
	if err := s.GoogleReaderUserCheckPassword("reader", "gr-pass"); err != nil {
		t.Fatal(err)
	}
	if s.GoogleReaderUserCheckPassword("reader", "bad") == nil {
		t.Fatal("GoogleReaderUserCheckPassword accepted a bad password")
	}
	if gr, err := s.GoogleReaderUserGetIntegration("reader"); err != nil || gr.UserID != user.ID {
		t.Fatal("GoogleReaderUserGetIntegration", gr, err)
	}

	meta, err := s.GetNavMetadata(user.ID)
	if err != nil || !meta.HasSaveEntry || meta.CountUnread != 0 {
		t.Fatal("GetNavMetadata", meta, err)
	}
}

func TestIconsAndBatch(t *testing.T) {
	s := newTestStorage(t)
	user := mustCreateUser(t, s, "alice")
	all, _ := s.FirstCategory(user.ID)
	f1 := mustCreateFeed(t, s, user.ID, all.ID, "https://a.example/1")
	f2 := mustCreateFeed(t, s, user.ID, all.ID, "https://a.example/2")
	mustCreateFeed(t, s, user.ID, all.ID, "https://b.example/1")

	icon := &model.Icon{Hash: "h", MimeType: "IMAGE/PNG", Content: []byte{1, 2}}
	if err := s.StoreFeedIcon(f1.ID, icon); err != nil {
		t.Fatal(err)
	}
	if err := s.StoreFeedIcon(f2.ID, &model.Icon{Hash: "h", MimeType: "image/png", Content: []byte{1, 2}}); err != nil {
		t.Fatal(err)
	}
	got, err := s.IconByFeedID(user.ID, f2.ID)
	if err != nil || got == nil || got.ID != icon.ID || got.MimeType != "image/png" || got.ExternalID == "" {
		t.Fatal("IconByFeedID", got, err)
	}
	if feed, _ := s.FeedByID(user.ID, f1.ID); feed.Icon.IconID != icon.ID || feed.Icon.ExternalIconID != got.ExternalID {
		t.Fatalf("feed icon: %+v", feed.Icon)
	}
	s.RemoveFeed(user.ID, f1.ID)
	s.RemoveFeed(user.ID, f2.ID)
	if n, err := s.CleanupOrphanIcons(); err != nil || n != 1 {
		t.Fatal("CleanupOrphanIcons", n, err)
	}

	s.db.Exec(`UPDATE feeds SET next_check_at = unixepoch() - 60`)
	mustCreateFeed(t, s, user.ID, all.ID, "https://b.example/2")
	mustCreateFeed(t, s, user.ID, all.ID, "https://c.example/1")
	s.db.Exec(`UPDATE feeds SET next_check_at = unixepoch() - 30 WHERE feed_url <> 'https://b.example/1'`)
	jobs, err := s.NewBatchBuilder().WithNextCheckExpired().WithoutDisabledFeeds().WithLimitPerHost(1).WithBatchSize(10).FetchJobs()
	if err != nil || !slices.Equal(jobs.FeedURLs(), []string{"https://b.example/1", "https://c.example/1"}) {
		t.Fatal("FetchJobs", jobs, err)
	}
	if weekly, err := s.WeeklyFeedEntryCount(user.ID, f1.ID); err != nil || weekly != 0 {
		t.Fatal("WeeklyFeedEntryCount without entries", weekly, err)
	}
	now := time.Now()
	busy := mustCreateFeed(t, s, user.ID, all.ID, "https://d.example/",
		newEntry("1", "", "", now.Add(-72*time.Hour)), newEntry("2", "", "", now.Add(-48*time.Hour)), newEntry("3", "", "", now.Add(-24*time.Hour)),
		newEntry("old", "", "", now.Add(-30*24*time.Hour)))
	if weekly, err := s.WeeklyFeedEntryCount(user.ID, busy.ID); err != nil || weekly != 7 {
		t.Fatal("WeeklyFeedEntryCount", weekly, err)
	}
}

func mustHash(t *testing.T, password string) string {
	t.Helper()
	hash, err := crypto.HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	return hash
}

func TestFTSQuery(t *testing.T) {
	tests := map[string]string{
		"":                     "",
		"  go  ":               `"go"`,
		`"brown fox" quick`:    `"brown fox" "quick"`,
		"a or b":               `"a" OR "b"`,
		"or a":                 `"a"`,
		"a -b":                 `"a" NOT "b"`,
		"-b":                   "",
		`he said "hi`:          `"he" "said" "hi"`,
		`x"y`:                  `"x""y"`,
		`NEAR(a b) AND title:`: `"NEAR(a" "b)" "AND" "title:"`,
	}
	for input, want := range tests {
		if got := ftsQuery(input); got != want {
			t.Errorf("ftsQuery(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestWebAuthnCredentials(t *testing.T) {
	s := newTestStorage(t)
	user := mustCreateUser(t, s, "alice")

	credential := &webauthn.Credential{ID: []byte("cred"), PublicKey: []byte("pk"), AttestationType: "none"}
	credential.Authenticator.SignCount = 3
	credential.Flags.BackupEligible = true
	if err := s.AddWebAuthnCredential(user.ID, []byte("handle"), credential); err != nil {
		t.Fatal(err)
	}

	userID, got, err := s.WebAuthnCredentialByHandle([]byte("handle"))
	if err != nil || userID != user.ID || string(got.Credential.ID) != "cred" || got.Credential.Authenticator.SignCount != 3 ||
		!got.Credential.Flags.BackupEligible || got.AddedOn == nil {
		t.Fatalf("WebAuthnCredentialByHandle: %+v %v", got, err)
	}

	credential.Authenticator.SignCount = 4
	if err := s.WebAuthnSaveLogin([]byte("handle"), credential); err != nil {
		t.Fatal(err)
	}
	if err := s.WebAuthnUpdateName(user.ID, []byte("handle"), "laptop"); err != nil {
		t.Fatal("WebAuthnUpdateName", err)
	}
	creds, err := s.WebAuthnCredentialsByUserID(user.ID)
	if err != nil || len(creds) != 1 || creds[0].Name != "laptop" || creds[0].Credential.Authenticator.SignCount != 4 {
		t.Fatal("WebAuthnCredentialsByUserID", creds, err)
	}
	if err := s.DeleteCredentialByHandle(user.ID, []byte("handle")); err != nil {
		t.Fatal("DeleteCredentialByHandle", err)
	}
	if creds, err = s.WebAuthnCredentialsByUserID(user.ID); err != nil || len(creds) != 0 {
		t.Fatal("DeleteCredentialByHandle", err)
	}
}
