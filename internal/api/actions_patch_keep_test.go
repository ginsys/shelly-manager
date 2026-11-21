package api

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/ginsys/shelly-manager/internal/configuration"
)

// The URL list of an action is a secret the API never shows, so a client that edits an action cannot
// send it back. These tests pin what each write route does with an action that arrives without its
// list, or with a list that cannot be stored.

const storedActionOverrides = `{"actions":[{"name":"out_on_url","index":0,"enabled":true,"secret_urls":["http://admin:hunter2@192.168.1.4/on?token=s3cr3t"]}]}`

func storedActionURLs(t *testing.T, handler *Handler, id uint) []string {
	t.Helper()
	stored, err := handler.ConfigService.ConfigurationSvc.GetDeviceOverrides(id)
	require.NoError(t, err)
	require.NotNil(t, stored.Actions)
	require.Len(t, *stored.Actions, 1)
	return urlsOf((*stored.Actions)[0])
}

func storedAction(t *testing.T, handler *Handler, id uint) configuration.ActionConfig {
	t.Helper()
	stored, err := handler.ConfigService.ConfigurationSvc.GetDeviceOverrides(id)
	require.NoError(t, err)
	require.NotNil(t, stored.Actions)
	require.Len(t, *stored.Actions, 1)
	return (*stored.Actions)[0]
}

// A list that holds the mask beside other lines is neither the stored list nor a new one. It is read
// as not sent, so the stored list stays and the mask is stored nowhere.
func TestActionURLs_APutWithAMaskBesideAURLKeepsTheStoredList(t *testing.T) {
	for name, list := range map[string]string{
		"a mask and a new URL": `["********","http://new.example/on"]`,
		"two masks":            `["********","********"]`,
		"a new URL and a mask": `["http://new.example/on","********"]`,
	} {
		t.Run("PUT "+name, func(t *testing.T) {
			handler, cleanup := setupTestHandler(t)
			defer cleanup()
			id := keepOverrideDevice(t, handler, storedActionOverrides)

			w := sendOverrides(t, handler, "PUT", id,
				`{"actions":[{"name":"out_on_url","index":0,"enabled":true,"secret_urls":`+list+`}]}`)
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())

			assert.Equal(t, []string{"http://admin:hunter2@192.168.1.4/on?token=s3cr3t"}, storedActionURLs(t, handler, id))
			assertNoURLSecret(t, "the answer", w.Body.String())
		})
		t.Run("PATCH "+name, func(t *testing.T) {
			handler, cleanup := setupTestHandler(t)
			defer cleanup()
			id := keepOverrideDevice(t, handler, storedActionOverrides)

			w := sendOverrides(t, handler, "PATCH", id,
				`{"actions":[{"name":"out_on_url","index":0,"enabled":true,"secret_urls":`+list+`}]}`)
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())

			assert.Equal(t, []string{"http://admin:hunter2@192.168.1.4/on?token=s3cr3t"}, storedActionURLs(t, handler, id))
			assertNoURLSecret(t, "the answer", w.Body.String())
		})
	}
}

