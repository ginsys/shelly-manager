package main

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ginsys/shelly-manager/internal/database"
	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/metrics"
	"github.com/ginsys/shelly-manager/internal/testutil"
)

// The timeouts the tests run the server with. Short, so a test that expects a close finishes in
// well under a second; the production values are pinned by TestServerTimeoutValues.
const (
	testHeaderTimeout = 200 * time.Millisecond
	testIdleTimeout   = 300 * time.Millisecond
	// closeWithin is how long a test waits for a close that is due after the timeout above.
	closeWithin = 2 * time.Second
)

// startServer runs a server built by newHTTPServerWith on a local listener and returns its address.
func startTimeoutServer(t *testing.T, handler http.Handler) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	srv := newHTTPServerWith(ln.Addr().String(), handler, testHeaderTimeout, testIdleTimeout)
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
	return ln.Addr().String()
}

func dial(t *testing.T, addr string) net.Conn {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

// requireClosedByServer reads until the connection ends and fails if it is still open after
// closeWithin: a read that times out means the server never closed it.
func requireClosedByServer(t *testing.T, conn net.Conn, what string) time.Duration {
	t.Helper()
	start := time.Now()
	if err := conn.SetReadDeadline(start.Add(closeWithin)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	buf := make([]byte, 512)
	for {
		_, err := conn.Read(buf)
		if err == nil {
			continue
		}
		var ne net.Error
		if errors.As(err, &ne) && ne.Timeout() {
			t.Fatalf("%s: the server still held the connection open after %s", what, closeWithin)
		}
		return time.Since(start)
	}
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "ok")
	})
}

const getRequest = "GET / HTTP/1.1\r\nHost: x\r\n\r\n"

// A client that never finishes its header is cut after the header timeout.
func TestServerClosesSlowHeader(t *testing.T) {
	addr := startTimeoutServer(t, okHandler())
	conn := dial(t, addr)
	// Request line and one header, and no blank line that ends the header.
	if _, err := io.WriteString(conn, "GET / HTTP/1.1\r\nHost: x\r\n"); err != nil {
		t.Fatalf("write: %v", err)
	}
	requireClosedByServer(t, conn, "slow header")
}

// A keep-alive connection that sends no further request is cut after the idle timeout.
func TestServerClosesIdleKeepAlive(t *testing.T) {
	addr := startTimeoutServer(t, okHandler())
	conn := dial(t, addr)
	if _, err := io.WriteString(conn, getRequest); err != nil {
		t.Fatalf("write: %v", err)
	}
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatalf("read the answer: %v", err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	requireClosedByServer(t, conn, "idle keep-alive")
}

// Guard: a body that arrives slowly, after a complete header, is not cut. This is what stops a
// ReadTimeout from being added later: it would count header and body together.
func TestServerKeepsAStartedBodyOpen(t *testing.T) {
	var got atomic.Value
	addr := startTimeoutServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		got.Store(string(b))
		_, _ = w.Write(b)
	}))
	conn := dial(t, addr)
	if _, err := io.WriteString(conn, "POST / HTTP/1.1\r\nHost: x\r\nContent-Length: 6\r\n\r\n"); err != nil {
		t.Fatalf("write header: %v", err)
	}
	// Three chunks over about 3 x the header timeout.
	for _, chunk := range []string{"ab", "cd", "ef"} {
		time.Sleep(testHeaderTimeout)
		if _, err := io.WriteString(conn, chunk); err != nil {
			t.Fatalf("write body chunk %q: %v", chunk, err)
		}
	}
	if err := conn.SetReadDeadline(time.Now().Add(closeWithin)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatalf("read the answer: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if got.Load() != "abcdef" {
		t.Fatalf("handler read %v, want abcdef", got.Load())
	}
}

