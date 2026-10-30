// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

// Package worker refreshes feeds in the background and runs periodic cleanup.
package worker

import (
	"log/slog"
	"sync"
	"time"

	"sss/internal/config"
	"sss/internal/model"
	"sss/internal/reader/handler"
	"sss/internal/storage"
)

// Pool is a set of background workers refreshing feeds.
type Pool struct {
	queue        chan model.Job
	shutdown     chan struct{}
	shutdownOnce sync.Once
	wg           sync.WaitGroup
}

// NewPool starts nbWorkers workers.
func NewPool(store *storage.Storage, nbWorkers int) *Pool {
	p := &Pool{queue: make(chan model.Job), shutdown: make(chan struct{})}
	for range nbWorkers {
		p.wg.Go(func() {
			for {
				select {
				case <-p.shutdown:
					return
				case job := <-p.queue:
					if err := handler.RefreshFeed(store, job.UserID, job.FeedID, false); err != nil {
						slog.Warn("Unable to refresh feed", slog.Int64("user_id", job.UserID), slog.Int64("feed_id", job.FeedID), slog.Any("error", err))
					}
				}
			}
		})
	}
	return p
}

// Push hands jobs to the workers, blocking until each is picked up.
// Jobs pushed after Shutdown are discarded.
func (p *Pool) Push(jobs model.JobList) {
	for _, job := range jobs {
		select {
		case p.queue <- job:
		case <-p.shutdown:
			return
		}
	}
}

// Shutdown stops accepting jobs and waits for running jobs to finish.
func (p *Pool) Shutdown() {
	p.shutdownOnce.Do(func() { close(p.shutdown) })
	p.wg.Wait()
}

// RefreshBatch pushes one batch of feeds due for a refresh to the pool.
func RefreshBatch(store *storage.Storage, pool *Pool) {
	jobs, err := store.NewBatchBuilder().
		WithBatchSize(config.Opts.BatchSize).
		WithErrorLimit(config.Opts.PollingParsingErrorLimit).
		WithoutDisabledFeeds().
		WithNextCheckExpired().
		WithLimitPerHost(config.Opts.PollingLimitPerHost).
		FetchJobs()
	if err != nil {
		slog.Error("Unable to fetch jobs from database", slog.Any("error", err))
		return
	}
	if len(jobs) > 0 {
		slog.Debug("Feed URLs in this batch", slog.Any("feed_urls", jobs.FeedURLs()))
		pool.Push(jobs)
	}
}

// RunCleanupTasks removes old sessions, archives old entries and deletes orphan icons.
func RunCleanupTasks(store *storage.Storage) {
	if n, err := store.CleanOldWebSessions(config.Opts.CleanupRemoveSessionsInterval); err != nil {
		slog.Error("Unable to clean old web sessions", slog.Any("error", err))
	} else {
		slog.Info("Sessions cleanup completed", slog.Int64("web_sessions_removed", n))
	}

	if n, err := store.ArchiveEntries(model.EntryStatusRead, config.Opts.CleanupArchiveReadInterval, config.Opts.CleanupArchiveBatchSize); err != nil {
		slog.Error("Unable to archive read entries", slog.Any("error", err))
	} else {
		slog.Info("Archiving read entries completed", slog.Int64("read_entries_archived", n))
	}

	if n, err := store.ArchiveEntries(model.EntryStatusUnread, config.Opts.CleanupArchiveUnreadInterval, config.Opts.CleanupArchiveBatchSize); err != nil {
		slog.Error("Unable to archive unread entries", slog.Any("error", err))
	} else {
		slog.Info("Archiving unread entries completed", slog.Int64("unread_entries_archived", n))
	}

	if n, err := store.CleanupOrphanIcons(); err != nil {
		slog.Error("Unable to clean orphan icons", slog.Any("error", err))
	} else {
		slog.Info("Orphan icons cleanup completed", slog.Int64("orphan_icons_removed", n))
	}
}

// StartScheduler polls feeds every POLLING_FREQUENCY and runs cleanup tasks
// every CLEANUP_FREQUENCY_HOURS, in background goroutines.
func StartScheduler(store *storage.Storage, pool *Pool) {
	go func() {
		for range time.Tick(config.Opts.PollingFrequency) {
			RefreshBatch(store, pool)
		}
	}()
	go func() {
		for range time.Tick(config.Opts.CleanupFrequency) {
			RunCleanupTasks(store)
		}
	}()
}
