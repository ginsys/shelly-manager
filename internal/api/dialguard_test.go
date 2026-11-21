package api

import (
	"context"
	"errors"
	"testing"

	"github.com/ginsys/shelly-manager/internal/testutil/dialguard"
)

func TestMain(m *testing.M) {
	// A create probes the address it is given (ginsys/shelly-manager#811), and the tests of this package
	// add devices at addresses that are not loopback. The probe fails here, so a create keeps its
	// Gen1 outcome and sends nothing; a test that needs another answer sets detectGeneration itself.
	detectGeneration = func(context.Context, string) (int, error) {
		return 0, errors.New("no device is probed in a test")
	}
	dialguard.Main(m)
}
