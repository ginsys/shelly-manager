package gen2

import (
	"context"
	"crypto/md5" // #nosec G501 -- the test recomputes the MD5 digest that older firmware asks for
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ginsys/shelly-manager/internal/shelly"
)

const (
	testDeviceID = "shellypro4pm-f008d1d8b8b8"
	// ha1Vector is sha256 of "admin:shellypro4pm-f008d1d8b8b8:secret" in lowercase hex.
	ha1Vector = "897c98c0aae196728d3a78eae02705c9fef67f9948e42b185f93b5c1974df346"
)

func skipWithoutSockets(t *testing.T) {
	t.Helper()
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Skipf("Skipping due to restricted socket permissions: %v", err)
	}
	_ = ln.Close()
}

// authorizationFields splits a Digest Authorization header into its key=value pairs.
func authorizationFields(header string) map[string]string {
	fields := map[string]string{}
	for _, part := range strings.Split(strings.TrimPrefix(header, "Digest "), ",") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) == 2 {
			fields[kv[0]] = strings.Trim(kv[1], `"`)
		}
	}
	return fields
}

// digestServer answers every POST /rpc with a digest challenge until the request carries an
// Authorization header, then checks the response the way a device does with the hash of the
// challenge's algorithm, records the body of the authenticated request and answers with answer.
type digestServer struct {
	*httptest.Server
	mu         sync.Mutex
	challenge  string
	hash       func(string) string
	user, pass string
	bodies     [][]byte // bodies of the requests that carried a valid Authorization header
	rejected   int      // authenticated requests whose response did not match
}

func newDigestServer(t *testing.T, challenge string, hash func(string) string, answer string) *digestServer {
	t.Helper()
	skipWithoutSockets(t)
	d := &digestServer{challenge: challenge, hash: hash, user: "admin", pass: "secret"}
	d.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		header := r.Header.Get("Authorization")
		if header == "" {
			w.Header().Set("WWW-Authenticate", d.challenge)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		f := authorizationFields(header)
		ha1 := d.hash(fmt.Sprintf("%s:%s:%s", d.user, f["realm"], d.pass))
		ha2 := d.hash(fmt.Sprintf("%s:%s", r.Method, f["uri"]))
		want := d.hash(fmt.Sprintf("%s:%s:%s:%s:%s:%s", ha1, f["nonce"], f["nc"], f["cnonce"], f["qop"], ha2))
		d.mu.Lock()
		defer d.mu.Unlock()
		if f["username"] != d.user || f["response"] != want {
			d.rejected++
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		d.bodies = append(d.bodies, body)
		_, _ = w.Write([]byte(answer))
	}))
	t.Cleanup(d.Close)
	return d
}

