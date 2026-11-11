package sma

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func templateObject(extra map[string]interface{}) map[string]interface{} {
	object := map[string]interface{}{
		"id": float64(1), "generation": float64(1), "name": "t", "description": "", "device_type": "SHPLG-S",
		"config": map[string]interface{}{}, "variables": map[string]interface{}{},
		"is_default": false, "created_at": "2026-01-02T03:04:05Z", "updated_at": "2026-01-02T03:04:05Z",
	}
	for key, value := range extra {
		object[key] = value
	}
	return object
}

// The scope is optional, so an archive written before it travelled stays valid, and a scope that
// is named must be one a template can hold.
func TestTemplateScopeIsOptionalAndChecked(t *testing.T) {
	require.NoError(t, validateTemplate(templateObject(nil)), "no scope: an older archive")
	for _, scope := range []string{"global", "group", "device_type"} {
		require.NoError(t, validateTemplate(templateObject(map[string]interface{}{"scope": scope})), scope)
	}
	for name, scope := range map[string]interface{}{
		"unknown": "team", "empty": "", "number": float64(1), "null": nil, "wrong case": "Group",
	} {
		require.Error(t, validateTemplate(templateObject(map[string]interface{}{"scope": scope})), name)
	}
}
