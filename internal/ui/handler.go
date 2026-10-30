// Copyright The Miniflux Authors. Modified by The sss Authors.
// Derived from Miniflux (https://github.com/miniflux/v2).
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"path"
	"slices"
	"strings"
	"time"

	"sss/internal/config"
	"sss/internal/http/request"
	"sss/internal/http/response"
	"sss/internal/mediaproxy"
	"sss/internal/model"
	"sss/internal/reader/sanitizer"
	"sss/internal/storage"
	"sss/internal/timezone"
	"sss/internal/ui/static"
	"sss/internal/urllib"
	"sss/internal/worker"
)

//go:embed templates/*.html
var templateFS embed.FS

// handler holds the dependencies shared by every UI route.
type handler struct {
	store *storage.Storage
	pool  *worker.Pool
	pages map[string]*template.Template
}

// asset is an embedded static file with its content hash.
type asset struct {
	data     []byte
	checksum string
}

// assets maps "css/base.css", "js/shortcuts.js", "service-worker.js" to their content.
var assets = loadAssets()

func loadAssets() map[string]asset {
	m := make(map[string]asset)
	err := fs.WalkDir(static.FS, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := static.FS.ReadFile(name)
		if err != nil {
			return err
		}
		m[name] = asset{data: data, checksum: checksum(data)}
		return nil
	})
	if err != nil {
		panic(err)
	}
	return m
}

func checksum(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:8])
}

// route builds an application path: base path + fmt.Sprintf(format, args...).
func route(format string, args ...any) string {
	if len(args) > 0 {
		return config.Opts.BasePath + fmt.Sprintf(format, args...)
	}
	return config.Opts.BasePath + format
}

// parseTemplates builds one template set per page: layout.html and every
// *partials.html file plus the page file, which defines "content".
func parseTemplates() map[string]*template.Template {
	base := template.Must(template.New("layout.html").Funcs(templateFuncs).ParseFS(templateFS, "templates/layout.html", "templates/*partials.html"))
	files, _ := fs.Glob(templateFS, "templates/*.html")
	pages := make(map[string]*template.Template, len(files))
	for _, file := range files {
		name := strings.TrimSuffix(path.Base(file), ".html")
		if name == "layout" || strings.HasSuffix(name, "partials") {
			continue
		}
		pages[name] = template.Must(template.Must(base.Clone()).ParseFS(templateFS, file))
	}
	return pages
}

var templateFuncs = template.FuncMap{
	"route": route,
	"proxyURL": func(link string) string {
		mode := config.Opts.MediaProxyMode
		if mode == "all" || (mode != "none" && !urllib.IsHTTPS(link)) {
			return mediaproxy.ProxifyRelativeURL(link)
		}
		return link
	},
	// mediaURL returns the URL to use for an enclosure of the given media kind (image, audio, video).
	"mediaURL": func(link, kind string) template.URL {
		if slices.Contains(config.Opts.MediaProxyResourceTypes, kind) {
			mode := config.Opts.MediaProxyMode
			if mode == "all" || (mode != "none" && !urllib.IsHTTPS(link)) {
				return template.URL(mediaproxy.ProxifyRelativeURL(link))
			}
		}
		return untrustedURL(link)
	},
	// content returns sanitized entry HTML (sanitized at ingest) with media rewritten for the proxy.
	"content": func(html string) template.HTML {
		return template.HTML(mediaproxy.RewriteDocumentWithRelativeProxyURL(html))
	},
	// safeHTML marks already sanitized HTML as safe.
	"safeHTML":     func(html string) template.HTML { return template.HTML(html) },
	"untrustedURL": untrustedURL,
	// newTab takes any because the public share page passes a nil user.
	"newTab": func(user any) template.HTMLAttr {
		if u, ok := user.(*model.User); ok && u != nil && u.OpenExternalLinksInNewTab {
			return `target="_blank"`
		}
		return ""
	},
	"iconURL": func(icon *model.FeedIcon) string {
		if icon == nil || icon.ExternalIconID == "" {
			return ""
		}
		return route("/feed-icon/%s", icon.ExternalIconID)
	},
	"isodate": func(t time.Time) string { return t.UTC().Format(time.RFC3339) },
	"date": func(tz string, t time.Time) string {
		if t.IsZero() {
			return "never"
		}
		return timezone.Convert(tz, t).Format("Monday, January 2, 2006 15:04 MST")
	},
	"elapsed":  elapsed,
	"truncate": truncate,
	"plural":   plural,
	"domain":   urllib.Domain,
	"fileSize": fileSize,
	"query": func(pairs ...string) string {
		values := url.Values{}
		for i := 0; i+1 < len(pairs); i += 2 {
			if pairs[i+1] != "" {
				values.Set(pairs[i], pairs[i+1])
			}
		}
		if len(values) == 0 {
			return ""
		}
		return "?" + values.Encode()
	},
	"deref": func(i *int) int {
		if i == nil {
			return 0
		}
		return *i
	},
	"add":        func(a, b int) int { return a + b },
	"pathEscape": url.PathEscape,
	// bookmarklet is the javascript: link that opens the subscription page for the current page.
	"bookmarklet": func() template.URL {
		return template.URL("javascript:location.href='" + config.Opts.RootURL + route("/bookmarklet") + "?uri='+encodeURIComponent(window.location.href)")
	},
	"dict": func(pairs ...any) map[string]any {
		m := make(map[string]any, len(pairs)/2)
		for i := 0; i+1 < len(pairs); i += 2 {
			m[pairs[i].(string)] = pairs[i+1]
		}
		return m
	},
}

