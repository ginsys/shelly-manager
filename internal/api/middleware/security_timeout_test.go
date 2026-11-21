package middleware

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// timeoutTestConfig returns a security config with only the request timeout
// set; the logger stays nil so the tests print nothing.
func timeoutTestConfig(d time.Duration) *SecurityConfig {
	cfg := DefaultSecurityConfig()
	cfg.RequestTimeout = d
	return cfg
}

// timeoutBody asserts that body is the REQUEST_TIMEOUT answer and nothing else.
func assertTimeoutBody(t *testing.T, body string) {
	t.Helper()
	var got struct {
		Success bool `json:"success"`
		Error   struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal([]byte(body), &got), "body must be exactly one JSON document: %q", body)
	assert.False(t, got.Success)
	assert.Equal(t, "REQUEST_TIMEOUT", got.Error.Code)
	assert.Equal(t, "Request processing timeout", got.Error.Message)
}

// A handler that keeps writing after the 408 went out must not reach the
// client: the timed-out request has exactly one writer.
func TestTimeoutMiddleware_WritesAfterTimeoutAreDropped(t *testing.T) {
	release := make(chan struct{})
	finished := make(chan struct{})
	var writeErr error

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(finished)
		<-release
		w.Header().Set("X-Late", "1")
		w.WriteHeader(http.StatusOK)
		_, writeErr = w.Write([]byte("late body"))
	})

	rr := httptest.NewRecorder()
	TimeoutMiddleware(timeoutTestConfig(50*time.Millisecond), nil)(handler).
		ServeHTTP(rr, httptest.NewRequest("GET", "/api/v1/test", nil))

	close(release)
	<-finished

	assert.Equal(t, http.StatusRequestTimeout, rr.Code)
	assertTimeoutBody(t, rr.Body.String())
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))
	assert.Equal(t, strconv.Itoa(rr.Body.Len()), rr.Header().Get("Content-Length"))
	assert.Empty(t, rr.Header().Get("X-Late"), "a header set after the timeout must not reach the client")
	assert.ErrorIs(t, writeErr, http.ErrHandlerTimeout)
}

// A handler that had already started its response at the deadline keeps what it sent: no 408 is
// appended to its body, later writes are dropped, and the transfer is cut (see
// TestTimeoutMiddleware_DeadlineAfterStartAbortsTheTransfer for the client's view).
func TestTimeoutMiddleware_StartedResponseIsNotCorrupted(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	finished := make(chan struct{})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(finished)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("partial"))
		close(started)
		<-release
		_, _ = w.Write([]byte("late"))
	})

	rr := httptest.NewRecorder()
	aborted := serveRecordingAbort(TimeoutMiddleware(timeoutTestConfig(50*time.Millisecond), nil)(handler),
		rr, httptest.NewRequest("GET", "/api/v1/test", nil))

	<-started
	close(release)
	<-finished

	assert.True(t, aborted)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "partial", rr.Body.String())
}

// Headers: the handler sees what upstream middleware set, may change or delete
// it, and what it sets reaches the client on the normal path.
func TestTimeoutMiddleware_NormalPathHeaders(t *testing.T) {
	var sawUpstream bool
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawUpstream = w.Header().Get("X-Upstream") == "u"
		w.Header().Del("X-Del")
		w.Header().Set("X-Handler", "h")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("ok"))
		_, _ = w.Write([]byte("!"))
	})

	rr := httptest.NewRecorder()
	rr.Header().Set("X-Upstream", "u")
	rr.Header().Set("X-Del", "d")
	TimeoutMiddleware(timeoutTestConfig(time.Second), nil)(handler).
		ServeHTTP(rr, httptest.NewRequest("GET", "/api/v1/test", nil))

	assert.True(t, sawUpstream, "the handler must see the headers set upstream")
	assert.Equal(t, http.StatusCreated, rr.Code)
	assert.Equal(t, "ok!", rr.Body.String())
	assert.Equal(t, "h", rr.Header().Get("X-Handler"))
	assert.Equal(t, "u", rr.Header().Get("X-Upstream"))
	assert.Empty(t, rr.Header().Get("X-Del"), "a header the handler deleted must be gone")
}

