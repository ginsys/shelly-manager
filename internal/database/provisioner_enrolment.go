package database

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ErrEnrolmentState is an admin act that the row's enrolment state does not allow: approving a row that is
// legacy or already approved, rejecting one that is legacy or already rejected, pre-approving a
// fingerprint that already has a row in any other state than pending.
var ErrEnrolmentState = errors.New("the agent's enrolment state does not allow this")

// ErrFingerprintInvalid is a fingerprint that is neither 32 hexadecimal characters nor eight groups of four
// joined by hyphens (NormalizeFingerprint), in either case: a wrong length, a character that is not a hexadecimal
// digit, or hyphens anywhere else.
var ErrFingerprintInvalid = errors.New("a fingerprint is 32 hexadecimal characters")

// keyedAgentIDPrefix starts the id of an agent enrolled by key: the prefix and the fingerprint.
const keyedAgentIDPrefix = "key-"

// NormalizeFingerprint returns the stored form of a fingerprint, 32 lowercase hexadecimal characters, from
// that form or from the readable one (FormatFingerprint), and whether the input was one of the two.
func NormalizeFingerprint(in string) (string, bool) {
	if len(in) == 39 {
		parts := strings.Split(in, "-")
		if len(parts) != 8 {
			return "", false
		}
		for _, p := range parts {
			if len(p) != 4 {
				return "", false
			}
		}
		in = strings.Join(parts, "")
	}
	if len(in) != 32 {
		return "", false
	}
	out := []byte(in)
	for i, c := range out {
		switch {
		case c >= '0' && c <= '9', c >= 'a' && c <= 'f':
		case c >= 'A' && c <= 'F':
			out[i] = c + 'a' - 'A'
		default:
			return "", false
		}
	}
	return string(out), true
}

// FormatFingerprint is the readable form of a stored fingerprint: eight groups of four characters joined by
// a hyphen, which the agent logs and the operator compares. A value of another length is returned as it is.
func FormatFingerprint(fp string) string {
	if len(fp) != 32 {
		return fp
	}
	groups := make([]string, 0, 8)
	for i := 0; i < 32; i += 4 {
		groups = append(groups, fp[i:i+4])
	}
	return strings.Join(groups, "-")
}

// lockAgentRow locks the row of the agent, or returns ErrAgentNotFound.
func lockAgentRow(tx *gorm.DB, agentID string) (ProvisionerAgent, error) {
	var agent ProvisionerAgent
	err := tx.Clauses(forUpdate()).Where("agent_id = ?", agentID).First(&agent).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return agent, ErrAgentNotFound
	}
	return agent, err
}

// approveLocked moves the locked row of a pending or rejected agent on, and writes the new state as its
// status. A rejected row that never had a key goes back to preapproved, since it has none to approve.
func approveLocked(tx *gorm.DB, agent ProvisionerAgent, now time.Time) error {
	state := EnrolmentApproved
	updates := map[string]any{"approved_at": now}
	if agent.State() == EnrolmentRejected && agent.PublicKey == nil {
		state, updates = EnrolmentPreapproved, map[string]any{}
	}
	updates["enrolment_state"], updates["status"] = state, state
	return tx.Model(&ProvisionerAgent{}).Where("id = ?", agent.ID).Updates(updates).Error
}

// reread returns the agent row as it stands in tx.
func reread(tx *gorm.DB, id uint) (ProvisionerAgent, error) {
	var agent ProvisionerAgent
	err := tx.Where("id = ?", id).First(&agent).Error
	return agent, err
}

