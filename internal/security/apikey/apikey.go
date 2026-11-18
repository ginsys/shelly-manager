// Package apikey compares API keys. Every guard that checks a configured key goes through it, so the
// compare is written once and does not leak how much of a guess was right.
package apikey

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"strings"
)

// Equal reports whether presented is the configured key. It compares SHA-256 digests in constant time,
// so neither the position of the first wrong byte nor a length difference shows in the timing. An unset
// key (empty) never matches, so an empty credential cannot pass a guard whose key is unset.
func Equal(presented, key string) bool {
	if key == "" {
		return false
	}
	p := sha256.Sum256([]byte(presented))
	k := sha256.Sum256([]byte(key))
	return subtle.ConstantTimeCompare(p[:], k[:]) == 1
}

// Authorized reports whether the request carries one of keys, as `Authorization: Bearer <key>` or
// `X-API-Key: <key>`. The "Bearer " prefix is matched exactly. Unset (empty) keys are skipped.
func Authorized(r *http.Request, keys ...string) bool {
	var presented []string
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		presented = append(presented, strings.TrimPrefix(auth, "Bearer "))
	}
	if x := r.Header.Get("X-API-Key"); x != "" {
		presented = append(presented, x)
	}
	// Test every presented value against every key: no early exit on the first match.
	ok := false
	for _, p := range presented {
		for _, k := range keys {
			if Equal(p, k) {
				ok = true
			}
		}
	}
	return ok
}
