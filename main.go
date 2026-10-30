// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

// Command sss is a minimalist, self-hosted feed reader.
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"
	_ "time/tzdata"

	"golang.org/x/term"

	"sss/internal/config"
	"sss/internal/database"
	"sss/internal/model"
	"sss/internal/reader/opml"
	"sss/internal/server"
	"sss/internal/storage"
	"sss/internal/systemd"
	"sss/internal/validator"
	"sss/internal/version"
	"sss/internal/worker"
)

func main() {
	var (
		flagInfo, flagVersion, flagMigrate, flagFlushSessions, flagCreateAdmin bool
		flagResetPassword, flagResetFeedErrors, flagResetNextCheckAt           bool
		flagDebug, flagConfigDump, flagRefreshFeeds, flagRunCleanupTasks       bool
		flagHealthCheck, flagExportUserFeeds, flagConfigFile                   string
	)
	flag.StringVar(&flagConfigFile, "config-file", "", "Load configuration file (default $XDG_CONFIG_HOME/sss/sss.conf if it exists)")
	flag.StringVar(&flagConfigFile, "c", "", "Load configuration file (default $XDG_CONFIG_HOME/sss/sss.conf if it exists)")
	flag.BoolVar(&flagInfo, "info", false, "Show build information")
	flag.BoolVar(&flagInfo, "i", false, "Show build information")
	flag.BoolVar(&flagVersion, "version", false, "Show application version")
	flag.BoolVar(&flagVersion, "v", false, "Show application version")
	flag.BoolVar(&flagMigrate, "migrate", false, "Run SQL migrations")
	flag.BoolVar(&flagFlushSessions, "flush-sessions", false, "Flush all sessions (disconnect users)")
	flag.BoolVar(&flagCreateAdmin, "create-admin", false, "Create an admin user from an interactive terminal")
	flag.BoolVar(&flagResetPassword, "reset-password", false, "Reset user password")
	flag.BoolVar(&flagResetFeedErrors, "reset-feed-errors", false, "Clear all feed errors for all users")
	flag.BoolVar(&flagResetNextCheckAt, "reset-feed-next-check-at", false, "Reset the next check time for all feeds")
	flag.BoolVar(&flagDebug, "debug", false, "Show debug logs")
	flag.BoolVar(&flagConfigDump, "config-dump", false, "Print parsed configuration values")
	flag.StringVar(&flagHealthCheck, "healthcheck", "", `Perform a health check on the given endpoint (the value "auto" tries to guess the health check endpoint)`)
	flag.BoolVar(&flagRefreshFeeds, "refresh-feeds", false, "Refresh a batch of feeds and exit")
	flag.BoolVar(&flagRunCleanupTasks, "run-cleanup-tasks", false, "Run cleanup tasks (delete old sessions and archive old entries)")
	flag.StringVar(&flagExportUserFeeds, "export-user-feeds", "", "Export user feeds (provide the username as argument)")
	flag.Parse()

	if err := config.Load(flagConfigFile); err != nil {
		fatal(err)
	}
	cfg := config.Opts

	switch {
	case flagConfigDump:
		fmt.Print(cfg.Dump())
		return
	case flagInfo:
		fmt.Println("Version:", version.Version)
		fmt.Println("Commit:", version.Commit)
		fmt.Println("Build Date:", version.BuildDate)
		fmt.Println("Go Version:", runtime.Version())
		fmt.Println("Compiler:", runtime.Compiler)
		fmt.Println("Arch:", runtime.GOARCH)
		fmt.Println("OS:", runtime.GOOS)
		return
	case flagVersion:
		fmt.Println(version.Version)
		return
	}

	if flagDebug {
		cfg.LogLevel = "debug"
	}
	if err := setupLogger(); err != nil {
		fatal(err)
	}

	if flagHealthCheck != "" {
		healthCheck(flagHealthCheck)
		return
	}

	db, err := database.Open(cfg.DatabaseURL)
	if err != nil {
		fatal(fmt.Errorf("unable to open database: %w", err))
	}
	defer db.Close()
	store := storage.New(db)
	if err := store.Ping(); err != nil {
		fatal(err)
	}

	switch {
	case flagMigrate:
		check(database.Migrate(db))
		return
	case flagResetFeedErrors:
		check(store.ResetFeedErrors())
		return
	case flagResetNextCheckAt:
		check(store.ResetNextCheckAt())
		return
	case flagExportUserFeeds != "":
		exportUserFeeds(store, flagExportUserFeeds)
		return
	case flagFlushSessions:
		fmt.Println("Flushing all sessions (disconnect users)")
		check(store.FlushAllSessions())
		return
	case flagCreateAdmin:
		username, password := askCredentials()
		createAdmin(store, username, password)
		return
	case flagResetPassword:
		username, password := askCredentials()
		resetPassword(store, username, password)
		return
	}

	if cfg.RunMigrations {
		check(database.Migrate(db))
	}
	upToDate, err := database.IsSchemaUpToDate(db)
	check(err)
	if !upToDate {
		fatal(errors.New("the database schema is not up to date: run sss -migrate or set RUN_MIGRATIONS=1"))
	}

	if cfg.CreateAdmin {
		createAdmin(store, cfg.AdminUsername, cfg.AdminPassword)
	}

	switch {
	case flagRefreshFeeds:
		start := time.Now()
		pool := worker.NewPool(store, cfg.WorkerPoolSize)
		worker.RefreshBatch(store, pool)
		pool.Shutdown()
		slog.Info("Refreshed a batch of feeds", slog.Duration("duration", time.Since(start)))
		return
	case flagRunCleanupTasks:
		worker.RunCleanupTasks(store)
		return
	}

	runDaemon(store)
}

