// Package netguard is the one place the HTTP transports that build their own take their dial
// function from, so that a test binary can replace it (see internal/testutil/dialguard).
// Production never replaces it.
package netguard

import (
	"context"
	"net"
)

// DialFunc has the signature of net.Dialer.DialContext and of http.Transport.DialContext.
type DialFunc func(ctx context.Context, network, addr string) (net.Conn, error)

var dial DialFunc = (&net.Dialer{}).DialContext

// Set replaces the dial function and returns the one it replaced. Only test code calls it.
func Set(f DialFunc) DialFunc {
	old := dial
	dial = f
	return old
}

// DialContext dials like a zero net.Dialer, through whatever Set installed.
func DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	return dial(ctx, network, addr)
}
