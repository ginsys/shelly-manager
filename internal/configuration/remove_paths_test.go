package configuration

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNullPathsListsEveryNullField(t *testing.T) {
	raw := []byte(`{"MQTT":{"Pass":null,"user":"iot"},"led":null,"auth":{"pass":""},"x":[null]}`)

	assert.ElementsMatch(t, [][]string{{"mqtt", "pass"}, {"led"}}, NullPaths(raw))
}

func TestNullPathsIgnoresABodyThatIsNotAnObject(t *testing.T) {
	assert.Empty(t, NullPaths([]byte(`[null]`)))
	assert.Empty(t, NullPaths([]byte(`not json`)))
	assert.Empty(t, NullPaths([]byte(`{}`)))
}

func TestRemovePathsDropsTheFieldAndASectionItEmpties(t *testing.T) {
	config := &DeviceConfiguration{
		MQTT: &MQTTConfiguration{Password: StringPtr("x")},
		LED:  &LEDConfig{},
		Auth: &AuthConfiguration{Password: StringPtr("y"), Username: StringPtr("admin")},
	}

	got, err := RemovePaths(config, [][]string{{"mqtt", "pass"}, {"auth", "pass"}})
	require.NoError(t, err)

	assert.Nil(t, got.MQTT, "a section left empty is removed")
	require.NotNil(t, got.Auth, "a section with another field stays")
	assert.Nil(t, got.Auth.Password)
	assert.Equal(t, "admin", *got.Auth.Username)
}

func TestRemovePathsWithNoPathsReturnsTheConfigurationAsIs(t *testing.T) {
	config := &DeviceConfiguration{MQTT: &MQTTConfiguration{Password: StringPtr("x")}}

	got, err := RemovePaths(config, nil)
	require.NoError(t, err)

	assert.Same(t, config, got)
}