func runDaemon(store *storage.Storage) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool := worker.NewPool(store, config.Opts.WorkerPoolSize)
	if !config.Opts.DisableSchedulerService && !config.Opts.MaintenanceMode {
		worker.StartScheduler(store, pool)
	}

	var servers []*http.Server
	if !config.Opts.DisableHTTPService {
		var err error
		if servers, err = server.Start(store, pool); err != nil {
			fatal(err)
		}
	}

	if systemd.HasNotifySocket() {
		if err := systemd.SdNotify(systemd.SdNotifyReady); err != nil {
			slog.Error("Unable to send readiness notification to systemd", slog.Any("error", err))
		}
		if config.Opts.Watchdog && systemd.HasSystemdWatchdog() {
			go runWatchdog(ctx, store)
		}
	}

	<-ctx.Done()
	slog.Info("Shutting down")
	if err := systemd.SdNotify(systemd.SdNotifyStopping); err != nil {
		slog.Error("Unable to send stopping notification to systemd", slog.Any("error", err))
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, srv := range servers {
		if err := srv.Shutdown(shutdownCtx); err != nil {
			slog.Error("HTTP server shutdown error", slog.String("addr", srv.Addr), slog.Any("error", err))
		}
	}
	pool.Shutdown()
}

// runWatchdog pings the systemd watchdog every third of its interval while the database responds.
func runWatchdog(ctx context.Context, store *storage.Storage) {
	interval, err := systemd.WatchdogInterval()
	if err != nil {
		slog.Error("Unable to get watchdog interval from systemd", slog.Any("error", err))
		return
	}
	ticker := time.NewTicker(interval / 3)
	defer ticker.Stop()
	for {
		if err := store.Ping(); err != nil {
			slog.Error("Unable to ping database", slog.Any("error", err))
		} else if err := systemd.SdNotify(systemd.SdNotifyWatchdog); err != nil {
			slog.Error("Unable to send watchdog notification to systemd", slog.Any("error", err))
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func setupLogger() error {
	cfg := config.Opts
	var out io.Writer
	switch cfg.LogFile {
	case "stdout":
		out = os.Stdout
	case "stderr":
		out = os.Stderr
	default:
		f, err := os.OpenFile(cfg.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			return fmt.Errorf("unable to open log file: %w", err)
		}
		out = f // closed by process exit
	}

	opts := &slog.HandlerOptions{Level: map[string]slog.Level{
		"debug": slog.LevelDebug, "info": slog.LevelInfo, "warning": slog.LevelWarn, "error": slog.LevelError,
	}[cfg.LogLevel]}
	if !cfg.LogDateTime {
		opts.ReplaceAttr = func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.Attr{}
			}
			return a
		}
	}
	if cfg.LogFormat == "json" {
		slog.SetDefault(slog.New(slog.NewJSONHandler(out, opts)))
	} else {
		slog.SetDefault(slog.New(slog.NewTextHandler(out, opts)))
	}
	return nil
}

func healthCheck(endpoint string) {
	if endpoint == "auto" {
		endpoint = "http://" + config.Opts.ListenAddr[0] + config.Opts.BasePath + "/healthcheck"
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(endpoint)
	if err != nil {
		fatal(fmt.Errorf("health check failure: %w", err))
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fatal(fmt.Errorf("health check failed with status code %d", resp.StatusCode))
	}
}

func exportUserFeeds(store *storage.Storage, username string) {
	user, err := store.UserByUsername(username)
	check(err)
	if user == nil {
		fatal(fmt.Errorf("user %q not found", username))
	}
	feeds, err := store.Feeds(user.ID)
	check(err)
	out, err := opml.Serialize(feeds)
	check(err)
	fmt.Println(out)
}

func createAdmin(store *storage.Storage, username, password string) {
	req := &model.UserCreationRequest{Username: username, Password: password, IsAdmin: true}
	if store.UserExists(username) {
		slog.Info("Skipping admin user creation because it already exists", slog.String("username", username))
		return
	}
	check(validator.ValidateUserCreationWithPassword(store, req))
	user, err := store.CreateUser(req)
	check(err)
	slog.Info("Created new admin user", slog.String("username", user.Username), slog.Int64("user_id", user.ID))
}

func resetPassword(store *storage.Storage, username, password string) {
	user, err := store.UserByUsername(username)
	check(err)
	if user == nil {
		fatal(errors.New("user not found"))
	}
	check(validator.ValidateUserModification(store, user.ID, &model.UserModificationRequest{Password: &password}))
	user.Password = password
	check(store.UpdateUser(user))
	check(store.RemoveUserWebSessionsExcept(user.ID, ""))
	fmt.Println("Password changed!")
}

func askCredentials() (string, string) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		fatal(errors.New("this is not an interactive terminal, exiting"))
	}
	fmt.Print("Enter Username: ")
	username, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		fatal(fmt.Errorf("unable to read username: %w", err))
	}
	fmt.Print("Enter Password: ")
	password, err := term.ReadPassword(fd)
	fmt.Println()
	if err != nil {
		fatal(fmt.Errorf("unable to read password: %w", err))
	}
	return strings.TrimSpace(username), strings.TrimSpace(string(password))
}

func check(err error) {
	if err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
