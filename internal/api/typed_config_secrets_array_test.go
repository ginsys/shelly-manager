package api

import (
	"math/big"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests pin the three edges of the array-secret restore: a raw part that is an array at the
// top, ambiguity judged at the secrets a save restores, and numbers compared by value.

// A raw part that is an array at the top restores like one inside an object: a mask returns to its
// own entry, an omitted secret is kept by the entry at its position, and an entry that does not
// match takes nothing.
func TestTypedConfigRoutes_TopLevelRawArrayKeepsEntrySecrets(t *testing.T) {
	const stored = `{"wifi":{"enable":true,"ssid":"home"},"raw":[` +
		`{"name":"alice","password":"t7pe-alice-s3cret"},{"name":"bob","password":"t7pe-bob-s3cret"}]}`
	cases := map[string]struct {
		raw   string
		alice bool
		bob   bool
	}{
		"omitted":       {`[{"name":"alice"},{"name":"bob"}]`, true, true},
		"masked":        {`[{"name":"alice","password":"` + secretMask + `"},{"name":"bob","password":"` + secretMask + `"}]`, true, true},
		"swapped":       {`[{"name":"bob"},{"name":"alice"}]`, false, false},
		"first removed": {`[{"name":"bob"}]`, false, false},
		"empty clears":  {`[{"name":"alice","password":""},{"name":"bob"}]`, false, true},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			h, cleanup := typedHandler(t)
			defer cleanup()
			id := typedDevice(t, h, stored)

			body := `{"configuration":{"wifi":{"enable":true,"ssid":"home"},"raw":` + c.raw + `}}`
			put := serve(h.UpdateTypedDeviceConfig, "PUT", "/api/v1/devices/1/config/typed", id, body)
			require.Equal(t, http.StatusOK, put.Code, put.Body.String())
			got := storedConfig(t, h, id)
			assert.Equal(t, c.alice, strings.Contains(got, "t7pe-alice-s3cret"), "stored: %s", got)
			assert.Equal(t, c.bob, strings.Contains(got, "t7pe-bob-s3cret"), "stored: %s", got)
			assert.NotContains(t, got, secretMask)
		})
	}
}

// An entry of a top-level array that matches more than one stored entry and leaves out a secret on
// which they differ is refused, the same as inside an object.
func TestTypedConfigRoutes_TopLevelRawArrayAmbiguousEntryIsRefused(t *testing.T) {
	const stored = `{"wifi":{"enable":true,"ssid":"home"},"raw":[` +
		`{"token":"alice","password":"t7pe-alice-s3cret"},{"token":"bob","password":"t7pe-bob-s3cret"}]}`
	h, cleanup := typedHandler(t)
	defer cleanup()
	id := typedDevice(t, h, stored)

	body := `{"configuration":{"wifi":{"enable":true,"ssid":"home"},"raw":[{"token":"bob"}]}}`
	put := serve(h.UpdateTypedDeviceConfig, "PUT", "/api/v1/devices/1/config/typed", id, body)
	require.Equal(t, http.StatusBadRequest, put.Code, put.Body.String())
	assert.Contains(t, put.Body.String(), "more than one stored entry")
	assert.NotContains(t, put.Body.String(), "t7pe-")
	assert.Equal(t, stored, storedConfig(t, h, id), "a refused PUT stores nothing")
}

// Two stored entries that hold the same password and differ in another secret are not ambiguous
// for the password: a save that replaces both tokens and sends the password as the mask, or leaves
// it out, gets that password back. Where the entries disagree at a secret the save restores, the
// request is still refused.
func TestTypedConfigRoutes_AmbiguityIsJudgedAtTheRestoredSecretsOnly(t *testing.T) {
	const same = `{"wifi":{"enable":true,"ssid":"home"},"raw":{"accounts":[` +
		`{"name":"x","password":"t7pe-shared-s3cret","token":"t7pe-tok1-s3cret"},` +
		`{"name":"x","password":"t7pe-shared-s3cret","token":"t7pe-tok2-s3cret"}]}}`
	const differ = `{"wifi":{"enable":true,"ssid":"home"},"raw":{"accounts":[` +
		`{"name":"x","password":"t7pe-one-s3cret","token":"t7pe-tok1-s3cret"},` +
		`{"name":"x","password":"t7pe-two-s3cret","token":"t7pe-tok2-s3cret"}]}}`
	const maskedPass = `,"password":"` + secretMask + `"`
	const masked = `{"name":"x","token":"t7pe-new1-s3cret"` + maskedPass + `},{"name":"x","token":"t7pe-new2-s3cret"` + maskedPass + `}`
	const omitted = `{"name":"x","token":"t7pe-new1-s3cret"},{"name":"x","token":"t7pe-new2-s3cret"}`

	cases := map[string]struct {
		stored string
		accts  string
		status int
	}{
		"same password, masked":      {same, masked, http.StatusOK},
		"same password, omitted":     {same, omitted, http.StatusOK},
		"differing password masked":  {differ, masked, http.StatusBadRequest},
		"differing password omitted": {differ, omitted, http.StatusBadRequest},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			h, cleanup := typedHandler(t)
			defer cleanup()
			id := typedDevice(t, h, c.stored)

			body := `{"configuration":{"wifi":{"enable":true,"ssid":"home"},"raw":{"accounts":[` + c.accts + `]}}}`
			put := serve(h.UpdateTypedDeviceConfig, "PUT", "/api/v1/devices/1/config/typed", id, body)
			require.Equal(t, c.status, put.Code, put.Body.String())
			got := storedConfig(t, h, id)
			if c.status == http.StatusBadRequest {
				assert.Contains(t, put.Body.String(), "more than one stored entry")
				assert.Equal(t, c.stored, got, "a refused PUT stores nothing")
				return
			}
			assert.Equal(t, 2, strings.Count(got, "t7pe-shared-s3cret"), "stored: %s", got)
			assert.NotContains(t, got, "t7pe-tok1-s3cret")
			assert.NotContains(t, got, secretMask)
		})
	}
}

