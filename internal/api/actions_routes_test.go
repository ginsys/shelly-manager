package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/configuration"
)

// Five bodies, the same on every route that stores a configuration which apply can read. The URL list
// of an action is kept through a write by two helpers, and the explicit clear [] must survive them.
// One table says that each route gives each body the same answer, so a route that loses its helper
// goes red in its own row. The rows pin what each route does today: a new URL sent beside a mask is
// dropped (the stored list stays), and an empty line is stored as sent. Neither is ruled as wanted.

const actionsRouteStoredURL = "http://admin:hunter2@192.168.1.4/on?token=s3cr3t"

// actionsRoute is one write route: send writes a body, stored reads back the list of the one action.
type actionsRoute struct {
	hasStored bool // a list is stored before the call
	okCode    int  // the answer to a write that is stored
	send      func(body string) *httptest.ResponseRecorder
	stored    func() (urls []string, set bool)
	raw       func() string
}

func actionsRouteBody(list string) string {
	if list != "" {
		list = `,"secret_urls":` + list
	}
	return `{"actions":[{"name":"out_on_url","index":0,"enabled":false` + list + `}]}`
}

func actionsOfConfig(t *testing.T, cfg *configuration.DeviceConfiguration) (urls []string, set bool) {
	t.Helper()
	require.NotNil(t, cfg.Actions)
	require.Len(t, *cfg.Actions, 1)
	entry := (*cfg.Actions)[0]
	// The stored action starts enabled and every body sends it disabled, so a body that a route
	// refused, or answered 200 without storing, leaves it enabled.
	require.NotNil(t, entry.Enabled)
	require.False(t, *entry.Enabled, "the body was not stored")
	return urlsOf(entry), entry.SecretURLs != nil
}

func overridesRoute(t *testing.T, method string) actionsRoute {
	handler, cleanup := setupTestHandler(t)
	t.Cleanup(cleanup)
	id := keepOverrideDevice(t, handler, storedActionOverrides)
	return actionsRoute{
		hasStored: true,
		okCode:    http.StatusOK,
		send:      func(body string) *httptest.ResponseRecorder { return sendOverrides(t, handler, method, id, body) },
		stored: func() ([]string, bool) {
			cfg, err := handler.ConfigService.ConfigurationSvc.GetDeviceOverrides(id)
			require.NoError(t, err)
			return actionsOfConfig(t, cfg)
		},
		raw: func() string {
			cfg, err := handler.ConfigService.ConfigurationSvc.GetDeviceOverrides(id)
			require.NoError(t, err)
			text, err := json.Marshal(cfg)
			require.NoError(t, err)
			return string(text)
		},
	}
}

func bulkApplyRoute(t *testing.T) actionsRoute {
	handler, db, fleet := applyFleet(t, 1)
	id := fleet[0].device.ID
	setOverrides(t, db, id, storedActionOverrides)
	return actionsRoute{
		hasStored: true,
		okCode:    http.StatusOK,
		send:      func(body string) *httptest.ResponseRecorder { return postApply(handler, applyBody(body, false, id)) },
		stored: func() ([]string, bool) {
			cfg, err := handler.ConfigService.ConfigurationSvc.GetDeviceOverrides(id)
			require.NoError(t, err)
			return actionsOfConfig(t, cfg)
		},
		raw: func() string { return storedOverrides(t, db, id) },
	}
}

func templateUpdateRoute(t *testing.T) actionsRoute {
	handler, cleanup := setupTestHandler(t)
	t.Cleanup(cleanup)
	id := newKeepTemplate(t, handler, storedActionOverrides)
	return actionsRoute{
		hasStored: true,
		okCode:    http.StatusOK,
		send: func(body string) *httptest.ResponseRecorder {
			return putKeepTemplate(t, handler, id, `{"config":`+body+`}`)
		},
		stored: func() ([]string, bool) {
			var cfg configuration.DeviceConfiguration
			require.NoError(t, json.Unmarshal([]byte(storedTemplateConfig(t, handler, id)), &cfg))
			return actionsOfConfig(t, &cfg)
		},
		raw: func() string { return storedTemplateConfig(t, handler, id) },
	}
}

// templateCreateRoute stores nothing before the call, so a mask or an omitted list has nothing to take.
func templateCreateRoute(t *testing.T) actionsRoute {
	handler, cleanup := setupTestHandler(t)
	t.Cleanup(cleanup)
	var created uint
	return actionsRoute{
		okCode: http.StatusCreated,
		send: func(body string) *httptest.ResponseRecorder {
			req := httptest.NewRequest("POST", "/api/v1/config/templates/new",
				strings.NewReader(`{"name":"m","scope":"global","config":`+body+`}`))
			w := httptest.NewRecorder()
			handler.CreateNewConfigTemplate(w, req)
			// Every body is created, so the stored row is always read back.
			require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
			var resp struct {
				Data struct {
					Template TemplateResponse `json:"template"`
				} `json:"data"`
			}
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
			created = resp.Data.Template.ID
			require.NotZero(t, created, "the answer names no template")
			return w
		},
		stored: func() ([]string, bool) {
			var cfg configuration.DeviceConfiguration
			require.NoError(t, json.Unmarshal([]byte(storedTemplateConfig(t, handler, created)), &cfg))
			return actionsOfConfig(t, &cfg)
		},
		raw: func() string { return storedTemplateConfig(t, handler, created) },
	}
}

func TestActionRoutes_EveryBodyGivesEveryRouteTheSameAnswer(t *testing.T) {
	routes := []struct {
		name string
		make func(t *testing.T) actionsRoute
	}{
		{"PUT /devices/{id}/overrides/new", func(t *testing.T) actionsRoute { return overridesRoute(t, "PUT") }},
		{"PATCH /devices/{id}/overrides/new", func(t *testing.T) actionsRoute { return overridesRoute(t, "PATCH") }},
		{"POST /config/bulk-apply", bulkApplyRoute},
		{"PUT /config/templates/new/{id}", templateUpdateRoute},
		{"POST /config/templates/new", templateCreateRoute},
	}
	bodies := []struct{ name, list string }{
		{"clear", `[]`},
		{"mask", `["********"]`},
		{"omitted", ``},
		{"mixed mask", `["********","http://new.example/on"]`},
		{"empty line", `[""]`},
	}
	for _, route := range routes {
		for _, body := range bodies {
			t.Run(route.name+" "+body.name, func(t *testing.T) {
				r := route.make(t)
				w := r.send(actionsRouteBody(body.list))
				urls, set := r.stored()

				switch {
				case body.name == "clear":
					require.Equal(t, r.okCode, w.Code, w.Body.String())
					assert.True(t, set, "the clear was stored as no list")
					assert.NotNil(t, urls)
					assert.Empty(t, urls)
				case body.name == "empty line":
					// Every route stores the line as sent. It is never the clear, and a route that
					// began to refuse it would change this answer on purpose.
					require.Equal(t, r.okCode, w.Code, w.Body.String())
					assert.Equal(t, []string{""}, urls, "an empty line is never the clear")
				case r.hasStored:
					require.Equal(t, r.okCode, w.Code, w.Body.String())
					assert.Equal(t, []string{actionsRouteStoredURL}, urls)
				default:
					require.Equal(t, r.okCode, w.Code, w.Body.String())
					assert.False(t, set, "a body with nothing stored to take from became a list: %v", urls)
				}
				assert.NotContains(t, r.raw(), secretMask, "the mask was stored")
				assertNoURLSecret(t, "the answer", w.Body.String())
			})
		}
	}
}
