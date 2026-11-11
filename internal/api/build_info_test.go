package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime/debug"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRevisionFromSettings(t *testing.T) {
	tests := []struct {
		name     string
		settings []debug.BuildSetting
		want     string
	}{
		{"revision present", []debug.BuildSetting{{Key: "vcs", Value: "git"}, {Key: "vcs.revision", Value: "abc123"}}, "abc123"},
		{"no revision setting", []debug.BuildSetting{{Key: "vcs", Value: "git"}}, ""},
		{"no settings", nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, revisionFromSettings(tt.settings))
		})
	}
}

// The UI compares this value with its own build revision to decide whether to show the banner, so
// the field must always be present, and empty when the build carries no revision.
func TestVersionReportsGitSHA(t *testing.T) {
	for _, sha := range []string{"0123456789abcdef0123456789abcdef01234567", ""} {
		h := &Handler{serverStartedAt: time.Now(), gitSHA: sha}
		rec := httptest.NewRecorder()
		h.Version(rec, httptest.NewRequest(http.MethodGet, "/api/v1/version", nil))
		require.Equal(t, http.StatusOK, rec.Code)

		var body struct {
			Success bool `json:"success"`
			Data    map[string]any
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		require.True(t, body.Success)
		got, present := body.Data["git_sha"]
		require.True(t, present, "git_sha must be present for sha %q", sha)
		assert.Equal(t, sha, got)
	}
}
