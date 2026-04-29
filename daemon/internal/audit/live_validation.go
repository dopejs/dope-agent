package audit

import (
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/livevalidation"
)

const LiveValidationAuditEventKind = "live_validation.audit_recorded"

type LiveValidationAuditInput struct {
	Attempt    livevalidation.Attempt
	Denials    []livevalidation.Denial
	Action     string
	Outcome    string
	ReasonCode string
	CreatedAt  time.Time
}

type LiveValidationLedgerAuditInput struct {
	Entry      livevalidation.SideEffectLedgerEntry
	Action     string
	Outcome    string
	ReasonCode string
	CreatedAt  time.Time
}

type LiveValidationReconciliationAuditInput struct {
	Resolution livevalidation.ReconciliationResolution
	Action     string
	Outcome    string
	ReasonCode string
}

type LiveValidationKillSwitchAuditInput struct {
	KillSwitch livevalidation.KillSwitch
	Action     string
	Outcome    string
	ReasonCode string
}

func BuildLiveValidationAuditEvent(input LiveValidationAuditInput) identity.TenantAuditEvent {
	createdAt := input.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	outcome := input.Outcome
	if outcome == "" {
		outcome = identity.AuditOutcomeSucceeded
	}
	reasonCode := input.ReasonCode
	if reasonCode == "" && len(input.Denials) > 0 {
		reasonCode = input.Denials[0].ReasonCode
	}
	document := map[string]any{
		"action":       stringOrDefault(input.Action, "live_validation.start"),
		"validationId": input.Attempt.ValidationID,
		"candidateId":  input.Attempt.CandidateID,
		"status":       string(input.Attempt.Status),
		"denials":      input.Denials,
	}
	return identity.TenantAuditEvent{
		EventKind:   LiveValidationAuditEventKind,
		TenantID:    input.Attempt.TenantID,
		PrincipalID: input.Attempt.RequestedBy,
		Outcome:     outcome,
		ReasonCode:  reasonCode,
		CreatedAt:   createdAt.UTC(),
		Document:    document,
	}
}

func BuildLiveValidationLedgerAuditEvent(input LiveValidationLedgerAuditInput) identity.TenantAuditEvent {
	createdAt := input.CreatedAt
	if createdAt.IsZero() {
		createdAt = input.Entry.UpdatedAt
	}
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	return identity.TenantAuditEvent{
		EventKind:  LiveValidationAuditEventKind,
		TenantID:   input.Entry.TenantID,
		Outcome:    stringOrDefault(input.Outcome, identity.AuditOutcomeSucceeded),
		ReasonCode: stringOrDefault(input.ReasonCode, input.Entry.ReasonCode),
		CreatedAt:  createdAt.UTC(),
		Document: map[string]any{
			"action":        stringOrDefault(input.Action, "live_validation.ledger"),
			"validationId":  input.Entry.ValidationID,
			"ledgerEntryId": input.Entry.LedgerEntryID,
			"toolClass":     input.Entry.ToolClass,
			"outcome":       input.Entry.Outcome,
		},
	}
}

func BuildLiveValidationReconciliationAuditEvent(input LiveValidationReconciliationAuditInput) identity.TenantAuditEvent {
	return identity.TenantAuditEvent{
		EventKind:   LiveValidationAuditEventKind,
		TenantID:    input.Resolution.TenantID,
		PrincipalID: input.Resolution.ResolvedBy,
		Outcome:     stringOrDefault(input.Outcome, identity.AuditOutcomeSucceeded),
		ReasonCode:  stringOrDefault(input.ReasonCode, "live_validation.reconciliation_resolved"),
		CreatedAt:   input.Resolution.ResolvedAt.UTC(),
		Document: map[string]any{
			"action":            stringOrDefault(input.Action, "live_validation.reconcile"),
			"reconciliationId":  input.Resolution.ReconciliationID,
			"ambiguousCommitId": input.Resolution.AmbiguousCommitID,
			"resolution":        input.Resolution.Resolution,
		},
	}
}

func BuildLiveValidationKillSwitchAuditEvent(input LiveValidationKillSwitchAuditInput) identity.TenantAuditEvent {
	return identity.TenantAuditEvent{
		EventKind:   LiveValidationAuditEventKind,
		TenantID:    input.KillSwitch.TenantID,
		PrincipalID: input.KillSwitch.ChangedBy,
		Outcome:     stringOrDefault(input.Outcome, identity.AuditOutcomeSucceeded),
		ReasonCode:  stringOrDefault(input.ReasonCode, "live_validation.kill_switch_changed"),
		CreatedAt:   input.KillSwitch.ChangedAt.UTC(),
		Document: map[string]any{
			"action":       stringOrDefault(input.Action, "live_validation.kill_switch"),
			"killSwitchId": input.KillSwitch.KillSwitchID,
			"scope":        input.KillSwitch.Scope,
			"enabled":      input.KillSwitch.Enabled,
			"reason":       input.KillSwitch.Reason,
		},
	}
}
