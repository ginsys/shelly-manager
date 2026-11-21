package database

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"gorm.io/gorm"
)

var (
	// ErrPendingCapReached is a register of a new key while the cap of rows that wait for the operator is full.
	ErrPendingCapReached = errors.New("too many agents wait for approval")
	// ErrChallengeFailed is a signed register with no challenge stored, an expired one, a key that is not the
	// row's, or a signature that does not verify. It writes nothing, so a bad signature does not spend the
	// challenge.
	ErrChallengeFailed = errors.New("the challenge is missing or expired, or the signature does not verify")
	// ErrKeyFormat is a public key that is not 32 bytes or a signature that is not 64: ed25519.Verify panics on
	// the first, so the store refuses it itself and does not rely on its caller.
	ErrKeyFormat = errors.New("a public key is 32 bytes and a signature 64")
)

// registerMessageHeader starts the text an agent signs. It keeps the key from being used to sign anything
// else, and the zero bytes between the fields keep one field from shifting into the next.
const registerMessageHeader = "shelly-manager provisioner register v1"

// challengeLifetime is how long an agent has to answer the challenge it was given.
const challengeLifetime = 60 * time.Second

// KeyFingerprint is the fingerprint of a public key: the first 16 bytes of the SHA-256 of its 32 bytes, as 32
// lowercase hexadecimal characters.
func KeyFingerprint(key ed25519.PublicKey) string {
	sum := sha256.Sum256(key)
	return hex.EncodeToString(sum[:16])
}

// KeyedAgentID is the id of an agent enrolled by key: the prefix and its fingerprint.
func KeyedAgentID(fingerprint string) string { return keyedAgentIDPrefix + fingerprint }

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

// RegisterOutcome is what a keyed register came to.
type RegisterOutcome string

// The four answers of a keyed register. The first three are normal enrolment answers and carry no token.
const (
	OutcomePending    RegisterOutcome = "pending"
	OutcomeChallenge  RegisterOutcome = "challenge"
	OutcomeRejected   RegisterOutcome = "rejected"
	OutcomeRegistered RegisterOutcome = "registered"
)

// KeyedRegistration is one register by key. Signature is nil for the unsigned request. Details carries what
// the agent reported (hostname, IP, version, capabilities, metadata) and the token hash to issue on a good
// signature; PendingCap is the most rows that may wait for the operator.
type KeyedRegistration struct {
	PublicKey  ed25519.PublicKey
	Signature  []byte
	Details    ProvisionerAgent
	PendingCap int
}

// KeyedResult is the answer: the outcome, the row as it stands and, for a challenge, its 64 hex characters.
type KeyedResult struct {
	Outcome   RegisterOutcome
	Agent     ProvisionerAgent
	Challenge string
}