// untrustedURL lets feed-supplied URLs with any allowed scheme through, and
// turns unsafe ones (javascript:, data:) into an inert "#".
func untrustedURL(rawURL string) template.URL {
	if !sanitizer.HasValidURIScheme(rawURL) {
		return "#"
	}
	return template.URL(rawURL)
}

func truncate(s string, max int) string {
	n := 0
	for i := range s {
		if n == max {
			return s[:i] + "…"
		}
		n++
	}
	return s
}

// plural returns "1 entry" / "3 entries" style text.
func plural(n int, singular, pluralForm string) string {
	if n == 1 {
		return "1 " + singular
	}
	return fmt.Sprintf("%d %s", n, pluralForm)
}

func elapsed(tz string, t time.Time) string {
	if t.IsZero() {
		return "not yet"
	}
	now := timezone.Now(tz)
	t = timezone.Convert(tz, t)
	if now.Before(t) {
		return "not yet"
	}
	diff := now.Sub(t)
	days := int(diff.Hours() / 24)
	switch {
	case diff < time.Minute:
		return "just now"
	case diff < time.Hour:
		return plural(int(diff.Minutes()), "minute", "minutes") + " ago"
	case diff < 24*time.Hour:
		return plural(int(diff.Hours()), "hour", "hours") + " ago"
	case days == 1:
		return "yesterday"
	case days < 21:
		return plural(days, "day", "days") + " ago"
	case days < 31:
		return plural(int(math.Round(float64(days)/7)), "week", "weeks") + " ago"
	case days < 365:
		return plural(int(math.Round(float64(days)/30)), "month", "months") + " ago"
	default:
		return plural(int(math.Round(float64(days)/365)), "year", "years") + " ago"
	}
}

func fileSize(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	base := math.Log(float64(b)) / math.Log(unit)
	number := math.Pow(unit, base-math.Floor(base))
	return fmt.Sprintf("%.1f %ciB", number, "KMGTPE"[int64(base)-1])
}

// stylesheet is a <link rel="stylesheet"> in the layout.
type stylesheet struct {
	Href  string
	Media string
}

