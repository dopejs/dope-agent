package connectors

import (
	"testing"
	"time"
)

func TestClassifyDiagnosticUsesContractReasonMetadata(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 7, 10, 0, 0, 0, time.UTC)
	state, err := ClassifyDiagnostic(DiagnosticInput{
		TenantID:           "ten_033",
		ConnectorID:        "connector_discord",
		ConnectorAccountID: "acct_redacted",
		ReasonCode:         DiagnosticRateLimited,
		EvidenceTimestamp:  now,
		RedactionReliable:  true,
		SafeEvidence:       map[string]string{"providerRequestId": "req_redacted"},
	})
	if err != nil {
		t.Fatalf("ClassifyDiagnostic returned error: %v", err)
	}
	if state.Status != LifecycleStateRateLimited {
		t.Fatalf("status=%s, want rate_limited", state.Status)
	}
	if state.RemediationOwner != RemediationOwnerProvider {
		t.Fatalf("remediation owner=%s, want provider", state.RemediationOwner)
	}
	if state.RetrySafety != RetrySafetyRetryAfter {
		t.Fatalf("retry safety=%s, want retry_after", state.RetrySafety)
	}
	if state.FreshnessState != FreshnessFresh {
		t.Fatalf("freshness=%s, want fresh", state.FreshnessState)
	}
	if got, want := state.RetentionExpiresAt, now.Add(90*24*time.Hour); !got.Equal(want) {
		t.Fatalf("retention=%s, want %s", got, want)
	}
}

func TestClassifyDiagnosticSuppressesEvidenceWhenRedactionFails(t *testing.T) {
	t.Parallel()

	state, err := ClassifyDiagnostic(DiagnosticInput{
		ConnectorID:       "connector_discord",
		ReasonCode:        DiagnosticUnknownConnectorFailure,
		RedactionReliable: false,
		SafeEvidence: map[string]string{
			"rawProviderPayload": "authorization bearer token",
		},
	})
	if err != nil {
		t.Fatalf("ClassifyDiagnostic returned error: %v", err)
	}
	if state.RedactionStatus != RedactionStatusSuppressed {
		t.Fatalf("redaction status=%s, want suppressed", state.RedactionStatus)
	}
	if len(state.SafeEvidence) != 0 {
		t.Fatalf("expected evidence to be suppressed, got %#v", state.SafeEvidence)
	}
	if state.RedactionFailureID == "" {
		t.Fatal("expected redaction failure id")
	}
}

func TestFreshnessAtMarksDiagnosticsStaleAfterFifteenMinutes(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 7, 10, 16, 0, 0, time.UTC)
	if got := FreshnessAt(now.Add(-15*time.Minute), now); got != FreshnessFresh {
		t.Fatalf("freshness at boundary=%s, want fresh", got)
	}
	if got := FreshnessAt(now.Add(-15*time.Minute-time.Nanosecond), now); got != FreshnessStale {
		t.Fatalf("freshness after boundary=%s, want stale", got)
	}
}

func TestSlackDiagnosticFreshnessAndRetentionContract(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
	state, err := ClassifyDiagnostic(DiagnosticInput{
		DiagnosticStateID:  "diag_slack_reply_failed",
		TenantID:           "ten_slack",
		ConnectorID:        "slack-main",
		ConnectorAccountID: "workspace_binding_redacted",
		ReasonCode:         DiagnosticReplyFailed,
		EvidenceTimestamp:  now,
		RedactionReliable:  true,
		SafeEvidence: map[string]string{
			"stage":       "message_loop",
			"workspaceId": "workspace_redacted",
		},
	})
	if err != nil {
		t.Fatalf("ClassifyDiagnostic returned error: %v", err)
	}
	if state.FreshnessState != FreshnessFresh {
		t.Fatalf("freshness=%s, want fresh", state.FreshnessState)
	}
	if got, want := state.RetentionExpiresAt, now.Add(90*24*time.Hour); !got.Equal(want) {
		t.Fatalf("retention=%s, want %s", got, want)
	}
	if got := FreshnessAt(state.EvidenceTimestamp, now.Add(2*time.Minute)); got != FreshnessFresh {
		t.Fatalf("Slack diagnostics must remain fresh for 2-minute support inspection bound, got %s", got)
	}
	if got := FreshnessAt(state.EvidenceTimestamp, now.Add(16*time.Minute)); got != FreshnessStale {
		t.Fatalf("Slack diagnostic should be stale after freshness window, got %s", got)
	}
}
