// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestDefaultDatabasePathFollowsXDG(t *testing.T) {
	for _, tt := range []struct {
		environ []string
		want    string
	}{
		{[]string{"XDG_DATA_HOME=/xdg/data", "HOME=/home/u"}, "/xdg/data/sss/sss.db"},
		{[]string{"XDG_DATA_HOME=relative", "HOME=/home/u"}, "/home/u/.local/share/sss/sss.db"}, // XDG requires absolute paths.
		{[]string{"HOME=/home/u"}, "/home/u/.local/share/sss/sss.db"},
		{nil, "sss.db"},
		{[]string{"HOME=/home/u", "DATABASE_PATH=/srv/x.db"}, "/srv/x.db"},
	} {
		o, err := Parse(tt.environ)
		if err != nil {
			t.Fatal(err)
		}
		if o.DatabasePath != tt.want {
			t.Errorf("%v: got %q, want %q", tt.environ, o.DatabasePath, tt.want)
		}
	}
}

func TestLoadConfigFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("BASE_URL", "https://env.example.org")
	if err := os.MkdirAll(filepath.Join(dir, "sss"), 0o700); err != nil {
		t.Fatal(err)
	}
	conf := "# comment\n\nBASE_URL=https://file.example.org\nLISTEN_ADDR = 127.0.0.1:9000\nnot a setting\n"
	if err := os.WriteFile(filepath.Join(dir, "sss", "sss.conf"), []byte(conf), 0o600); err != nil {
		t.Fatal(err)
	}

	// The XDG default file is read, and environment variables override it.
	if err := Load(""); err != nil {
		t.Fatal(err)
	}
	if got := Opts.ListenAddr; !slices.Equal(got, []string{"127.0.0.1:9000"}) {
		t.Errorf("ListenAddr from file = %v", got)
	}
	if Opts.BaseURL != "https://env.example.org" {
		t.Errorf("BaseURL = %q, want the environment value", Opts.BaseURL)
	}

	// A missing explicit file is skipped (env-only containers): no file values, env still applies.
	if err := Load(filepath.Join(dir, "missing.conf")); err != nil {
		t.Fatalf("Load with a missing explicit file: %v", err)
	}
	if Opts.BaseURL != "https://env.example.org" || slices.Equal(Opts.ListenAddr, []string{"127.0.0.1:9000"}) {
		t.Errorf("missing file: BaseURL = %q, ListenAddr = %v; want the env value and no file values", Opts.BaseURL, Opts.ListenAddr)
	}

	// An existing but unreadable default file is an error, not silently ignored.
	if os.Geteuid() != 0 {
		if err := os.Chmod(filepath.Join(dir, "sss", "sss.conf"), 0); err != nil {
			t.Fatal(err)
		}
		if err := Load(""); err == nil {
			t.Error("Load with an unreadable default file succeeded")
		}
	}
}

func TestDefaults(t *testing.T) {
	o, err := Parse(nil)
	if err != nil {
		t.Fatal(err)
	}
	if o.BaseURL != "http://localhost" || o.RootURL != "http://localhost" || o.BasePath != "" {
		t.Errorf("base URL: %q %q %q", o.BaseURL, o.RootURL, o.BasePath)
	}
	if o.DatabasePath != "sss.db" || o.BatchSize != 100 || o.WorkerPoolSize != 16 {
		t.Errorf("defaults: %q %d %d", o.DatabasePath, o.BatchSize, o.WorkerPoolSize)
	}
	if o.CleanupArchiveReadInterval != 60*24*time.Hour || o.PollingFrequency != time.Hour || o.HTTPClientTimeout != 20*time.Second {
		t.Errorf("durations: %v %v %v", o.CleanupArchiveReadInterval, o.PollingFrequency, o.HTTPClientTimeout)
	}
	if o.HTTPClientMaxBodySize != 15*1024*1024 {
		t.Errorf("max body size: %d", o.HTTPClientMaxBodySize)
	}
	if !slices.Equal(o.ListenAddr, []string{"127.0.0.1:8080"}) || !slices.Equal(o.MediaProxyResourceTypes, []string{"image"}) {
		t.Errorf("lists: %v %v", o.ListenAddr, o.MediaProxyResourceTypes)
	}
	if len(o.MediaProxyPrivateKey) != 16 {
		t.Errorf("media proxy key length: %d", len(o.MediaProxyPrivateKey))
	}
	if o.YouTubeEmbedDomain != "www.youtube-nocookie.com" || !strings.HasPrefix(o.HTTPClientUserAgent, "Mozilla/5.0 (compatible; sss/") {
		t.Errorf("derived: %q %q", o.YouTubeEmbedDomain, o.HTTPClientUserAgent)
	}
	if o.HTTPClientProxy != nil || o.TrustedReverseProxyNetworks != nil {
		t.Errorf("unset values: %v %v", o.HTTPClientProxy, o.TrustedReverseProxyNetworks)
	}
}

