package notification

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/logging"
)

const (
	urlToken  = "LEAKTOKEN652"
	urlPwd    = "LEAKPWD652"
	urlQuery  = "LEAKQUERY652"
	urlRedir  = "LEAKREDIRECT652"
	urlParsed = "LEAKPARSE652"
)

var urlLeaks = []string{urlToken, urlPwd, urlQuery, urlRedir, urlParsed}

func assertNoURLText(t *testing.T, name string, err error) {
	t.Helper()
	require.Error(t, err, name)
	for _, leak := range urlLeaks {
		assert.NotContains(t, err.Error(), leak, "%s: %v", name, err)
	}
	assert.NotContains(t, err.Error(), "127.0.0.1", name)
	assert.NotContains(t, err.Error(), "http://", name)
}

func closedLoopbackPort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := l.Addr().(*net.TCPAddr).Port
	require.NoError(t, l.Close())
	return port
}

// NewService rewrites the old rows and logs how many it changed, never what they held; a second start
// changes and logs nothing.
func TestNewService_RewritesOldRowsAndLogsTheCount(t *testing.T) {
	_, db, cleanup := setupSimpleTestService(t)
	defer cleanup()

	channel := &NotificationChannel{Name: "hook", Type: "webhook", Enabled: true}
	require.NoError(t, db.Create(channel).Error)
	rule := &NotificationRule{Name: "r", Enabled: true, ChannelID: channel.ID, AlertLevel: "all"}
	require.NoError(t, db.Create(rule).Error)
	h := &NotificationHistory{
		RuleID: rule.ID, ChannelID: channel.ID, TriggerType: "test", Status: "failed", CreatedAt: time.Now(),
		Error: `failed to send webhook: Post "http://h.example/` + urlToken + `": refused`,
	}
	require.NoError(t, db.Create(h).Error)

	start := func() string {
		logFile := filepath.Join(t.TempDir(), "notification.log")
		logger, err := logging.New(logging.Config{Level: "debug", Format: "text", Output: logFile})
		require.NoError(t, err)
		NewService(db, logger, EmailSMTPConfig{})
		logged, err := os.ReadFile(logFile)
		require.NoError(t, err)
		return string(logged)
	}

	first := start()
	assert.Contains(t, first, "Rewrote old delivery error texts")
	assert.Contains(t, first, "rows=1")
	assert.NotContains(t, first, urlToken)
	var stored NotificationHistory
	require.NoError(t, db.First(&stored, h.ID).Error)
	assert.Equal(t, "failed to send webhook: details removed", stored.Error)

	assert.NotContains(t, start(), "Rewrote old delivery error texts", "a second start changes nothing")
}

// A query that fails is reported, and NewService warns and carries on: a closed database stands for a lost
// connection, a dropped table for a database that was never migrated.
func TestRemoveLegacyDeliveryErrors_ReportsAFailedQuery(t *testing.T) {
	_, db, cleanup := setupSimpleTestService(t)
	defer cleanup()
	logFile := filepath.Join(t.TempDir(), "notification.log")
	logger, err := logging.New(logging.Config{Level: "debug", Format: "text", Output: logFile})
	require.NoError(t, err)

	require.NoError(t, db.Migrator().DropTable(&NotificationHistory{}))
	changed, err := RemoveLegacyDeliveryErrors(db)
	require.Error(t, err, "a missing table is an error, not nothing to do")
	assert.EqualValues(t, 0, changed)

	require.NotPanics(t, func() { NewService(db, logger, EmailSMTPConfig{}) })
	logged, err := os.ReadFile(logFile)
	require.NoError(t, err)
	assert.Contains(t, string(logged), "Could not rewrite old delivery error texts")

	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	_, err = RemoveLegacyDeliveryErrors(db)
	require.Error(t, err, "a lost connection is an error")
}

// A successful webhook send logs the channel and the status, and no part of the URL that carries the token.
func TestSendWebhook_SuccessLogCarriesNoURL(t *testing.T) {
	_, db, cleanup := setupSimpleTestService(t)
	defer cleanup()

	logFile := filepath.Join(t.TempDir(), "notification.log")
	logger, err := logging.New(logging.Config{Level: "debug", Format: "text", Output: logFile})
	require.NoError(t, err)
	svc := NewService(db, logger, EmailSMTPConfig{})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))
	t.Cleanup(srv.Close)
	channel := &NotificationChannel{
		Name: "hook", Type: "webhook", Enabled: true,
		Config: json.RawMessage(`{"url":"` + srv.URL + `/` + urlToken + `?t=` + urlQuery + `"}`),
	}
	require.NoError(t, svc.CreateChannel(channel))

	require.NoError(t, svc.TestChannel(context.Background(), channel.ID))

	logged, err := os.ReadFile(logFile)
	require.NoError(t, err)
	assert.Contains(t, string(logged), "Sent webhook notification", "the premise: the success line was written")
	assert.NotContains(t, string(logged), urlToken)
	assert.NotContains(t, string(logged), urlQuery)
	assert.NotContains(t, string(logged), srv.URL)
}