// pageAssets returns the stylesheets, color-scheme and scripts for user at now.
// user may be nil (login page), in which case sessionTheme is used.
func pageAssets(user *model.User, sessionTheme string, now time.Time) (styles []stylesheet, colorScheme string, scripts []string) {
	themeUser := user
	if themeUser == nil {
		themeUser = &model.User{Theme: sessionTheme}
	}
	scheme, serif := themeUser.ThemeAt(now)
	css := func(name string) string { return route("/stylesheets/%s/%s", assets["css/"+name].checksum, name) }
	switch scheme {
	case model.SchemeLight, model.SchemeDark:
		styles = []stylesheet{{Href: css("base.css")}, {Href: css(scheme + ".css")}}
		colorScheme = scheme
	case model.ThemeNone:
		colorScheme = "light dark"
	default: // system
		styles = []stylesheet{
			{Href: css("base.css")},
			{Href: css("light.css"), Media: "(prefers-color-scheme: light)"},
			{Href: css("dark.css"), Media: "(prefers-color-scheme: dark)"},
		}
		colorScheme = "light dark"
	}
	if styles != nil && serif {
		styles = append(styles, stylesheet{Href: css("serif.css")})
	}
	if user == nil {
		return styles, colorScheme, nil
	}
	if user.Stylesheet != "" {
		styles = append(styles, stylesheet{Href: route("/custom.css?%s", checksum([]byte(user.Stylesheet)))})
	}
	js := func(name string) string { return route("/js/%s/%s", assets["js/"+name].checksum, name) }
	if user.KeyboardShortcuts {
		scripts = append(scripts, js("shortcuts.js"))
	}
	if user.EntrySwipe || (user.GestureNav != "" && user.GestureNav != "none") {
		scripts = append(scripts, js("swipe.js"))
	}
	if user.MarkReadOnScroll {
		scripts = append(scripts, js("scroll.js"))
	}
	if user.ServiceWorker {
		scripts = append(scripts, js("sw-register.js"))
	}
	if user.CustomJS != "" {
		scripts = append(scripts, route("/custom.js?%s", checksum([]byte(user.CustomJS))))
	}
	return styles, colorScheme, scripts
}

// render executes the page template with data, filling the layout keys:
// user, countUnread, countErrorFeeds, hasSaveEntry, successMessage,
// errorMessage (unless set by the caller), stylesheets, colorScheme, scripts.
// data["webauthn"] = true adds webauthn.js when WEBAUTHN is enabled.
func (h *handler) render(w http.ResponseWriter, r *http.Request, page string, data map[string]any) {
	tpl, ok := h.pages[page]
	if !ok {
		response.HTMLServerError(w, r, fmt.Errorf("ui: unknown template %q", page))
		return
	}
	if data == nil {
		data = map[string]any{}
	}
	user := currentUser(r)
	data["user"] = user
	sessionTheme := model.ThemeDefault
	if session := request.WebSession(r); session != nil {
		sessionTheme = session.Theme()
		success, failure := session.ConsumeMessages()
		data["successMessage"] = success
		if data["errorMessage"] == nil || data["errorMessage"] == "" {
			data["errorMessage"] = failure
		}
	}
	styles, colorScheme, scripts := pageAssets(user, sessionTheme, time.Now())
	if webauthn, _ := data["webauthn"].(bool); webauthn && config.Opts.WebAuthn {
		scripts = append(scripts, route("/js/%s/webauthn.js", assets["js/webauthn.js"].checksum))
	}
	data["stylesheets"], data["colorScheme"], data["scripts"] = styles, colorScheme, scripts
	if user != nil {
		nav, err := h.store.GetNavMetadata(user.ID)
		if err != nil {
			response.HTMLServerError(w, r, err)
			return
		}
		data["countUnread"], data["countErrorFeeds"], data["hasSaveEntry"] = nav.CountUnread, nav.CountErrorFeeds, nav.HasSaveEntry
	}

	var buf bytes.Buffer
	if err := tpl.ExecuteTemplate(&buf, "layout.html", data); err != nil {
		slog.Error("Unable to render template", slog.String("page", page), slog.Any("error", err))
		response.HTMLServerError(w, r, err)
		return
	}
	response.HTML(w, r, buf.Bytes())
}

// redirectBack sends a 303 to the same-origin Referer path, or to fallback
// (an unprefixed route path) when the Referer is missing or foreign.
func (h *handler) redirectBack(w http.ResponseWriter, r *http.Request, fallback string) {
	response.HTMLRedirect(w, r, backURL(r, fallback))
}

func backURL(r *http.Request, fallback string) string {
	if ref, err := url.Parse(r.Referer()); err == nil && ref.Host == r.Host && ref.Host != "" &&
		(ref.Scheme == "http" || ref.Scheme == "https") &&
		strings.HasPrefix(ref.Path, "/") && !strings.HasPrefix(ref.Path, "//") && !strings.Contains(ref.Path, `\`) {
		return (&url.URL{Path: ref.Path, RawQuery: ref.RawQuery}).String()
	}
	return config.Opts.BasePath + fallback
}
