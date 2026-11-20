package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// A path that is not an export id must never reach the export-result handler. The
// `/export/{id}` route is limited to the id's own shape, so the client's GitOps paths that the
// server has no GET route for are refused by the router itself (its plain 404 body, or a 405),
// never answered by the result handler.
func TestUnroutedExportPathsDoNotFallThroughToTheResultHandler(t *testing.T) {
	router := newRouteContractRouter(t)

	for _, path := range []string{
		"/api/v1/export/gitops",
		"/api/v1/export/gitops-statistics",
		"/api/v1/export/gitops-preview",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		// Without a user agent the validation layer answers 400 first and hides the handler.
		req.Header.Set("User-Agent", "Mozilla/5.0 (route test)")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		require.Contains(t, []int{http.StatusNotFound, http.StatusMethodNotAllowed}, rr.Code, "%s: %s", path, rr.Body.String())
		if rr.Code == http.StatusNotFound {
			require.Contains(t, rr.Body.String(), "page not found", "%s was answered by a handler, not the router", path)
		}
	}
}
