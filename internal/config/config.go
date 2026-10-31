// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

// Package config parses sss settings from a KEY=VALUE config file and environment variables.
package config

import (
	"bufio"
	"bytes"
	"cmp"
	"crypto/rand"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"math"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"sss/internal/version"
)

// Opts holds the parsed configuration. It is set once at startup by Load.
var Opts *Options

// Options holds every configuration value. Field names mirror the environment
// variable names; durations and sizes are already converted to their units.
type Options struct {
	AdminPassword                      string
	AdminUsername                      string
	AuthProxyHeader                    string
	AuthProxyUserCreation              bool
	BaseURL                            string // BASE_URL without trailing slash.
	RootURL                            string // Scheme and host of BASE_URL.
	BasePath                           string // Path of BASE_URL, "" when served at the root.
	BatchSize                          int
	CertDomain                         string
	CertFile                           string
	KeyFile                            string
	CleanupArchiveBatchSize            int
	CleanupArchiveReadInterval         time.Duration
	CleanupArchiveUnreadInterval       time.Duration
	CleanupFrequency                   time.Duration
	CleanupRemoveSessionsInterval      time.Duration
	CreateAdmin                        bool
	DatabasePath                       string // SQLite file path or file: URI.
	DisableAPI                         bool
	DisableHSTS                        bool
	DisableHTTPService                 bool
	DisableLocalAuth                   bool
	DisableSchedulerService            bool
	FetcherAllowPrivateNetworks        bool
	FetchYouTubeWatchTime              bool
	ForceRefreshInterval               time.Duration
	HTTPClientMaxBodySize              int64 // In bytes (env value is MiB).
	HTTPClientProxy                    *url.URL
	HTTPClientTimeout                  time.Duration
	HTTPClientUserAgent                string // Defaults to the sss user agent.
	HTTPServerTimeout                  time.Duration
	HTTPS                              bool
	IntegrationAllowPrivateNetworks    bool
	InvidiousInstance                  string
	ListenAddr                         []string // Overridden by ":PORT" when PORT is set.
	Port                               string
	LogDateTime                        bool
	LogFile                            string
	LogFormat                          string
	LogLevel                           string
	MaintenanceMessage                 string
	MaintenanceMode                    bool
	MediaProxyCustomURL                *url.URL
	MediaProxyHTTPClientTimeout        time.Duration
	MediaProxyMode                     string
	MediaProxyPrivateKey               []byte // Random when unset.
	MediaProxyResourceTypes            []string
	MetricsAllowedNetworks             []string
	MetricsCollector                   bool
	MetricsPassword                    string
	MetricsRefreshInterval             time.Duration
	MetricsUsername                    string
	OAuth2AdditionalScopes             []string
	OAuth2AdminGroup                   string
	OAuth2ClientID                     string
	OAuth2ClientSecret                 string
	OAuth2GroupClaim                   string
	OAuth2OIDCDiscoveryEndpoint        string
	OAuth2OIDCProviderName             string
	OAuth2Provider                     string
	OAuth2RedirectURL                  string
	OAuth2UserCreation                 bool
	PollingFrequency                   time.Duration
	PollingLimitPerHost                int
	PollingParsingErrorLimit           int
	PollingScheduler                   string
	PprofListenAddr                    string // "" disables the pprof listener.
	RunMigrations                      bool
	SchedulerEntryFrequencyFactor      int
	SchedulerEntryFrequencyMaxInterval time.Duration
	SchedulerEntryFrequencyMinInterval time.Duration
	SchedulerRoundRobinMaxInterval     time.Duration
	SchedulerRoundRobinMinInterval     time.Duration
	TrustedReverseProxyNetworks        []string
	Watchdog                           bool
	WebAuthn                           bool
	WorkerPoolSize                     int
	YouTubeAPIKey                      string
	YouTubeEmbedURLOverride            string
	YouTubeEmbedDomain                 string // Host of YOUTUBE_EMBED_URL_OVERRIDE.

	dump map[string]string
}

// Load parses the config file and the process environment into Opts. Environment
// variables override file values. An empty path means
// $XDG_CONFIG_HOME/sss/sss.conf. A missing file is skipped, so a container can be
// configured with environment variables only.
func Load(path string) error {
	environ := os.Environ()
	explicit := path != ""
	if !explicit {
		path = xdgPath(os.Getenv("XDG_CONFIG_HOME"), os.Getenv("HOME"), ".config", "sss.conf")
	}
	if path != "" {
		lines, err := readConfigFile(path)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			// sss: upstream exits on a missing -c file; main logs a warning instead.
		case err != nil:
			return err
		default:
			environ = append(lines, environ...)
		}
	}
	opts, err := Parse(environ)
	if err != nil {
		return err
	}
	Opts = opts
	return nil
}

