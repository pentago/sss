// Copyright The Miniflux Authors. Modified by The sss Authors.
// Derived from Miniflux (https://github.com/miniflux/v2).
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"database/sql"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"sss/internal/model"
	"sss/internal/urllib"
)

// BatchBuilder selects the feeds to refresh as jobs.
type BatchBuilder struct {
	db           *sql.DB
	args         []any
	conditions   []string
	batchSize    int
	limitPerHost int
}

// NewBatchBuilder returns a new BatchBuilder.
func (s *Storage) NewBatchBuilder() *BatchBuilder {
	return &BatchBuilder{db: s.db}
}

func (b *BatchBuilder) nextArg(value any) string {
	b.args = append(b.args, value)
	return "$" + strconv.Itoa(len(b.args))
}

// WithBatchSize limits the number of selected feeds.
func (b *BatchBuilder) WithBatchSize(batchSize int) *BatchBuilder {
	b.batchSize = batchSize
	return b
}

// WithUserID selects the feeds of the given user.
func (b *BatchBuilder) WithUserID(userID int64) *BatchBuilder {
	b.conditions = append(b.conditions, "user_id = "+b.nextArg(userID))
	return b
}

// WithCategoryID selects the feeds of the given category.
func (b *BatchBuilder) WithCategoryID(categoryID int64) *BatchBuilder {
	b.conditions = append(b.conditions, "category_id = "+b.nextArg(categoryID))
	return b
}

// WithErrorLimit skips feeds with at least limit parsing errors (no limit when <= 0).
func (b *BatchBuilder) WithErrorLimit(limit int) *BatchBuilder {
	if limit > 0 {
		b.conditions = append(b.conditions, "parsing_error_count < "+b.nextArg(limit))
	}
	return b
}

// WithNextCheckExpired selects feeds whose next check date is past.
func (b *BatchBuilder) WithNextCheckExpired() *BatchBuilder {
	b.conditions = append(b.conditions, "next_check_at < unixepoch()")
	return b
}

// WithoutDisabledFeeds skips disabled feeds.
func (b *BatchBuilder) WithoutDisabledFeeds() *BatchBuilder {
	b.conditions = append(b.conditions, "disabled = 0")
	return b
}

// WithLimitPerHost limits the number of jobs per feed hostname (no limit when <= 0).
func (b *BatchBuilder) WithLimitPerHost(limit int) *BatchBuilder {
	if limit > 0 {
		b.limitPerHost = limit
	}
	return b
}

// FetchJobs retrieves the jobs matching the builder conditions, oldest next check first.
func (b *BatchBuilder) FetchJobs() (model.JobList, error) {
	query := `SELECT id, user_id, feed_url FROM feeds`
	if len(b.conditions) > 0 {
		query += " WHERE " + strings.Join(b.conditions, " AND ")
	}
	query += " ORDER BY next_check_at ASC"
	if b.batchSize > 0 {
		query += " LIMIT " + strconv.Itoa(b.batchSize)
	}

	rows, err := b.db.Query(query, b.args...)
	if err != nil {
		return nil, fmt.Errorf("storage: unable to fetch batch of jobs: %w", err)
	}
	defer rows.Close()

	jobs := make(model.JobList, 0, b.batchSize)
	hosts := make(map[string]int)
	nbRows, nbSkippedFeeds := 0, 0
	for rows.Next() {
		var job model.Job
		if err := rows.Scan(&job.FeedID, &job.UserID, &job.FeedURL); err != nil {
			return nil, fmt.Errorf("storage: unable to fetch job record: %w", err)
		}
		nbRows++

		if b.limitPerHost > 0 {
			hostname := urllib.Domain(job.FeedURL)
			if hosts[hostname] >= b.limitPerHost {
				nbSkippedFeeds++
				continue
			}
			hosts[hostname]++
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("storage: error iterating on job records: %w", err)
	}

	slog.Info("Created a batch of feeds",
		slog.Int("batch_size", b.batchSize),
		slog.Int("rows_count", nbRows),
		slog.Int("skipped_feeds_count", nbSkippedFeeds),
		slog.Int("jobs_count", len(jobs)),
	)
	return jobs, nil
}
