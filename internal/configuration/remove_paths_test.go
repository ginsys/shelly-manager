package configuration

import (
	"encoding/json"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNullPathsListsEveryNullField(t *testing.T) {
	raw := []byte(`{"MQTT":{"Pass":null,"user":"iot"},"led":null,"auth":{"pass":""},"x":[1,null]}`)

	got, err := NullPaths(raw)
	require.NoError(t, err)

	assert.ElementsMatch(t, [][]string{{"MQTT", "Pass"}, {"led"}, {"x", "1"}}, got, "keys keep their own spelling")
}

// The decoder treats "İ" (U+0130) as a different letter from "i". Neither the removal nor the
// duplicate test may fold it into "i".
func TestNullPathsAndRemovePathsFoldKeysLikeTheDecoder(t *testing.T) {
	paths, err := NullPaths([]byte("{\"wİfİ\":null}"))
	require.NoError(t, err)
	config := &DeviceConfiguration{MQTT: &MQTTConfiguration{Password: StringPtr("x")}}
	RemovePaths(config, [][]string{{"wİfİ"}, {"mqtt", "İd"}})
	require.NotNil(t, config.MQTT, "a key the decoder does not match removes nothing")
	require.NotNil(t, config.MQTT.Password)
	assert.Len(t, paths, 1)

	_, err = NullPaths([]byte("{\"mqtt\":{\"id\":\"client\",\"İd\":1}}"))
	assert.NoError(t, err, "two keys the decoder keeps apart are not duplicates")
}

func TestFoldKeyAgreesWithEqualFold(t *testing.T) {
	keys := []string{"id", "ID", "Id", "İd", "ıd", "k", "K", "K", "s", "S", "ſ", "σ", "ς", "Σ", "ǆ", "ǅ", "Ǆ", ""}
	for _, a := range keys {
		for _, b := range keys {
			assert.Equal(t, strings.EqualFold(a, b), foldKey(a) == foldKey(b), "%q vs %q", a, b)
		}
	}
}

func TestNullPathsReadsAFieldInsideAnArrayElement(t *testing.T) {
	got, err := NullPaths([]byte(`{"relay":{"relays":[{"id":0},{"id":1,"name":null}]}}`))
	require.NoError(t, err)

	assert.Equal(t, [][]string{{"relay", "relays", "1", "name"}}, got)
}

func TestNullPathsAcceptsNumbersOfAnySize(t *testing.T) {
	got, err := NullPaths([]byte(`{"unknown":1e400,"big":9223372036854775808123,"mqtt":{"pass":null}}`))
	require.NoError(t, err)

	assert.Equal(t, [][]string{{"mqtt", "pass"}}, got)
}

// No field of the configuration sits deeper than maxPathDepth, so a deeper null addresses nothing.
// It must not be kept, or a small body of nested arrays costs memory in proportion to its depth
// times its null count.
func TestNullPathsDropsAPathThatIsTooDeepToAddressAField(t *testing.T) {
	deep := `{"unknown":` + strings.Repeat("[", 200) + `null,null` + strings.Repeat("]", 200) + `,"mqtt":{"pass":null}}`

	got, err := NullPaths([]byte(deep))
	require.NoError(t, err)

	assert.Equal(t, [][]string{{"mqtt", "pass"}}, got)
}

// Dropping a deep path is not enough: building it must cost nothing too. Each of 10000 sibling nulls
// below 8704 nested arrays used to copy the whole path, about 1.8 GB for a body of 67 KB.
func TestNullPathsDoesNotCopyADeepPathForEverySibling(t *testing.T) {
	const depth = 8704
	deep := `{"unknown":` + strings.Repeat("[", depth) + strings.TrimSuffix(strings.Repeat("null,", 10000), ",") +
		strings.Repeat("]", depth) + `}`

	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	got, err := NullPaths([]byte(deep))
	runtime.ReadMemStats(&after)

	require.NoError(t, err)
	assert.Empty(t, got)
	assert.Less(t, after.TotalAlloc-before.TotalAlloc, uint64(64<<20), "bytes allocated for a 67 KB body")
}

func TestNullPathsRefusesAKeyThatAppearsTwice(t *testing.T) {
	for _, raw := range []string{
		`{"mqtt":{"pass":"a"},"mqtt":{"pass":null}}`,
		`{"mqtt":{"pass":"a","PASS":null}}`,
		`{"x":[{"a":1,"a":null}]}`,
	} {
		_, err := NullPaths([]byte(raw))
		assert.Error(t, err, raw)
	}
}

func TestNullPathsIgnoresABodyWithNoNull(t *testing.T) {
	for _, raw := range []string{`{}`, `null`, `{"a":{"b":1}}`} {
		got, err := NullPaths([]byte(raw))
		require.NoError(t, err, raw)
		assert.Empty(t, got, raw)
	}
}

func TestRemovePathsDropsTheFieldAndASectionItEmpties(t *testing.T) {
	config := &DeviceConfiguration{
		MQTT: &MQTTConfiguration{Password: StringPtr("x")},
		LED:  &LEDConfig{},
		Auth: &AuthConfiguration{Password: StringPtr("y"), Username: StringPtr("admin")},
	}

	RemovePaths(config, [][]string{{"mqtt", "pass"}, {"AUTH", "Pass"}})

	assert.Nil(t, config.MQTT, "a section left empty is removed")
	require.NotNil(t, config.Auth, "a section with another field stays")
	assert.Nil(t, config.Auth.Password)
	assert.Equal(t, "admin", *config.Auth.Username)
	assert.NotNil(t, config.LED, "a section off the path stays")
}

// Whatever spelling the decoder fills a field from, RemovePaths must clear that field.
func TestRemovePathsMatchesEverySpellingTheDecoderMatches(t *testing.T) {
	cases := []struct{ key, value string }{
		{"server", `"v"`}, {"SERVER", `"v"`}, {"ſerver", `"v"`}, {"sİrver", `"v"`},
		{"keep_alive", `1`}, {"Keep_alive", `1`}, {"KEEP_ALIVE", `1`},
	}
	matched := 0
	for _, tc := range cases {
		var config DeviceConfiguration
		require.NoError(t, json.Unmarshal([]byte(`{"mqtt":{"`+tc.key+`":`+tc.value+`}}`), &config))
		filled := config.MQTT != nil && (config.MQTT.Server != nil || config.MQTT.KeepAlive != nil)

		RemovePaths(&config, [][]string{{"mqtt", tc.key}})

		if filled {
			matched++
			assert.Nil(t, config.MQTT, "the decoder filled a field from %q, so the removal must clear it", tc.key)
		}
	}
	assert.GreaterOrEqual(t, matched, 3, "the control: the decoder fills a field for at least the plain spellings")
}

// A section whose JSON still holds a field that is written at its zero value is not empty, and a
// path that matches nothing must not remove the section around it.
func TestRemovePathsKeepsASectionThatStillWritesAField(t *testing.T) {
	config := &DeviceConfiguration{System: &SystemConfiguration{Sleep: &SleepConfig{}}}

	RemovePaths(config, [][]string{{"system", "sleep_mode", "unknown"}})
	require.NotNil(t, config.System)
	require.NotNil(t, config.System.Sleep, "a path that matches nothing changes nothing")

	RemovePaths(config, [][]string{{"system", "sleep_mode", "enable"}})
	require.NotNil(t, config.System)
	require.NotNil(t, config.System.Sleep, "enable has no omitempty, so the section still writes it")
	assert.False(t, config.System.Sleep.Enable)
}

// A null for a field that is not stored changes nothing, and so does the section around it.
func TestRemovePathsLeavesAnEmptySectionWhenTheFieldIsAlreadyAbsent(t *testing.T) {
	config := &DeviceConfiguration{MQTT: &MQTTConfiguration{}}

	RemovePaths(config, [][]string{{"mqtt", "pass"}})

	assert.NotNil(t, config.MQTT)
}

func TestRemovePathsRemovesAWholeSection(t *testing.T) {
	config := &DeviceConfiguration{
		MQTT: &MQTTConfiguration{User: StringPtr("iot"), Password: StringPtr("x")},
		LED:  &LEDConfig{},
	}

	RemovePaths(config, [][]string{{"mqtt"}})

	assert.Nil(t, config.MQTT)
	assert.NotNil(t, config.LED)
}

func TestRemovePathsKeepsANumberExact(t *testing.T) {
	big := 9007199254740993
	config := &DeviceConfiguration{MQTT: &MQTTConfiguration{Port: &big, Password: StringPtr("x")}}

	RemovePaths(config, [][]string{{"mqtt", "pass"}, {"nothing", "here"}})

	require.NotNil(t, config.MQTT.Port)
	assert.Equal(t, 9007199254740993, *config.MQTT.Port)
}

func TestRemovePathsIgnoresAPathThatIsNotStored(t *testing.T) {
	config := &DeviceConfiguration{MQTT: &MQTTConfiguration{User: StringPtr("iot")}}

	RemovePaths(config, [][]string{{"mqtt", "pass"}, {"auth", "pass"}, {"mqtt", "user", "deeper"}, {"mqtt", "3", "x"}})

	require.NotNil(t, config.MQTT)
	assert.Equal(t, "iot", *config.MQTT.User)
}
