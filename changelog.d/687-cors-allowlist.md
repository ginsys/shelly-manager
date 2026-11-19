Fixed
- With `security.cors.allowed_origins` set, a request from an origin outside the list no longer gets
  `Access-Control-Allow-Origin: *`. It gets no `Access-Control-*` header, so a browser refuses its
  preflight and will not let the page read the response. A listed origin, a request with no `Origin`
  header, and a server with no allowlist (still `*`) behave as before. Fixes
  ginsys/shelly-manager#687.
