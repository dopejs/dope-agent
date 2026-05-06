package setupwizard

import (
	"context"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/identity"
)

func (s *Service) Diagnostics(ctx context.Context, tenantContext identity.TenantContext, sessionID string) ([]SetupDiagnostic, error) {
	session, err := s.Get(ctx, tenantContext, sessionID)
	if err != nil {
		return nil, err
	}
	return []SetupDiagnostic{DiagnosticForSession(session, s.now())}, nil
}

func DiagnosticForSession(session SetupSession, now time.Time) SetupDiagnostic {
	reason := session.ReasonCode
	if reason == "" && session.State == StateReady {
		reason = ReasonHealthy
	}
	return SetupDiagnostic{
		SetupSessionID:       session.SetupSessionID,
		TargetID:             session.TargetID,
		DiagnosticResultID:   ensureDiagnosticID(session),
		DiagnosticRunID:      session.DiagnosticRunID,
		DiagnosticStage:      session.DiagnosticStage,
		DiagnosticSourceKind: session.DiagnosticSourceKind,
		DiagnosticSourceID:   session.DiagnosticSourceID,
		Status:               session.State,
		ReasonCode:           reason,
		RetrySafety:          retrySafetyForState(session.State),
		RemediationOwner:     session.RemediationOwner,
		AllowedCapabilities:  append([]string(nil), session.DiagnosticAllowedUse...),
		CheckedAt:            now,
		StaleAfter:           now.Add(15 * time.Minute),
		RedactionStatus:      session.RedactionStatus,
	}
}

func ClassifyDiagnosticReason(reason string) (SetupState, RemediationOwner, RetrySafety) {
	switch reason {
	case ReasonHealthy:
		return StateReady, OwnerNoneRequired, RetryNoActionNeeded
	case ReasonScopeMissing, ReasonTenantApprovalPending, ReasonCredentialMissing, ReasonTokenMissing, ReasonTokenExpired, ReasonTokenRevoked, ReasonOAuthDenied, ReasonOAuthExpired, ReasonOAuthReplay, ReasonTenantMismatch:
		return StateActionRequired, OwnerTenantAdmin, RetryRetryable
	case ReasonProviderUnavailable, ReasonNetworkFailed, ReasonRateLimited:
		return StateUnavailable, OwnerProvider, RetryRetryable
	case ReasonUnsupportedTarget:
		return StateActionRequired, OwnerOperator, RetryBlocked
	case ReasonRedactionFailedClosed:
		return StateActionRequired, OwnerOperator, RetryBlocked
	default:
		return StateUnavailable, OwnerOperator, RetryRetryable
	}
}