// readConfigFile returns the KEY=VALUE lines of a config file, skipping blanks and # comments.
func readConfigFile(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	defer f.Close()
	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "#") && strings.Index(line, "=") > 0 {
			lines = append(lines, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("config: %s: %w", path, err)
	}
	return lines, nil
}

// xdgPath returns <base>/sss/<name>, where base is the XDG directory variable when it
// is an absolute path, else home/<fallback>. It returns "" when neither is usable.
func xdgPath(xdgDir, home, fallback, name string) string {
	if !filepath.IsAbs(xdgDir) {
		if home == "" {
			return ""
		}
		xdgDir = filepath.Join(home, fallback)
	}
	return filepath.Join(xdgDir, "sss", name)
}

// Parse builds Options from KEY=VALUE pairs; later duplicates win and unknown keys are ignored.
func Parse(environ []string) (*Options, error) {
	p := &parser{env: make(map[string]string), dump: make(map[string]string)}
	for _, kv := range environ {
		if k, v, ok := strings.Cut(kv, "="); ok {
			p.env[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}

	o := &Options{
		AdminPassword:                      p.secret("ADMIN_PASSWORD", ""),
		AdminUsername:                      p.fileValue("ADMIN_USERNAME", "", false),
		AuthProxyHeader:                    p.str("AUTH_PROXY_HEADER", ""),
		AuthProxyUserCreation:              p.boolean("AUTH_PROXY_USER_CREATION", false),
		BaseURL:                            p.str("BASE_URL", "http://localhost"),
		BatchSize:                          p.integer("BATCH_SIZE", "100", 1),
		CertDomain:                         p.str("CERT_DOMAIN", ""),
		CertFile:                           p.str("CERT_FILE", ""),
		KeyFile:                            p.str("KEY_FILE", ""),
		CleanupArchiveBatchSize:            p.integer("CLEANUP_ARCHIVE_BATCH_SIZE", "10000", 1),
		CleanupArchiveReadInterval:         p.duration("CLEANUP_ARCHIVE_READ_DAYS", "60", 24*time.Hour, math.MinInt),
		CleanupArchiveUnreadInterval:       p.duration("CLEANUP_ARCHIVE_UNREAD_DAYS", "180", 24*time.Hour, math.MinInt),
		CleanupFrequency:                   p.duration("CLEANUP_FREQUENCY_HOURS", "24", time.Hour, 1),
		CleanupRemoveSessionsInterval:      p.duration("CLEANUP_REMOVE_SESSIONS_DAYS", "30", 24*time.Hour, math.MinInt),
		CreateAdmin:                        p.boolean("CREATE_ADMIN", false),
		DatabasePath:                       p.str("DATABASE_PATH", cmp.Or(xdgPath(p.env["XDG_DATA_HOME"], p.env["HOME"], ".local/share", "sss.db"), "sss.db")),
		DisableAPI:                         p.boolean("DISABLE_API", false),
		DisableHSTS:                        p.boolean("DISABLE_HSTS", false),
		DisableHTTPService:                 p.boolean("DISABLE_HTTP_SERVICE", false),
		DisableLocalAuth:                   p.boolean("DISABLE_LOCAL_AUTH", false),
		DisableSchedulerService:            p.boolean("DISABLE_SCHEDULER_SERVICE", false),
		FetcherAllowPrivateNetworks:        p.boolean("FETCHER_ALLOW_PRIVATE_NETWORKS", false),
		FetchYouTubeWatchTime:              p.boolean("FETCH_YOUTUBE_WATCH_TIME", false),
		ForceRefreshInterval:               p.duration("FORCE_REFRESH_INTERVAL", "30", time.Minute, 1),
		HTTPClientMaxBodySize:              int64(p.integer("HTTP_CLIENT_MAX_BODY_SIZE", "15", 1)) * 1024 * 1024,
		HTTPClientProxy:                    p.url("HTTP_CLIENT_PROXY", true),
		HTTPClientTimeout:                  p.duration("HTTP_CLIENT_TIMEOUT", "20", time.Second, 1),
		HTTPClientUserAgent:                p.str("HTTP_CLIENT_USER_AGENT", ""),
		HTTPServerTimeout:                  p.duration("HTTP_SERVER_TIMEOUT", "300", time.Second, 1),
		HTTPS:                              p.boolean("HTTPS", false),
		IntegrationAllowPrivateNetworks:    p.boolean("INTEGRATION_ALLOW_PRIVATE_NETWORKS", false),
		InvidiousInstance:                  p.str("INVIDIOUS_INSTANCE", "yewtu.be"),
		ListenAddr:                         p.list("LISTEN_ADDR", "127.0.0.1:8080"),
		Port:                               p.str("PORT", ""),
		LogDateTime:                        p.boolean("LOG_DATE_TIME", false),
		LogFile:                            p.str("LOG_FILE", "stderr"),
		LogFormat:                          p.str("LOG_FORMAT", "text", "text", "json"),
		LogLevel:                           p.str("LOG_LEVEL", "info", "debug", "info", "warning", "error"),
		MaintenanceMessage:                 p.str("MAINTENANCE_MESSAGE", "sss is currently under maintenance"),
		MaintenanceMode:                    p.boolean("MAINTENANCE_MODE", false),
		MediaProxyCustomURL:                p.url("MEDIA_PROXY_CUSTOM_URL", false),
		MediaProxyHTTPClientTimeout:        p.duration("MEDIA_PROXY_HTTP_CLIENT_TIMEOUT", "120", time.Second, 1),
		MediaProxyMode:                     p.str("MEDIA_PROXY_MODE", "http-only", "none", "http-only", "all"),
		MediaProxyPrivateKey:               []byte(p.raw("MEDIA_PROXY_PRIVATE_KEY", "", true)),
		MediaProxyResourceTypes:            p.list("MEDIA_PROXY_RESOURCE_TYPES", "image"),
		MetricsAllowedNetworks:             p.list("METRICS_ALLOWED_NETWORKS", "127.0.0.1/8"),
		MetricsCollector:                   p.boolean("METRICS_COLLECTOR", false),
		MetricsPassword:                    p.secret("METRICS_PASSWORD", ""),
		MetricsRefreshInterval:             p.duration("METRICS_REFRESH_INTERVAL", "60", time.Second, 1),
		MetricsUsername:                    p.fileValue("METRICS_USERNAME", "", false),
		OAuth2AdditionalScopes:             p.list("OAUTH2_ADDITIONAL_SCOPES", ""),
		OAuth2AdminGroup:                   p.str("OAUTH2_ADMIN_GROUP", ""),
		OAuth2ClientID:                     p.secret("OAUTH2_CLIENT_ID", ""),
		OAuth2ClientSecret:                 p.secret("OAUTH2_CLIENT_SECRET", ""),
		OAuth2GroupClaim:                   p.str("OAUTH2_GROUP_CLAIM", ""),
		OAuth2OIDCDiscoveryEndpoint:        p.str("OAUTH2_OIDC_DISCOVERY_ENDPOINT", ""),
		OAuth2OIDCProviderName:             p.str("OAUTH2_OIDC_PROVIDER_NAME", "OpenID Connect"),
		OAuth2Provider:                     p.str("OAUTH2_PROVIDER", "", "", "oidc", "google"),
		OAuth2RedirectURL:                  p.str("OAUTH2_REDIRECT_URL", ""),
		OAuth2UserCreation:                 p.boolean("OAUTH2_USER_CREATION", false),
		PollingFrequency:                   p.duration("POLLING_FREQUENCY", "60", time.Minute, 1),
		PollingLimitPerHost:                p.integer("POLLING_LIMIT_PER_HOST", "0", 0),
		PollingParsingErrorLimit:           p.integer("POLLING_PARSING_ERROR_LIMIT", "3", 0),
		PollingScheduler:                   p.str("POLLING_SCHEDULER", "round_robin", "round_robin", "entry_frequency"),
		PprofListenAddr:                    p.str("PPROF_LISTEN_ADDR", ""),
		RunMigrations:                      p.boolean("RUN_MIGRATIONS", false),
		SchedulerEntryFrequencyFactor:      p.integer("SCHEDULER_ENTRY_FREQUENCY_FACTOR", "1", 1),
		SchedulerEntryFrequencyMaxInterval: p.duration("SCHEDULER_ENTRY_FREQUENCY_MAX_INTERVAL", "1440", time.Minute, 1),
		SchedulerEntryFrequencyMinInterval: p.duration("SCHEDULER_ENTRY_FREQUENCY_MIN_INTERVAL", "5", time.Minute, 1),
		SchedulerRoundRobinMaxInterval:     p.duration("SCHEDULER_ROUND_ROBIN_MAX_INTERVAL", "1440", time.Minute, 1),
		SchedulerRoundRobinMinInterval:     p.duration("SCHEDULER_ROUND_ROBIN_MIN_INTERVAL", "60", time.Minute, 1),
		TrustedReverseProxyNetworks:        p.list("TRUSTED_REVERSE_PROXY_NETWORKS", ""),
		Watchdog:                           p.boolean("WATCHDOG", true),
		WebAuthn:                           p.boolean("WEBAUTHN", false),
		WorkerPoolSize:                     p.integer("WORKER_POOL_SIZE", "16", 1),
		YouTubeAPIKey:                      p.raw("YOUTUBE_API_KEY", "", true),
		YouTubeEmbedURLOverride:            p.str("YOUTUBE_EMBED_URL_OVERRIDE", "https://www.youtube-nocookie.com/embed/"),
		dump:                               p.dump,
	}

	if len(o.MediaProxyResourceTypes) == 0 {
		p.fail("MEDIA_PROXY_RESOURCE_TYPES", errors.New("at least one resource type is required"))
	}
	for _, t := range o.MediaProxyResourceTypes {
		if !slices.Contains([]string{"image", "video", "audio"}, t) {
			p.fail("MEDIA_PROXY_RESOURCE_TYPES", fmt.Errorf("unknown resource type %q", t))
		}
	}
	for _, network := range o.TrustedReverseProxyNetworks {
		if _, _, err := net.ParseCIDR(network); err != nil {
			p.fail("TRUSTED_REVERSE_PROXY_NETWORKS", err)
		}
	}
	if o.Port != "" {
		if n, err := strconv.Atoi(o.Port); err != nil || n < 1 || n > 65535 {
			p.fail("PORT", errors.New("value must be between 1 and 65535"))
		}
		o.ListenAddr = []string{":" + o.Port}
	}

	o.BaseURL = strings.TrimSuffix(o.BaseURL, "/")
	if u, err := url.Parse(o.BaseURL); err != nil {
		p.fail("BASE_URL", err)
	} else if scheme := strings.ToLower(u.Scheme); scheme != "http" && scheme != "https" {
		p.fail("BASE_URL", errors.New("scheme must be http or https"))
	} else {
		o.BasePath = u.Path
		u.Path = ""
		o.RootURL = u.String()
	}

	if o.YouTubeEmbedURLOverride != "" {
		if u, err := url.Parse(o.YouTubeEmbedURLOverride); err != nil {
			p.fail("YOUTUBE_EMBED_URL_OVERRIDE", err)
		} else {
			o.YouTubeEmbedDomain = u.Hostname()
		}
	}

	if o.HTTPClientUserAgent == "" {
		o.HTTPClientUserAgent = "Mozilla/5.0 (compatible; sss/" + version.Version + ")"
	}

	if len(o.MediaProxyPrivateKey) == 0 {
		o.MediaProxyPrivateKey = make([]byte, 16)
		rand.Read(o.MediaProxyPrivateKey)
	}

	if err := errors.Join(p.errs...); err != nil {
		return nil, err
	}
	if err := o.validate(); err != nil {
		return nil, err
	}
	return o, nil
}

// validate checks invalid option combinations.
func (o *Options) validate() error {
	switch {
	case o.OAuth2Provider == "oidc" && o.OAuth2OIDCDiscoveryEndpoint == "":
		return errors.New("config: OAUTH2_OIDC_DISCOVERY_ENDPOINT must be configured when using the OIDC provider")
	case o.DisableLocalAuth && o.OAuth2Provider == "" && o.AuthProxyHeader == "" && !o.WebAuthn:
		return errors.New("config: DISABLE_LOCAL_AUTH is enabled but none of OAUTH2_PROVIDER, AUTH_PROXY_HEADER, or WEBAUTHN is set")
	case o.AuthProxyHeader != "" && len(o.TrustedReverseProxyNetworks) == 0:
		return errors.New("config: TRUSTED_REVERSE_PROXY_NETWORKS must be configured when AUTH_PROXY_HEADER is used")
	case (o.CertFile != "") != (o.KeyFile != ""):
		return errors.New("config: CERT_FILE and KEY_FILE must both be provided")
	case o.CertDomain != "" && o.CertFile != "":
		return errors.New("config: CERT_DOMAIN and CERT_FILE/KEY_FILE are mutually exclusive")
	case o.SchedulerRoundRobinMinInterval > o.SchedulerRoundRobinMaxInterval:
		return errors.New("config: SCHEDULER_ROUND_ROBIN_MIN_INTERVAL must be less than or equal to SCHEDULER_ROUND_ROBIN_MAX_INTERVAL")
	case o.SchedulerEntryFrequencyMinInterval > o.SchedulerEntryFrequencyMaxInterval:
		return errors.New("config: SCHEDULER_ENTRY_FREQUENCY_MIN_INTERVAL must be less than or equal to SCHEDULER_ENTRY_FREQUENCY_MAX_INTERVAL")
	}
	return nil
}

// Dump returns sorted NAME=value lines of the raw settings with secrets redacted.
func (o *Options) Dump() string {
	var b strings.Builder
	for _, k := range slices.Sorted(maps.Keys(o.dump)) {
		b.WriteString(k + "=" + o.dump[k] + "\n")
	}
	return b.String()
}

type parser struct {
	env  map[string]string
	dump map[string]string
	errs []error
}

func (p *parser) fail(name string, err error) {
	p.errs = append(p.errs, fmt.Errorf("config: invalid %s: %w", name, err))
}

// raw returns the value of name, or def when unset or empty, and records it for Dump.
func (p *parser) raw(name, def string, secret bool) string {
	v := p.env[name]
	if v == "" {
		v = def
	}
	p.dump[name] = v
	if secret && v != "" {
		p.dump[name] = "<redacted>"
	}
	return v
}

// str returns a string value restricted to choices when any are given.
func (p *parser) str(name, def string, choices ...string) string {
	v := p.raw(name, def, false)
	if len(choices) > 0 && !slices.Contains(choices, v) {
		p.fail(name, fmt.Errorf("value must be one of: %s", strings.Join(choices, ", ")))
	}
	return v
}

// secret returns a redacted value, read from the NAME_FILE file when that is set.
func (p *parser) secret(name, def string) string {
	return p.fileValue(name, def, true)
}

// fileValue returns a value read from the NAME_FILE file when that is set, else from NAME.
func (p *parser) fileValue(name, def string, redact bool) string {
	file := p.env[name+"_FILE"]
	if file == "" {
		return p.raw(name, def, redact)
	}
	p.dump[name+"_FILE"] = file
	data, err := os.ReadFile(file)
	if err != nil {
		p.fail(name+"_FILE", err)
		return ""
	}
	v := string(bytes.TrimSpace(data))
	if v == "" {
		p.fail(name+"_FILE", errors.New("secret file is empty"))
	}
	p.dump[name] = v
	if redact {
		p.dump[name] = "<redacted>"
	}
	return v
}

func (p *parser) boolean(name string, def bool) bool {
	d := "0"
	if def {
		d = "1"
	}
	switch strings.ToLower(p.raw(name, d, false)) {
	case "1", "yes", "true", "on":
		return true
	case "0", "no", "false", "off":
		return false
	}
	p.fail(name, errors.New("value must be a boolean"))
	return def
}

func (p *parser) integer(name, def string, minValue int) int {
	v, err := strconv.Atoi(p.raw(name, def, false))
	if err != nil {
		p.fail(name, errors.New("value must be an integer"))
	} else if v < minValue {
		p.fail(name, fmt.Errorf("value must be greater or equal than %d", minValue))
	}
	return v
}

func (p *parser) duration(name, def string, unit time.Duration, minValue int) time.Duration {
	return time.Duration(p.integer(name, def, minValue)) * unit
}

// list returns the comma-separated, trimmed, de-duplicated values.
func (p *parser) list(name, def string) []string {
	var values []string
	for item := range strings.SplitSeq(p.raw(name, def, false), ",") {
		if item = strings.TrimSpace(item); item != "" && !slices.Contains(values, item) {
			values = append(values, item)
		}
	}
	return values
}

func (p *parser) url(name string, secret bool) *url.URL {
	v := p.raw(name, "", secret)
	if v == "" {
		return nil
	}
	u, err := url.Parse(v)
	if err != nil {
		p.fail(name, err)
	}
	return u
}
