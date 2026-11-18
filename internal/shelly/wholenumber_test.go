package shelly

import (
	"encoding/json"
	"math"
	"testing"
)

func TestWholeNumber(t *testing.T) {
	cases := []struct {
		name   string
		raw    interface{}
		want   int
		wantOK bool
	}{
		{"whole", float64(300), 300, true},
		{"zero", float64(0), 0, true},
		{"negative whole", float64(-5), -5, true},
		{"negative zero", math.Copysign(0, -1), 0, true},
		{"fractional", 7.5, 0, false},
		{"fractional below one", 0.5, 0, false},
		{"largest exact integer", float64(1<<53 - 1), 1<<53 - 1, true},
		{"smallest exact integer", -float64(1<<53 - 1), -(1<<53 - 1), true},
		{"one past the exact integers", float64(1 << 53), 0, false},
		{"out of range high", 1e300, 0, false},
		{"out of range low", -1e300, 0, false},
		{"NaN", math.NaN(), 0, false},
		{"positive infinity", math.Inf(1), 0, false},
		{"negative infinity", math.Inf(-1), 0, false},
		{"int is not a decoded number", 300, 0, false},
		{"string", "300", 0, false},
		{"bool", true, 0, false},
		{"nil", nil, 0, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := WholeNumber(tc.raw)
			if got != tc.want || ok != tc.wantOK {
				t.Errorf("WholeNumber(%v) = (%d, %v), want (%d, %v)", tc.raw, got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

// A document decoded from JSON gives a float64 for every number, so the decoded forms read the same.
func TestWholeNumber_DecodedJSON(t *testing.T) {
	var doc map[string]interface{}
	if err := json.Unmarshal([]byte(`{"a": 300, "b": 7.5, "c": 3e2, "d": 1e300}`), &doc); err != nil {
		t.Fatal(err)
	}
	if got, ok := WholeNumber(doc["a"]); !ok || got != 300 {
		t.Errorf("a: (%d, %v)", got, ok)
	}
	if _, ok := WholeNumber(doc["b"]); ok {
		t.Error("b: a fraction read as a number")
	}
	if got, ok := WholeNumber(doc["c"]); !ok || got != 300 {
		t.Errorf("c: (%d, %v)", got, ok)
	}
	if _, ok := WholeNumber(doc["d"]); ok {
		t.Error("d: out of range read as a number")
	}
}
