// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"database/sql"
	"errors"
	"fmt"

	"sss/internal/model"
)

// ErrNoCategory is returned when a user does not have any category.
var ErrNoCategory = errors.New("storage: no category found")

// AnotherCategoryExists checks if another category exists with the same title.
func (s *Storage) AnotherCategoryExists(userID, categoryID int64, title string) bool {
	var result bool
	s.db.QueryRow(`SELECT true FROM categories WHERE user_id=$1 AND id != $2 AND lower(title)=lower($3) LIMIT 1`, userID, categoryID, title).Scan(&result)
	return result
}

// CategoryTitleExists checks if the given category exists into the database.
func (s *Storage) CategoryTitleExists(userID int64, title string) bool {
	var result bool
	s.db.QueryRow(`SELECT true FROM categories WHERE user_id=$1 AND lower(title)=lower($2) LIMIT 1`, userID, title).Scan(&result)
	return result
}

// CategoryIDExists checks if the given category exists into the database.
// A missing category is reported as (false, nil).
func (s *Storage) CategoryIDExists(userID, categoryID int64) (bool, error) {
	var result bool
	err := s.db.QueryRow(`SELECT true FROM categories WHERE user_id=$1 AND id=$2 LIMIT 1`, userID, categoryID).Scan(&result)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return false, fmt.Errorf("storage: unable to check if category exists: %w", err)
	}
	return result, nil
}

// Category returns a category from the database.
func (s *Storage) Category(userID, categoryID int64) (*model.Category, error) {
	return s.fetchCategory(`SELECT id, user_id, title, hide_globally FROM categories WHERE user_id=$1 AND id=$2`, userID, categoryID)
}

// FirstCategory returns the first category for the given user.
// It returns ErrNoCategory if the user does not have any category.
func (s *Storage) FirstCategory(userID int64) (*model.Category, error) {
	category, err := s.fetchCategory(`SELECT id, user_id, title, hide_globally FROM categories WHERE user_id=$1 ORDER BY title ASC LIMIT 1`, userID)
	if err == nil && category == nil {
		return nil, ErrNoCategory
	}
	return category, err
}

// CategoryByTitle finds a category by the title.
func (s *Storage) CategoryByTitle(userID int64, title string) (*model.Category, error) {
	return s.fetchCategory(`SELECT id, user_id, title, hide_globally FROM categories WHERE user_id=$1 AND title=$2`, userID, title)
}

func (s *Storage) fetchCategory(query string, args ...any) (*model.Category, error) {
	var category model.Category
	err := s.db.QueryRow(query, args...).Scan(&category.ID, &category.UserID, &category.Title, &category.HideGlobally)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return nil, nil
	case err != nil:
		return nil, fmt.Errorf("storage: unable to fetch category: %w", err)
	}
	return &category, nil
}

// Categories returns all categories that belongs to the given user.
func (s *Storage) Categories(userID int64) (model.Categories, error) {
	rows, err := s.db.Query(`SELECT id, user_id, title, hide_globally FROM categories WHERE user_id=$1 ORDER BY title ASC`, userID)
	if err != nil {
		return nil, fmt.Errorf("storage: unable to fetch categories: %w", err)
	}
	defer rows.Close()

	categories := make(model.Categories, 0)
	for rows.Next() {
		var category model.Category
		if err := rows.Scan(&category.ID, &category.UserID, &category.Title, &category.HideGlobally); err != nil {
			return nil, fmt.Errorf("storage: unable to fetch category row: %w", err)
		}
		categories = append(categories, category)
	}
	return categories, rows.Err()
}

