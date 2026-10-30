// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

// Package server builds the root HTTP handler and runs the HTTP listeners.
package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"

	"sss/internal/api"
	"sss/internal/config"
	"sss/internal/fever"
	"sss/internal/googlereader"
	"sss/internal/http/request"
	"sss/internal/storage"
	"sss/internal/ui"
	"sss/internal/worker"
)

// NewHandler returns the root handler: health probes at the server root and
// the application (API, Fever, Google Reader, UI) under BasePath.
func NewHandler(store *storage.Storage, pool *worker.Pool) http.Handler {
	readiness := readinessProbe(store)

	app := http.NewServeMux()
	app.HandleFunc("GET /healthcheck", readiness)
	app.Handle("/fever/", fever.NewHandler(store))
	gr := googlereader.NewHandler(store)
	app.Handle("POST /accounts/ClientLogin", gr)
	app.Handle("/reader/api/0/", gr)
	if !config.Opts.DisableAPI {
		app.Handle("/v1/", api.NewHandler(store, pool))
	}
	app.Handle("/", ui.NewHandler(store, pool))

	var appHandler http.Handler = app
	if config.Opts.MaintenanceMode {
		appHandler = maintenance(app)
	}
	appHandler = recovery(middleware(appHandler))

	root := http.NewServeMux()
	// Probes ignore BasePath, as upstream.
	root.HandleFunc("/liveness", liveness)
	root.HandleFunc("/healthz", liveness)
	root.HandleFunc("/readiness", readiness)
	root.HandleFunc("/readyz", readiness)
	if bp := config.Opts.BasePath; bp != "" {
		root.Handle(bp+"/", http.StripPrefix(bp, appHandler))
	} else {
		root.Handle("/", appHandler)
	}
	return root
}

func liveness(w http.ResponseWriter, _ *http.Request) {
	w.Write([]byte("OK"))
}

func readinessProbe(store *storage.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if err := store.Ping(); err != nil {
			http.Error(w, fmt.Sprintf("Database Connection Error: %q", err), http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte("OK"))
	}
}

// maintenance answers every request except the health check with MAINTENANCE_MESSAGE.
func maintenance(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthcheck" {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(config.Opts.MaintenanceMessage))
	})
}

// middleware stores the client IP in the request context, logs the request and sets HSTS.
func middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		trusted := config.Opts.TrustedReverseProxyNetworks
		isTrustedProxy := request.IsTrustedIP(request.FindRemoteIP(r), trusted)
		clientIP := request.FindClientIP(r, isTrustedProxy, trusted)

		isHTTPS := config.Opts.HTTPS || (isTrustedProxy && r.Header.Get("X-Forwarded-Proto") == "https")

		start := time.Now()
		defer func() {
			slog.Debug("Incoming request",
				slog.String("client_ip", clientIP),
				slog.Group("request",
					slog.String("method", r.Method),
					slog.String("uri", r.RequestURI),
					slog.String("protocol", r.Proto),
					slog.Duration("execution_time", time.Since(start)),
				),
			)
		}()

		if isHTTPS && !config.Opts.DisableHSTS {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000")
		}

		ctx := context.WithValue(r.Context(), request.ClientIPContextKey, clientIP)
		ctx = context.WithValue(ctx, request.IsHTTPSContextKey, isHTTPS)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// recovery turns handler panics into 500 responses.
func recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				if v == http.ErrAbortHandler {
					panic(v)
				}
				slog.Error("Panic while serving request", slog.String("uri", r.RequestURI), slog.Any("panic", v))
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// Start listens on every LISTEN_ADDR (TCP address or Unix socket path), with
// TLS from CERT_FILE/KEY_FILE or Let's Encrypt for CERT_DOMAIN, and returns
// the running servers. Listen errors are returned; serve errors exit the process.
func Start(store *storage.Storage, pool *worker.Pool) ([]*http.Server, error) {
	cfg := config.Opts
	hasCertFiles := cfg.CertFile != "" && cfg.KeyFile != ""
	var servers []*http.Server

	var autocertTLS *tls.Config
	if cfg.CertDomain != "" {
		m := &autocert.Manager{
			Cache:      storage.NewCertificateCache(store),
			Prompt:     autocert.AcceptTOS,
			HostPolicy: autocert.HostWhitelist(cfg.CertDomain),
		}
		autocertTLS = &tls.Config{GetCertificate: m.GetCertificate, NextProtos: []string{"h2", "http/1.1", acme.ALPNProto}}
		challenge := newHTTPServer(":http", m.HTTPHandler(nil))
		ln, err := net.Listen("tcp", challenge.Addr)
		if err != nil {
			return nil, fmt.Errorf("server: ACME challenge listener: %w", err)
		}
		serve(challenge, ln, false)
		servers = append(servers, challenge)
	}

	var certTLS *tls.Config
	if hasCertFiles {
		cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
		if err != nil {
			return servers, fmt.Errorf("server: unable to load TLS certificate: %w", err)
		}
		certTLS = &tls.Config{Certificates: []tls.Certificate{cert}}
	}
	if certTLS != nil || autocertTLS != nil {
		cfg.HTTPS = true // set before any request is served
	}

	handler := NewHandler(store, pool)
	for i, addr := range cfg.ListenAddr {
		unix := strings.HasPrefix(addr, "/")
		srv := newHTTPServer(addr, handler)
		switch {
		case certTLS != nil:
			srv.TLSConfig = certTLS
		case !unix && autocertTLS != nil && (addr == ":https" || (i == 0 && strings.Contains(addr, ":"))):
			srv.TLSConfig = autocertTLS
		}
		ln, err := listen(addr, unix)
		if err != nil {
			return servers, err
		}
		serve(srv, ln, srv.TLSConfig != nil)
		servers = append(servers, srv)
	}
	return servers, nil
}

func newHTTPServer(addr string, handler http.Handler) *http.Server {
	timeout := config.Opts.HTTPServerTimeout
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadTimeout:       timeout,
		WriteTimeout:      timeout,
		IdleTimeout:       timeout,
		ReadHeaderTimeout: timeout,
	}
}

func listen(addr string, unix bool) (net.Listener, error) {
	if !unix {
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			return nil, fmt.Errorf("server: unable to listen on %s: %w", addr, err)
		}
		return ln, nil
	}
	if err := os.Remove(addr); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("server: unable to remove existing Unix socket %s: %w", addr, err)
	}
	ln, err := net.Listen("unix", addr)
	if err != nil {
		return nil, fmt.Errorf("server: unable to listen on Unix socket %s: %w", addr, err)
	}
	if err := os.Chmod(addr, 0o660); err != nil {
		ln.Close()
		return nil, fmt.Errorf("server: unable to change socket permission for %s: %w", addr, err)
	}
	return ln, nil
}

func serve(srv *http.Server, ln net.Listener, useTLS bool) {
	slog.Info("Starting HTTP server", slog.String("listen_address", srv.Addr), slog.Bool("tls", useTLS))
	go func() {
		var err error
		if useTLS {
			err = srv.ServeTLS(ln, "", "")
		} else {
			err = srv.Serve(ln)
		}
		if err != http.ErrServerClosed {
			slog.Error("HTTP server failed", slog.String("listen_address", srv.Addr), slog.Any("error", err))
			os.Exit(1)
		}
	}()
}
