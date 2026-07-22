package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/configuration"
	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/testutil"
)

// The legacy template endpoints used to write straight to the database with no
// validation, so a client could store a template with an empty or nonsense
// scope. The next startup would then refuse to migrate that row (#275). These
// tests pin both halves of the repair: the rows are rejected, and they are
// rejected as a 400 rather than the blanket 500 the handlers used to return.

func setupTemplateScopeHandler(t *testing.T) (*Handler, func()) {
	t.Helper()

	db, cleanup := testutil.TestDatabase(t)
	logger := logging.GetDefault()

	handler := &Handler{
		DB:            db,
		Service:       testShellyService(t, db),
		logger:        logger,
		ConfigService: configuration.NewService(db.GetDB(), logger),
	}

	return handler, cleanup
}

func countTemplates(t *testing.T, handler *Handler) int64 {
	t.Helper()
	var count int64
	require.NoError(t, handler.DB.GetDB().Table("config_templates").Count(&count).Error)
	return count
}

func TestCreateConfigTemplate_RejectsInvalidScope(t *testing.T) {
	tests := []struct {
		name    string
		payload map[string]any
	}{
		{
			name:    "empty scope",
			payload: map[string]any{"name": "no-scope", "scope": "", "config": json.RawMessage(`{}`)},
		},
		{
			name:    "unknown scope",
			payload: map[string]any{"name": "garbage-scope", "scope": "garbage", "config": json.RawMessage(`{}`)},
		},
		{
			name:    "device_type scope without a device type",
			payload: map[string]any{"name": "unscoped", "scope": "device_type", "config": json.RawMessage(`{}`)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, cleanup := setupTemplateScopeHandler(t)
			defer cleanup()

			before := countTemplates(t, handler)

			body, err := json.Marshal(tt.payload)
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/config/templates", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.CreateConfigTemplate(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code, "invalid scope is a client error, not a server fault")
			assert.Equal(t, before, countTemplates(t, handler), "rejected template must never reach the database")
		})
	}
}

func TestCreateConfigTemplate_AcceptsValidScopes(t *testing.T) {
	valid := []map[string]any{
		{"name": "global-template", "scope": "global", "config": json.RawMessage(`{}`)},
		{"name": "group-template", "scope": "group", "config": json.RawMessage(`{}`)},
		{"name": "device-template", "scope": "device_type", "device_type": "SHSW-1", "config": json.RawMessage(`{}`)},
	}

	handler, cleanup := setupTemplateScopeHandler(t)
	defer cleanup()

	for i, payload := range valid {
		body, err := json.Marshal(payload)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/config/templates", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.CreateConfigTemplate(w, req)

		require.Equal(t, http.StatusCreated, w.Code, "payload %v was rejected", payload)
		assert.Equal(t, int64(i+1), countTemplates(t, handler))
	}
}

func TestUpdateConfigTemplate_RejectsInvalidScope(t *testing.T) {
	handler, cleanup := setupTemplateScopeHandler(t)
	defer cleanup()

	existing := &configuration.ConfigTemplate{
		Name:       "starts-valid",
		Scope:      "device_type",
		DeviceType: "SHSW-1",
		Config:     json.RawMessage(`{}`),
	}
	require.NoError(t, handler.Service.ConfigSvc.CreateTemplate(existing))

	// Dropping the device type would leave a device_type template with nothing
	// to match — exactly the row the migration preflight aborts on.
	body, err := json.Marshal(map[string]any{
		"name": "starts-valid", "scope": "device_type", "device_type": "", "config": json.RawMessage(`{}`),
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPut,
		fmt.Sprintf("/api/v1/config/templates/%d", existing.ID), bytes.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": fmt.Sprintf("%d", existing.ID)})
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.UpdateConfigTemplate(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var stored configuration.ConfigTemplate
	require.NoError(t, handler.DB.GetDB().First(&stored, existing.ID).Error)
	assert.Equal(t, "SHSW-1", stored.DeviceType, "rejected update must not modify the stored row")
}
