// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"cmp"
	"net/http"
	"strings"
	"time"

	"sss/internal/http/response"
	"sss/internal/mediaproxy"
)

func appIconURL(name string) string {
	return route("/icon/%s/%s", assets["icons/"+name].checksum, name)
}

const assetCacheDuration = 365 * 24 * time.Hour

func (h *handler) showStylesheet(w http.ResponseWriter, r *http.Request) {
	h.serveAsset(w, r, "css/"+r.PathValue("filename"), "text/css; charset=utf-8")
}

func (h *handler) showJavascript(w http.ResponseWriter, r *http.Request) {
	h.serveAsset(w, r, "js/"+r.PathValue("filename"), "text/javascript; charset=utf-8")
}

// serveAsset serves an embedded file; its URL contains the content hash, so it is cached for a long time.
func (h *handler) serveAsset(w http.ResponseWriter, r *http.Request, name, contentType string) {
	a, ok := assets[name]
	if !ok {
		response.HTMLNotFound(w, r)
		return
	}
	response.NewBuilder(w, r).WithCaching(a.checksum, assetCacheDuration, func(b *response.Builder) {
		b.WithHeader("Content-Type", contentType).WithBodyAsBytes(a.data).Write()
	})
}

func (h *handler) showServiceWorker(w http.ResponseWriter, r *http.Request) {
	response.NewBuilder(w, r).
		WithHeader("Content-Type", "text/javascript; charset=utf-8").
		WithHeader("Cache-Control", "no-cache").
		WithBodyAsBytes(assets["service-worker.js"].data).
		Write()
}

func (h *handler) showCustomCSS(w http.ResponseWriter, r *http.Request) {
	var body string
	if user := currentUser(r); user != nil {
		body = user.Stylesheet
	}
	serveUserContent(w, r, body, "text/css; charset=utf-8")
}

func (h *handler) showCustomJS(w http.ResponseWriter, r *http.Request) {
	var body string
	if user := currentUser(r); user != nil {
		body = user.CustomJS
	}
	serveUserContent(w, r, body, "text/javascript; charset=utf-8")
}

// serveUserContent serves per-user CSS/JS with private caching and ETag revalidation.
func serveUserContent(w http.ResponseWriter, r *http.Request, body, contentType string) {
	etag := `"` + checksum([]byte(body)) + `"`
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "private, no-cache")
	w.Header().Set("Vary", "Cookie")
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	response.NewBuilder(w, r).WithHeader("Content-Type", contentType).WithBodyAsString(body).Write()
}

// showFavicon serves /favicon.ico and the /icon/{checksum}/{filename} app icons.
func (h *handler) showFavicon(w http.ResponseWriter, r *http.Request) {
	name := cmp.Or(r.PathValue("filename"), "favicon.ico")
	a, ok := assets["icons/"+name]
	if !ok {
		response.HTMLNotFound(w, r)
		return
	}
	response.NewBuilder(w, r).WithCaching(a.checksum, 7*24*time.Hour, func(b *response.Builder) {
		b.WithHeader("Content-Type", http.DetectContentType(a.data)).WithBodyAsBytes(a.data).Write()
	})
}

func (h *handler) showWebManifest(w http.ResponseWriter, r *http.Request) {
	type icon struct {
		Source  string `json:"src"`
		Sizes   string `json:"sizes"`
		Type    string `json:"type"`
		Purpose string `json:"purpose"`
	}
	type shortcut struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	}
	displayMode := "standalone"
	if user := currentUser(r); user != nil && user.DisplayMode != "" {
		displayMode = user.DisplayMode
	}
	response.JSON(w, r, map[string]any{
		"name":        "sss",
		"short_name":  "sss",
		"description": "Minimalist feed reader",
		"display":     displayMode,
		"start_url":   route("/"),
		"scope":       route("/"),
		"icons": []icon{
			{Source: appIconURL("android-chrome-192x192.png"), Sizes: "192x192", Type: "image/png", Purpose: "any"},
			{Source: appIconURL("android-chrome-512x512.png"), Sizes: "512x512", Type: "image/png", Purpose: "any"},
		},
		"share_target": map[string]any{
			"action":  route("/bookmarklet"),
			"method":  http.MethodGet,
			"enctype": "application/x-www-form-urlencoded",
			"params":  map[string]string{"url": "uri", "text": "text"},
		},
		"shortcuts": []shortcut{
			{Name: "Add feed", URL: route("/subscribe")},
			{Name: "Unread", URL: route("/unread")},
			{Name: "Starred", URL: route("/starred")},
			{Name: "History", URL: route("/history")},
			{Name: "Feeds", URL: route("/feeds")},
			{Name: "Categories", URL: route("/categories")},
			{Name: "Search", URL: route("/search")},
			{Name: "Settings", URL: route("/settings")},
		},
	})
}

func (h *handler) showOfflinePage(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, "offline", map[string]any{"title": "Offline"})
}

func (h *handler) showFeedIcon(w http.ResponseWriter, r *http.Request) {
	icon, err := h.store.IconByExternalID(r.PathValue("externalIconID"))
	if err != nil {
		response.HTMLServerError(w, r, err)
		return
	}
	if icon == nil {
		response.HTMLNotFound(w, r)
		return
	}
	response.NewBuilder(w, r).WithCaching(icon.Hash, 72*time.Hour, func(b *response.Builder) {
		b.WithHeader("Content-Security-Policy", response.ContentSecurityPolicyForUntrustedContent)
		b.WithHeader("Content-Type", icon.MimeType)
		b.WithBodyAsBytes(icon.Content)
		if !strings.HasPrefix(icon.MimeType, "image/svg") {
			b.WithoutCompression()
		}
		b.Write()
	})
}

func (h *handler) mediaProxy(w http.ResponseWriter, r *http.Request) {
	mediaproxy.Serve(w, r, r.PathValue("encodedDigest"), r.PathValue("encodedURL"))
}
