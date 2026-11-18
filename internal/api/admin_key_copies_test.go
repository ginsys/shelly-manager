package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ginsys/shelly-manager/internal/logging"
)

// Handler, ImportHandlers and SyncHandlers each keep their own copy of the admin-key guard, and all
// three compare through internal/security/apikey. Each copy must still accept the admin key in both
// header forms, refuse the rest, and stay open when no key is set.
func TestAdminKeyGuards_EachCopyKeepsItsBehaviour(t *testing.T) {
	logger := logging.GetDefault()

	type guard struct {
		name  string
		setup func(key string) func(w http.ResponseWriter, r *http.Request) bool
	}
	guards := []guard{
		{"Handler", func(key string) func(http.ResponseWriter, *http.Request) bool {
			h := &Handler{logger: logger}
			h.SetAdminAPIKey(key)
			return h.requireAdmin
		}},
		{"ImportHandlers", func(key string) func(http.ResponseWriter, *http.Request) bool {
			ih := NewImportHandlers(nil, logger)
			ih.SetAdminAPIKey(key)
			return ih.requireAdmin
		}},
		{"SyncHandlers", func(key string) func(http.ResponseWriter, *http.Request) bool {
			eh := NewSyncHandlers(nil, logger)
			eh.SetAdminAPIKey(key)
			return eh.requireAdmin
		}},
	}

	call := func(check func(http.ResponseWriter, *http.Request) bool, header http.Header) (ok bool, code int) {
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		for k, v := range header {
			req.Header[k] = v
		}
		w := httptest.NewRecorder()
		ok = check(w, req)
		return ok, w.Code
	}

	for _, g := range guards {
		t.Run(g.name, func(t *testing.T) {
			check := g.setup("admin-secret")

			ok, _ := call(check, http.Header{"Authorization": {"Bearer admin-secret"}})
			assert.True(t, ok, "bearer")
			ok, _ = call(check, http.Header{"X-Api-Key": {"admin-secret"}})
			assert.True(t, ok, "X-API-Key")

			for name, header := range map[string]http.Header{
				"no credential":    nil,
				"wrong bearer":     {"Authorization": {"Bearer wrong"}},
				"wrong X-API-Key":  {"X-Api-Key": {"wrong"}},
				"empty bearer":     {"Authorization": {"Bearer "}},
				"bearer lowercase": {"Authorization": {"bearer admin-secret"}},
				"no prefix":        {"Authorization": {"admin-secret"}},
			} {
				passed, code := call(check, header)
				assert.False(t, passed, name)
				assert.Equal(t, http.StatusUnauthorized, code, name)
			}

			open := g.setup("")
			ok, _ = call(open, nil)
			assert.True(t, ok, "no key configured stays open")
		})
	}
}
