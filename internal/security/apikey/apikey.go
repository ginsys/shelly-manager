// Package apikey compares API keys. Every guard that checks a configured key goes through it, so the
// compare is written once and does not leak how much of a guess was right.
package apikey

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"strings"
)

// macKey is random for each process. Both sides of a compare are MACed with it, so the two values that
// reach subtle.ConstantTimeCompare always have the same length.
var macKey = func() []byte {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("apikey: no random bytes: " + err.Error())
	}
	return b
}()

func mac(s string) []byte {
	m := hmac.New(sha256.New, macKey)
	m.Write([]byte(s))
	return m.Sum(nil)
}

// Equal reports whether presented is the configured key. It compares fixed-length HMACs in constant time,
// so neither the position of the first wrong byte nor a length difference shows in the timing. The HMAC
// is a length equalizer, not password storage. An unset key (empty) never matches, so an empty
// credential cannot pass a guard whose key is unset.
func Equal(presented, key string) bool {
	if key == "" {
		return false
	}
	return subtle.ConstantTimeCompare(mac(presented), mac(key)) == 1
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
