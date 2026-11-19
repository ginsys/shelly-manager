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

// presented returns the non-empty credential values of the request: `Authorization: Bearer <key>` (the
// "Bearer " prefix is matched exactly) and `X-API-Key: <key>`. These are the only two channels.
func presented(r *http.Request) []string {
	var values []string
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		if v := strings.TrimPrefix(auth, "Bearer "); v != "" {
			values = append(values, v)
		}
	}
	if x := r.Header.Get("X-API-Key"); x != "" {
		values = append(values, x)
	}
	return values
}

// Presents reports whether the request carries any credential at all, right or wrong. It decides
// nothing about the key: a caller that must refuse a request with no credential, before it knows which
// key the route takes, asks this.
func Presents(r *http.Request) bool { return len(presented(r)) > 0 }

// Authorized reports whether the request carries one of keys, as `Authorization: Bearer <key>` or
// `X-API-Key: <key>`. The "Bearer " prefix is matched exactly. Unset (empty) keys are skipped.
func Authorized(r *http.Request, keys ...string) bool {
	// Test every presented value against every key: no early exit on the first match.
	ok := false
	for _, p := range presented(r) {
		for _, k := range keys {
			if Equal(p, k) {
				ok = true
			}
		}
	}
	return ok
}