// With nothing stored the mask is dropped and no list is stored: the mask is never a URL.
func TestActionURLs_AMixedMaskWithNothingStoredStoresNoList(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()
	id := keepOverrideDevice(t, handler, `{"mqtt":{"user":"iot"}}`)

	w := sendOverrides(t, handler, "PUT", id,
		`{"actions":[{"name":"out_on_url","index":0,"enabled":true,"secret_urls":["********","http://new.example/on"]}]}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	assert.Nil(t, storedAction(t, handler, id).SecretURLs)
}

// A patch changes what it names. An action sent without its list changes its other keys and leaves
// the list as stored, which is what a full body does.
func TestActionURLs_APatchWithoutTheListKeepsTheStoredList(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()
	id := keepOverrideDevice(t, handler, storedActionOverrides)

	w := sendOverrides(t, handler, "PATCH", id, `{"actions":[{"name":"out_on_url","index":0,"enabled":false}]}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	got := storedAction(t, handler, id)
	assert.Equal(t, []string{"http://admin:hunter2@192.168.1.4/on?token=s3cr3t"}, urlsOf(got))
	require.NotNil(t, got.Enabled)
	assert.False(t, *got.Enabled, "the key the patch named changed")
	assertNoURLSecret(t, "the answer", w.Body.String())
}

func TestActionURLs_APatchNullDropsTheStoredList(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()
	id := keepOverrideDevice(t, handler, storedActionOverrides)

	w := sendOverrides(t, handler, "PATCH", id, `{"actions":[{"name":"out_on_url","index":0,"enabled":false,"secret_urls":null}]}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	assert.Nil(t, storedAction(t, handler, id).SecretURLs, "a null removes the list, it is not kept")
}

func TestActionURLs_APatchClearKeepsAsAClear(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()
	id := keepOverrideDevice(t, handler, storedActionOverrides)

	w := sendOverrides(t, handler, "PATCH", id, `{"actions":[{"name":"out_on_url","index":0,"secret_urls":[]}]}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	got := storedAction(t, handler, id).SecretURLs
	require.NotNil(t, got, "the clear was lost")
	assert.Empty(t, *got)
}

// A patch entry that matches no stored entry by name and index has nothing to take.
func TestActionURLs_APatchForAnotherActionTakesNothing(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()
	id := keepOverrideDevice(t, handler, storedActionOverrides)

	w := sendOverrides(t, handler, "PATCH", id, `{"actions":[{"name":"out_off_url","index":0,"enabled":true}]}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	assert.Nil(t, storedAction(t, handler, id).SecretURLs)
}

// A stored list of one empty URL is not the empty list: apply refuses the first and writes the clear
// for the second. Keeping an omitted list must hand it over as it is.
func TestActionURLs_AStoredListOfOneEmptyURLIsNotTurnedIntoAClear(t *testing.T) {
	const seeded = `{"actions":[{"name":"out_on_url","index":0,"enabled":true,"secret_urls":[""]}]}`
	omitted := `{"actions":[{"name":"out_on_url","index":0,"enabled":false}]}`

	for _, method := range []string{"PUT", "PATCH"} {
		t.Run(method, func(t *testing.T) {
			handler, cleanup := setupTestHandler(t)
			defer cleanup()
			id := keepOverrideDevice(t, handler, seeded)

			w := sendOverrides(t, handler, method, id, omitted)
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())

			assert.Equal(t, []string{""}, storedActionURLs(t, handler, id))
		})
	}
	t.Run("bulk apply", func(t *testing.T) {
		handler, db, fleet := applyFleet(t, 1)
		id := fleet[0].device.ID
		setOverrides(t, db, id, seeded)

		w := postApply(handler, applyBody(omitted, false, id))
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())

		assert.Equal(t, []string{""}, storedActionURLs(t, handler, id))
	})
	t.Run("the helper copies the slice", func(t *testing.T) {
		stored := actionsOf(actionEntryOf("out_on_url", 0, ""))
		incoming := actionsOf(configuration.ActionConfig{Name: "out_on_url", Index: 0})

		keepOmittedActionURLs(incoming, stored, nil)

		assert.Equal(t, []string{""}, urlsOf((*incoming.Actions)[0]))
		(*(*incoming.Actions)[0].SecretURLs)[0] = "http://changed"
		assert.Equal(t, []string{""}, urlsOf((*stored.Actions)[0]), "the stored list shares memory with the incoming one")
	})
}

func TestKeepOmittedActionURLs_TouchesNothingButTheActionLists(t *testing.T) {
	stored := actionsOf(actionEntryOf("out_on_url", 0, secretActionURLs[0]))
	user := "iot"
	stored.MQTT = &configuration.MQTTConfiguration{User: &user, Password: &user}
	incoming := actionsOf(configuration.ActionConfig{Name: "out_on_url", Index: 0})

	keepOmittedActionURLs(incoming, stored, nil)

	assert.Equal(t, secretActionURLs[:1], urlsOf((*incoming.Actions)[0]))
	assert.Nil(t, incoming.MQTT, "a patch's other sections are the service's to merge")
}

// The service drops a list sent as null by itself, so the handlers' routes cannot tell whether the
// helper honours it: only a call on the helper can.
func TestKeepOmittedActionURLs_ANullListIsNotKept(t *testing.T) {
	stored := actionsOf(
		actionEntryOf("out_on_url", 0, secretActionURLs[0]),
		actionEntryOf("out_off_url", 0, secretActionURLs[1]),
	)
	incoming := actionsOf(
		configuration.ActionConfig{Name: "out_on_url", Index: 0},
		configuration.ActionConfig{Name: "out_off_url", Index: 0},
	)

	keepOmittedActionURLs(incoming, stored, map[string]bool{actionNullMark(0): true})

	assert.Nil(t, (*incoming.Actions)[0].SecretURLs, "the list sent as null was kept")
	assert.Equal(t, secretActionURLs[1:], urlsOf((*incoming.Actions)[1]), "a null on one entry reached another")
}

func TestKeepOmittedActionURLs_ADuplicateTakesNothing(t *testing.T) {
	stored := actionsOf(
		actionEntryOf("out_on_url", 0, secretActionURLs[0]),
		actionEntryOf("out_on_url", 0, secretActionURLs[1]),
	)
	incoming := actionsOf(configuration.ActionConfig{Name: "out_on_url", Index: 0})

	keepOmittedActionURLs(incoming, stored, nil)

	assert.Nil(t, (*incoming.Actions)[0].SecretURLs)
}

func TestKeepOmittedActionURLs_NilSidesAreLeftAlone(t *testing.T) {
	incoming := actionsOf(configuration.ActionConfig{Name: "out_on_url", Index: 0})

	keepOmittedActionURLs(incoming, nil, nil)
	keepOmittedActionURLs(incoming, &configuration.DeviceConfiguration{}, nil)
	keepOmittedActionURLs(nil, actionsOf(actionEntryOf("out_on_url", 0, secretActionURLs[0])), nil)
	keepOmittedActionURLs(&configuration.DeviceConfiguration{}, actionsOf(actionEntryOf("out_on_url", 0, secretActionURLs[0])), nil)

	assert.Nil(t, (*incoming.Actions)[0].SecretURLs)
}

// failDeviceReadNumber makes the nth read of the devices table after the call fail once, and returns
// how many times it fired. The PATCH route reads the device to check the body, then reads the stored
// overrides, then the service reads them again: the second is the one that must not be skipped.
func failDeviceReadNumber(t *testing.T, handler *Handler, n int) *int {
	t.Helper()
	const name = "test:fail-device-read"
	seen, fired := 0, 0
	require.NoError(t, handler.DB.GetDB().Callback().Query().Before("gorm:query").Register(name, func(tx *gorm.DB) {
		if tx.Statement.Table != "devices" {
			return
		}
		seen++
		if seen == n {
			fired++
			_ = tx.AddError(errors.New("injected read failure"))
		}
	}))
	// The database belongs to this test alone and is closed with it, so the callback is not removed.
	return &fired
}

// When the stored overrides cannot be read, a PATCH or a PUT that carries actions is refused: the
// action list replaces the stored one whole, so a store that went on would drop the URLs the body
// only masks or leaves out. A body without actions goes on as before. The read under test is the
// stored-overrides read of the route: the first read of the devices table on PUT, the second on
// PATCH, after the check of the body.
func TestActionURLs_AWriteWithActionsIsRefusedWhenTheStoredOverridesCannotBeRead(t *testing.T) {
	routes := map[string]int{"PUT": 1, "PATCH": 2}
	for method, read := range routes {
		for name, body := range map[string]string{
			"an action sent without its list": `{"actions":[{"name":"out_on_url","index":0,"enabled":false}]}`,
			"an action sent as the mask":      `{"actions":[{"name":"out_on_url","index":0,"secret_urls":["********"]}]}`,
		} {
			t.Run(method+" "+name, func(t *testing.T) {
				handler, cleanup := setupTestHandler(t)
				defer cleanup()
				id := keepOverrideDevice(t, handler, storedActionOverrides)
				fired := failDeviceReadNumber(t, handler, read)

				w := sendOverrides(t, handler, method, id, body)

				require.Equal(t, 1, *fired, "the injected failure did not reach the read under test")
				assert.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
				assert.Equal(t, []string{"http://admin:hunter2@192.168.1.4/on?token=s3cr3t"}, storedActionURLs(t, handler, id),
					"the stored list was replaced")
			})
		}

		t.Run(method+" without actions goes on", func(t *testing.T) {
			handler, cleanup := setupTestHandler(t)
			defer cleanup()
			id := keepOverrideDevice(t, handler, storedActionOverrides)
			fired := failDeviceReadNumber(t, handler, read)

			w := sendOverrides(t, handler, method, id, `{"mqtt":{"user":"iot"}}`)

			require.Equal(t, 1, *fired, "the injected failure did not reach the read under test")
			assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
			if method == "PATCH" {
				// A PATCH merges field by field, so the actions it does not name stay as stored. A PUT
				// replaces the whole record, so it holds no such claim.
				assert.Equal(t, []string{"http://admin:hunter2@192.168.1.4/on?token=s3cr3t"}, storedActionURLs(t, handler, id))
			}
		})
	}
}

// The bulk apply route stores its patch as the PATCH route does, so an action sent without its list
// keeps the list stored for the device, and a null drops it.
func TestActionURLs_BulkApplyKeepsTheStoredListOfAnActionSentWithoutOne(t *testing.T) {
	for name, tc := range map[string]struct {
		patch string
		want  bool
	}{
		"the list left out": {`{"actions":[{"name":"out_on_url","index":0,"enabled":false}]}`, true},
		"a null":            {`{"actions":[{"name":"out_on_url","index":0,"enabled":false,"secret_urls":null}]}`, false},
	} {
		t.Run(name, func(t *testing.T) {
			handler, db, fleet := applyFleet(t, 1)
			id := fleet[0].device.ID
			setOverrides(t, db, id, storedActionOverrides)

			w := postApply(handler, applyBody(tc.patch, false, id))
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())

			stored, err := handler.ConfigService.ConfigurationSvc.GetDeviceOverrides(id)
			require.NoError(t, err)
			require.NotNil(t, stored.Actions)
			require.Len(t, *stored.Actions, 1)
			entry := (*stored.Actions)[0]
			if tc.want {
				assert.Equal(t, []string{"http://admin:hunter2@192.168.1.4/on?token=s3cr3t"}, urlsOf(entry))
			} else {
				assert.Nil(t, entry.SecretURLs)
			}
			assertNoURLSecret(t, "the answer", w.Body.String())
		})
	}
}
