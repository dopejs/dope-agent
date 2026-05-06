package setupwizard

import (
	"testing"
	"time"
)

func TestAuditRecordForAttemptIsMetadataOnlyAndUsesEventFamilies(t *testing.T) {
	session := SetupSession{
		SetupSessionID:     "setup_audit",
		TenantID:           "ten_audit",
		ActorPrincipalID:   "prn_audit",
		TargetID:           TargetOpenAICompatible,
		TargetKind:         TargetKindProvider,
		SetupStyle:         SetupStyleSubmittedSecret,
		State:              StateReady,
		Retryable:          false,
		RemediationOwner:   OwnerNoneRequired,
		SafeUseMode:        SafeUseNormal,
		DiagnosticResultID: "diag_audit",
		ResourceRefs:       []ResourceRef{{Kind: "tenant_secret", ID: "provider/openai-compatible"}},
		RedactionStatus:    RedactionRedacted,
	}
	attempt := SetupAttempt{
		AttemptID:        "attempt_audit",
		SetupSessionID:   session.SetupSessionID,
		TenantID:         session.TenantID,
		ActorPrincipalID: "prn_audit",
		Operation:        OperationSubmitSecret,
		FromState:        StateInProgress,
		ToState:          StateReady,
		RedactionStatus:  RedactionRedacted,
		CreatedAt:        time.Date(2026, 5, 6, 12, 30, 0, 0, time.UTC),
	}
	record := AuditRecordForAttempt(session, attempt)
	if record.EventKind != "credential_setup.secret_submitted" || record.Outcome != "succeeded" {
		t.Fatalf("unexpected audit record: %+v", record)
	}
	if ContainsForbiddenEvidence(record, []string{"R46_FAKE_OPENAI_COMPATIBLE_KEY_DO_NOT_LEAK", "access_token", "refresh_token"}) {
		t.Fatalf("audit record leaked forbidden evidence: %+v", record)
	}
}

func TestAuditRecordFailClosedOutcome(t *testing.T) {
	record := AuditRecordForAttempt(SetupSession{
		SetupSessionID:   "setup_redaction",
		TenantID:         "ten_redaction",
		TargetID:         TargetFeishuLark,
		TargetKind:       TargetKindIntegration,
		SetupStyle:       SetupStyleOAuth,
		RedactionStatus:  RedactionFailedClosed,
		RemediationOwner: OwnerOperator,
	}, SetupAttempt{
		Operation:       OperationOAuthCallback,
		ToState:         StateActionRequired,
		RedactionStatus: RedactionFailedClosed,
	})
	if record.Outcome != "failed_closed" || record.EventKind != "credential_setup.action_required" {
		t.Fatalf("unexpected fail-closed audit record: %+v", record)
	}
}
