// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

// Package metric exposes Prometheus metrics at /metrics when METRICS_COLLECTOR is enabled.
package metric

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"sss/internal/config"
	"sss/internal/crypto"
	"sss/internal/http/request"
	"sss/internal/storage"
)

// Histograms observed by the workers, the scraper and the cleanup tasks.
var (
	BackgroundFeedRefreshDuration = histogram("background_feed_refresh_duration", "Processing time to refresh feeds from the background workers", 15)
	ScraperRequestDuration        = histogram("scraper_request_duration", "Web scraper request duration", 25)
	ArchiveEntriesDuration        = histogram("archive_entries_duration", "Archive entries duration", 30)
)

var (
	usersGauge       = gauge("users", "Number of users")
	brokenFeedsGauge = gauge("broken_feeds", "Number of broken feeds")
	feedsGauge       = gaugeVec("feeds", "Number of feeds by status")
	entriesGauge     = gaugeVec("entries", "Number of entries by status")

	dbOpenConnectionsGauge              = gauge("db_open_connections", "The number of established connections both in use and idle")
	dbConnectionsInUseGauge             = gauge("db_connections_in_use", "The number of connections currently in use")
	dbConnectionsIdleGauge              = gauge("db_connections_idle", "The number of idle connections")
	dbConnectionsWaitCountGauge         = gauge("db_connections_wait_count", "The total number of connections waited for")
	dbConnectionsMaxIdleClosedGauge     = gauge("db_connections_max_idle_closed", "The total number of connections closed due to SetMaxIdleConns")
	dbConnectionsMaxIdleTimeClosedGauge = gauge("db_connections_max_idle_time_closed", "The total number of connections closed due to SetConnMaxIdleTime")
	dbConnectionsMaxLifetimeClosedGauge = gauge("db_connections_max_lifetime_closed", "The total number of connections closed due to SetConnMaxLifetime")
)

func histogram(name, help string, buckets int) *prometheus.HistogramVec {
	return prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "sss", Name: name, Help: help, Buckets: prometheus.LinearBuckets(1, 2, buckets),
	}, []string{"status"})
}

func gauge(name, help string) prometheus.Gauge {
	return prometheus.NewGauge(prometheus.GaugeOpts{Namespace: "sss", Name: name, Help: help})
}

func gaugeVec(name, help string) *prometheus.GaugeVec {
	return prometheus.NewGaugeVec(prometheus.GaugeOpts{Namespace: "sss", Name: name, Help: help}, []string{"status"})
}

// ObserveSince records the time elapsed since start, labelled "success" or "error".
func ObserveSince(h *prometheus.HistogramVec, start time.Time, err error) {
	status := "success"
	if err != nil {
		status = "error"
	}
	h.WithLabelValues(status).Observe(time.Since(start).Seconds())
}

// Collect registers the metrics and refreshes the storage gauges every
// METRICS_REFRESH_INTERVAL until ctx is done.
func Collect(ctx context.Context, store *storage.Storage) {
	prometheus.MustRegister(
		BackgroundFeedRefreshDuration, ScraperRequestDuration, ArchiveEntriesDuration,
		usersGauge, brokenFeedsGauge, feedsGauge, entriesGauge,
		dbOpenConnectionsGauge, dbConnectionsInUseGauge, dbConnectionsIdleGauge, dbConnectionsWaitCountGauge,
		dbConnectionsMaxIdleClosedGauge, dbConnectionsMaxIdleTimeClosedGauge, dbConnectionsMaxLifetimeClosedGauge,
	)

	ticker := time.NewTicker(config.Opts.MetricsRefreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		if c, err := store.MetricCounts(config.Opts.PollingParsingErrorLimit); err != nil {
			slog.Warn("Unable to collect metrics", slog.Any("error", err))
		} else {
			usersGauge.Set(float64(c.Users))
			brokenFeedsGauge.Set(float64(c.BrokenFeeds))
			feedsGauge.WithLabelValues("enabled").Set(float64(c.EnabledFeeds))
			feedsGauge.WithLabelValues("disabled").Set(float64(c.DisabledFeeds))
			feedsGauge.WithLabelValues("total").Set(float64(c.EnabledFeeds + c.DisabledFeeds))
			entriesGauge.WithLabelValues("unread").Set(float64(c.UnreadEntries))
			entriesGauge.WithLabelValues("read").Set(float64(c.ReadEntries))
			entriesGauge.WithLabelValues("total").Set(float64(c.UnreadEntries + c.ReadEntries))
		}

		stats := store.DBStats()
		dbOpenConnectionsGauge.Set(float64(stats.OpenConnections))
		dbConnectionsInUseGauge.Set(float64(stats.InUse))
		dbConnectionsIdleGauge.Set(float64(stats.Idle))
		dbConnectionsWaitCountGauge.Set(float64(stats.WaitCount))
		dbConnectionsMaxIdleClosedGauge.Set(float64(stats.MaxIdleClosed))
		dbConnectionsMaxIdleTimeClosedGauge.Set(float64(stats.MaxIdleTimeClosed))
		dbConnectionsMaxLifetimeClosedGauge.Set(float64(stats.MaxLifetimeClosed))
	}
}

// Handler serves the metrics to clients from METRICS_ALLOWED_NETWORKS that
// pass METRICS_USERNAME/METRICS_PASSWORD basic auth when both are set;
// everyone else gets a 404.
func Handler() http.Handler {
	metrics := promhttp.Handler()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !allowed(r) {
			slog.Warn("Authentication failed while accessing the metrics endpoint",
				slog.String("client_ip", request.ClientIP(r)),
				slog.String("client_user_agent", r.UserAgent()),
				slog.String("client_remote_addr", r.RemoteAddr),
			)
			http.NotFound(w, r)
			return
		}
		metrics.ServeHTTP(w, r)
	})
}

func allowed(r *http.Request) bool {
	if config.Opts.MetricsUsername != "" && config.Opts.MetricsPassword != "" {
		username, password, ok := r.BasicAuth()
		// Both comparisons run so the response time doesn't reveal which one failed.
		usernameOK := crypto.ConstantTimeCmp(username, config.Opts.MetricsUsername)
		passwordOK := crypto.ConstantTimeCmp(password, config.Opts.MetricsPassword)
		if !ok || !usernameOK || !passwordOK {
			return false
		}
	}
	return request.IsTrustedIP(request.FindRemoteIP(r), config.Opts.MetricsAllowedNetworks)
}
