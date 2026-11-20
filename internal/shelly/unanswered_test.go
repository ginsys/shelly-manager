package shelly

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"syscall"
	"testing"
)

type timeoutNetError struct{}

func (timeoutNetError) Error() string   { return "i/o timeout" }
func (timeoutNetError) Timeout() bool   { return true }
func (timeoutNetError) Temporary() bool { return true }

func TestIsUnanswered(t *testing.T) {
	refused := &net.OpError{Op: "dial", Err: syscall.ECONNREFUSED}
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"deadline", context.DeadlineExceeded, true},
		{"deadline wrapped by the client", &DeviceError{IP: "10.0.0.9", Err: fmt.Errorf("get: %w", context.DeadlineExceeded)}, true},
		{"deadline inside a url error", &url.Error{Op: "Get", URL: "http://10.0.0.9/status", Err: context.DeadlineExceeded}, true},
		{"network timeout", timeoutNetError{}, true},
		{"dial timeout", &net.OpError{Op: "dial", Err: timeoutNetError{}}, true},
		{"refused connection", refused, false},
		{"cancelled by the caller", context.Canceled, false},
		{"plain error", errors.New("boom"), false},
		{"auth failure", ErrAuthRequired, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := IsUnanswered(c.err); got != c.want {
				t.Fatalf("IsUnanswered(%v) = %v, want %v", c.err, got, c.want)
			}
		})
	}
}
