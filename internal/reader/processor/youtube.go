// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

package processor

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"sss/internal/config"
	"sss/internal/model"
	"sss/internal/reader/fetcher"
	"sss/internal/reader/readingtime"
)

// youtubeAPIURL is the YouTube Data API v3 videos endpoint; tests override it.
var youtubeAPIURL = "https://www.googleapis.com/youtube/v3/videos"

func isYouTubeVideoURL(websiteURL string) bool {
	return strings.Contains(websiteURL, "youtube.com/watch?v=")
}

func getVideoIDFromYouTubeURL(websiteURL string) string {
	parsedWebsiteURL, err := url.Parse(websiteURL)
	if err != nil {
		return ""
	}
	return parsedWebsiteURL.Query().Get("v")
}

// updateEntryReadingTime sets the YouTube watch time (scraped for new entries,
// reused from storage otherwise) or falls back to the text-based estimate.
func updateEntryReadingTime(store EntryStore, feed *model.Feed, entry *model.Entry, entryIsNew bool, user *model.User) {
	if !user.ShowReadingTime {
		return
	}

	if config.Opts.FetchYouTubeWatchTime && config.Opts.YouTubeAPIKey == "" && isYouTubeVideoURL(entry.URL) {
		if !entryIsNew {
			entry.ReadingTime = store.GetReadTime(feed.ID, entry.Hash)
		} else if watchTime, err := fetchYouTubeWatchTime(entry.URL); err != nil {
			slog.Warn("Unable to fetch watch time",
				slog.String("platform", "YouTube"),
				slog.Int64("user_id", user.ID),
				slog.String("entry_url", entry.URL),
				slog.Int64("feed_id", feed.ID),
				slog.Any("error", err),
			)
		} else {
			entry.ReadingTime = watchTime
		}
	}

	if entry.ReadingTime == 0 && entry.Content != "" {
		entry.ReadingTime = readingtime.EstimateReadingTime(entry.Content, user.DefaultReadingSpeed, user.CJKReadingSpeed)
	}
}

func newWatchTimeRequestBuilder() *fetcher.RequestBuilder {
	return fetcher.NewRequestBuilder().
		WithUserAgent("", config.Opts.HTTPClientUserAgent).
		WithTimeout(config.Opts.HTTPClientTimeout).
		WithCustomApplicationProxyURL(config.Opts.HTTPClientProxy)
}

// fetchYouTubeWatchTime scrapes the video page and returns its duration in minutes.
func fetchYouTubeWatchTime(websiteURL string) (int, error) {
	responseHandler := fetcher.NewResponseHandler(newWatchTimeRequestBuilder().ExecuteRequest(websiteURL))
	defer responseHandler.Close()

	if err := responseHandler.LocalizedError(); err != nil {
		return 0, err
	}

	doc, err := goquery.NewDocumentFromReader(responseHandler.Body(config.Opts.HTTPClientMaxBodySize))
	if err != nil {
		return 0, err
	}

	duration, exists := doc.FindMatcher(goquery.Single(`meta[itemprop="duration"]`)).Attr("content")
	if !exists {
		return 0, errors.New("duration not found")
	}

	parsedDuration, err := parseISO8601Duration(duration)
	if err != nil {
		return 0, fmt.Errorf("unable to parse iso duration %s: %v", duration, err)
	}
	return int(parsedDuration.Minutes()), nil
}

// fetchYouTubeWatchTimeInBulk sets the reading time of YouTube entries from the YouTube Data API.
func fetchYouTubeWatchTimeInBulk(entries model.Entries) {
	videosEntriesMapping := make(map[string]*model.Entry, len(entries))
	videoIDs := make([]string, 0, len(entries))

	for _, entry := range entries {
		if !isYouTubeVideoURL(entry.URL) {
			continue
		}
		if videoID := getVideoIDFromYouTubeURL(entry.URL); videoID != "" {
			videosEntriesMapping[videoID] = entry
			videoIDs = append(videoIDs, videoID)
		}
	}

	if len(videoIDs) == 0 {
		return
	}

	watchTimeMap, err := fetchYouTubeWatchTimeFromAPIInBulk(videoIDs)
	if err != nil {
		slog.Warn("Unable to fetch YouTube watch time in bulk", slog.Any("error", err))
		return
	}

	for videoID, watchTime := range watchTimeMap {
		if entry, ok := videosEntriesMapping[videoID]; ok {
			entry.ReadingTime = int(watchTime.Minutes())
		}
	}
}

func fetchYouTubeWatchTimeFromAPIInBulk(videoIDs []string) (map[string]time.Duration, error) {
	apiQuery := url.Values{}
	apiQuery.Set("id", strings.Join(videoIDs, ","))
	apiQuery.Set("key", config.Opts.YouTubeAPIKey)
	apiQuery.Set("part", "contentDetails")

	responseHandler := fetcher.NewResponseHandler(newWatchTimeRequestBuilder().ExecuteRequest(youtubeAPIURL + "?" + apiQuery.Encode()))
	defer responseHandler.Close()

	if err := responseHandler.LocalizedError(); err != nil {
		return nil, err
	}

	var videos struct {
		Items []struct {
			ID             string `json:"id"`
			ContentDetails struct {
				Duration string `json:"duration"`
			} `json:"contentDetails"`
		} `json:"items"`
	}
	if err := json.NewDecoder(responseHandler.Body(config.Opts.HTTPClientMaxBodySize)).Decode(&videos); err != nil {
		return nil, fmt.Errorf("youtube: unable to decode JSON: %v", err)
	}

	watchTimeMap := make(map[string]time.Duration, len(videos.Items))
	for _, video := range videos.Items {
		duration, err := parseISO8601Duration(video.ContentDetails.Duration)
		if err != nil {
			slog.Warn("Unable to parse ISO8601 duration", slog.Any("error", err))
			continue
		}
		watchTimeMap[video.ID] = duration
	}
	return watchTimeMap, nil
}

// parseISO8601Duration parses the PT[nH][nM][nS] subset of ISO 8601 durations used by YouTube.
func parseISO8601Duration(duration string) (time.Duration, error) {
	after, ok := strings.CutPrefix(duration, "PT")
	if !ok {
		return 0, errors.New("the period doesn't start with PT")
	}

	var d time.Duration
	start := 0

	for i := range len(after) {
		var unit time.Duration

		switch after[i] {
		case 'Y', 'W', 'D':
			return 0, fmt.Errorf("the '%c' specifier isn't supported", after[i])
		case 'H':
			unit = time.Hour
		case 'M':
			unit = time.Minute
		case 'S':
			unit = time.Second
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			continue
		default:
			return 0, errors.New("invalid character in the period")
		}

		val, err := strconv.Atoi(after[start:i])
		if err != nil {
			return 0, err
		}
		d += time.Duration(val) * unit
		start = i + 1
	}
	return d, nil
}
