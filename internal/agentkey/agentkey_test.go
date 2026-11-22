package agentkey

import (
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The seed, key, fingerprint and signature below were computed once, outside this package, with an independent
// Ed25519 implementation (Python's cryptography library, with SHA-256 of the 32 public key bytes and a signature
// over the text the design note defines); its control reproduced the signature pinned before the id prefix
// changed. The test checks the signature literal against the pinned key and message with the standard library.
// They are literals on purpose: the manager and the agent both read this package, so a change to
// the wire format would otherwise change both sides together and no test would notice.
const (
	vectorSeed        = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"
	vectorPublicKey   = "03a107bff3ce10be1d70dd18e74bc09967e4d6309ba50d5f1ddc8664125531b8"
	vectorFingerprint = "56475aa75463474c0285df5dbf2bcab7"
	vectorReadable    = "5647-5aa7-5463-474c-0285-df5d-bf2b-cab7"
	// The id is "keyed-" and the fingerprint, as one fixed value (never computed from the code under test).
	vectorID = "keyed-56475aa75463474c0285df5dbf2bcab7"
	// the signature over RegisterMessage(vectorID, 0xa0, 0xa1, ... 0xbf)
	vectorSignature = "7cca1191d866d2dc141322c514d3927eb66a587dfa1fd5f493ed224c44e8334a63de05da2c1929f5d504c2f407b55125254882e7c767c8a07cc35fcf40577001"
	// the bytes that signature covers: the text, 0, the id, 0, the 32 challenge bytes
	vectorMessage = "7368656c6c792d6d616e616765722070726f766973696f6e6572207265676973746572207631006b657965642d353634373561613735343633343734633032383564663564626632626361623700" +
		"a0a1a2a3a4a5a6a7a8a9aaabacadaeafb0b1b2b3b4b5b6b7b8b9babbbcbdbebf"
)

func vectorChallenge() []byte {
	b := make([]byte, 32)
	for i := range b {
		b[i] = byte(0xa0 + i)
	}
	return b
}

func TestFromSeed_KnownVector(t *testing.T) {
	key, err := FromSeed(vectorSeed)
	require.NoError(t, err)
	assert.Equal(t, vectorPublicKey, key.PublicKeyHex())
	assert.Equal(t, vectorFingerprint, key.Fingerprint())
	assert.Equal(t, vectorReadable, key.Readable())
	assert.Equal(t, vectorID, key.ID())
	assert.Equal(t, vectorFingerprint, Fingerprint(ed25519.PublicKey(mustHex(t, vectorPublicKey))))
	assert.Equal(t, vectorID, AgentID(vectorFingerprint))
	assert.Equal(t, vectorReadable, Readable(vectorFingerprint))
}

func TestFromSeed_SameSeedSameKeyEveryTime(t *testing.T) {
	first, err := FromSeed(vectorSeed)
	require.NoError(t, err)
	second, err := FromSeed(strings.ToUpper(vectorSeed))
	require.NoError(t, err, "a seed pasted in capitals is the same seed")
	assert.Equal(t, first.PublicKeyHex(), second.PublicKeyHex())
	assert.Equal(t, first.ID(), second.ID())
	other, err := FromSeed(strings.Repeat("ab", 32))
	require.NoError(t, err)
	assert.NotEqual(t, first.Fingerprint(), other.Fingerprint(), "a new seed is a new identity")
}

func TestRegisterMessage_Bytes(t *testing.T) {
	assert.Equal(t, vectorMessage, hex.EncodeToString(RegisterMessage(vectorID, vectorChallenge())))
}

func TestKeySign_VerifiesOverTheMessage(t *testing.T) {
	key, err := FromSeed(vectorSeed)
	require.NoError(t, err)
	sig := key.Sign(vectorID, vectorChallenge())
	assert.Equal(t, vectorSignature, sig, "ed25519 is deterministic: the signature is the literal")
	assert.True(t, ed25519.Verify(mustHex(t, vectorPublicKey), mustHex(t, vectorMessage), mustHex(t, sig)))
	assert.False(t, ed25519.Verify(mustHex(t, vectorPublicKey), RegisterMessage("keyed-other", vectorChallenge()), mustHex(t, sig)),
		"the id is part of what is signed")
}

func TestNewSeed(t *testing.T) {
	a, err := NewSeed()
	require.NoError(t, err)
	b, err := NewSeed()
	require.NoError(t, err)
	assert.Regexp(t, `^[0-9a-f]{64}$`, a)
	assert.NotEqual(t, a, b)
	_, err = FromSeed(a)
	require.NoError(t, err, "a generated seed is one FromSeed takes")
}

func TestFromSeed_RefusesABadSeedWithoutEchoingIt(t *testing.T) {
	for name, seed := range map[string]string{
		"empty":     "",
		"short":     vectorSeed[:62],
		"long":      vectorSeed + "00",
		"not hex":   strings.Repeat("zz", 32),
		"with dash": vectorSeed[:32] + "-" + vectorSeed[33:],
	} {
		t.Run(name, func(t *testing.T) {
			key, err := FromSeed(seed)
			require.Error(t, err)
			assert.Nil(t, key)
			if seed != "" {
				assert.NotContains(t, err.Error(), seed, "the error never carries the value it refused")
				assert.NotContains(t, err.Error(), seed[:8])
			}
		})
	}
}

// A stray %v or %+v of the key must print the fingerprint and never the private bytes.
func TestKey_PrintsTheFingerprintOnly(t *testing.T) {
	key, err := FromSeed(vectorSeed)
	require.NoError(t, err)
	for _, verb := range []string{"%v", "%+v", "%s", "%#v"} {
		out := fmt.Sprintf(verb, key)
		assert.NotContains(t, out, vectorSeed, verb)
		assert.NotContains(t, out, "000102030405", verb)
		assert.NotContains(t, out, "1, 2, 3", verb, "no byte dump")
	}
	assert.Equal(t, vectorReadable, fmt.Sprintf("%v", key))
}

func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	require.NoError(t, err)
	return b
}
