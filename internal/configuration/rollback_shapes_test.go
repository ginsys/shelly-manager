package configuration

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The answer of a rollback to each shape of stored version, before and after the merge stopped making a
// section that nothing was set in (ginsys/shelly-manager#843). The expected values were taken by running
// this table on origin/main e80826f, not by hand; no row changes its answer.

func rollbackAnswer(t *testing.T, svc *ConfigurationService, version uint) string {
	t.Helper()
	_, err := svc.RollbackDesired(1, version, nil)
	if err == nil {
		return "ok"
	}
	var rerr *RollbackError
	if errors.As(err, &rerr) && rerr.Kind == RollbackLoss {
		return "loss " + strings.Join(rerr.Paths, ",")
	}
	return "other: " + err.Error()
}

// besideARealSetting adds a real setting next to the shape, as a version of a device that has one.
func besideARealSetting(shape string) string {
	return strings.TrimSuffix(shape, "}") + `,"mqtt":{"server":"a.example"}}`
}

var rollbackShapes = []struct {
	name, version, want string
}{
	{"an empty section", `{"system":{"sntp":{}}}`, "ok"},
	{"an empty string", `{"system":{"sntp":{"server":""}}}`, "loss system.sntp.server"},
	{"a sleep mode that is only false", `{"system":{"sleep_mode":{"enable":false}}}`, "ok"},
	{"a ble that is only false", `{"system":{"device":{"ble":{"enable":false}}}}`, "ok"},
	{"a sleep mode false and a zero period", `{"system":{"sleep_mode":{"enable":false,"period":0}}}`, "loss system.sleep_mode.period"},
	{"an unknown empty object", `{"made_up":{}}`, "loss made_up"},
	{"an unknown object of a zero", `{"made_up":{"x":0}}`, "loss made_up"},
	{"a section in another case", `{"System":{"sntp":{}}}`, "loss System"},
	{"a null section", `{"system":null}`, "loss system"},
	{"a null in a section", `{"system":{"sntp":null}}`, "loss system.sntp"},
	{"an empty list of a real field", `{"relay":{"relays":[]}}`, "loss relay.relays"},
	// The dropped section sits inside a section the merge keeps, so the typed tree is asked below the root.
	{"an empty section beside a real value in its parent", `{"system":{"sntp":{},"sleep_mode":{"enable":true}}}`, "ok"},
	{"an empty string beside a real value in its parent", `{"system":{"sntp":{"server":""},"sleep_mode":{"enable":true}}}`, "loss system.sntp.server"},
}

func TestRollbackDesired_EachShapeOfStoredVersionKeepsItsAnswer(t *testing.T) {
	for _, shape := range rollbackShapes {
		for _, beside := range []bool{false, true} {
			version := shape.version
			name := shape.name
			if beside {
				version = besideARealSetting(version)
				name += " beside a real setting"
			}
			t.Run(name, func(t *testing.T) {
				db := historyDB(t)
				rollbackDevice(t, db, 1, "[]", `{}`, `{}`)
				id := addVersion(t, db, 1, str(version))
				got := rollbackAnswer(t, rollbackService(db), id)
				t.Logf("ROW %q => %s", version, got)
				assert.Equal(t, shape.want, got)
			})
		}
	}
}

// dropsSections is a merger that loses a section whose value is real, as a defective merge would.
type dropsSections struct{}

func (dropsSections) Merge([]ConfigLayer) (*MergeResult, error) {
	return &MergeResult{Config: &DeviceConfiguration{}, Sources: map[string]string{}}, nil
}

func TestRollbackDesired_ASectionWithARealValueTheMergeLosesIsReportedWhole(t *testing.T) {
	for _, version := range []string{
		`{"system":{"sntp":{"server":"time.example.org"}}}`,
		`{"system":{"sntp":{"server":"time.example.org"},"sleep_mode":{"enable":false}}}`,
	} {
		t.Run(version, func(t *testing.T) {
			db := historyDB(t)
			rollbackDevice(t, db, 1, "[]", `{}`, `{}`)
			id := addVersion(t, db, 1, str(version))
			svc := NewConfigurationService(NewGormConfigRepository(db, nil), dropsSections{}, nil)
			assert.Equal(t, "loss system", rollbackAnswer(t, svc, id))
			require.Equal(t, `{}`, deviceRow(t, db, 1).DesiredConfig)
		})
	}
}
