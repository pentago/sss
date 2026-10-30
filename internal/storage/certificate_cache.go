// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"context"
	"database/sql"
	"errors"

	"golang.org/x/crypto/acme/autocert"
)

var _ autocert.Cache = (*CertificateCache)(nil)

// CertificateCache is an autocert.Cache backed by the acme_cache table.
type CertificateCache struct {
	storage *Storage
}

// NewCertificateCache returns a CertificateCache using the given storage.
func NewCertificateCache(storage *Storage) *CertificateCache {
	return &CertificateCache{storage: storage}
}

// Get returns the cached data for key, or autocert.ErrCacheMiss.
func (c *CertificateCache) Get(ctx context.Context, key string) ([]byte, error) {
	var data []byte
	err := c.storage.db.QueryRowContext(ctx, `SELECT data FROM acme_cache WHERE key=$1`, key).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, autocert.ErrCacheMiss
	}
	return data, err
}

// Put stores data in the cache under key.
func (c *CertificateCache) Put(ctx context.Context, key string, data []byte) error {
	_, err := c.storage.db.ExecContext(ctx, `
		INSERT INTO acme_cache (key, data, updated_at) VALUES ($1, $2, unixepoch())
		ON CONFLICT (key) DO UPDATE SET data=excluded.data, updated_at=excluded.updated_at`, key, data)
	return err
}

// Delete removes key from the cache; a missing key is not an error.
func (c *CertificateCache) Delete(ctx context.Context, key string) error {
	_, err := c.storage.db.ExecContext(ctx, `DELETE FROM acme_cache WHERE key=$1`, key)
	return err
}