// CategoriesWithFeedCount returns all categories with the number of feeds and
// unread entries, sorted by unread count ("unread_count") or title ("alphabetical").
func (s *Storage) CategoriesWithFeedCount(userID int64, sortOrder string) (model.Categories, error) {
	query := `
		SELECT
			c.id,
			c.user_id,
			c.title,
			c.hide_globally,
			coalesce(fc.feed_count, 0),
			coalesce(uc.unread_count, 0)
		FROM categories c
		LEFT JOIN (
			SELECT category_id, count(*) AS feed_count
			FROM feeds
			WHERE user_id = $2
			GROUP BY category_id
		) fc ON fc.category_id = c.id
		LEFT JOIN (
			SELECT f.category_id, count(*) AS unread_count
			FROM entries e
			INNER JOIN feeds f ON f.id = e.feed_id
			WHERE e.user_id = $2 AND e.status = $1
			GROUP BY f.category_id
		) uc ON uc.category_id = c.id
		WHERE c.user_id = $2
	`
	if sortOrder == "alphabetical" {
		query += ` ORDER BY c.title ASC`
	} else {
		query += ` ORDER BY coalesce(uc.unread_count, 0) DESC, c.title ASC`
	}

	rows, err := s.db.Query(query, model.EntryStatusUnread, userID)
	if err != nil {
		return nil, fmt.Errorf("storage: unable to fetch categories: %w", err)
	}
	defer rows.Close()

	categories := make(model.Categories, 0)
	for rows.Next() {
		var category model.Category
		if err := rows.Scan(&category.ID, &category.UserID, &category.Title, &category.HideGlobally, &category.FeedCount, &category.TotalUnread); err != nil {
			return nil, fmt.Errorf("storage: unable to fetch category row: %w", err)
		}
		categories = append(categories, category)
	}
	return categories, rows.Err()
}

// CreateCategory creates a new category.
func (s *Storage) CreateCategory(userID int64, request *model.CategoryCreationRequest) (*model.Category, error) {
	var category model.Category
	err := s.db.QueryRow(`
		INSERT INTO categories (user_id, title, hide_globally) VALUES ($1, $2, $3)
		RETURNING id, user_id, title, hide_globally`,
		userID, request.Title, request.HideGlobally,
	).Scan(&category.ID, &category.UserID, &category.Title, &category.HideGlobally)
	if err != nil {
		return nil, fmt.Errorf("storage: unable to create category %q for user ID %d: %w", request.Title, userID, err)
	}
	return &category, nil
}

// UpdateCategory updates an existing category.
func (s *Storage) UpdateCategory(category *model.Category) error {
	_, err := s.db.Exec(`UPDATE categories SET title=$1, hide_globally=$2 WHERE id=$3 AND user_id=$4`,
		category.Title, category.HideGlobally, category.ID, category.UserID)
	if err != nil {
		return fmt.Errorf("storage: unable to update category: %w", err)
	}
	return nil
}

// RemoveCategory deletes a category.
func (s *Storage) RemoveCategory(userID, categoryID int64) error {
	result, err := s.db.Exec(`DELETE FROM categories WHERE id=$1 AND user_id=$2`, categoryID, userID)
	if err != nil {
		return fmt.Errorf("storage: unable to remove this category: %w", err)
	}
	if count, _ := result.RowsAffected(); count == 0 {
		return errors.New("storage: no category has been removed")
	}
	return nil
}

// RemoveAndReplaceCategoriesByName deletes categories with the given titles and
// reassigns affected feeds to the user's first remaining category. It returns
// an error if the deletion would leave the user without any categories.
func (s *Storage) RemoveAndReplaceCategoriesByName(userID int64, titles []string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("storage: unable to begin transaction: %w", err)
	}

	titlesJSON := jsonArray(titles)
	var count int
	err = tx.QueryRow(`SELECT count(*) FROM categories WHERE user_id=$1 AND title NOT IN (SELECT value FROM json_each($2))`, userID, titlesJSON).Scan(&count)
	if err != nil {
		return rollback(tx, fmt.Errorf("storage: unable to retrieve category count: %w", err))
	}
	if count < 1 {
		return rollback(tx, errors.New("storage: at least 1 category must remain after deletion"))
	}

	_, err = tx.Exec(`
		UPDATE feeds SET category_id = (
			SELECT id FROM categories
			WHERE user_id=$1 AND title NOT IN (SELECT value FROM json_each($2))
			ORDER BY title ASC
			LIMIT 1
		)
		WHERE user_id=$1 AND category_id IN (
			SELECT id FROM categories WHERE user_id=$1 AND title IN (SELECT value FROM json_each($2))
		)`, userID, titlesJSON)
	if err != nil {
		return rollback(tx, fmt.Errorf("storage: unable to replace categories: %w", err))
	}

	if _, err := tx.Exec(`DELETE FROM categories WHERE user_id=$1 AND title IN (SELECT value FROM json_each($2))`, userID, titlesJSON); err != nil {
		return rollback(tx, fmt.Errorf("storage: unable to delete categories: %w", err))
	}
	return tx.Commit()
}