// A handler that sets a header and returns without writing still sends it:
// net/http answers with the implicit 200 and the header.
func TestTimeoutMiddleware_HeaderSetWithoutWriteIsKept(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Test", "v")
	})

	rr := httptest.NewRecorder()
	TimeoutMiddleware(timeoutTestConfig(time.Second), nil)(handler).
		ServeHTTP(rr, httptest.NewRequest("GET", "/api/v1/test", nil))

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "v", rr.Header().Get("X-Test"))
}

// Headers set upstream survive on the 408; headers the handler set but never
// wrote do not leak onto it.
func TestTimeoutMiddleware_TimeoutAnswerHeaders(t *testing.T) {
	release := make(chan struct{})
	finished := make(chan struct{})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(finished)
		w.Header().Set("X-Unwritten", "x")
		<-release
	})

	rr := httptest.NewRecorder()
	rr.Header().Set("X-Upstream", "u")
	TimeoutMiddleware(timeoutTestConfig(50*time.Millisecond), nil)(handler).
		ServeHTTP(rr, httptest.NewRequest("GET", "/api/v1/test", nil))

	close(release)
	<-finished

	assert.Equal(t, http.StatusRequestTimeout, rr.Code)
	assert.Equal(t, "u", rr.Header().Get("X-Upstream"))
	assert.Empty(t, rr.Header().Get("X-Unwritten"))
}

// The handler mutates its headers while the deadline passes. Run under -race:
// the handler and the timeout path must not touch one header map.
func TestTimeoutMiddleware_ConcurrentHeaderWrites(t *testing.T) {
	stop := make(chan struct{})
	finished := make(chan struct{})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(finished)
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
				w.Header().Set("X-N", strconv.Itoa(i))
			}
		}
	})

	rr := httptest.NewRecorder()
	TimeoutMiddleware(timeoutTestConfig(20*time.Millisecond), nil)(handler).
		ServeHTTP(rr, httptest.NewRequest("GET", "/api/v1/test", nil))

	close(stop)
	<-finished

	assert.Equal(t, http.StatusRequestTimeout, rr.Code)
}

// When the handler has finished and the deadline fires in the same moment,
// select picks either case. The timeout branch must then keep the handler's
// response: no 408 on a finished handler, and its headers still go out.
// The timing cannot be forced through the middleware, so this calls expire.
func TestTimeoutWriter_ExpireAfterHandlerFinished(t *testing.T) {
	rr := httptest.NewRecorder()
	tw := newTimeoutWriter(rr)
	tw.Header().Set("X-Test", "v")
	done := make(chan struct{})
	close(done)

	expired, _ := tw.expire(done, func(http.ResponseWriter) {
		t.Error("a finished handler must not get a timeout answer")
	})

	assert.False(t, expired)
	assert.Equal(t, "v", rr.Header().Get("X-Test"))
	assert.Empty(t, rr.Body.String())
}

// After the handler returned, net/http owns the writer again: a write from a
// goroutine the handler left behind, or a late 500, must not reach it.
func TestTimeoutWriter_NothingAfterHandlerReturned(t *testing.T) {
	rr := httptest.NewRecorder()
	tw := newTimeoutWriter(rr)
	tw.finish()

	n, err := tw.Write([]byte("stray"))
	tw.WriteHeader(http.StatusTeapot)

	assert.Zero(t, n)
	assert.ErrorIs(t, err, errWriteAfterFinish)
	assert.False(t, tw.fail(), "no 500 after the handler returned")
	assert.Empty(t, rr.Body.String())
	assert.Equal(t, http.StatusOK, rr.Code)
}