// Guard: a handler that runs longer than the idle timeout still answers. The idle clock counts the
// time between requests, never a request in progress (bulk-apply runs for minutes).
func TestServerDoesNotCutALongHandler(t *testing.T) {
	addr := startTimeoutServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(3 * testIdleTimeout)
		_, _ = io.WriteString(w, "done")
	}))
	conn := dial(t, addr)
	if _, err := io.WriteString(conn, getRequest); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := conn.SetReadDeadline(time.Now().Add(closeWithin + 3*testIdleTimeout)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatalf("read the answer: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	if string(b) != "done" {
		t.Fatalf("answer %q, want done", b)
	}
}

// Guard: the real metrics WebSocket handler, hijacked from the server, stays usable well past the
// idle timeout. The control is TestServerClosesIdleKeepAlive: the same settings close a plain
// keep-alive connection after testIdleTimeout.
func TestServerKeepsTheMetricsWebSocketPastIdleTime(t *testing.T) {
	logger := logging.GetDefault()
	// A real (temporary SQLite) database: the hub collects metrics from it for each new client.
	db, err := database.NewManagerFromPath(filepath.Join(testutil.TempDir(t), "ws.db"))
	if err != nil {
		t.Fatalf("test database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	svc := metrics.NewService(db.GetDB(), logger, prometheus.NewRegistry())
	h := metrics.NewHandler(svc, logger)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go h.GetWebSocketHub().Run(ctx)

	mux := http.NewServeMux()
	mux.HandleFunc("/metrics/ws", h.HandleWebSocket)
	addr := startTimeoutServer(t, mux)

	conn, _, err := websocket.DefaultDialer.Dial("ws://"+addr+"/metrics/ws", nil)
	if err != nil {
		t.Fatalf("websocket dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	// A reader goroutine: it counts what the hub pushes and reports each pong or the first error.
	var pushed atomic.Int64
	pongs := make(chan struct{}, 4)
	failed := make(chan error, 1)
	conn.SetPongHandler(func(string) error { pongs <- struct{}{}; return nil })
	go func() {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				failed <- err
				return
			}
			pushed.Add(1)
		}
	}()

	time.Sleep(3 * testIdleTimeout)
	select {
	case err := <-failed:
		t.Fatalf("the server closed the websocket during %s of idle time: %v", 3*testIdleTimeout, err)
	default:
	}

	// One exchange each way after both timeouts have passed: a ping the server's read side must
	// take and a pong its write side must send, then a text message the server must accept.
	if err := conn.WriteControl(websocket.PingMessage, []byte("p"), time.Now().Add(time.Second)); err != nil {
		t.Fatalf("ping after the idle time: %v", err)
	}
	select {
	case <-pongs:
	case err := <-failed:
		t.Fatalf("no pong, the connection ended: %v", err)
	case <-time.After(closeWithin):
		t.Fatalf("no pong within %s of a ping sent after the idle time", closeWithin)
	}
	if err := conn.WriteMessage(websocket.TextMessage, []byte("x")); err != nil {
		t.Fatalf("text message after the idle time: %v", err)
	}
	t.Logf("messages the hub pushed to the client during the test: %d", pushed.Load())
}

// Pins the production values and that no ReadTimeout or WriteTimeout is set (see the comment on
// newHTTPServerWith for why).
func TestServerTimeoutValues(t *testing.T) {
	srv := newHTTPServer("127.0.0.1:0", okHandler())
	if srv.ReadHeaderTimeout != 10*time.Second {
		t.Errorf("ReadHeaderTimeout = %s, want 10s", srv.ReadHeaderTimeout)
	}
	if srv.IdleTimeout != 120*time.Second {
		t.Errorf("IdleTimeout = %s, want 120s", srv.IdleTimeout)
	}
	if srv.ReadTimeout != 0 || srv.WriteTimeout != 0 {
		t.Errorf("ReadTimeout = %s, WriteTimeout = %s, want both unset", srv.ReadTimeout, srv.WriteTimeout)
	}
	if srv.Addr != "127.0.0.1:0" || srv.Handler == nil {
		t.Errorf("Addr = %q, Handler nil = %v", srv.Addr, srv.Handler == nil)
	}
}
