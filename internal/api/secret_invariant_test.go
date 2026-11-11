package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/configuration"
	"github.com/ginsys/shelly-manager/internal/testutil"
)

// These tests close the class "a secret field is missing from the secret list", not one field at a
// time. They walk DeviceConfiguration (every section and the Network part) by reflection, fill every
// string with a sentinel named after its JSON path, and treat a field whose name looks like a
// secret as one that must never leave the API unmasked. A new secret field that is not added to
// secretSlots fails here, before any reviewer finds it.

// secretName matches the JSON key or the Go name of a field that holds a secret.
var secretName = regexp.MustCompile(`(?i)pass|key|token|secret|psk`)

const sentinelPrefix = "sentinel:"

// populated builds a DeviceConfiguration with every field set. Each string carries the sentinel for
// its JSON path. It returns the paths of the secret-named string fields and of all the others.
func populated(t *testing.T) (config *configuration.DeviceConfiguration, secrets, others []string) {
	t.Helper()
	config = &configuration.DeviceConfiguration{}
	fill(reflect.ValueOf(config).Elem(), "", false, &secrets, &others)
	require.NotEmpty(t, secrets)
	require.NotEmpty(t, others)
	return config, secrets, others
}

func jsonName(f reflect.StructField) string {
	name := strings.Split(f.Tag.Get("json"), ",")[0]
	if name == "" {
		return f.Name
	}
	return name
}

func fill(v reflect.Value, path string, inSecret bool, secrets, others *[]string) {
	switch v.Kind() {
	case reflect.Pointer:
		v.Set(reflect.New(v.Type().Elem()))
		fill(v.Elem(), path, inSecret, secrets, others)
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			f := v.Type().Field(i)
			if !f.IsExported() || f.Tag.Get("json") == "-" {
				continue
			}
			child := jsonName(f)
			if path != "" {
				child = path + "." + child
			}
			secret := secretName.MatchString(jsonName(f)) || secretName.MatchString(f.Name)
			fill(v.Field(i), child, secret, secrets, others)
		}
	case reflect.Slice:
		if v.Type().Elem().Kind() == reflect.Uint8 {
			return // raw bytes
		}
		v.Set(reflect.MakeSlice(v.Type(), 1, 1))
		fill(v.Index(0), path, inSecret, secrets, others)
	case reflect.String:
		v.SetString(sentinelPrefix + path)
		if inSecret {
			*secrets = append(*secrets, path)
		} else {
			*others = append(*others, path)
		}
	case reflect.Bool:
		v.SetBool(true)
	case reflect.Int, reflect.Int32, reflect.Int64:
		v.SetInt(1)
	case reflect.Uint, reflect.Uint32, reflect.Uint64:
		v.SetUint(1)
	case reflect.Float32, reflect.Float64:
		v.SetFloat(1)
	}
}

func marshalConfig(t *testing.T, c *configuration.DeviceConfiguration) string {
	t.Helper()
	b, err := json.Marshal(c)
	require.NoError(t, err)
	return string(b)
}

func quoted(path string) string { return `"` + sentinelPrefix + path + `"` }

func assertNoSecret(t *testing.T, where, body string, secrets []string) {
	t.Helper()
	for _, p := range secrets {
		assert.NotContains(t, body, quoted(p), "%s returns the secret %s unmasked", where, p)
	}
}

func TestSecretSlots_EverySecretNamedFieldIsRedactedMaskedAndRestored(t *testing.T) {
	full, secrets, others := populated(t)
	// The walk reaches the parts that were missed before.
	assert.Contains(t, secrets, "network.wifi.sta.pass")
	assert.Contains(t, secrets, "network.wifi.ap.pass")
	assert.Contains(t, secrets, "wifi.ap.key")

	// redact: no secret survives, every other field does.
	redacted, _, _ := populated(t)
	redactSecrets(redacted)
	body := marshalConfig(t, redacted)
	assertNoSecret(t, "redactSecrets", body, secrets)
	for _, p := range others {
		assert.Contains(t, body, quoted(p), "redactSecrets removed %s, which is not a secret", p)
	}

	// mask: the value is replaced by the mask, once per secret.
	masked, _, _ := populated(t)
	maskSecrets(masked)
	body = marshalConfig(t, masked)
	assertNoSecret(t, "maskSecrets", body, secrets)
	assert.Equal(t, len(secrets), strings.Count(body, `"`+secretMask+`"`), "one mask per secret")
	for _, p := range others {
		assert.Contains(t, body, quoted(p), "maskSecrets changed %s, which is not a secret", p)
	}

	// restore: the mask sent back becomes the stored value, for every secret.
	back, _, _ := populated(t)
	maskSecrets(back)
	restoreMaskedSecrets(back, full)
	assert.JSONEq(t, marshalConfig(t, full), marshalConfig(t, back))

	// restore with nothing stored: the mask is never kept.
	none, _, _ := populated(t)
	maskSecrets(none)
	restoreMaskedSecrets(none, &configuration.DeviceConfiguration{})
	body = marshalConfig(t, none)
	assert.NotContains(t, body, secretMask, "the mask must never be stored")
	assertNoSecret(t, "restoreMaskedSecrets", body, secrets)
}

