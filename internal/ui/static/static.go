// Package static embeds the built-in stylesheets, opt-in scripts and the
// service worker served by the web UI.
package static

import "embed"

// FS holds css/{base,light,dark}.css, js/*.js and service-worker.js.
//
//go:embed css js service-worker.js
var FS embed.FS
