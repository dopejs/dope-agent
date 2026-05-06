package setupwizard

import (
	"context"

	"github.com/dopejs/dope-agent/daemon/internal/identity"
)

type TenantAuditStore interface {
	AppendTenantAuditEvent(ctx context.Context, event identity.TenantAuditEvent) (identity.TenantAuditEvent, error)
}

type TenantAuditRecorder struct {
	store TenantAuditStore
}

func NewTenantAuditRecorder(store TenantAuditStore) TenantAuditRecorder {
	return TenantAuditRecorder{store: store}
}

func (r TenantAuditRecorder) RecordSetupAudit(ctx context.Context, record SetupAuditRecord) (string, error) {
	if r.store == nil {
		return "", nil
	}
	event := identity.TenantAuditEvent{
		EventKind:   record.EventKind,
		TenantID:    record.TenantID,
		PrincipalID: record.PrincipalID,
		Outcome:     tenantAuditOutcome(record.Outcome),
		ReasonCode:  firstNonEmpty(record.ReasonCode, "setup_transition"),
		CreatedAt:   record.CreatedAt,
		Document: map[string]any{
			"setupSessionId":     record.SetupSessionID,
			"targetId":           record.TargetID,
			"targetKind":         record.TargetKind,
			"setupStyle":         record.SetupStyle,
			"operation":          record.Operation,
			"fromState":          record.FromState,
			"toState":            record.ToState,
			"retryable":          record.Retryable,
			"remediationOwner":   record.RemediationOwner,
			"safeUseMode":        record.SafeUseMode,
			"diagnosticResultId": record.DiagnosticResultID,
			"redactionStatus":    record.RedactionStatus,
			"resourceRefs":       record.ResourceRefs,
		},
	}
	written, err := r.store.AppendTenantAuditEvent(ctx, event)
	if err != nil {
		return "", err
	}
	return written.AuditEventID, nil
}

func AuditRecordForAttempt(session SetupSession, attempt SetupAttempt) SetupAuditRecord {
	return SetupAuditRecord{
		EventKind:          "credential_setup." + auditEventSuffix(attempt.Operation, attempt.ToState),
		TenantID:           session.TenantID,
		PrincipalID:        firstNonEmpty(attempt.ActorPrincipalID, session.ActorPrincipalID),
		SetupSessionID:     session.SetupSessionID,
		TargetID:           session.TargetID,
		TargetKind:         session.TargetKind,
		SetupStyle:         session.SetupStyle,
		Operation:          attempt.Operation,
		FromState:          attempt.FromState,
		ToState:            attempt.ToState,
		ReasonCode:         attempt.ReasonCode,
		Retryable:          session.Retryable,
		RemediationOwner:   session.RemediationOwner,
		SafeUseMode:        session.SafeUseMode,
		DiagnosticResultID: session.DiagnosticResultID,
		ResourceRefs:       append([]ResourceRef(nil), session.ResourceRefs...),
		RedactionStatus:    attempt.RedactionStatus,
		Outcome:            auditOutcome(attempt.ToState, attempt.RedactionStatus),
		CreatedAt:          attempt.CreatedAt,
	}
}

func tenantAuditOutcome(outcome string) string {
	switch outcome {
	case "failed_closed":
		return identity.AuditOutcomeFailedClosed
	case "denied":
		return identity.AuditOutcomeDenied
	default:
		return identity.AuditOutcomeSucceeded
	}
}
