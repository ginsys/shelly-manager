// Package agentkey is the one definition of how a provisioner agent is identified by a key: the seed that makes
// the key, the fingerprint and the agent id derived from the public key, and the text an agent signs to answer
// a register challenge. The manager and the agent both read it, so the wire format cannot drift between them.
// It uses the standard library only, because the agent must not link the manager's database drivers.
package agentkey

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
)

// IDPrefix starts the id of an agent enrolled by key: the prefix and the fingerprint. It is not "key-": that
// prefix and 32 hex characters is the shape of a Mailgun key, and secret scanners would flag every agent id.
const IDPrefix = "keyed-"

// registerMessageHeader starts the text an agent signs. It keeps the key from being used to sign anything
// else, and the zero bytes between the fields keep one field from shifting into the next.
const registerMessageHeader = "shelly-manager provisioner register v1"

// ErrSeedFormat is a seed that is not 64 hexadecimal characters. It never carries the value it refused.
var ErrSeedFormat = errors.New("the agent seed must be 64 hexadecimal characters (32 bytes)")

// Fingerprint is the fingerprint of a public key: the first 16 bytes of the SHA-256 of its 32 bytes, as 32
// lowercase hexadecimal characters.
func Fingerprint(key ed25519.PublicKey) string {
	sum := sha256.Sum256(key)
	return hex.EncodeToString(sum[:16])
}

// AgentID is the id of an agent enrolled by key: the prefix and its fingerprint.
func AgentID(fingerprint string) string { return IDPrefix + fingerprint }

// Readable is the readable form of a stored fingerprint: eight groups of four characters joined by a hyphen,
// which the agent prints and the operator compares. A value of another length is returned as it is.
func Readable(fp string) string {
	if len(fp) != 32 {
		return fp
	}
	groups := make([]string, 0, 8)
	for i := 0; i < 32; i += 4 {
		groups = append(groups, fp[i:i+4])
	}
	return strings.Join(groups, "-")
}

// RegisterMessage is what an agent signs to answer a challenge: the ASCII text of registerMessageHeader, a zero
// byte, the agent id, a zero byte and the 32 challenge bytes (decoded, never the hex text).
func RegisterMessage(agentID string, challenge []byte) []byte {
	msg := make([]byte, 0, len(registerMessageHeader)+len(agentID)+len(challenge)+2)
	msg = append(msg, registerMessageHeader...)
	msg = append(msg, 0)
	msg = append(msg, agentID...)
	msg = append(msg, 0)
	return append(msg, challenge...)
}

// NewSeed returns a new seed: 32 random bytes from crypto/rand as 64 lowercase hexadecimal characters.
func NewSeed() (string, error) {
	seed := make([]byte, ed25519.SeedSize)
	if _, err := rand.Read(seed); err != nil {
		return "", err
	}
	return hex.EncodeToString(seed), nil
}

// Key is an agent's signing key. Its private half stays inside: printing a Key shows the fingerprint only.
type Key struct {
	private ed25519.PrivateKey
}

// FromSeed makes the key of a seed (64 hexadecimal characters, either case). The same seed gives the same key.
// The error never echoes the value.
func FromSeed(seedHex string) (*Key, error) {
	if len(seedHex) != 2*ed25519.SeedSize {
		return nil, ErrSeedFormat
	}
	seed, err := hex.DecodeString(seedHex)
	if err != nil {
		return nil, ErrSeedFormat
	}
	return &Key{private: ed25519.NewKeyFromSeed(seed)}, nil
}

func (k Key) public() ed25519.PublicKey { return k.private.Public().(ed25519.PublicKey) }

// PublicKeyHex is the public key as 64 lowercase hexadecimal characters, as the register body carries it.
func (k Key) PublicKeyHex() string { return hex.EncodeToString(k.public()) }

// Fingerprint is the stored form of the key's fingerprint, 32 lowercase hexadecimal characters.
func (k Key) Fingerprint() string { return Fingerprint(k.public()) }

// Readable is the fingerprint as eight groups of four.
func (k Key) Readable() string { return Readable(k.Fingerprint()) }

// ID is the agent id the key derives.
func (k Key) ID() string { return AgentID(k.Fingerprint()) }

// Sign answers a challenge: the signature over RegisterMessage, as 128 lowercase hexadecimal characters.
func (k Key) Sign(agentID string, challenge []byte) string {
	return hex.EncodeToString(ed25519.Sign(k.private, RegisterMessage(agentID, challenge)))
}

// String and GoString print the fingerprint, so a stray %v, %+v, %s or %#v never prints the private bytes.
func (k Key) String() string { return k.Readable() }

// GoString: see String.
func (k Key) GoString() string { return k.Readable() }
