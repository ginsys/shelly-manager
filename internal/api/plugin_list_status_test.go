package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// The plugin list answers each plugin with a status of two keys, available and enabled. It has no
// configured, because there is no stored plugin configuration for it to describe, and no error or
// last_used, which nothing ever set (ginsys/shelly-manager#290).
func TestPluginListStatusHasAvailableAndEnabledOnly(t *testing.T) {
	router, _, _, cleanup := setupSecuredRouter(t, "k")
	defer cleanup()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/export/plugins", nil)
	req.Header.Set("Authorization", "Bearer k")
	router.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	var wrap struct {
		Data struct {
			Plugins []struct {
				Name   string         `json:"name"`
				Status map[string]any `json:"status"`
			} `json:"plugins"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &wrap))
	require.NotEmpty(t, wrap.Data.Plugins, "the registered mock plugin must be listed")

	for _, p := range wrap.Data.Plugins {
		require.Equal(t, map[string]any{"available": true, "enabled": true}, p.Status, "plugin %s", p.Name)
	}
}
