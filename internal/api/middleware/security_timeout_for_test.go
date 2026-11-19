package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TimeoutMiddlewareFor lets one request have a deadline of its own and changes nothing for any other.
// The handler takes 150 ms; the default deadline is 50 ms.
func TestTimeoutMiddlewareFor(t *testing.T) {
	slow := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(150 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})
	longer := func(*http.Request) (time.Duration, bool) { return 2 * time.Second, true }
	none := func(*http.Request) (time.Duration, bool) { return 0, false }
	zero := func(*http.Request) (time.Duration, bool) { return 0, true }

	tests := []struct {
		name     string
		timeout  time.Duration
		override func(*http.Request) (time.Duration, bool)
		want     int
	}{
		{"an override that answers true gives the request its own deadline", 50 * time.Millisecond, longer, http.StatusOK},
		{"an override that answers false leaves the default", 50 * time.Millisecond, none, http.StatusRequestTimeout},
		{"no override leaves the default", 50 * time.Millisecond, nil, http.StatusRequestTimeout},
		{"an override of zero is no deadline and leaves the default", 50 * time.Millisecond, zero, http.StatusRequestTimeout},
		{"a config with no timeout has none, override or not", 0, longer, http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			TimeoutMiddlewareFor(timeoutTestConfig(tt.timeout), nil, tt.override)(slow).
				ServeHTTP(rr, httptest.NewRequest("GET", "/api/v1/test", nil))
			assert.Equal(t, tt.want, rr.Code)
		})
	}
}

// The override can also be shorter than the default: it replaces the deadline, it does not only extend it.
func TestTimeoutMiddlewareFor_AShorterOverrideCutsSooner(t *testing.T) {
	slow := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(150 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})
	shorter := func(*http.Request) (time.Duration, bool) { return 30 * time.Millisecond, true }
	rr := httptest.NewRecorder()
	TimeoutMiddlewareFor(timeoutTestConfig(2*time.Second), nil, shorter)(slow).
		ServeHTTP(rr, httptest.NewRequest("GET", "/api/v1/test", nil))
	assert.Equal(t, http.StatusRequestTimeout, rr.Code)
}
