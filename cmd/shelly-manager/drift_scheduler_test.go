package main

import (
	"context"
	"net"
	"net/http"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/ginsys/shelly-manager/internal/config"
	"github.com/ginsys/shelly-manager/internal/configuration"
	"github.com/ginsys/shelly-manager/internal/logging"
)

func driftTestDB(t *testing.T, migrate bool) (*gorm.DB, *configuration.Service, *logging.Logger) {
	t.Helper()
	testLog, err := logging.New(logging.Config{Level: "error", Format: "text"})
	if err != nil {
		t.Fatalf("logger: %v", err)
	}
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	svc := configuration.NewService(db, testLog)
	if migrate {
		if err := db.AutoMigrate(&configuration.DriftDetectionSchedule{}, &configuration.DriftDetectionRun{}, &configuration.DriftScheduleDevice{}); err != nil {
			t.Fatalf("migrate: %v", err)
		}
	}
	return db, svc, testLog
}

func driftConfig(enabled bool) *config.Config {
	c := &config.Config{}
	c.Drift.SchedulerEnabled = enabled
	return c
}

// With the gate off nothing is built and no goroutine starts: the scheduler contacts devices on a
// timer, so it never starts by itself.
func TestStartScheduler_GateOffBuildsNothing(t *testing.T) {
	db, svc, testLog := driftTestDB(t, true)
	before := runtime.NumGoroutine()

	scheduler, err := startScheduler(context.Background(), driftConfig(false), db, svc, testLog)

	if err != nil || scheduler != nil {
		t.Fatalf("gate off: got scheduler %v, err %v", scheduler, err)
	}
	if after := runtime.NumGoroutine(); after != before {
		t.Errorf("gate off started goroutines: %d before, %d after", before, after)
	}
	if s, err := startScheduler(context.Background(), nil, db, svc, testLog); s != nil || err != nil {
		t.Errorf("a nil config must read as gate off: %v, %v", s, err)
	}
}

func TestStartScheduler_GateOnStartsAndStopsTwice(t *testing.T) {
	db, svc, testLog := driftTestDB(t, true)

	scheduler, err := startScheduler(context.Background(), driftConfig(true), db, svc, testLog)
	if err != nil || scheduler == nil {
		t.Fatalf("gate on: got scheduler %v, err %v", scheduler, err)
	}
	if !scheduler.IsRunning() {
		t.Error("the scheduler is not running after start")
	}
	for i := 0; i < 2; i++ {
		if err := scheduler.Stop(); err != nil {
			t.Fatalf("stop %d: %v", i+1, err)
		}
	}
	if scheduler.IsRunning() {
		t.Error("the scheduler still runs after stop")
	}
}

// A schedule table the first read cannot reach stops the start-up with the error, never a silent
// scheduler that runs nothing.
func TestStartScheduler_GateOnWithUnreadableTableFails(t *testing.T) {
	db, svc, testLog := driftTestDB(t, false)

	scheduler, err := startScheduler(context.Background(), driftConfig(true), db, svc, testLog)

	if err == nil {
		t.Fatal("expected an error")
	}
	if scheduler != nil {
		t.Error("a failed start returned a scheduler")
	}
}

// A signal ends the server cleanly: the scheduler is stopped first, and the server's own close is
// not a failure.
func TestServe_ContextEndStopsSchedulerThenServer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var stopped atomic.Int32
	server := newHTTPServerWith("127.0.0.1:0", http.NewServeMux(), testHeaderTimeout, testIdleTimeout)
	done := make(chan error, 1)
	go func() { done <- serve(ctx, server, func() { stopped.Add(1) }, time.Second) }()

	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("a signal ended the server with an error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("serve did not return after the context ended")
	}
	if stopped.Load() != 1 {
		t.Errorf("the scheduler was stopped %d times, want 1", stopped.Load())
	}
}

// A server that cannot listen is a failure, and the scheduler is left to the caller.
func TestServe_ListenFailureIsReported(t *testing.T) {
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = occupied.Close() }()
	server := newHTTPServerWith(occupied.Addr().String(), http.NewServeMux(), testHeaderTimeout, testIdleTimeout)

	err = serve(context.Background(), server, func() { t.Error("stop called for a server that never ran") }, time.Second)

	if err == nil {
		t.Fatal("expected the listen error")
	}
}
