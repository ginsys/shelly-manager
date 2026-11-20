package opnsense

import (
	"strings"
	"testing"
)

// Nothing expands an environment reference in a plugin configuration, so the schema must not show
// one: a client that copies the example would send the text as the key.
func TestOPNSenseSchemaExampleIsAsWritten(t *testing.T) {
	schema := NewOPNSenseExporter().ConfigSchema()

	if len(schema.Examples) == 0 {
		t.Fatal("the schema has no example, so the check below proves nothing")
	}
	for i, example := range schema.Examples {
		for key, value := range example {
			if text, ok := value.(string); ok && strings.Contains(text, "${") {
				t.Errorf("example %d: %s is %q, which looks like an environment reference nothing expands", i, key, text)
			}
		}
	}

	for _, name := range []string{"api_key", "api_secret"} {
		if description := schema.Properties[name].Description; !strings.Contains(description, "as written") {
			t.Errorf("%s description %q does not say the value is used as written", name, description)
		}
	}
}

// The claim the description makes: the plugin passes the value on unchanged.
func TestOPNSenseConfigValueIsUsedAsWritten(t *testing.T) {
	o := NewOPNSenseExporter()
	const reference = "${OPNSENSE_API_KEY}"
	if got := o.getStringConfig(map[string]interface{}{"api_key": reference}, "api_key", ""); got != reference {
		t.Errorf("getStringConfig changed the value: got %q, want %q", got, reference)
	}
}
