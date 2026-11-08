package discovery

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/mdns"
)

// The mDNS client sends a ServiceEntry on the channel at the first record of a service and
// writes more fields into the same entry at each later record. A reader that looks at the
// entry as soon as it arrives therefore races with the client. This test plays the client's
// part with a fake query, so it needs no device and no network: it sends an entry and then
// keeps writing its fields until the query returns. Run under `go test -race`, a scanner
// that reads the entry before the query returns is reported as a data race.
func TestDiscoverDevicesReadsEntriesAfterTheQueryReturns(t *testing.T) {
	original := mdnsQuery
	t.Cleanup(func() { mdnsQuery = original })

	queries := 0
	mdnsQuery = func(params *mdns.QueryParam) error {
		queries++
		// The name carries no "shelly", so the scanner has to read Host and InfoFields to
		// rule the entry out. A scanner that did not read them would not be tested.
		entry := &mdns.ServiceEntry{Name: "printer._http._tcp.local."}
		params.Entries <- entry
		for i := 0; i < 200; i++ {
			entry.Host = fmt.Sprintf("printer-%d.local.", i)
			entry.InfoFields = []string{fmt.Sprintf("note=%d", i)}
			time.Sleep(100 * time.Microsecond)
		}
		return nil
	}

	devices, err := NewMDNSScanner(time.Second).DiscoverDevices(context.Background())
	if err != nil {
		t.Fatalf("DiscoverDevices returned an error: %v", err)
	}
	if len(devices) != 0 {
		t.Errorf("expected no device for a non-Shelly entry, got %d", len(devices))
	}
	if queries != 1 {
		t.Errorf("expected one query, got %d", queries)
	}
}
