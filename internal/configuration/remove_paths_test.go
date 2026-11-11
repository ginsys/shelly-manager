package configuration

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNullPathsListsEveryNullField(t *testing.T) {
	raw := []byte(`{"MQTT":{"Pass":null,"user":"iot"},"led":null,"auth":{"pass":""},"x":[1,null]}`)

	got, err := NullPaths(raw)
	require.NoError(t, err)

	assert.ElementsMatch(t, [][]string{{"mqtt", "pass"}, {"led"}, {"x", "1"}}, got)
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