// RegisterKeyed registers an agent by its key. The id is derived from the key, so one key has one row.
//
//   - No row: a pending row is written (ErrPendingCapReached when the cap is full) and the outcome is pending.
//   - A pending row is refreshed with the reported details and the outcome is pending. A rejected row is left
//     as it is and the outcome is rejected. A legacy row, or one in a state this code does not know, is
//     ErrEnrolmentState.
//   - An approved or preapproved row, unsigned: a new challenge is stored, and nothing else is written.
//   - An approved or preapproved row, signed: the challenge must be stored and not expired, the key the row's
//     (a preapproved row has none yet, so the key must hash to its fingerprint) and the signature must verify,
//     else ErrChallengeFailed with nothing written. A good signature spends the challenge, replaces the token
//     with Details.TokenHash, takes a preapproved row to approved and stores its key, and writes the reported
//     details, in one transaction under the row's lock. No silence test applies: the key is the authorization.
func (s *ProvisionerStore) RegisterKeyed(ctx context.Context, reg KeyedRegistration, now time.Time) (KeyedResult, error) {
	if len(reg.PublicKey) != ed25519.PublicKeySize || (reg.Signature != nil && len(reg.Signature) != ed25519.SignatureSize) {
		return KeyedResult{}, ErrKeyFormat
	}
	fp, key := KeyFingerprint(reg.PublicKey), hex.EncodeToString(reg.PublicKey)
	hash, hashErr := neverIssuedTokenHash()
	if hashErr != nil {
		return KeyedResult{}, hashErr
	}
	pending := EnrolmentPending
	row := reg.Details
	row.AgentID, row.TokenHash, row.Status = KeyedAgentID(fp), hash, EnrolmentPending
	row.PublicKey, row.Fingerprint, row.EnrolmentState = &key, &fp, &pending
	row.LastSeen, row.RegisteredAt = now, now

	var out KeyedResult
	err := s.tx(ctx, func(tx *gorm.DB) error {
		cur, wrote, err := s.insertThenLock(tx, &row)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrEnrolmentState // another id holds this fingerprint: no register produces that
		}
		if err != nil {
			return err
		}
		if wrote {
			var waiting int64
			if err = tx.Model(&ProvisionerAgent{}).Where("enrolment_state = ?", EnrolmentPending).Count(&waiting).Error; err != nil {
				return err
			}
			if waiting > int64(reg.PendingCap) { // the count includes the row just written
				return ErrPendingCapReached
			}
			out = KeyedResult{Outcome: OutcomePending, Agent: cur}
			return nil
		}
		switch cur.State() {
		case EnrolmentPending:
			cur.Hostname, cur.IP, cur.Version = reg.Details.Hostname, reg.Details.IP, reg.Details.Version
			cur.Capabilities, cur.Metadata, cur.Status, cur.LastSeen = reg.Details.Capabilities, reg.Details.Metadata, EnrolmentPending, now
			out = KeyedResult{Outcome: OutcomePending, Agent: cur}
			return tx.Save(&cur).Error
		case EnrolmentRejected:
			out = KeyedResult{Outcome: OutcomeRejected, Agent: cur}
			return nil
		case EnrolmentApproved, EnrolmentPreapproved:
			if reg.Signature == nil {
				raw := make([]byte, 32)
				if _, err = rand.Read(raw); err != nil {
					return err
				}
				// Truncated to the millisecond: MySQL stores datetime(3) and rounds, so an unrounded expiry could
				// read back up to half a millisecond after the 60 seconds it promised.
				challenge, expires := hex.EncodeToString(raw), now.Add(challengeLifetime).Truncate(time.Millisecond)
				cur.Challenge, cur.ChallengeExpiresAt = &challenge, &expires
				out = KeyedResult{Outcome: OutcomeChallenge, Agent: cur, Challenge: challenge}
				// UpdateColumns, not Updates: Updates also moves updated_at, and the note says nothing but the
				// challenge is written here.
				return tx.Model(&ProvisionerAgent{}).Where("id = ?", cur.ID).
					UpdateColumns(map[string]any{"challenge": challenge, "challenge_expires_at": expires}).Error
			}
			if err = verifyChallenge(cur, reg, fp, key, now); err != nil {
				return err
			}
			if cur.State() == EnrolmentPreapproved {
				cur.ApprovedAt = &now
			}
			approved := EnrolmentApproved
			cur.TokenHash, cur.PublicKey, cur.EnrolmentState = reg.Details.TokenHash, &key, &approved
			cur.Challenge, cur.ChallengeExpiresAt = nil, nil
			cur.Hostname, cur.IP, cur.Version = reg.Details.Hostname, reg.Details.IP, reg.Details.Version
			cur.Capabilities, cur.Metadata, cur.Status, cur.LastSeen = reg.Details.Capabilities, reg.Details.Metadata, "online", now
			out = KeyedResult{Outcome: OutcomeRegistered, Agent: cur}
			return tx.Save(&cur).Error
		}
		return ErrEnrolmentState
	})
	return out, err
}

// verifyChallenge reads the locked row of an approved or preapproved agent against a signed register: the
// challenge stored and not expired, the key the row's, and the signature good over the message for the row's id.
func verifyChallenge(cur ProvisionerAgent, reg KeyedRegistration, fp, key string, now time.Time) error {
	if cur.Challenge == nil || cur.ChallengeExpiresAt == nil || !now.Before(*cur.ChallengeExpiresAt) {
		return ErrChallengeFailed
	}
	if cur.State() == EnrolmentApproved && (cur.PublicKey == nil || *cur.PublicKey != key) {
		return ErrChallengeFailed
	}
	if cur.State() == EnrolmentPreapproved && (cur.Fingerprint == nil || *cur.Fingerprint != fp) {
		return ErrChallengeFailed
	}
	challenge, err := hex.DecodeString(*cur.Challenge)
	if err != nil || !ed25519.Verify(reg.PublicKey, RegisterMessage(cur.AgentID, challenge), reg.Signature) {
		return ErrChallengeFailed
	}
	return nil
}