func (d *digestServer) client(user, pass string) *Client {
	return NewClient(d.URL[len("http://"):], WithAuth(user, pass), WithRetry(0, time.Millisecond))
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func md5Hex(s string) string {
	sum := md5.Sum([]byte(s)) // #nosec G401 -- see the import
	return hex.EncodeToString(sum[:])
}

const infoAnswer = `{"id":1,"result":{"id":"` + testDeviceID + `","mac":"F008D1D8B8B8","model":"SPSW-004PE16EU","gen":2,"auth_en":true,"auth_domain":"` + testDeviceID + `"}}`

func TestDigest_AnswersASHA256Challenge(t *testing.T) {
	challenge := `Digest qop="auth", realm="` + testDeviceID + `", nonce="AAAAAABnnonce", algorithm=SHA-256`
	srv := newDigestServer(t, challenge, sha256Hex, `{"id":1,"result":{"restart_required":false}}`)

	_, err := srv.client("admin", "secret").CallConfigSetter(context.Background(), "Sys.SetConfig", map[string]interface{}{"config": map[string]interface{}{}})
	if err != nil {
		t.Fatalf("a SHA-256 challenge was not answered: %v", err)
	}
	if srv.rejected != 0 || len(srv.bodies) != 1 {
		t.Fatalf("rejected=%d accepted=%d, want 0 and 1", srv.rejected, len(srv.bodies))
	}
}

// A plain authenticated read (no SetAuth involved) goes through the same challenge and retry.
func TestDigest_APlainAuthenticatedReadWorks(t *testing.T) {
	challenge := `Digest qop="auth", realm="` + testDeviceID + `", nonce="AAAAAABnnonce", algorithm=SHA-256`
	srv := newDigestServer(t, challenge, sha256Hex, infoAnswer)

	info, err := srv.client("admin", "secret").GetInfo(context.Background())
	if err != nil {
		t.Fatalf("an authenticated read failed: %v", err)
	}
	if info.ID != testDeviceID || !info.AuthEn {
		t.Fatalf("the read answered %+v", info)
	}
	var got struct {
		Method string `json:"method"`
	}
	if len(srv.bodies) != 1 || json.Unmarshal(srv.bodies[0], &got) != nil || got.Method != "Shelly.GetDeviceInfo" {
		t.Fatalf("the authenticated read carried the body %q", srv.bodies)
	}
}

func TestDigest_SendsTheAlgorithmItAnswersWith(t *testing.T) {
	skipWithoutSockets(t)
	var seen string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			w.Header().Set("WWW-Authenticate", `Digest qop="auth", realm="x", nonce="n", algorithm=SHA-256`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		seen = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"id":1,"result":{}}`))
	}))
	defer server.Close()

	c := NewClient(server.URL[len("http://"):], WithAuth("admin", "secret"), WithRetry(0, time.Millisecond))
	if _, err := c.CallConfigSetter(context.Background(), "Sys.SetConfig", nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(seen, "algorithm=SHA-256") {
		t.Fatalf("the Authorization header names no algorithm: %s", seen)
	}
}

func TestDigest_StillAnswersAnMD5Challenge(t *testing.T) {
	challenge := `Digest qop="auth", realm="` + testDeviceID + `", nonce="AAAAAABnnonce"`
	srv := newDigestServer(t, challenge, md5Hex, `{"id":1,"result":{"restart_required":false}}`)

	if _, err := srv.client("admin", "secret").CallConfigSetter(context.Background(), "Sys.SetConfig", nil); err != nil {
		t.Fatalf("an MD5 challenge was not answered: %v", err)
	}
}

// The authenticated retry must carry the same JSON as the first request: a device that sees an
// empty body answers an error, and a fake that ignores the body never shows it.
func TestDigest_RetryCarriesTheRequestBody(t *testing.T) {
	challenge := `Digest qop="auth", realm="` + testDeviceID + `", nonce="AAAAAABnnonce", algorithm=SHA-256`
	srv := newDigestServer(t, challenge, sha256Hex, `{"id":1,"result":{"restart_required":false}}`)

	params := map[string]interface{}{"config": map[string]interface{}{"name": "kitchen"}}
	if _, err := srv.client("admin", "secret").CallConfigSetter(context.Background(), "Sys.SetConfig", params); err != nil {
		t.Fatal(err)
	}
	if len(srv.bodies) != 1 {
		t.Fatalf("authenticated requests: %d, want 1", len(srv.bodies))
	}
	var got struct {
		Method string                 `json:"method"`
		Params map[string]interface{} `json:"params"`
	}
	if err := json.Unmarshal(srv.bodies[0], &got); err != nil {
		t.Fatalf("the authenticated request carries no JSON (%q): %v", srv.bodies[0], err)
	}
	if got.Method != "Sys.SetConfig" || !reflect.DeepEqual(got.Params, params) {
		t.Fatalf("the authenticated request body is %q", srv.bodies[0])
	}
}

func TestDigest_RefusesToRetryWithoutAReplayableBody(t *testing.T) {
	skipWithoutSockets(t)
	hits := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Header().Set("WWW-Authenticate", `Digest qop="auth", realm="x", nonce="n", algorithm=SHA-256`)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	req, err := http.NewRequest(http.MethodPost, server.URL+"/rpc", io.NopCloser(strings.NewReader(`{"a":1}`)))
	if err != nil {
		t.Fatal(err)
	}
	if req.GetBody != nil {
		t.Fatal("the test needs a request with no GetBody")
	}
	if _, err = doRequestWithDigestAuth(server.Client(), req, "admin", "secret"); err == nil {
		t.Fatal("a request with a body that cannot be replayed was retried")
	}
	if hits != 0 {
		t.Fatalf("the server saw %d requests, want 0 (refused before anything is sent)", hits)
	}
}

// rpcRecorder serves GetDeviceInfo and records the parameters of Shelly.SetAuth.
func rpcRecorder(t *testing.T, id string) (*Client, *[]map[string]interface{}) {
	t.Helper()
	skipWithoutSockets(t)
	var calls []map[string]interface{}
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string                 `json:"method"`
			Params map[string]interface{} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		switch req.Method {
		case "Shelly.GetDeviceInfo":
			_, _ = fmt.Fprintf(w, `{"id":1,"result":{"id":%q,"mac":"F008D1D8B8B8","gen":2}}`, id)
		case "Shelly.SetAuth":
			mu.Lock()
			calls = append(calls, req.Params)
			mu.Unlock()
			_, _ = w.Write([]byte(`{"id":1,"result":null}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	return NewClient(server.URL[len("http://"):], WithRetry(0, time.Millisecond)), &calls
}

func TestSetAuth_SendsUserRealmAndHA1(t *testing.T) {
	client, calls := rpcRecorder(t, testDeviceID)

	if err := client.SetAuth(context.Background(), "admin", "secret"); err != nil {
		t.Fatal(err)
	}
	want := map[string]interface{}{"user": "admin", "realm": testDeviceID, "ha1": ha1Vector}
	if len(*calls) != 1 || !reflect.DeepEqual((*calls)[0], want) {
		t.Fatalf("Shelly.SetAuth params = %v, want %v", *calls, want)
	}
}

func TestResetAuth_SendsANullHA1(t *testing.T) {
	client, calls := rpcRecorder(t, testDeviceID)

	if err := client.ResetAuth(context.Background()); err != nil {
		t.Fatal(err)
	}
	want := map[string]interface{}{"user": "admin", "realm": testDeviceID, "ha1": nil}
	if len(*calls) != 1 || !reflect.DeepEqual((*calls)[0], want) {
		t.Fatalf("Shelly.SetAuth params = %v, want %v", *calls, want)
	}
}

func TestSetAuth_SendsNothingWithoutADeviceID(t *testing.T) {
	client, calls := rpcRecorder(t, "")

	if err := client.SetAuth(context.Background(), "admin", "secret"); err == nil {
		t.Fatal("SetAuth succeeded with no device id to use as the realm")
	}
	if err := client.ResetAuth(context.Background()); err == nil {
		t.Fatal("ResetAuth succeeded with no device id to use as the realm")
	}
	if len(*calls) != 0 {
		t.Fatalf("Shelly.SetAuth was called %d times", len(*calls))
	}
}

func TestSetAuth_RefusesAUserOtherThanAdmin(t *testing.T) {
	client, calls := rpcRecorder(t, testDeviceID)

	if err := client.SetAuth(context.Background(), "operator", "secret"); err == nil {
		t.Fatal("SetAuth accepted a user other than admin")
	}
	if len(*calls) != 0 {
		t.Fatalf("Shelly.SetAuth was called %d times", len(*calls))
	}
}

// A retry of Shelly.SetAuth would go out with the old login. If the first attempt changed the
// login and only its answer was lost, the retry gets 401 and the lost answer reads as a refusal.
// Neither a lost answer nor a 5xx is retried.
func TestSetAuth_IsSentOnceWhateverTheClientRetries(t *testing.T) {
	skipWithoutSockets(t)
	for _, tc := range []struct {
		name   string
		answer func(w http.ResponseWriter)
	}{
		{"a lost answer", func(w http.ResponseWriter) {
			conn, _, err := w.(http.Hijacker).Hijack()
			if err == nil {
				_ = conn.Close()
			}
		}},
		{"a 503", func(w http.ResponseWriter) { w.WriteHeader(http.StatusServiceUnavailable) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var mu sync.Mutex
			setAuthCalls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				if strings.Contains(string(body), "Shelly.GetDeviceInfo") {
					_, _ = fmt.Fprintf(w, `{"id":1,"result":{"id":%q}}`, testDeviceID)
					return
				}
				mu.Lock()
				setAuthCalls++
				mu.Unlock()
				tc.answer(w)
			}))
			defer server.Close()

			client := NewClient(server.URL[len("http://"):], WithRetry(3, time.Millisecond))
			if err := client.SetAuth(context.Background(), "admin", "secret"); err == nil {
				t.Fatal("expected an error")
			}
			if err := client.ResetAuth(context.Background()); err == nil {
				t.Fatal("expected an error")
			}
			mu.Lock()
			calls := setAuthCalls
			mu.Unlock()
			if calls != 2 {
				t.Fatalf("Shelly.SetAuth requests = %d, want 2 (one SetAuth and one ResetAuth, no retry)", calls)
			}
		})
	}
}

// A reply that ends before its declared length is a device error that holds the read error, which
// is how the apply layer tells it from a refusal the device wrote.
func TestSetAuth_ATruncatedAnswerHoldsTheReadError(t *testing.T) {
	skipWithoutSockets(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), "Shelly.GetDeviceInfo") {
			_, _ = fmt.Fprintf(w, `{"id":1,"result":{"id":%q}}`, testDeviceID)
			return
		}
		w.Header().Set("Content-Length", "100")
		_, _ = w.Write([]byte(`{"id":1,"res`))
	}))
	defer server.Close()

	client := NewClient(server.URL[len("http://"):], WithRetry(0, time.Millisecond))
	err := client.SetAuth(context.Background(), "admin", "secret")

	var deviceErr *shelly.DeviceError
	if !errors.As(err, &deviceErr) || deviceErr.Err == nil {
		t.Fatalf("want a device error holding the read error, got %v", err)
	}
}