// Real errors from net/http, each of which quotes a request URL that carries a token.
func TestRequestError_KeepsNoURLText(t *testing.T) {
	client := &http.Client{Timeout: 200 * time.Millisecond}

	do := func(t *testing.T, rawURL string, c *http.Client) error {
		t.Helper()
		req, err := http.NewRequestWithContext(context.Background(), "POST", rawURL, nil)
		if err != nil {
			return requestError("request not built", err)
		}
		resp, err := c.Do(req)
		if err == nil {
			_ = resp.Body.Close()
			return nil
		}
		return requestError("request failed", err)
	}
	slow := func(t *testing.T) *httptest.Server {
		t.Helper()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { time.Sleep(time.Second) }))
		t.Cleanup(srv.Close)
		return srv
	}

	t.Run("connection refused", func(t *testing.T) {
		raw := "http://hook:" + urlPwd + "@127.0.0.1:" + strconv.Itoa(closedLoopbackPort(t)) + "/" + urlToken + "?t=" + urlQuery
		err := do(t, raw, client)
		assertNoURLText(t, "refused", err)
		assert.Equal(t, "request failed: Post connection refused", err.Error())
	})

	t.Run("timeout", func(t *testing.T) {
		err := do(t, slow(t).URL+"/"+urlToken+"?t="+urlQuery, client)
		assertNoURLText(t, "timeout", err)
		assert.Equal(t, "request failed: Post timed out", err.Error())
	})

	t.Run("canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		req, err := http.NewRequestWithContext(ctx, "POST", slow(t).URL+"/"+urlToken, nil)
		require.NoError(t, err)
		_, doErr := http.DefaultClient.Do(req)
		got := requestError("request failed", doErr)
		assertNoURLText(t, "canceled", got)
		assert.Equal(t, "request failed: Post canceled", got.Error())
	})

	t.Run("host not found", func(t *testing.T) {
		err := do(t, "http://no-such-host.invalid/"+urlToken+"?t="+urlQuery, client)
		assertNoURLText(t, "dns", err)
		// A sandbox without a resolver fails the lookup in another way or times out.
		assert.Contains(t, []string{"request failed: Post host not found", "request failed: Post failed", "request failed: Post timed out"}, err.Error())
	})

	t.Run("a missing name is told from a failing resolver", func(t *testing.T) {
		wrap := func(dns *net.DNSError) error {
			return &url.Error{Op: "Post", URL: "http://h.example/" + urlToken, Err: &net.OpError{Op: "dial", Net: "tcp", Err: dns}}
		}
		missing := requestError("request failed", wrap(&net.DNSError{Err: "no such host", Name: "h.example", IsNotFound: true}))
		assert.Equal(t, "request failed: Post host not found", missing.Error())

		servfail := requestError("request failed", wrap(&net.DNSError{Err: "server misbehaving", Name: "h.example", Server: "10.0.0.1:53"}))
		assert.Equal(t, "request failed: Post failed", servfail.Error())
		assertNoURLText(t, "servfail", servfail)
		assert.NotContains(t, servfail.Error(), "10.0.0.1")
	})

	t.Run("a redirect that cannot be followed quotes more URLs", func(t *testing.T) {
		dead := strconv.Itoa(closedLoopbackPort(t))
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "http://127.0.0.1:"+dead+"/"+urlRedir+"?t="+urlQuery, http.StatusFound)
		}))
		t.Cleanup(srv.Close)
		err := do(t, srv.URL+"/"+urlToken, client)
		assertNoURLText(t, "redirect", err)
	})

	t.Run("a URL that does not parse", func(t *testing.T) {
		err := do(t, "http://host:bad-port/"+urlParsed+"?t="+urlQuery, client)
		assertNoURLText(t, "parse", err)
		assert.Equal(t, "request not built: parse failed", err.Error())
	})

	t.Run("an error that is no url.Error keeps its text", func(t *testing.T) {
		_, err := http.NewRequestWithContext(context.Background(), "BAD METHOD", "http://h.example/x", nil)
		require.Error(t, err)
		got := requestError("request not built", err)
		assert.Contains(t, got.Error(), "invalid method")
		assert.NotContains(t, got.Error(), "h.example")
	})
}
