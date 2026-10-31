// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

package response

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
)

const jsonContentTypeHeader = `application/json`

// JSON creates a new JSON response with a 200 status code.
func JSON(w http.ResponseWriter, r *http.Request, body any) {
	jsonStatus(w, r, http.StatusOK, body)
}

// JSONCreated sends a created response to the client.
func JSONCreated(w http.ResponseWriter, r *http.Request, body any) {
	jsonStatus(w, r, http.StatusCreated, body)
}

// JSONAccepted sends an accepted response to the client.
func JSONAccepted(w http.ResponseWriter, r *http.Request) {
	NewBuilder(w, r).
		WithStatus(http.StatusAccepted).
		WithHeader("Content-Type", jsonContentTypeHeader).
		Write()
}

// JSONServerError sends an internal error to the client.
func JSONServerError(w http.ResponseWriter, r *http.Request, err error) {
	writeError(w, r, slog.LevelError, http.StatusInternalServerError, err,
		string(generateJSONError(err)), "Content-Type", jsonContentTypeHeader)
}

// JSONBadRequest sends a bad request error to the client.
func JSONBadRequest(w http.ResponseWriter, r *http.Request, err error) {
	writeError(w, r, slog.LevelWarn, http.StatusBadRequest, err,
		string(generateJSONError(err)), "Content-Type", jsonContentTypeHeader)
}

// JSONUnauthorized sends a not authorized error to the client.
func JSONUnauthorized(w http.ResponseWriter, r *http.Request) {
	writeError(w, r, slog.LevelWarn, http.StatusUnauthorized, nil,
		string(generateJSONError(errors.New("access unauthorized"))), "Content-Type", jsonContentTypeHeader)
}

// JSONForbidden sends a forbidden error to the client.
func JSONForbidden(w http.ResponseWriter, r *http.Request) {
	writeError(w, r, slog.LevelWarn, http.StatusForbidden, nil,
		string(generateJSONError(errors.New("access forbidden"))), "Content-Type", jsonContentTypeHeader)
}

// JSONNotFound sends a page not found error to the client.
func JSONNotFound(w http.ResponseWriter, r *http.Request) {
	writeError(w, r, slog.LevelWarn, http.StatusNotFound, nil,
		string(generateJSONError(errors.New("resource not found"))), "Content-Type", jsonContentTypeHeader)
}

func jsonStatus(w http.ResponseWriter, r *http.Request, status int, body any) {
	responseBody, err := json.Marshal(body)
	if err != nil {
		JSONServerError(w, r, err)
		return
	}

	NewBuilder(w, r).
		WithStatus(status).
		WithHeader("Content-Type", jsonContentTypeHeader).
		WithBodyAsBytes(responseBody).
		Write()
}

func generateJSONError(err error) []byte {
	type errorMsg struct {
		ErrorMessage string `json:"error_message"`
	}

	encodedBody, _ := json.Marshal(errorMsg{ErrorMessage: err.Error()})
	return encodedBody
}
