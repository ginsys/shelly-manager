// Package dialguard makes a test binary refuse every network dial that leaves the host (rule G1:
// tests never reach a real address). A package's tests call Main from TestMain. A dial to an
// address that is not loopback returns an error that names it and is recorded; Main fails the
// package if any was recorded, so a test that swallows the error cannot pass.
//
// It covers http.DefaultTransport (every client that sets no Transport), the transports that take
// their dial from internal/netguard, and net.DefaultResolver. It does not cover a raw net.Dial,
// smtp.SendMail, multicast sockets or a database driver that dials by itself.
//
// This package imports neither internal/testutil nor any test helper, so a TestMain in
// internal/testutil can call it without an import cycle.
package dialguard

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"os"
	"sync"
	"testing"

	"github.com/ginsys/shelly-manager/internal/netguard"
)

var (
	mu         sync.Mutex
	violations []string
	capture    *[]string
)

// Dial allows loopback addresses, the name localhost and unix sockets, and refuses the rest.
func Dial(ctx context.Context, network, addr string) (net.Conn, error) {
	switch network {
	case "unix", "unixgram", "unixpacket":
		return (&net.Dialer{}).DialContext(ctx, network, addr)
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	// netip accepts an IPv6 zone ("::1%lo"), which net.ParseIP does not.
	if ip, parseErr := netip.ParseAddr(host); host == "localhost" || (parseErr == nil && ip.IsLoopback()) {
		return (&net.Dialer{}).DialContext(ctx, network, addr)
	}
	return nil, refuse(fmt.Sprintf("dialguard: a test dialled %s %s; tests must send nothing off this host", network, addr))
}

// resolve refuses every DNS query, the loopback stub included: a local stub resolver forwards the
// query to a server off this host, and the dial to the stub would look like loopback.
func resolve(_ context.Context, network, addr string) (net.Conn, error) {
	return nil, refuse(fmt.Sprintf("dialguard: a test resolved a name through DNS (%s %s); use an IP address or localhost", network, addr))
}

func refuse(msg string) error {
	mu.Lock()
	defer mu.Unlock()
	if capture != nil {
		*capture = append(*capture, msg)
	} else {
		violations = append(violations, msg)
	}
	return errors.New(msg)
}

// Install routes http.DefaultTransport, net.DefaultResolver and the netguard dial through the guard.
func Install() {
	netguard.Set(Dial)
	http.DefaultTransport = &http.Transport{DialContext: Dial}
	// PreferGo stays on: the Go resolver reads /etc/hosts before it dials a server, so localhost
	// resolves without a query, while the cgo resolver would bypass Dial altogether.
	net.DefaultResolver = &net.Resolver{PreferGo: true, Dial: resolve}
}

// Capture collects the refusals made until the returned function is called, which returns them.
// A test that proves the guard refuses uses it, so those refusals do not fail the package. It is
// process-wide: do not call it from parallel tests.
func Capture() func() []string {
	var got []string
	mu.Lock()
	capture = &got
	mu.Unlock()
	return func() []string {
		mu.Lock()
		defer mu.Unlock()
		capture = nil
		return got
	}
}

// Main installs the guard, runs the package's tests and fails the run if a dial was refused.
func Main(m *testing.M) {
	Install()
	code := m.Run()
	mu.Lock()
	for _, v := range violations {
		fmt.Fprintln(os.Stderr, v)
	}
	if len(violations) > 0 && code == 0 {
		code = 1
	}
	mu.Unlock()
	os.Exit(code)
}
