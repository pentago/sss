package validator

// Store is the subset of storage queries needed by validators; *storage.Storage implements it.
type Store interface {
	UserExists(username string) bool
	AnotherUserExists(userID int64, username string) bool
	APIKeyExists(userID int64, description string) bool
	CategoryTitleExists(userID int64, title string) bool
	AnotherCategoryExists(userID, categoryID int64, title string) bool
	FeedURLExists(userID int64, feedURL string) bool
	AnotherFeedURLExists(userID, feedID int64, feedURL string) bool
	CategoryIDExists(userID, categoryID int64) (bool, error)
}
