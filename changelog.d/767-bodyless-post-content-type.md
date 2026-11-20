Fixed
- A POST, PUT or PATCH with no body is no longer refused for lacking a `Content-Type` header. The
  Metrics page's collection controls (enable, disable, collect) and the notification channel test
  sent no body, so axios sent no content type and the server answered 400 "Content-Type header is
  required"; curl without `-H` failed the same way. A request with a body and no type, or with an
  unlisted type, is still refused. Fixes ginsys/shelly-manager#767.
