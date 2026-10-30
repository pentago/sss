// Copyright The Miniflux Authors. Modified by The sss Authors.
// Derived from Miniflux (https://github.com/miniflux/v2).
// SPDX-License-Identifier: Apache-2.0

package model

// Job represents a payload sent to the processing queue.
type Job struct {
	UserID  int64
	FeedID  int64
	FeedURL string
}

// JobList represents a list of jobs.
type JobList []Job

// FeedURLs returns a list of feed URLs from the job list.
// This is useful for logging or debugging purposes to see which feeds are being processed.
func (jl *JobList) FeedURLs() []string {
	feedURLs := make([]string, len(*jl))
	for i, job := range *jl {
		feedURLs[i] = job.FeedURL
	}
	return feedURLs
}
