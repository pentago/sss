// Copyright The Miniflux Authors. Modified by The sss Authors.
// Derived from Miniflux (https://github.com/miniflux/v2).
// SPDX-License-Identifier: Apache-2.0

package mediaproxy

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"time"

	"sss/internal/config"
	"sss/internal/crypto"
	"sss/internal/reader/fetcher"
	"sss/internal/reader/rewrite"
)

const untrustedContentCSP = `default-src 'none'; form-action 'none'; sandbox;`

// Serve verifies the signed media URL from the /proxy/{encodedDigest}/{encodedURL} route and streams the remote resource.
func Serve(w http.ResponseWriter, r *http.Request, encodedDigest, encodedURL string) {
	// If we receive a "If-None-Match" header, we assume the media is already stored in browser cache.
	if r.Header.Get("If-None-Match") != "" {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	decodedDigest, err := base64.URLEncoding.DecodeString(encodedDigest)
	if err != nil {
		http.Error(w, "unable to decode this digest", http.StatusBadRequest)
		return
	}

	decodedURL, err := base64.URLEncoding.DecodeString(encodedURL)
	if err != nil || len(decodedURL) == 0 {
		http.Error(w, "unable to decode this URL", http.StatusBadRequest)
		return
	}

	mac := hmac.New(sha256.New, config.Opts.MediaProxyPrivateKey)
	mac.Write(decodedURL)
	if !hmac.Equal(decodedDigest, mac.Sum(nil)) {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}

	mediaURL := string(decodedURL)
	parsedMediaURL, err := url.Parse(mediaURL)
	if err != nil || (parsedMediaURL.Scheme != "http" && parsedMediaURL.Scheme != "https") || parsedMediaURL.Host == "" {
		http.Error(w, "invalid URL provided", http.StatusBadRequest)
		return
	}

	requestBuilder := fetcher.NewRequestBuilder().
		WithTimeout(config.Opts.MediaProxyHTTPClientTimeout).
		WithoutCompression() // Compression is not implemented for media proxy requests.

	if referer := rewrite.GetRefererForURL(mediaURL); referer != "" {
		requestBuilder = requestBuilder.WithHeader("Referer", referer)
	}

	for _, name := range [...]string{"Range", "Accept", "Accept-Encoding", "User-Agent"} {
		if value := r.Header.Get(name); value != "" {
			requestBuilder = requestBuilder.WithHeader(name, value)
		}
	}

	resp, err := requestBuilder.ExecuteRequest(mediaURL)
	if err != nil {
		if errors.Is(err, fetcher.ErrPrivateNetworkHost) {
			slog.Warn("MediaProxy: Refused remote resource", slog.String("media_url", mediaURL), slog.Any("error", err))
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			return
		}
		slog.Error("MediaProxy: Unable to fetch remote resource", slog.String("media_url", mediaURL), slog.Any("error", err))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusRequestedRangeNotSatisfiable {
		if contentRange := resp.Header.Get("Content-Range"); contentRange != "" {
			w.Header().Set("Content-Range", contentRange)
		}
		http.Error(w, http.StatusText(resp.StatusCode), resp.StatusCode)
		return
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		slog.Warn("MediaProxy: Unexpected response status code", slog.String("media_url", mediaURL), slog.Int("status_code", resp.StatusCode))
		// Forward the status code from the origin.
		http.Error(w, "Origin status code is "+strconv.Itoa(resp.StatusCode), resp.StatusCode)
		return
	}

	const cacheDuration = 72 * time.Hour
	h := w.Header()
	h.Set("ETag", `"`+crypto.HashFromBytes(decodedURL)+`"`)
	h.Set("Cache-Control", "public, max-age="+strconv.Itoa(int(cacheDuration.Seconds()))+", immutable")
	h.Set("Expires", time.Now().Add(cacheDuration).UTC().Format(http.TimeFormat))
	h.Set("Content-Security-Policy", untrustedContentCSP)
	h.Set("X-Content-Type-Options", "nosniff")
	if filename := path.Base(parsedMediaURL.Path); filename != "" && filename != "." && filename != "/" {
		if disposition := mime.FormatMediaType("inline", map[string]string{"filename": filename}); disposition != "" {
			h.Set("Content-Disposition", disposition)
		}
	}
	for _, name := range [...]string{"Content-Encoding", "Content-Type", "Content-Length", "Accept-Ranges", "Content-Range"} {
		if value := resp.Header.Get(name); value != "" {
			h.Set(name, value)
		}
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}