// A bare Write, with no WriteHeader before it, sends the handler's headers
// with the implicit 200.
func TestTimeoutMiddleware_BareWriteSendsHeaders(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Bare", "b")
		_, _ = w.Write([]byte("body"))
	})

	rr := httptest.NewRecorder()
	TimeoutMiddleware(timeoutTestConfig(time.Second), nil)(handler).
		ServeHTTP(rr, httptest.NewRequest("GET", "/api/v1/test", nil))

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "b", rr.Header().Get("X-Bare"))
	assert.Equal(t, "body", rr.Body.String())
}

// statusLog is a response writer that records every WriteHeader call.
type statusLog struct {
	h     http.Header
	codes []int
	body  strings.Builder
}

func newStatusLog() *statusLog { return &statusLog{h: http.Header{}} }

func (s *statusLog) Header() http.Header { return s.h }
func (s *statusLog) WriteHeader(code int) {
	s.codes = append(s.codes, code)
}
func (s *statusLog) Write(b []byte) (int, error) { return s.body.Write(b) }

// An informational status (103) is not the final one: the handler's own status
// that follows must still reach the client.
func TestTimeoutWriter_InformationalStatusIsNotFinal(t *testing.T) {
	sl := newStatusLog()
	tw := newTimeoutWriter(sl)

	tw.WriteHeader(http.StatusEarlyHints)
	tw.Header().Set("X-After", "a")
	tw.WriteHeader(http.StatusCreated)
	_, _ = tw.Write([]byte("body"))

	assert.Equal(t, []int{http.StatusEarlyHints, http.StatusCreated}, sl.codes)
	assert.Equal(t, "a", sl.h.Get("X-After"))
	assert.Equal(t, "body", sl.body.String())
}

// A timeout or panic answer after an informational status starts from the
// headers upstream set: a handler header already copied over by the 103 (here
// Content-Encoding) must not survive onto the plain 408 or 500 answer.
func TestTimeoutWriter_AnswerAfterInformationalStatusStartsFromUpstreamHeaders(t *testing.T) {
	setup := func() (*statusLog, *timeoutWriter) {
		sl := newStatusLog()
		sl.h.Set("X-Upstream", "u")
		tw := newTimeoutWriter(sl)
		tw.Header().Set("Content-Encoding", "gzip")
		tw.Header().Del("X-Upstream")
		tw.WriteHeader(http.StatusEarlyHints)
		return sl, tw
	}

	t.Run("408", func(t *testing.T) {
		sl, tw := setup()
		expired, _ := tw.expire(make(chan struct{}), func(w http.ResponseWriter) {
			w.WriteHeader(http.StatusRequestTimeout)
		})
		assert.True(t, expired)
		assert.Equal(t, []int{http.StatusEarlyHints, http.StatusRequestTimeout}, sl.codes)
		assert.Empty(t, sl.h.Get("Content-Encoding"))
		assert.Equal(t, "u", sl.h.Get("X-Upstream"))
	})
	t.Run("500", func(t *testing.T) {
		sl, tw := setup()
		assert.True(t, tw.fail())
		assert.Equal(t, []int{http.StatusEarlyHints, http.StatusInternalServerError}, sl.codes)
		assert.Empty(t, sl.h.Get("Content-Encoding"))
		assert.Equal(t, "u", sl.h.Get("X-Upstream"))
	})
}

// After the middleware answered a panic with its own 500, finishing the
// request must not reconcile the handler's trailers onto that answer.
func TestTimeoutWriter_FinishAfterFailLeavesTheAnswerAlone(t *testing.T) {
	sl := newStatusLog()
	sl.h.Set("Trailer", "X-Result")
	sl.h.Set("X-Result", "upstream")
	tw := newTimeoutWriter(sl)
	tw.Header().Set("X-Result", "changed")
	tw.WriteHeader(http.StatusEarlyHints)

	assert.True(t, tw.fail())
	tw.finish()

	assert.Equal(t, "upstream", sl.h.Get("X-Result"))
	assert.Equal(t, []int{http.StatusEarlyHints, http.StatusInternalServerError}, sl.codes)
}

