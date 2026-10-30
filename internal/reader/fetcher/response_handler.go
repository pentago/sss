// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

package fetcher

import (
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// ResponseHandler inspects the result of ExecuteRequest.
type ResponseHandler struct {
	httpResponse *http.Response
	clientErr    error
}

// NewResponseHandler wraps a response and the client error returned with it.
func NewResponseHandler(httpResponse *http.Response, clientErr error) *ResponseHandler {
	return &ResponseHandler{httpResponse: httpResponse, clientErr: clientErr}
}

// EffectiveURL returns the final URL after redirects.
func (r *ResponseHandler) EffectiveURL() string {
	return r.httpResponse.Request.URL.String()
}

// ContentType returns the Content-Type header.
func (r *ResponseHandler) ContentType() string {
	return r.httpResponse.Header.Get("Content-Type")
}

// LastModified returns the Last-Modified header unless the response disables caching.
func (r *ResponseHandler) LastModified() string {
	if r.httpResponse.Header.Get("Expires") == "0" {
		return ""
	}
	return r.httpResponse.Header.Get("Last-Modified")
}

// ETag returns the ETag header unless the response disables caching.
func (r *ResponseHandler) ETag() string {
	if r.httpResponse.Header.Get("Expires") == "0" {
		return ""
	}
	return r.httpResponse.Header.Get("ETag")
}

// Expires returns the time until the Expires header, rounded up to the minute.
func (r *ResponseHandler) Expires() time.Duration {
	if v := r.httpResponse.Header.Get("Expires"); v != "" {
		if t, err := time.Parse(time.RFC1123, v); err == nil {
			return time.Until(t).Truncate(time.Minute) + time.Minute
		}
	}
	return 0
}

// CacheControlMaxAge returns the Cache-Control max-age directive.
func (r *ResponseHandler) CacheControlMaxAge() time.Duration {
	for directive := range strings.SplitSeq(r.httpResponse.Header.Get("Cache-Control"), ",") {
		if after, ok := strings.CutPrefix(strings.TrimSpace(directive), "max-age="); ok {
			if maxAge, err := strconv.Atoi(after); err == nil {
				return time.Duration(maxAge) * time.Second
			}
		}
	}
	return 0
}

// ParseRetryDelay returns the Retry-After delay (seconds or HTTP date).
func (r *ResponseHandler) ParseRetryDelay() time.Duration {
	v := r.httpResponse.Header.Get("Retry-After")
	if v == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(v); err == nil {
		return time.Duration(seconds) * time.Second
	}
	if t, err := time.Parse(time.RFC1123, v); err == nil {
		return time.Until(t).Truncate(time.Second)
	}
	return 0
}

// IsRateLimited reports a 429 response.
func (r *ResponseHandler) IsRateLimited() bool {
	return r.httpResponse != nil && r.httpResponse.StatusCode == http.StatusTooManyRequests
}

// IsModified reports whether the resource changed compared to the stored validators.
func (r *ResponseHandler) IsModified(lastEtagValue, lastModifiedValue string) bool {
	if r.httpResponse.StatusCode == http.StatusNotModified {
		return false
	}
	if r.ETag() != "" {
		return r.ETag() != lastEtagValue
	}
	if r.LastModified() != "" {
		return r.LastModified() != lastModifiedValue
	}
	return true
}

// IsRedirect reports a 301/302/303/307/308 response.
func (r *ResponseHandler) IsRedirect() bool {
	if r.httpResponse == nil {
		return false
	}
	switch r.httpResponse.StatusCode {
	case http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther, http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
		return true
	}
	return false
}

// Close closes the response body.
func (r *ResponseHandler) Close() {
	if r.httpResponse != nil && r.httpResponse.Body != nil {
		r.httpResponse.Body.Close()
	}
}

// Body returns the response body limited to maxBodySize bytes.
func (r *ResponseHandler) Body(maxBodySize int64) io.ReadCloser {
	return http.MaxBytesReader(nil, r.httpResponse.Body, maxBodySize)
}

// ReadBody reads the whole body, failing when empty or larger than maxBodySize.
func (r *ResponseHandler) ReadBody(maxBodySize int64) ([]byte, error) {
	buffer, err := io.ReadAll(r.Body(maxBodySize))
	if err != nil {
		if maxErr, ok := errors.AsType[*http.MaxBytesError](err); ok {
			return nil, fmt.Errorf("fetcher: response body too large: %d bytes", maxErr.Limit)
		}
		return nil, fmt.Errorf("fetcher: unable to read response body: %w", err)
	}
	if len(buffer) == 0 {
		return nil, errors.New("fetcher: empty response body")
	}
	return buffer, nil
}

// LocalizedError returns a plain-English error for client failures and error
// status codes, or nil.
func (r *ResponseHandler) LocalizedError() error {
	if r.clientErr != nil {
		switch {
		case isSSLError(r.clientErr):
			return fmt.Errorf("fetcher: TLS error: %w", r.clientErr)
		case os.IsTimeout(r.clientErr):
			return fmt.Errorf("fetcher: network timeout: %w", r.clientErr)
		case errors.Is(r.clientErr, io.EOF):
			return fmt.Errorf("fetcher: empty response from server: %w", r.clientErr)
		case isNetworkError(r.clientErr):
			return fmt.Errorf("fetcher: network error: %w", r.clientErr)
		default:
			return fmt.Errorf("fetcher: %w", r.clientErr)
		}
	}

	code := r.httpResponse.StatusCode
	if code == http.StatusForbidden &&
		strings.EqualFold(r.httpResponse.Header.Get("cf-mitigated"), "challenge") &&
		strings.HasPrefix(strings.ToLower(r.ContentType()), "text/html") {
		return fmt.Errorf("fetcher: blocked by Cloudflare challenge (%d status code)", code)
	}

	switch code {
	case http.StatusUnauthorized:
		return errors.New("fetcher: access unauthorized (401 status code)")
	case http.StatusForbidden:
		return errors.New("fetcher: access forbidden (403 status code)")
	case http.StatusTooManyRequests:
		return errors.New("fetcher: too many requests (429 status code)")
	case http.StatusNotFound, http.StatusGone:
		return fmt.Errorf("fetcher: resource not found (%d status code)", code)
	case http.StatusInternalServerError:
		return errors.New("fetcher: remote server error (500 status code)")
	case http.StatusBadGateway:
		return errors.New("fetcher: bad gateway (502 status code)")
	case http.StatusServiceUnavailable:
		return errors.New("fetcher: service unavailable (503 status code)")
	case http.StatusGatewayTimeout:
		return errors.New("fetcher: gateway timeout (504 status code)")
	}
	if code >= 400 {
		return fmt.Errorf("fetcher: unexpected status code (%d status code)", code)
	}
	// ContentLength is -1 when unknown.
	if code != http.StatusNotModified && r.httpResponse.ContentLength == 0 {
		return errors.New("fetcher: empty response body")
	}
	return nil
}

func isNetworkError(err error) bool {
	if _, ok := errors.AsType[*url.Error](err); ok {
		return true
	}
	_, ok := errors.AsType[*net.OpError](err)
	return ok
}

func isSSLError(err error) bool {
	if _, ok := errors.AsType[x509.UnknownAuthorityError](err); ok {
		return true
	}
	if _, ok := errors.AsType[x509.HostnameError](err); ok {
		return true
	}
	_, ok := errors.AsType[x509.InsecureAlgorithmError](err)
	return ok
}
