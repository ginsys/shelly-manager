package apikey

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEqual(t *testing.T) {
	cases := []struct {
		name, presented, key string
		want                 bool
	}{
		{"same key", "s3cret", "s3cret", true},
		{"wrong key", "s3cret!", "s3cret", false},
		{"shorter key", "s3cre", "s3cret", false},
		{"longer key", "s3cret-and-more", "s3cret", false},
		{"different case", "S3CRET", "s3cret", false},
		{"empty presented", "", "s3cret", false},
		// An unset key must never match: an empty credential would otherwise pass an unset guard.
		{"empty configured, empty presented", "", "", false},
		{"empty configured, something presented", "x", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, Equal(c.presented, c.key))
		})
	}
}

func request(header, value string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/x", nil)
	if header != "" {
		r.Header.Set(header, value)
	}
	return r
}

func TestAuthorized(t *testing.T) {
	cases := []struct {
		name string
		r    *http.Request
		keys []string
		want bool
	}{
		{"bearer", request("Authorization", "Bearer admin"), []string{"admin"}, true},
		{"x-api-key", request("X-API-Key", "admin"), []string{"admin"}, true},
		{"no credential", request("", ""), []string{"admin"}, false},
		{"wrong bearer", request("Authorization", "Bearer nope"), []string{"admin"}, false},
		{"wrong x-api-key", request("X-API-Key", "nope"), []string{"admin"}, false},
		{"second key matches", request("Authorization", "Bearer agent"), []string{"admin", "agent"}, true},
		{"first key matches", request("X-API-Key", "admin"), []string{"admin", "agent"}, true},
		{"no key matches", request("Authorization", "Bearer other"), []string{"admin", "agent"}, false},
		// The prefix is matched exactly, as the guards did before this package: "bearer " is not "Bearer ".
		{"lowercase bearer prefix", request("Authorization", "bearer admin"), []string{"admin"}, false},
		{"bare token, no prefix", request("Authorization", "admin"), []string{"admin"}, false},
		{"empty bearer", request("Authorization", "Bearer "), []string{"admin"}, false},
		// An unset key is skipped, so an empty credential matches nothing.
		{"empty bearer, unset second key", request("Authorization", "Bearer "), []string{"admin", ""}, false},
		{"unset key only", request("Authorization", "Bearer x"), []string{""}, false},
		{"no keys", request("Authorization", "Bearer x"), nil, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, Authorized(c.r, c.keys...))
		})
	}
}

// Presents answers "is any credential there", right or wrong, over the same two channels Authorized reads.
func TestPresents(t *testing.T) {
	both := request("Authorization", "Bearer a")
	both.Header.Set("X-API-Key", "b")
	cases := []struct {
		name string
		r    *http.Request
		want bool
	}{
		{"bearer", request("Authorization", "Bearer anything"), true},
		{"x-api-key", request("X-API-Key", "anything"), true},
		{"both", both, true},
		{"no credential", request("", ""), false},
		{"lowercase bearer prefix", request("Authorization", "bearer admin"), false},
		{"basic", request("Authorization", "Basic dXNlcjpwYXNz"), false},
		{"empty bearer", request("Authorization", "Bearer "), false},
		{"empty x-api-key", request("X-API-Key", ""), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, Presents(c.r))
			// A request that Authorized accepts always Presents a credential.
			if Authorized(c.r, "a", "b", "anything", "admin") {
				assert.True(t, Presents(c.r))
			}
		})
	}
}
