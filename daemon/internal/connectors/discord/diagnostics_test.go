package discord

import (
	"errors"
	"testing"
	"time"

	baseconnectors "github.com/dopejs/dope-agent/daemon/internal/connectors"
)

func TestDiagnosticReasonForErrorMapsDiscordFailureFamilies(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
		want baseconnectors.DiagnosticReasonCode
	}{
		{name: "auth", err: errors.New("401 Unauthorized: invalid token"), want: baseconnectors.DiagnosticAuthMissing},
		{name: "permission", err: errors.New("403 Forbidden: missing send messages permission"), want: baseconnectors.DiagnosticPermissionMissing},
		{name: "rate_limit", err: errors.New("429 rate limit exceeded"), want: baseconnectors.DiagnosticRateLimited},
		{name: "network", err: errors.New("gateway connection reset"), want: baseconnectors.DiagnosticNetworkFailed},
		{name: "provider", err: errors.New("Discord provider unavailable 5xx"), want: baseconnectors.DiagnosticProviderUnavailable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := DiagnosticReasonForError(tc.err); got != tc.want {
				t.Fatalf("DiagnosticReasonForError=%s, want %s", got, tc.want)
			}
		})
	}
}

func TestBuildDiagnosticStateUsesFreshnessRetentionAndRedactedEvidence(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 7, 10, 0, 0, 0, time.UTC)
	state, err := BuildDiagnosticState("ten_discord", "discord-main", "acct_redacted", baseconnectors.DiagnosticPermissionMissing, map[string]string{"permission": "send_messages"}, now)
	if err != nil {
		t.Fatalf("BuildDiagnosticState returned error: %v", err)
	}
	if state.FreshnessState != baseconnectors.FreshnessFresh {
		t.Fatalf("freshness=%s, want fresh", state.FreshnessState)
	}
	if state.RetentionExpiresAt.Sub(now) != 90*24*time.Hour {
		t.Fatalf("retention=%s, want 90 days", state.RetentionExpiresAt)
	}
	if state.SafeEvidence["permission"] != "send_messages" || state.RedactionStatus != baseconnectors.RedactionStatusRedacted {
		t.Fatalf("unexpected evidence/redaction: %+v", state)
	}
}
