package provider

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mysqlReproDSNs hold operator text the MySQL driver quotes in its own errors:
// its DSN parser names the value it refused (go-sql-driver dsn.go), and its dial
// error names the address it tried.
var mysqlReproDSNs = map[string]string{
	"an unknown tls name":                          "app:pw@tcp(127.0.0.1:1)/shelly?tls=LEAKWORD7",
	"a bad boolean value":                          "app:pw@tcp(127.0.0.1:1)/shelly?parseTime=LEAKWORD7",
	"a bad bool, second place":                     "app:pw@tcp(127.0.0.1:1)/shelly?tls=false&interpolateParams=LEAKWORD7",
	"a refused port with the password in the user": "LEAKWORD7:LEAKWORD7@tcp(127.0.0.1:1)/shelly?timeout=1s",
	"an address that never resolves":               "app:pw@tcp(LEAKWORD7.invalid:3306)/shelly?timeout=1s",
}

func TestMySQLConnectError_HoldsNoDSNText(t *testing.T) {
	for name, dsn := range mysqlReproDSNs {
		t.Run(name, func(t *testing.T) {
			logger, readLog := fileLogger(t)
			p := NewMySQLProvider(logger)

			err := p.Connect(DatabaseConfig{Provider: "mysql", DSN: dsn, LogLevel: "info"})

			require.Error(t, err, "nothing listens on these addresses")
			assert.NotContains(t, err.Error(), leakWord, "the returned error")
			assert.NotContains(t, fmt.Sprintf("%+v", err), leakWord, "the error with %+v")
			assert.NotContains(t, readLog(), leakWord, "the captured log")
		})
	}
}
