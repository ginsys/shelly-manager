package api

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/database"
)

// idRows counts the registry rows with this id.
func idRows(t *testing.T, db *database.Manager, id string) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.GetDB().Model(&database.ProvisionerAgent{}).Where("agent_id = ?", id).Count(&n).Error)
	return n
}

// An id that starts with "key-" in any case is the shape of a keyed agent id. A keyless register of one
// is refused before the store, so the shared key cannot reserve an id a key will derive.
func TestProvisionerEnrolment_KeyPrefixIsRefusedInAnyCase(t *testing.T) {
	srv, db := idServer(t)
	ids := []string{"key-an-agent-id-a-key-would-derive", "KEY-", "key-"}
	for mask := 0; mask < 8; mask++ { // every upper and lower case form of the three letters
		prefix := []byte("key")
		for i := range prefix {
			if mask&(1<<i) != 0 {
				prefix[i] -= 'a' - 'A'
			}
		}
		ids = append(ids, string(prefix)+"-abc")
	}
	for _, id := range ids {
		for name, hdr := range map[string]map[string]string{"agent key": idAgentHdr(""), "admin key": idAdminHdr()} {
			code, _, body := idRegister(t, srv, id, hdr)
			assert.Equal(t, http.StatusBadRequest, code, "%q with the %s: %s", id, name, body)
		}
		assert.EqualValues(t, 0, idRows(t, db, id), "%q must leave no row", id)
	}
}

func TestProvisionerEnrolment_OtherIdsKeepRegistering(t *testing.T) {
	srv, _ := idServer(t)
	for _, id := range []string{"key", "keyboard", "ke-y", "monkey-1", "Key", "provisioner-host-1"} {
		code, _, body := idRegister(t, srv, id, idAgentHdr(""))
		assert.Equal(t, http.StatusCreated, code, "%q: %s", id, body)
	}
}

// The fold is over ASCII bytes. strings.EqualFold would match U+212A (the Kelvin sign) to "k".
func TestProvisionerEnrolment_PrefixFoldIsASCIIOnly(t *testing.T) {
	assert.True(t, hasKeyPrefix("key-x"))
	assert.True(t, hasKeyPrefix("KeY-x"))
	assert.False(t, hasKeyPrefix("Key-x"), "U+212A is not an ASCII k")
	assert.False(t, hasKeyPrefix("ke"))
	assert.False(t, hasKeyPrefix("key_x"))
	assert.False(t, hasKeyPrefix(""))
}

// A keyless register that finds a row in any state but legacy is a 409 for every caller, the admin key
// included, and the silence of the row changes nothing.
func TestProvisionerEnrolment_KeylessRegisterNeverReplacesAnEnrolledRow(t *testing.T) {
	srv, db := idServer(t)
	silent := time.Now().Add(-time.Hour)
	for _, state := range []string{"pending", "preapproved", "approved", "rejected"} {
		id := "enrolled-" + state
		state := state
		row := database.ProvisionerAgent{
			AgentID: id, TokenHash: database.HashAgentToken("tok-" + id), Hostname: id, Status: "online",
			LastSeen: silent, RegisteredAt: silent, EnrolmentState: &state,
		}
		require.NoError(t, db.GetDB().Create(&row).Error)
		for name, hdr := range map[string]map[string]string{"agent key": idAgentHdr(""), "admin key": idAdminHdr(), "its own token": idAgentHdr("tok-" + id)} {
			code, token, body := idRegister(t, srv, id, hdr)
			assert.Equal(t, http.StatusConflict, code, "%s, %s: %s", state, name, body)
			assert.Empty(t, token, "%s, %s: no token may be issued", state, name)
		}
	}
}

// A row written before the column existed has no state and registers exactly as it always did.
func TestProvisionerEnrolment_LegacyRowRegistersAsBefore(t *testing.T) {
	srv, db := idServer(t)
	idMustRegister(t, srv, "old-agent")
	require.NoError(t, db.GetDB().Model(&database.ProvisionerAgent{}).Where("agent_id = ?", "old-agent").
		UpdateColumn("enrolment_state", nil).Error)
	idSetLastSeen(t, db, "old-agent", time.Now().Add(-time.Hour))

	code, token, body := idRegister(t, srv, "old-agent", idAgentHdr(""))
	assert.Equal(t, http.StatusCreated, code, body)
	assert.NotEmpty(t, token)
}