// The text helper of the device routes judges ambiguity the same way, for every shape a secret can
// take: plain, multi-line, leading dash and a number.
func TestRestoreMaskedText_AmbiguityAtRestoredSecretsOnly(t *testing.T) {
	for name, pass := range map[string]string{
		"plain":     `"A"`,
		"multiline": `"line1\nline2"`,
		"dash":      `"-x"`,
		"number":    `12345678`,
	} {
		t.Run(name, func(t *testing.T) {
			stored := `{"a":[{"n":"x","password":` + pass + `,"token":"T1"},{"n":"x","password":` + pass + `,"token":"T2"}]}`
			in := `{"a":[{"n":"x","password":"` + secretMask + `","token":"N1"},{"n":"x","password":"` + secretMask + `","token":"N2"}]}`
			got, err := restoreMaskedText(in, stored)
			require.NoError(t, err)
			assert.Equal(t, 2, strings.Count(got, `"password":`+pass), "restored: %s", got)
			assert.NotContains(t, got, secretMask)
		})
	}

	// A stored entry that holds the secret and one that holds none disagree at the path, so the
	// request is refused.
	_, err := restoreMaskedText(`{"a":[{"n":"x","password":"`+secretMask+`"}]}`,
		`{"a":[{"n":"x","password":"A"},{"n":"x"}]}`)
	assert.ErrorIs(t, err, errAmbiguousEntry)
}

// Entries pair on everything but their secrets, and a number is the same number however it is
// spelled, so a client that re-serializes 1.0 as 1 still gets its password back. Two different
// numbers never pair, however close they are.
func TestRestoreMaskedText_NumbersPairByValue(t *testing.T) {
	cases := map[string]struct {
		stored string
		sent   string
		paired bool
	}{
		"1.0 and 1":       {`1.0`, `1`, true},
		"1e0 and 1":       {`1e0`, `1`, true},
		"10 and 1e1":      {`10`, `1e1`, true},
		"0.50 and 0.5":    {`0.50`, `0.5`, true},
		"-0 and 0":        {`-0`, `0`, true},
		"1.0E+2 and 100":  {`1.0E+2`, `100`, true},
		"1 and 2":         {`1`, `2`, false},
		"1 and -1":        {`1`, `-1`, false},
		"big neighbours":  {`9007199254740993`, `9007199254740992`, false},
		"fraction digits": {`0.1`, `0.10000000000000001`, false},
		"huge exponent":   {`1e999999999999`, `1e999999999998`, false},

		"zero with a huge exponent":          {`0e1000000001`, `0`, true},
		"zero with a negative huge exponent": {`-0.0E-99999999999999999999`, `0`, true},
		"equal values beyond a billion":      {`10e1000000000`, `1e1000000001`, true},
		"equal values past int64":            {`1e99999999999999999999`, `1e99999999999999999999`, true},
		"respelled past int64":               {`1e99999999999999999999`, `10e99999999999999999998`, true},
		"respelled with a fraction":          {`0.1e99999999999999999999`, `1e99999999999999999998`, true},
		"explicit plus sign":                 {`1E+99999999999999999999`, `1e99999999999999999999`, true},
		"zero-padded exponent":               {`1e` + strings.Repeat("0", 1001), `1`, true},
		"zero-padded nonzero exponent":       {`1e` + strings.Repeat("0", 1001) + `5`, `100000`, true},
		"zero-padded negative exponent":      {`1E-` + strings.Repeat("0", 1001) + `2`, `0.01`, true},
		"long exponent, sign and plus":       {`1e` + strings.Repeat("1", 1001), `1E+` + strings.Repeat("1", 1001), true},
		"long exponent, carry":               {`10e` + strings.Repeat("9", 1001), `1e1` + strings.Repeat("0", 1001), true},
		"long exponent, borrow":              {`0.1e1` + strings.Repeat("0", 1001), `1e` + strings.Repeat("9", 1001), true},
		"long negative exponent, borrow":     {`10e-1` + strings.Repeat("0", 1001), `1e-` + strings.Repeat("9", 1001), true},
		"long exponents one apart":           {`1e1` + strings.Repeat("0", 1001), `1e` + strings.Repeat("9", 1001), false},
		"long exponents of opposite sign":    {`1e` + strings.Repeat("1", 1001), `1e-` + strings.Repeat("1", 1001), false},
		"different values past int64":        {`1e99999999999999999999`, `1e99999999999999999998`, false},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			stored := `{"a":[{"retries":` + c.stored + `,"password":"A"}]}`
			got, err := restoreMaskedText(`{"a":[{"retries":`+c.sent+`,"password":"`+secretMask+`"}]}`, stored)
			require.NoError(t, err)
			assert.Equal(t, c.paired, strings.Contains(got, `"password":"A"`), "restored: %s", got)
		})
	}
}

