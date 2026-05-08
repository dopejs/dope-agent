package telegram

import (
	"errors"
	"strings"
	"testing"
	"time"

	baseconnectors "github.com/dopejs/dope-agent/daemon/internal/connectors"
)

func TestDiagnosticReasonForErrorMapsTelegramFailures(t *testing.T) {
	t.Parallel()

	cases := map[string]baseconnectors.DiagnosticReasonCode{
		"401 unauthorized bot token":         baseconnectors.DiagnosticAuthMissing,
		"403 forbidden missing permission":   baseconnectors.DiagnosticPermissionMissing,
		"429 too many requests rate limit":   baseconnectors.DiagnosticRateLimited,
		"telegram provider unavailable 5xx":  baseconnectors.DiagnosticProviderUnavailable,
		"network connection reset by peer":   baseconnectors.DiagnosticNetworkFailed,
		"unsupported attachment voice input": baseconnectors.DiagnosticUnsupportedCapability,
	}
	for message, want := range cases {
		if got := DiagnosticReasonForError(errors.New(message)); got != want {
			t.Fatalf("%q mapped to %s, want %s", message, got, want)
		}
	}
}

func TestBuildDiagnosticStateRedactsUnsafeEvidence(t *testing.T) {
	t.Parallel()

	state, err := BuildDiagnosticState("ten_telegram", "telegram-main", "bot_redacted", baseconnectors.DiagnosticAuthMissing, map[string]string{
		"token": "123:SECRET",
		"hint":  "safe",
	}, time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("BuildDiagnosticState returned error: %v", err)
	}
	if state.RedactionStatus != baseconnectors.RedactionStatusSuppressed || state.RedactionFailureID == "" {
		t.Fatalf("expected suppressed redaction failure, got status=%s failure=%s", state.RedactionStatus, state.RedactionFailureID)
	}
	for _, value := range state.SafeEvidence {
		if strings.Contains(value, "SECRET") {
			t.Fatalf("unsafe evidence leaked: %+v", state.SafeEvidence)
		}
	}
}