// A status code the real writer rejects (it panics) leaves the response not
// started, so the panic can still be answered with the 500.
func TestTimeoutWriter_RejectedStatusCanStillBeAnswered(t *testing.T) {
	rr := httptest.NewRecorder()
	tw := newTimeoutWriter(rr)

	func() {
		defer func() { _ = recover() }()
		tw.WriteHeader(0)
	}()

	assert.True(t, tw.fail())
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

// The real writer never shares a value slice with the handler: a handler that
// edits a value in place after its response started changes nothing there.
func TestTimeoutWriter_HeaderValuesAreCopied(t *testing.T) {
	rr := httptest.NewRecorder()
	tw := newTimeoutWriter(rr)
	tw.Header().Set("X-V", "orig")
	_, _ = tw.Write([]byte("x"))

	tw.Header()["X-V"][0] = "changed"

	assert.Equal(t, "orig", rr.Header().Get("X-V"))
}

// Trailers set after the body, declared or with the prefix, reach the client
// on the normal path. This goes through a real server.
func TestTimeoutMiddleware_Trailers(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Trailer", "X-Declared")
		_, _ = w.Write([]byte("body"))
		w.Header().Set("X-Declared", "d")
		w.Header().Set(http.TrailerPrefix+"X-Prefixed", "p")
	})
	srv := httptest.NewServer(TimeoutMiddleware(timeoutTestConfig(time.Second), nil)(handler))
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	assert.Equal(t, "body", string(body))
	assert.Equal(t, "d", resp.Trailer.Get("X-Declared"))
	assert.Equal(t, "p", resp.Trailer.Get("X-Prefixed"))
}

// A trailer the handler deletes after setting it is not sent with its old
// value: declared names and prefixed keys alike.
func TestTimeoutMiddleware_DeletedTrailersAreNotSent(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Trailer", "X-Result")
		w.Header().Set("X-Result", "pending")
		w.Header().Set(http.TrailerPrefix+"X-Extra", "pending")
		_, _ = w.Write([]byte("body"))
		w.Header().Del("X-Result")
		w.Header().Del(http.TrailerPrefix + "X-Extra")
	})
	srv := httptest.NewServer(TimeoutMiddleware(timeoutTestConfig(time.Second), nil)(handler))
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	_, err = io.ReadAll(resp.Body)
	require.NoError(t, err)

	assert.Empty(t, resp.Trailer.Get("X-Result"))
	assert.Empty(t, resp.Trailer.Get("X-Extra"))
}

// fail is atomic with the timeout: once either answered, the other does not.
func TestTimeoutWriter_FailAndExpireAnswerOnce(t *testing.T) {
	t.Run("500 first, no 408 after", func(t *testing.T) {
		rr := httptest.NewRecorder()
		tw := newTimeoutWriter(rr)
		assert.True(t, tw.fail())
		expired, started := tw.expire(make(chan struct{}), func(http.ResponseWriter) {
			t.Error("the 500 went out, so no timeout answer may follow")
		})
		assert.True(t, expired)
		assert.True(t, started)
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Equal(t, "Internal Server Error\n", rr.Body.String())
	})
	t.Run("started response, no 500 appended", func(t *testing.T) {
		rr := httptest.NewRecorder()
		tw := newTimeoutWriter(rr)
		_, _ = tw.Write([]byte("partial"))
		assert.False(t, tw.fail())
		assert.Equal(t, "partial", rr.Body.String())
	})
}

// The handler writes in a loop while the deadline passes. Run under -race:
// the handler's WriteHeader and Write and the timeout path must be ordered by
// the writer's lock.
func TestTimeoutMiddleware_ConcurrentWrites(t *testing.T) {
	stop := make(chan struct{})
	finished := make(chan struct{})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(finished)
		for {
			select {
			case <-stop:
				return
			default:
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("x"))
			}
		}
	})

	rr := httptest.NewRecorder()
	aborted := serveRecordingAbort(TimeoutMiddleware(timeoutTestConfig(20*time.Millisecond), nil)(handler),
		rr, httptest.NewRequest("GET", "/api/v1/test", nil))

	close(stop)
	<-finished

	assert.True(t, aborted, "the handler had started its response at the deadline")
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.NotContains(t, rr.Body.String(), "REQUEST_TIMEOUT")
}

