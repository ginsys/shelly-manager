package notification

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"syscall"
)

// A webhook or Slack URL is the credential: its path, query or userinfo carries the token. Go quotes the
// request URL in a *url.Error, and a bad redirect quotes more of them in the inner error, so the text of
// such an error is never kept. requestError names the operation and the failure class instead.
func requestError(what string, err error) error {
	var ue *url.Error
	if !errors.As(err, &ue) {
		return fmt.Errorf("%s: %w", what, err) // an invalid method: the text names no URL
	}
	class := "failed"
	switch {
	case ue.Timeout():
		class = "timed out"
	case errors.Is(err, context.Canceled):
		class = "canceled"
	case errors.Is(err, syscall.ECONNREFUSED):
		class = "connection refused"
	default:
		var dns *net.DNSError
		if errors.As(err, &dns) && dns.IsNotFound { // a SERVFAIL or a refused query stays "failed" // a SERVFAIL or a refused query stays "failed"
			class = "host not found"
		}
	}
	return fmt.Errorf("%s: %s %s", what, ue.Op, class)
}