func idRequest(method, target, id, body string) *http.Request {
	req := httptest.NewRequest(method, target, bytes.NewBufferString(body))
	return mux.SetURLVars(req, map[string]string{"id": id})
}

// No route that returns a configuration returns a secret value: the desired configuration, the
// set-templates response, the overrides routes and the template routes, each filled with a secret
// in every field.
func TestSecretInvariant_NoResponsePathReturnsASecret(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	full, secrets, _ := populated(t)
	fullJSON := marshalConfig(t, full)

	device := testutil.TestDevice()
	require.NoError(t, handler.DB.AddDevice(device))
	id := strconv.FormatUint(uint64(device.ID), 10)
	svc := handler.ConfigService.ConfigurationSvc

	template := &configuration.ServiceConfigTemplate{Name: "Global", Scope: "global", Config: json.RawMessage(fullJSON)}
	require.NoError(t, svc.CreateTemplate(template))
	require.NoError(t, svc.SetDeviceOverrides(device.ID, full))

	call := func(name string, run func(http.ResponseWriter, *http.Request), req *http.Request) {
		t.Helper()
		w := httptest.NewRecorder()
		run(w, req)
		require.Less(t, w.Code, 300, "%s: %s", name, w.Body.String())
		assertNoSecret(t, name, w.Body.String(), secrets)
	}

	templateID := strconv.FormatUint(uint64(template.ID), 10)
	call("set templates", handler.SetDeviceNewTemplates,
		idRequest("PUT", "/api/v1/devices/1/templates/new", id, `{"template_ids":[`+templateID+`]}`))
	call("desired config", handler.GetDeviceDesiredConfig, idRequest("GET", "/api/v1/devices/1/desired-config", id, ""))
	call("get overrides", handler.GetDeviceNewOverrides, idRequest("GET", "/api/v1/devices/1/overrides/new", id, ""))
	call("put overrides", handler.SetDeviceNewOverrides, idRequest("PUT", "/api/v1/devices/1/overrides/new", id, fullJSON))
	call("list templates", handler.GetNewConfigTemplates, httptest.NewRequest("GET", "/api/v1/configuration/templates/new", nil))
	call("get template", handler.GetNewConfigTemplate, idRequest("GET", "/api/v1/configuration/templates/new/1", templateID, ""))

	// A masked GET followed by a PUT of the same body is what the form does on an unrelated edit.
	// It keeps every stored secret and stores the mask nowhere.
	w := httptest.NewRecorder()
	handler.GetDeviceNewOverrides(w, idRequest("GET", "/api/v1/devices/1/overrides/new", id, ""))
	var got struct {
		Data struct {
			Overrides json.RawMessage `json:"overrides"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	w = httptest.NewRecorder()
	handler.SetDeviceNewOverrides(w, idRequest("PUT", "/api/v1/devices/1/overrides/new", id, string(got.Data.Overrides)))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	stored, err := svc.GetDeviceOverrides(device.ID)
	require.NoError(t, err)
	body := marshalConfig(t, stored)
	assert.NotContains(t, body, secretMask, "the mask was stored as a value")
	assert.JSONEq(t, fullJSON, body, "a read-modify-write of the masked overrides lost or changed a field")

	// PATCH with the mask in every secret slot changes no stored secret. Only the secrets are
	// compared: the merge of a patch is not what this test is about. The Network passwords are
	// among them, since the Network fields are pointers and the merge keeps them.
	patch, _, _ := populated(t)
	maskSecrets(patch)
	call("patch overrides", handler.PatchDeviceNewOverrides,
		idRequest("PATCH", "/api/v1/devices/1/overrides/new", id, marshalConfig(t, patch)))
	stored, err = svc.GetDeviceOverrides(device.ID)
	require.NoError(t, err)
	body = marshalConfig(t, stored)
	assert.NotContains(t, body, secretMask, "the mask was stored as a value")
	for _, p := range secrets {
		assert.Contains(t, body, quoted(p), "a PATCH that sent the mask lost the stored secret %s", p)
	}
}
