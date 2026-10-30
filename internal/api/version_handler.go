// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"net/http"
	"runtime"

	"sss/internal/http/response"
	"sss/internal/version"
)

func (h *handler) versionHandler(w http.ResponseWriter, r *http.Request) {
	response.JSON(w, r, &versionResponse{
		Version:   version.Version,
		Commit:    version.Commit,
		BuildDate: version.BuildDate,
		GoVersion: runtime.Version(),
		Compiler:  runtime.Compiler,
		Arch:      runtime.GOARCH,
		OS:        runtime.GOOS,
	})
}
