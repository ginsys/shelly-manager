package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A NUL byte in a register is a 400 at the handler, before the store (ginsys/shelly-manager#852).
// PostgreSQL cannot hold a NUL in text, so on that server the insert fails in the driver and the handler
// would answer 500. The proof does not need that server: the handler is tested on a database that errors
// on every call, so a 400 there means the store was not reached, whatever the provider.

// closedDBServer is idServer with its database closed after the router was built: every store call
// answers an error, which the handler turns into a 500.
func closedDBServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv, db := idServer(t)
	sqlDB, err := db.GetDB().DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	return srv
}

type nulRegister struct {
	name  string
	id    string // the id the case sends; the row it must not leave
	field string // the start of the 400 message, naming the field that holds the NUL
	body  map[string]any
}

func nulRegisters() []nulRegister {
	return []nulRegister{
		{"the id", "nul\x00id", "Agent id must not contain a NUL character",
			map[string]any{"id": "nul\x00id", "hostname": "h"}},
		{"the id with the exempt prefix", "provisioner-a\x00b", "Agent id must not contain a NUL character",
			map[string]any{"id": "provisioner-a\x00b", "hostname": "h"}},
		{"the hostname", "plain-1", "Agent hostname must not contain a NUL character",
			map[string]any{"id": "plain-1", "hostname": "h\x00"}},
		{"the version", "plain-1", "Agent version must not contain a NUL character",
			map[string]any{"id": "plain-1", "hostname": "h", "version": "v\x00"}},
		{"a capability", "plain-1", "Agent capabilities must not contain a NUL character",
			map[string]any{"id": "plain-1", "hostname": "h", "capabilities": []string{"a\x00"}}},
		{"a metadata key", "plain-1", "Agent metadata must not contain a NUL character",
			map[string]any{"id": "plain-1", "hostname": "h", "metadata": map[string]string{"k\x00": "v"}}},
		{"a metadata value", "plain-1", "Agent metadata must not contain a NUL character",
			map[string]any{"id": "plain-1", "hostname": "h", "metadata": map[string]string{"k": "v\x00"}}},
	}
}

func postRegister(t *testing.T, srv *httptest.Server, body map[string]any, headers map[string]string) (int, string) {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	return idCall(t, srv, "POST", "/api/v1/provisioner/agents/register", string(raw), headers)
}

// Checks 1, 2, 3 and 6: each case is a 400 that names its field and never echoes the value, with the
// agent key and with the admin key, and the database is never asked.
func TestKeylessRegister_NULIsRefusedBeforeTheStore(t *testing.T) {
	srv := closedDBServer(t)

	// The control: a clean register on the same server reaches the store and fails there. Without it a
	// 400 above would prove nothing about where the refusal came from.
	code, body := postRegister(t, srv, map[string]any{"id": "plain-1", "hostname": "h"}, idAgentHdr(""))
	require.Equal(t, http.StatusInternalServerError, code, "the closed database must fail wherever the store is reached: %s", body)

	for _, c := range nulRegisters() {
		for name, hdr := range map[string]map[string]string{"agent key": idAgentHdr(""), "admin key": idAdminHdr()} {
			code, body := postRegister(t, srv, c.body, hdr)
			assert.Equal(t, http.StatusBadRequest, code, "%s with the %s: %s", c.name, name, body)
			assert.Contains(t, body, c.field, "%s with the %s", c.name, name)
			assert.NotContains(t, body, `\u0000`, "%s with the %s: the value must not be echoed", c.name, name)
		}
	}
}

// Check 1: the refusal leaves no row.
func TestKeylessRegister_NULLeavesNoRow(t *testing.T) {
	srv, db := idServer(t)
	for _, c := range nulRegisters() {
		code, body := postRegister(t, srv, c.body, idAgentHdr(""))
		assert.Equal(t, http.StatusBadRequest, code, "%s: %s", c.name, body)
		assert.EqualValues(t, 0, idRows(t, db, c.id), "%s must leave no row", c.name)
	}
}

// Check 4: a keyed register with a NUL in its id is a 400 too. The id is compared with the one the key
// derives before the store is read, so a NUL id never gets further.
func TestKeyedRegister_NULInTheIDIs400BeforeTheStore(t *testing.T) {
	srv := closedDBServer(t)
	priv := kdKey(0xb1)

	code, _, body := kdRegister(t, srv, priv, kdID(priv), "", "")
	require.Equal(t, http.StatusInternalServerError, code, "control: a good keyed register reaches the store and fails there: %s", body)

	code, _, body = kdRegister(t, srv, priv, kdID(priv)+"\x00", "", "")
	assert.Equal(t, http.StatusBadRequest, code, body)
	assert.NotContains(t, body, `\u0000`, "the value must not be echoed")
}