// A stored value that is the mask itself is not a saved secret: a save that sends the mask or leaves
// the secret out never gets it back, in an object and in an array, at the top or inside.
func TestTypedConfigRoutes_StoredMaskIsNeverRestored(t *testing.T) {
	cases := map[string]struct{ stored, entry string }{
		"top-level array, masked":  {`[{"name":"a","password":"` + secretMask + `"}]`, `[{"name":"a","password":"` + secretMask + `"}]`},
		"top-level array, omitted": {`[{"name":"a","password":"` + secretMask + `"}]`, `[{"name":"a"}]`},
		"nested array, omitted":    {`{"accounts":[{"name":"a","password":"` + secretMask + `"}]}`, `{"accounts":[{"name":"a"}]}`},
		"object, omitted":          {`{"password":"` + secretMask + `"}`, `{}`},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			h, cleanup := typedHandler(t)
			defer cleanup()
			id := typedDevice(t, h, `{"wifi":{"enable":true,"ssid":"home"},"raw":`+c.stored+`}`)

			body := `{"configuration":{"wifi":{"enable":true,"ssid":"home"},"raw":` + c.entry + `}}`
			put := serve(h.UpdateTypedDeviceConfig, "PUT", "/api/v1/devices/1/config/typed", id, body)
			require.Equal(t, http.StatusOK, put.Code, put.Body.String())
			assert.NotContains(t, storedConfig(t, h, id), secretMask)
		})
	}
}

// canonicalNumber agrees with exact arithmetic: over every small combination of sign, integer part,
// fraction and exponent, two spellings canonicalize alike exactly when their values are equal.
func TestCanonicalNumber_AgreesWithExactArithmetic(t *testing.T) {
	var spellings []string
	for _, sign := range []string{"", "-"} {
		for _, intPart := range []string{"0", "1", "10", "7", "100", "12"} {
			for _, frac := range []string{"", ".0", ".00", ".5", ".50", ".05", ".125"} {
				for _, exp := range []string{"", "e0", "E+0", "e-0", "e1", "e-1", "E+2", "e-2", "e03", "e-03", "e12", "e-12", "e30", "e-30"} {
					spellings = append(spellings, sign+intPart+frac+exp)
				}
			}
		}
	}
	value := func(s string) *big.Rat {
		r, ok := new(big.Rat).SetString(s)
		require.True(t, ok, s)
		return r
	}
	for _, a := range spellings {
		for _, b := range spellings {
			same := value(a).Cmp(value(b)) == 0
			if (canonicalNumber(a) == canonicalNumber(b)) != same {
				t.Fatalf("%s and %s: equal values %v, canonical %s and %s", a, b, same, canonicalNumber(a), canonicalNumber(b))
			}
		}
	}
}

// The same holds on the typed route: an unchanged read and save through a client that re-serializes
// its numbers keeps the entry's password, masked or left out.
func TestTypedConfigRoutes_NumberSpellingKeepsEntrySecret(t *testing.T) {
	const stored = `{"wifi":{"enable":true,"ssid":"home"},"raw":{"accounts":[{"name":"alice","retries":1.0,"password":"t7pe-alice-s3cret"}]}}`
	for name, entry := range map[string]string{
		"masked":  `{"name":"alice","retries":1,"password":"` + secretMask + `"}`,
		"omitted": `{"name":"alice","retries":1}`,
	} {
		t.Run(name, func(t *testing.T) {
			h, cleanup := typedHandler(t)
			defer cleanup()
			id := typedDevice(t, h, stored)

			body := `{"configuration":{"wifi":{"enable":true,"ssid":"home"},"raw":{"accounts":[` + entry + `]}}}`
			put := serve(h.UpdateTypedDeviceConfig, "PUT", "/api/v1/devices/1/config/typed", id, body)
			require.Equal(t, http.StatusOK, put.Code, put.Body.String())
			got := storedConfig(t, h, id)
			assert.Contains(t, got, "t7pe-alice-s3cret", "stored: %s", got)
			assert.NotContains(t, got, secretMask)
		})
	}
}
