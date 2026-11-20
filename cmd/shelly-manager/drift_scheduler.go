package main

import (
	"context"
	"errors"
	"net/http"
	"time"

	"gorm.io/gorm"

	"github.com/ginsys/shelly-manager/internal/config"
	"github.com/ginsys/shelly-manager/internal/configuration"
	"github.com/ginsys/shelly-manager/internal/logging"
)

// shutdownTimeout bounds how long a stop waits for requests in flight.
const shutdownTimeout = 10 * time.Second

// startScheduler starts the drift scheduler when drift.scheduler_enabled is set, and builds nothing
// when it is not: the scheduler reads devices on a timer, so an operator turns it on.
func startScheduler(ctx context.Context, cfg *config.Config, db *gorm.DB, svc *configuration.Service, logger *logging.Logger) (*configuration.Scheduler, error) {
	if cfg == nil || !cfg.Drift.SchedulerEnabled {
		return nil, nil
	}
	scheduler := configuration.NewScheduler(db, svc, logger)
	if err := scheduler.Start(ctx); err != nil {
		return nil, err
	}
	return scheduler, nil
}

// serve runs the server until it fails or ctx ends. When ctx ends it calls stop (the scheduler: the
// run in flight stops between devices, waiting at most the scheduler's own configuration.DriftStopWait for a
// device read already started), then shuts the server down within the timeout; the two together stay
// under Kubernetes' 30 second grace period. A server closed by that
// shutdown is a clean end, not a failure.
func serve(ctx context.Context, server *http.Server, stop func(), timeout time.Duration) error {
	failed := make(chan error, 1)
	go func() { failed <- server.ListenAndServe() }()

	select {
	case err := <-failed:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
	}

	stop()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	err := server.Shutdown(shutdownCtx)
	// ListenAndServe returns http.ErrServerClosed once Shutdown has begun; anything else is its own failure.
	if serveErr := <-failed; err == nil && !errors.Is(serveErr, http.ErrServerClosed) {
		err = serveErr
	}
	return err
}
