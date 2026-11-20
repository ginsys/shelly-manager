package gen1

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ginsys/shelly-manager/internal/shelly"
)

// A device that answers 500 makes the client retry after a delay. The delay waits on the caller's
// context, so a caller whose deadline has passed gets the answer at once and not after the sum of the
// delays (three retries of 1 s here, ginsys/shelly-manager#755). The failure it gets is the 500 the
// device sent, which proves the device was reachable.
func TestRetryLoops_StopWhenTheContextEnds(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "busy", http.StatusInternalServerError)
	}))
	defer server.Close()
	client := NewClient(strings.TrimPrefix(server.URL, "http://"), WithRetry(3, time.Second))

	calls := map[string]func(ctx context.Context) error{
		"a read (getJSON)": func(ctx context.Context) error {
			_, err := client.GetStatus(ctx)
			return err
		},
		"a write (postForm)": func(ctx context.Context) error {
			return client.SetInputSettings(ctx, 0, map[string]interface{}{"btn_type": "toggle"})
		},
	}

	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			start := time.Now()

			err := call(ctx)

			if took := time.Since(start); took > 600*time.Millisecond {
				t.Errorf("the call took %v after its 100ms context ended, want it to stop at once", took)
			}
			var answered *shelly.DeviceError
			if !errors.As(err, &answered) || answered.StatusCode != http.StatusInternalServerError {
				t.Errorf("error = %v, want the device's 500 as a *shelly.DeviceError", err)
			}
		})
	}
}

// The second attempt gets its 200 headers and then the body stalls until the deadline. The device
// did answer, with a 500 the first time, so the call must not be classed as unanswered.
func TestRetryLoops_AnEarlierAnswerSurvivesAStalledBody(t *testing.T) {
	var calls atomic.Int32
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			http.Error(w, "busy", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{"))
		w.(http.Flusher).Flush()
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer server.Close()
	defer close(release)
	client := NewClient(strings.TrimPrefix(server.URL, "http://"), WithRetry(2, 10*time.Millisecond))

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	_, err := client.GetStatus(ctx)

	var answered *shelly.DeviceError
	if !errors.As(err, &answered) || answered.StatusCode != http.StatusInternalServerError {
		t.Errorf("error = %v, want the device's earlier 500 as a *shelly.DeviceError", err)
	}
	if shelly.IsUnanswered(err) {
		t.Errorf("error = %v is classed as unanswered although the device answered", err)
	}
}
