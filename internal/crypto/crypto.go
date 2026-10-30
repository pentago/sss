// Copyright The Miniflux Authors. Modified by The sss Authors.
// Derived from Miniflux (https://github.com/miniflux/v2).
// SPDX-License-Identifier: Apache-2.0

package crypto

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"hash/fnv"

	"golang.org/x/crypto/bcrypt"
)

// HashFromBytes returns a non-cryptographic checksum of the input.
func HashFromBytes(value []byte) string {
	h := fnv.New128a()
	h.Write(value)
	return hex.EncodeToString(h.Sum(nil))
}

// SHA256 returns a SHA-256 checksum of a string.
func SHA256(value string) string {
	h := sha256.Sum256([]byte(value))
	return hex.EncodeToString(h[:])
}

// GenerateRandomBytes returns random bytes.
func GenerateRandomBytes(size int) []byte {
	b := make([]byte, size)
	rand.Read(b)
	return b
}

// GenerateRandomStringHex returns a random hexadecimal string.
func GenerateRandomStringHex(size int) string {
	return hex.EncodeToString(GenerateRandomBytes(size))
}

// HashPassword returns the bcrypt hash of a password.
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// GenerateSHA256Hmac returns the hex HMAC-SHA256 of data keyed by secret.
func GenerateSHA256Hmac(secret string, data []byte) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

// ConstantTimeCmp reports whether a and b are equal in constant time.
func ConstantTimeCmp(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
