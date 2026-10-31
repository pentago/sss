// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

// Package fetcher is the HTTP client used to download feeds, pages and icons.
package fetcher

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/net/http/httpproxy"
	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"

	"sss/internal/config"
	"sss/internal/model"
	"sss/internal/urllib"
)

const (
	defaultHTTPClientTimeout = 20 * time.Second
	defaultAcceptHeader      = "application/xml,application/atom+xml,application/rss+xml,application/rdf+xml,application/feed+json,text/html,*/*;q=0.9"
)

// ErrPrivateNetworkHost is returned when a request targets a private network address.
var ErrPrivateNetworkHost = errors.New("fetcher: refusing to access private network host")

// RequestBuilder configures and executes a single outgoing GET request.
type RequestBuilder struct {
	headers            http.Header
	cookie             string
	cookieSiteHost     string
	clientProxyURL     *url.URL
	clientTimeout      time.Duration
	useClientProxy     bool
	withoutRedirects   bool
	ignoreTLSErrors    bool
	disableHTTP2       bool
	disableCompression bool
	feedProxyURL       string
}

// NewRequestBuilder returns a builder with the configured HTTP client timeout
// (config.Opts.HTTPClientTimeout), or the fixed default when unset.
func NewRequestBuilder() *RequestBuilder {
	clientTimeout := defaultHTTPClientTimeout
	if config.Opts != nil && config.Opts.HTTPClientTimeout > 0 {
		clientTimeout = config.Opts.HTTPClientTimeout
	}
	return &RequestBuilder{headers: make(http.Header), clientTimeout: clientTimeout}
}

// Clone returns an independent copy of the builder.
func (r *RequestBuilder) Clone() *RequestBuilder {
	clone := *r
	clone.headers = r.headers.Clone()
	return &clone
}

// WithHeader sets a request header.
func (r *RequestBuilder) WithHeader(key, value string) *RequestBuilder {
	r.headers.Set(key, value)
	return r
}

// WithETag sets If-None-Match when etag is not empty.
func (r *RequestBuilder) WithETag(etag string) *RequestBuilder {
	if etag != "" {
		r.headers.Set("If-None-Match", etag)
	}
	return r
}

// WithLastModified sets If-Modified-Since when lastModified is not empty.
func (r *RequestBuilder) WithLastModified(lastModified string) *RequestBuilder {
	if lastModified != "" {
		r.headers.Set("If-Modified-Since", lastModified)
	}
	return r
}

// WithUserAgent sets the User-Agent, falling back to defaultUserAgent.
func (r *RequestBuilder) WithUserAgent(userAgent string, defaultUserAgent string) *RequestBuilder {
	if userAgent != "" {
		r.headers.Set("User-Agent", userAgent)
	} else {
		r.headers.Set("User-Agent", defaultUserAgent)
	}
	return r
}

// WithCookie sends cookie only to the website of siteURL (same host or registrable domain).
func (r *RequestBuilder) WithCookie(cookie, siteURL string) *RequestBuilder {
	if cookie != "" {
		r.cookie = cookie
		r.cookieSiteHost = ""
		if u, err := url.Parse(siteURL); err == nil {
			r.cookieSiteHost = u.Hostname()
		}
	}
	return r
}

// WithUsernameAndPassword sets HTTP basic auth when both values are set.
func (r *RequestBuilder) WithUsernameAndPassword(username, password string) *RequestBuilder {
	if username != "" && password != "" {
		r.headers.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(username+":"+password)))
	}
	return r
}

// WithCustomApplicationProxyURL sets the application proxy (HTTP_CLIENT_PROXY).
func (r *RequestBuilder) WithCustomApplicationProxyURL(proxyURL *url.URL) *RequestBuilder {
	r.clientProxyURL = proxyURL
	return r
}

// UseCustomApplicationProxyURL enables the application proxy.
func (r *RequestBuilder) UseCustomApplicationProxyURL(value bool) *RequestBuilder {
	r.useClientProxy = value
	return r
}

// WithCustomFeedProxyURL sets a per-feed proxy; it wins over the application proxy.
func (r *RequestBuilder) WithCustomFeedProxyURL(proxyURL string) *RequestBuilder {
	r.feedProxyURL = proxyURL
	return r
}

// WithTimeout sets the client timeout.
func (r *RequestBuilder) WithTimeout(timeout time.Duration) *RequestBuilder {
	r.clientTimeout = timeout
	return r
}

// WithoutRedirects disables following redirects.
func (r *RequestBuilder) WithoutRedirects() *RequestBuilder {
	r.withoutRedirects = true
	return r
}

// DisableHTTP2 disables HTTP/2 when value is true.
func (r *RequestBuilder) DisableHTTP2(value bool) *RequestBuilder {
	r.disableHTTP2 = value
	return r
}

// IgnoreTLSErrors accepts invalid certificates and insecure ciphers when value is true.
func (r *RequestBuilder) IgnoreTLSErrors(value bool) *RequestBuilder {
	r.ignoreTLSErrors = value
	return r
}

// WithoutCompression requests an uncompressed response.
func (r *RequestBuilder) WithoutCompression() *RequestBuilder {
	r.disableCompression = true
	return r
}