// A panic in the handler goroutine is outside every upstream recover and used
// to end the process. Each scenario runs in a child test process so that the
// red case is a clean failure and not a crashed test binary.
const timeoutPanicScenarioEnv = "TIMEOUT_PANIC_SCENARIO"

type panicResult struct {
	Code        int    `json:"code"`
	Body        string `json:"body"`
	ContentType string `json:"content_type"`
}

func runTimeoutPanicScenario(name string) panicResult {
	var handler http.HandlerFunc
	timeout := 30 * time.Millisecond
	switch name {
	case "before-write":
		timeout = time.Second
		handler = func(w http.ResponseWriter, r *http.Request) {
			panic("boom")
		}
	case "after-started":
		timeout = time.Second
		handler = func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("partial"))
			panic("boom")
		}
	case "after-deadline":
		handler = func(w http.ResponseWriter, r *http.Request) {
			<-r.Context().Done()
			time.Sleep(20 * time.Millisecond)
			panic("boom")
		}
	}

	if name == "before-write" {
		// The 500 must be written before ServeHTTP returns: read the recorder
		// at that instant, many times, and report the first wrong answer. If
		// the handler goroutine closed done before answering, some round would
		// return first (and -race would flag the late write).
		var res panicResult
		for i := 0; i < 3000; i++ {
			rr := httptest.NewRecorder()
			TimeoutMiddleware(timeoutTestConfig(timeout), nil)(handler).
				ServeHTTP(rr, httptest.NewRequest("GET", "/api/v1/test", nil))
			res = panicResult{Code: rr.Code, Body: rr.Body.String(), ContentType: rr.Header().Get("Content-Type")}
			if res.Code != http.StatusInternalServerError || res.Body != "Internal Server Error\n" {
				break
			}
		}
		return res
	}

	rr := httptest.NewRecorder()
	TimeoutMiddleware(timeoutTestConfig(timeout), nil)(handler).
		ServeHTTP(rr, httptest.NewRequest("GET", "/api/v1/test", nil))
	// Let a late panic happen before the process reports and exits.
	time.Sleep(150 * time.Millisecond)
	return panicResult{Code: rr.Code, Body: rr.Body.String(), ContentType: rr.Header().Get("Content-Type")}
}

func TestTimeoutMiddleware_HandlerPanic(t *testing.T) {
	if name := os.Getenv(timeoutPanicScenarioEnv); name != "" {
		out, _ := json.Marshal(runTimeoutPanicScenario(name))
		_, _ = os.Stdout.WriteString("PANIC_RESULT " + string(out) + "\n")
		return
	}

	run := func(t *testing.T, name string) panicResult {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestTimeoutMiddleware_HandlerPanic$", "-test.count=1")
		cmd.Env = append(os.Environ(), timeoutPanicScenarioEnv+"="+name)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "the child test process must survive the handler panic; output:\n%s", out)

		sc := bufio.NewScanner(strings.NewReader(string(out)))
		for sc.Scan() {
			if line, ok := strings.CutPrefix(sc.Text(), "PANIC_RESULT "); ok {
				var res panicResult
				require.NoError(t, json.Unmarshal([]byte(line), &res))
				return res
			}
		}
		require.Fail(t, "child printed no result", string(out))
		return panicResult{}
	}

	t.Run("before-write answers 500", func(t *testing.T) {
		res := run(t, "before-write")
		assert.Equal(t, http.StatusInternalServerError, res.Code)
		assert.Equal(t, "Internal Server Error\n", res.Body)
	})
	t.Run("after-started keeps the response", func(t *testing.T) {
		res := run(t, "after-started")
		assert.Equal(t, http.StatusOK, res.Code)
		assert.Equal(t, "partial", res.Body)
	})
	t.Run("after-deadline keeps the 408", func(t *testing.T) {
		res := run(t, "after-deadline")
		assert.Equal(t, http.StatusRequestTimeout, res.Code)
		assertTimeoutBody(t, res.Body)
	})
}