func TestParseValues(t *testing.T) {
	o, err := Parse([]string{
		"BASE_URL=https://example.com/app/",
		"PORT=3000",
		"LISTEN_ADDR=0.0.0.0:80",
		"DISABLE_API=yes",
		"HTTPS=On",
		"LOG_LEVEL=debug",
		"CLEANUP_ARCHIVE_READ_DAYS=-1",
		"OAUTH2_ADDITIONAL_SCOPES= a, b ,a,,",
		"MEDIA_PROXY_PRIVATE_KEY=secret",
		"HTTP_CLIENT_PROXY=http://proxy:3128",
		"YOUTUBE_EMBED_URL_OVERRIDE=https://yt.example.org/embed/",
		"UNRELATED=x",
		"BATCH_SIZE=1",
		"BATCH_SIZE=7",
	})
	if err != nil {
		t.Fatal(err)
	}
	if o.BaseURL != "https://example.com/app" || o.RootURL != "https://example.com" || o.BasePath != "/app" {
		t.Errorf("base URL: %q %q %q", o.BaseURL, o.RootURL, o.BasePath)
	}
	if !slices.Equal(o.ListenAddr, []string{":3000"}) {
		t.Errorf("PORT must override LISTEN_ADDR: %v", o.ListenAddr)
	}
	if !o.DisableAPI || !o.HTTPS || o.LogLevel != "debug" || o.BatchSize != 7 {
		t.Errorf("values: %v %v %q %d", o.DisableAPI, o.HTTPS, o.LogLevel, o.BatchSize)
	}
	if o.CleanupArchiveReadInterval != -24*time.Hour {
		t.Errorf("negative days: %v", o.CleanupArchiveReadInterval)
	}
	if !slices.Equal(o.OAuth2AdditionalScopes, []string{"a", "b"}) {
		t.Errorf("list: %v", o.OAuth2AdditionalScopes)
	}
	if string(o.MediaProxyPrivateKey) != "secret" || o.HTTPClientProxy.Host != "proxy:3128" || o.YouTubeEmbedDomain != "yt.example.org" {
		t.Errorf("values: %q %v %q", o.MediaProxyPrivateKey, o.HTTPClientProxy, o.YouTubeEmbedDomain)
	}
}

func TestSecretFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pw")
	if err := os.WriteFile(path, []byte("  hunter2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// The admin username is not a secret: like upstream, the dump shows it even when read from a file.
	userPath := filepath.Join(t.TempDir(), "user")
	if err := os.WriteFile(userPath, []byte("bob\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	o, err := Parse([]string{"ADMIN_PASSWORD=ignored", "ADMIN_PASSWORD_FILE=" + path, "ADMIN_USERNAME_FILE=" + userPath})
	if err != nil {
		t.Fatal(err)
	}
	if o.AdminPassword != "hunter2" || o.AdminUsername != "bob" {
		t.Errorf("got %q, %q", o.AdminPassword, o.AdminUsername)
	}
	dump := o.Dump()
	if strings.Contains(dump, "hunter2") || !strings.Contains(dump, "ADMIN_PASSWORD=<redacted>\n") || !strings.Contains(dump, "ADMIN_PASSWORD_FILE="+path+"\n") || !strings.Contains(dump, "ADMIN_USERNAME=bob\n") {
		t.Errorf("dump:\n%s", dump)
	}

	empty := filepath.Join(t.TempDir(), "empty")
	os.WriteFile(empty, nil, 0o600)
	for _, file := range []string{empty, "/nonexistent/file"} {
		if _, err := Parse([]string{"OAUTH2_CLIENT_SECRET_FILE=" + file}); err == nil || !strings.Contains(err.Error(), "OAUTH2_CLIENT_SECRET_FILE") {
			t.Errorf("%s: expected error naming OAUTH2_CLIENT_SECRET_FILE, got %v", file, err)
		}
	}
}

func TestDumpRedactsSecrets(t *testing.T) {
	o, err := Parse([]string{"DATABASE_PATH=/data/db.sqlite", "OAUTH2_CLIENT_SECRET=s3cr3t", "LOG_LEVEL=error"})
	if err != nil {
		t.Fatal(err)
	}
	dump := o.Dump()
	for _, want := range []string{"DATABASE_PATH=/data/db.sqlite\n", "OAUTH2_CLIENT_SECRET=<redacted>\n", "LOG_LEVEL=error\n", "ADMIN_PASSWORD=\n", "BATCH_SIZE=100\n"} {
		if !strings.Contains(dump, want) {
			t.Errorf("dump missing %q:\n%s", want, dump)
		}
	}
	if strings.Contains(dump, "s3cr3t") {
		t.Errorf("dump leaks secret:\n%s", dump)
	}
}

func TestInvalidValues(t *testing.T) {
	for _, kv := range []string{
		"BASE_URL=example.com/app",
		"BATCH_SIZE=0",
		"BATCH_SIZE=abc",
		"POLLING_LIMIT_PER_HOST=-1",
		"FORCE_REFRESH_INTERVAL=0",
		"HTTPS=maybe",
		"LOG_FORMAT=xml",
		"LOG_LEVEL=trace",
		"MEDIA_PROXY_MODE=sometimes",
		"MEDIA_PROXY_RESOURCE_TYPES=image,pdf",
		"MEDIA_PROXY_RESOURCE_TYPES=,",
		"OAUTH2_PROVIDER=github",
		"POLLING_SCHEDULER=random",
		"PORT=0",
		"PORT=65536",
		"TRUSTED_REVERSE_PROXY_NETWORKS=10.0.0.1",
		"HTTP_CLIENT_PROXY=http://[::1",
	} {
		name, _, _ := strings.Cut(kv, "=")
		_, err := Parse([]string{kv})
		if err == nil || !strings.Contains(err.Error(), name) {
			t.Errorf("%s: expected error naming %s, got %v", kv, name, err)
		}
	}
}

func TestValidateCombinations(t *testing.T) {
	for _, env := range [][]string{
		{"OAUTH2_PROVIDER=oidc"},
		{"DISABLE_LOCAL_AUTH=1"},
		{"AUTH_PROXY_HEADER=X-User"},
		{"CERT_FILE=cert.pem"},
		{"CERT_DOMAIN=example.org", "CERT_FILE=c", "KEY_FILE=k"},
		{"SCHEDULER_ROUND_ROBIN_MIN_INTERVAL=100", "SCHEDULER_ROUND_ROBIN_MAX_INTERVAL=10"},
		{"SCHEDULER_ENTRY_FREQUENCY_MIN_INTERVAL=100", "SCHEDULER_ENTRY_FREQUENCY_MAX_INTERVAL=10"},
	} {
		if _, err := Parse(env); err == nil {
			t.Errorf("%v: expected error", env)
		}
	}
	for _, env := range [][]string{
		{"OAUTH2_PROVIDER=oidc", "OAUTH2_OIDC_DISCOVERY_ENDPOINT=https://id.example.org"},
		{"DISABLE_LOCAL_AUTH=1", "WEBAUTHN=1"},
		{"AUTH_PROXY_HEADER=X-User", "TRUSTED_REVERSE_PROXY_NETWORKS=10.0.0.0/8, fd00::/8"},
		{"CERT_FILE=c", "KEY_FILE=k"},
	} {
		if _, err := Parse(env); err != nil {
			t.Errorf("%v: unexpected error %v", env, err)
		}
	}
}
