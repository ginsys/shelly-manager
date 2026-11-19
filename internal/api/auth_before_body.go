package api

import (
	"net/http"

	"github.com/ginsys/shelly-manager/internal/api/middleware"
	apiresp "github.com/ginsys/shelly-manager/internal/api/response"
	"github.com/ginsys/shelly-manager/internal/security/apikey"
)

// refuseKeylessBeforeBody answers 401 to a request that carries no credential at all, before the JSON
// check reads its body. With an admin key set, every mutating route refuses such a request (the route
// guard test proves it), so the answer is the one the route would give, without the parse.
//
// It decides only "no credential": whether a key is right depends on the route (the agent routes take a
// second key) and stays with the route's own guard. With no admin key set, a route is open and so is
// this step. A GET is left alone, so the open GET routes keep their answer.
func (h *Handler) refuseKeylessBeforeBody(config *middleware.SecurityConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if h.adminKey.Get() != "" && isMutatingMethod(r.Method) && r.ContentLength != 0 && !apikey.Presents(r) {
				// The CORS middleware runs after this step: a browser UI on another origin needs these
				// headers to read the 401 and open its admin-key prompt.
				setCORSHeaders(w, r, config)
				h.responseWriter().WriteError(w, r, http.StatusUnauthorized, apiresp.ErrCodeUnauthorized, "Authorization required", nil)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func isMutatingMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	}
	return false
}
