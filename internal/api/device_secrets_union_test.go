package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/redact"
)

// The device walker is the shared redact walker (ginsys/shelly-manager#653), so a device text gains what the
// notification walker learned in ginsys/shelly-manager#620. One case per gain; each fails on the walker
// the device side had before.

// D1: a key the JSON decoder reads as a secret name (U+017F for s, U+212A for k) is a secret here too.
func TestDeviceWalker_FoldedKeyNamesAreSecrets(t *testing.T) {
	const stored = `{"ſecret":"credential","apiKey":"k1","name":"n"}`
	got := maskJSONText(stored)
	assert.NotContains(t, got, "credential")
	assert.NotContains(t, got, "k1")

	back, err := restoreMaskedText(got, stored)
	require.NoError(t, err)
	assert.JSONEq(t, stored, back, "the returned mask takes the stored value")
}

// D2: the stored secret is found without regard to key case, and two stored keys that fold together
// are ambiguous, so the mask is dropped.
func TestDeviceWalker_StoredKeyIsFoundWithoutRegardToCase(t *testing.T) {
	got, err := restoreMaskedText(`{"secret":"`+secretMask+`"}`, `{"Secret":"s3"}`)
	require.NoError(t, err)
	assert.JSONEq(t, `{"secret":"s3"}`, got)

	got, err = restoreMaskedText(`{"SECRET":"`+secretMask+`"}`, `{"Secret":"a","secret":"b"}`)
	require.NoError(t, err)
	assert.JSONEq(t, `{}`, got)
}

// D3: a secret key that holds a list or an object is one secret: masked whole on a read, and the mask
// brings the whole stored value back.
func TestDeviceWalker_ContainerUnderASecretKeyIsMaskedWhole(t *testing.T) {
	const stored = `{"tokens":["a1","b1"],"password":{"x":"y1"},"opts":{"name":"n"}}`
	got := maskJSONText(stored)
	assert.JSONEq(t, `{"tokens":"`+secretMask+`","password":"`+secretMask+`","opts":{"name":"n"}}`, got)

	back, err := restoreMaskedText(got, stored)
	require.NoError(t, err)
	assert.JSONEq(t, stored, back)
}

// D4: a whole text that is the mask as a JSON string, plain or escaped, takes the stored text. Before,
// it was stored as the text "********".
func TestDeviceWalker_WholeMaskAsAJSONStringTakesTheStoredText(t *testing.T) {
	const stored = `{"pass":"kept"}`
	for name, incoming := range map[string]string{
		"plain":   `"` + secretMask + `"`,
		"escaped": `"` + `*` + `*******"`,
	} {
		got, err := restoreMaskedText(incoming, stored)
		require.NoError(t, err, name)
		assert.Equal(t, stored, got, name)
	}
}

// D5: a text that repeats a member name is refused on save: the read would show an unmasked "" while
// the stored text keeps "s".
func TestDeviceWalker_RepeatedMemberIsRefused(t *testing.T) {
	_, err := restoreMaskedText(`{"pass":"s","pass":""}`, `{"pass":"s"}`)
	require.Error(t, err)
	assert.ErrorIs(t, err, redact.ErrRepeatedMember)
	assert.Equal(t, "device configuration text repeats a member name: send each name once", err.Error())

	_, err = restoreMaskedText(`{"opts":{"a":1,"A":2}}`, ``)
	assert.ErrorIs(t, err, redact.ErrRepeatedMember, "any depth, any spelling, with no placeholder in the text")

	handler, cleanup := setupTestHandler(t)
	defer cleanup()
	device := secretDevice()
	require.NoError(t, handler.DB.AddDevice(device))
	body, err := json.Marshal(map[string]any{"name": "x", "settings": `{"pass":"s","pass":""}`})
	require.NoError(t, err)
	put := serve(handler.UpdateDevice, "PUT", "/api/v1/devices/1", idString(device), string(body))
	assert.Equal(t, http.StatusBadRequest, put.Code, put.Body.String())
	assert.Contains(t, put.Body.String(), "device configuration text repeats a member name: send each name once")
}

// D6: list entries pair when only a container under a secret key differs, so such an entry gets its
// secret back.
func TestDeviceWalker_ListEntriesPairThroughASecretContainer(t *testing.T) {
	got, err := restoreMaskedText(
		`{"l":[{"name":"a","tokens":"`+secretMask+`"}]}`,
		`{"l":[{"name":"a","tokens":["x","y"]}]}`)
	require.NoError(t, err)
	assert.JSONEq(t, `{"l":[{"name":"a","tokens":["x","y"]}]}`, got)
}

// D7: the typed raw part keeps a stored secret container that a save leaves out, like a stored scalar.
func TestDeviceWalker_TypedRawKeepsAnOmittedSecretContainer(t *testing.T) {
	h, cleanup := typedHandler(t)
	defer cleanup()
	id := typedDevice(t, h, `{"wifi":{"enable":true,"ssid":"home"},"raw":{"tokens":["t7pe-token-a"],"name":"x"}}`)

	body := `{"configuration":{"wifi":{"enable":true,"ssid":"home"},"raw":{"name":"y"}}}`
	put := serve(h.UpdateTypedDeviceConfig, "PUT", "/api/v1/devices/1/config/typed", id, body)
	require.Equal(t, http.StatusOK, put.Code, put.Body.String())
	assert.Contains(t, storedConfig(t, h, id), "t7pe-token-a", "the omitted secret container was lost")
}