// ApproveAgent takes a pending or rejected agent to approved, or a rejected one that never had a key to
// preapproved, and writes the new state as its status. Its token is not touched: the agent gets one by a
// signed register. Any other state is ErrEnrolmentState, a legacy row included.
func (s *ProvisionerStore) ApproveAgent(ctx context.Context, agentID string, now time.Time) (ProvisionerAgent, error) {
	var out ProvisionerAgent
	err := s.tx(ctx, func(tx *gorm.DB) error {
		agent, err := lockAgentRow(tx, agentID)
		if err != nil {
			return err
		}
		if state := agent.State(); state != EnrolmentPending && state != EnrolmentRejected {
			return ErrEnrolmentState
		}
		if err = approveLocked(tx, agent, now); err != nil {
			return err
		}
		out, err = reread(tx, agent.ID)
		return err
	})
	return out, err
}

// RejectAgent takes a pending, preapproved or approved agent to rejected. It replaces the token, so a
// running agent loses its session at once, clears the challenge, writes rejected as the status and fails
// the agent's assigned and in-progress tasks. Its pending tasks stay. Any other state is ErrEnrolmentState.
func (s *ProvisionerStore) RejectAgent(ctx context.Context, agentID string, now time.Time) (ProvisionerAgent, error) {
	var out ProvisionerAgent
	err := s.tx(ctx, func(tx *gorm.DB) error {
		agent, err := lockAgentRow(tx, agentID)
		if err != nil {
			return err
		}
		if state := agent.State(); state != EnrolmentPending && state != EnrolmentPreapproved && state != EnrolmentApproved {
			return ErrEnrolmentState
		}
		hash, err := neverIssuedTokenHash()
		if err != nil {
			return err
		}
		if err = failStranded(tx, agent.AgentID, "the agent was rejected", now); err != nil {
			return err
		}
		err = tx.Model(&ProvisionerAgent{}).Where("id = ?", agent.ID).Updates(map[string]any{
			"enrolment_state": EnrolmentRejected, "status": EnrolmentRejected, "token_hash": hash,
			"challenge": nil, "challenge_expires_at": nil,
		}).Error
		if err != nil {
			return err
		}
		out, err = reread(tx, agent.ID)
		return err
	})
	return out, err
}

// PreapproveFingerprint names a fingerprint ahead of its agent: it creates a preapproved row, or approves the
// pending row that has the fingerprint, and reports whether the row is new. A row that exists in any other
// state is ErrEnrolmentState. The id is the prefix and the fingerprint, so one fingerprint has one row, and
// the insert goes first as RegisterAgent's does, so two admins naming it at once do not deadlock.
func (s *ProvisionerStore) PreapproveFingerprint(ctx context.Context, fingerprint string, now time.Time) (ProvisionerAgent, bool, error) {
	fp, ok := NormalizeFingerprint(fingerprint)
	if !ok {
		return ProvisionerAgent{}, false, ErrFingerprintInvalid
	}
	hash, hashErr := neverIssuedTokenHash()
	if hashErr != nil {
		return ProvisionerAgent{}, false, hashErr
	}
	state := EnrolmentPreapproved
	row := ProvisionerAgent{
		AgentID: keyedAgentIDPrefix + fp, TokenHash: hash, Status: state, LastSeen: now, RegisteredAt: now,
		EnrolmentState: &state, Fingerprint: &fp,
	}
	var out ProvisionerAgent
	created := false
	err := s.tx(ctx, func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; err != nil {
			return err
		}
		cur, err := lockAgentRow(tx, row.AgentID)
		// No row at the id after the insert: either the row was deleted in between, which a 409 states
		// honestly, or the insert was a no-op because another id holds this fingerprint (its own unique
		// index), a state no register produces, since an id is derived from its fingerprint.
		if errors.Is(err, ErrAgentNotFound) {
			return ErrEnrolmentState
		}
		if err != nil {
			return err
		}
		// The token is new and random, so the row holds its hash only if this insert wrote the row.
		if cur.TokenHash == hash {
			out, created = cur, true
			return nil
		}
		if cur.State() != EnrolmentPending || cur.Fingerprint == nil || *cur.Fingerprint != fp {
			return ErrEnrolmentState
		}
		if err = approveLocked(tx, cur, now); err != nil {
			return err
		}
		out, err = reread(tx, cur.ID)
		return err
	})
	return out, created, err
}