// ExecuteRequest performs a GET request to requestURL.
func (r *RequestBuilder) ExecuteRequest(requestURL string) (*http.Response, error) {
	var proxyURL *url.URL
	switch {
	case r.feedProxyURL != "":
		var err error
		if proxyURL, err = url.Parse(r.feedProxyURL); err != nil {
			return nil, fmt.Errorf("fetcher: invalid feed proxy URL %q: %w", r.feedProxyURL, err)
		}
	case r.useClientProxy:
		proxyURL = r.clientProxyURL
		if proxyURL == nil && config.Opts != nil {
			proxyURL = config.Opts.HTTPClientProxy
		}
	}

	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 15 * time.Second}
	proxyDialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 15 * time.Second}

	// httpproxy rather than http.ProxyFromEnvironment, which caches the environment.
	envProxyFunc := httpproxy.FromEnvironment().ProxyFunc()
	trusted := &trustedProxyAddresses{}
	// A per-feed proxy is user-controlled, so it is not a trusted hop.
	if r.feedProxyURL == "" {
		trusted.add(proxyURL)
	}

	transport := &http.Transport{
		Proxy: func(req *http.Request) (*url.URL, error) {
			u, err := envProxyFunc(req.URL)
			if err == nil {
				trusted.add(u)
			}
			return u, err
		},
		ForceAttemptHTTP2: true,
		MaxIdleConns:      50,
		IdleConnTimeout:   10 * time.Second,
		DialContext:       dialer.DialContext,
	}

	if config.Opts != nil && !config.Opts.FetcherAllowPrivateNetworks {
		// Checked after DNS resolution on the exact IP dialed (no DNS rebinding).
		dialer.Control = func(network, address string, c syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			if urllib.IsNonPublicIP(net.ParseIP(host)) {
				return fmt.Errorf("%w %q", ErrPrivateNetworkHost, host)
			}
			return nil
		}
		// Explicitly configured proxies are a trusted hop.
		transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			if trusted.contains(normalizeDialAddress(addr)) {
				return proxyDialer.DialContext(ctx, network, addr)
			}
			return dialer.DialContext(ctx, network, addr)
		}
	}

	if r.ignoreTLSErrors {
		ciphers := slices.Concat(tls.CipherSuites(), tls.InsecureCipherSuites())
		ids := make([]uint16, 0, len(ciphers))
		for _, c := range ciphers {
			ids = append(ids, c.ID)
		}
		transport.TLSClientConfig = &tls.Config{CipherSuites: ids, InsecureSkipVerify: true}
	}

	if r.disableHTTP2 {
		transport.ForceAttemptHTTP2 = false
		transport.TLSNextProto = map[string]func(string, *tls.Conn) http.RoundTripper{}
	}

	if proxyURL != nil {
		transport.Proxy = http.ProxyURL(proxyURL)
	}

	client := &http.Client{Timeout: r.clientTimeout, Transport: transport}
	if r.withoutRedirects {
		client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	}

	req, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header = r.headers.Clone()
	if r.cookie != "" && isSameSite(req.URL.Hostname(), r.cookieSiteHost) {
		req.Header.Set("Cookie", r.cookie)
	}
	// Without an explicit Accept-Encoding the transport negotiates and decodes gzip itself.
	if r.disableCompression {
		req.Header.Set("Accept-Encoding", "identity")
	}
	if req.Header.Get("Accept") == "" {
		req.Header.Set("Accept", defaultAcceptHeader)
	}
	req.Header.Set("Connection", "close")

	return client.Do(req)
}

// isSameSite reports whether host and siteHost are equal or share a registrable domain.
func isSameSite(host, siteHost string) bool {
	if host == "" || siteHost == "" {
		return false
	}
	if host == siteHost {
		return true
	}
	d1, err := registrableDomain(host)
	if err != nil {
		return false
	}
	d2, err := registrableDomain(siteHost)
	return err == nil && d1 == d2
}

func registrableDomain(host string) (string, error) {
	ascii, err := idna.Lookup.ToASCII(host)
	if err != nil {
		return "", err
	}
	return publicsuffix.EffectiveTLDPlusOne(ascii)
}

func normalizeDialAddress(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return ""
	}
	return net.JoinHostPort(strings.ToLower(host), port)
}

func normalizeProxyDialAddress(proxyURL *url.URL) string {
	if proxyURL == nil {
		return ""
	}
	port := proxyURL.Port()
	if port == "" {
		switch strings.ToLower(proxyURL.Scheme) {
		case "", "http":
			port = "80"
		case "https":
			port = "443"
		case "socks5", "socks5h":
			port = "1080"
		default:
			return ""
		}
	}
	return net.JoinHostPort(strings.ToLower(proxyURL.Hostname()), port)
}

// trustedProxyAddresses holds dial addresses of the proxies used for a request
// (redirects may pick a different environment proxy).
type trustedProxyAddresses struct {
	mu        sync.Mutex
	addresses []string
}

func (p *trustedProxyAddresses) add(proxyURL *url.URL) {
	address := normalizeProxyDialAddress(proxyURL)
	if address == "" {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if !slices.Contains(p.addresses, address) {
		p.addresses = append(p.addresses, address)
	}
}

func (p *trustedProxyAddresses) contains(address string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Contains(p.addresses, address)
}

// ForFeed returns a request builder with the feed's HTTP settings: user agent
// (falling back to defaultUserAgent), cookie, per-feed and application proxy,
// TLS and HTTP/2 options. The application proxy URL is resolved from
// config.Opts.HTTPClientProxy by ExecuteRequest.
func ForFeed(feed *model.Feed, defaultUserAgent string) *RequestBuilder {
	return NewRequestBuilder().
		WithUserAgent(feed.UserAgent, defaultUserAgent).
		WithCookie(feed.Cookie, feed.FeedURL).
		WithCustomFeedProxyURL(feed.ProxyURL).
		UseCustomApplicationProxyURL(feed.FetchViaProxy).
		IgnoreTLSErrors(feed.AllowSelfSignedCertificates).
		DisableHTTP2(feed.DisableHTTP2)
}
