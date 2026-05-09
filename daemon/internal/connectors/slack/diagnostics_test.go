package slack

import (
	"errors"
	"strings"
	"testing"
	"time"

	baseconnectors "github.com/dopejs/dope-agent/daemon/internal/connectors"
)

func TestDiagnosticReasonForErrorMapsSlackFailures(t *testing.T) {
	t.Parallel()

	cases := map[string]baseconnectors.DiagnosticReasonCode{
		"invalid_auth oauth grant revoked":           baseconnectors.DiagnosticAuthMissing,
		"missing_scope not_in_channel permission":    baseconnectors.DiagnosticPermissionMissing,
		"slack web api returned 429 rate limited":    baseconnectors.DiagnosticRateLimited,
		"slack provider unavailable 5xx":             baseconnectors.DiagnosticProviderUnavailable,
		"event delivery timeout network failed":      baseconnectors.DiagnosticNetworkFailed,
		"unsupported slack huddle interactive block": baseconnectors.DiagnosticUnsupportedCapability,
	}
	for message, want := range cases {
		if got := DiagnosticReasonForError(errors.New(message)); got != want {
			t.Fatalf("%q mapped to %s, want %s", message, got, want)
		}
	}
}

func TestBuildDiagnosticStateRedactsSlackUnsafeEvidence(t *testing.T) {
	t.Parallel()

	state, err := BuildDiagnosticState("ten_slack", "slack-main", "workspace_binding_redacted", baseconnectors.DiagnosticAuthMissing, map[string]string{
		"botToken": "xoxb-secret",
		"hint":     "workspace redacted",
	}, time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("BuildDiagnosticState returned error: %v", err)
	}
	if state.RedactionStatus != baseconnectors.RedactionStatusSuppressed || state.RedactionFailureID == "" {
		t.Fatalf("expected suppressed redaction failure, got status=%s failure=%s", state.RedactionStatus, state.RedactionFailureID)
	}
	for _, value := range state.SafeEvidence {
		if strings.Contains(value, "xoxb-secret") {
			t.Fatalf("unsafe evidence leaked: %+v", state.SafeEvidence)
		}
	}
}
