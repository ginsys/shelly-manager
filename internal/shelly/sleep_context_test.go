package shelly

import (
	"context"
	"testing"
	"time"
)

func TestSleepContext_WaitsTheFullDelayWhenTheContextLives(t *testing.T) {
	start := time.Now()

	if !SleepContext(context.Background(), 50*time.Millisecond) {
		t.Fatal("SleepContext reported an ended context for a live one")
	}
	if took := time.Since(start); took < 50*time.Millisecond {
		t.Errorf("SleepContext returned after %v, before the 50ms delay", took)
	}
}

func TestSleepContext_ReturnsAtOnceWhenTheContextIsAlreadyEnded(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()

	if SleepContext(ctx, time.Minute) {
		t.Fatal("SleepContext waited out a minute on a cancelled context")
	}
	if took := time.Since(start); took > time.Second {
		t.Errorf("SleepContext took %v on a cancelled context, want about 0", took)
	}
}

func TestSleepContext_ReturnsWhenTheContextEndsDuringTheWait(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()

	if SleepContext(ctx, time.Minute) {
		t.Fatal("SleepContext reported a full wait for a context that ended")
	}
	if took := time.Since(start); took > time.Second {
		t.Errorf("SleepContext took %v for a 50ms deadline", took)
	}
}
