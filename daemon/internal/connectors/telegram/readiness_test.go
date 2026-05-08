package telegram

import (
	"testing"
	"time"

	baseconnectors "github.com/dopejs/dope-agent/daemon/internal/connectors"
)

func TestEvaluateHostedSetupRequiresCredentialBindingAndAllowmentBeforeReady(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	setup := EvaluateHostedSetup(HostedSetupInput{
		TenantID:    "ten_telegram",
		ConnectorID: "telegram-main",
		DisplayName: "Telegram Main",
		Credential:  CredentialValid,
		AccountBinding: AccountBinding{
			ConnectorAccountID:   "bot_redacted",
			ProviderAccountLabel: "agent_bot",
			PermissionState:      PermissionValid,
		},
		Allowments: []AllowmentValidation{{
			AllowmentID:     "allow_dm",
			ScopeType:       ScopeDirectChat,
			ScopeID:         "chat_redacted",
			Enabled:         true,
			ValidationState: AllowmentValid,
		}},
		ValidatedAt: now,
	})

	if setup.TerminalState != TerminalReady || !setup.HostedReady {
		t.Fatalf("expected ready hosted setup, got state=%s hostedReady=%v reason=%s", setup.TerminalState, setup.HostedReady, setup.ReasonCode)
	}
	if setup.Status != baseconnectors.LifecycleStateHealthy || setup.RetentionExpiresAt.Sub(now) != 90*24*time.Hour {
		t.Fatalf("unexpected status/retention: status=%s retention=%s", setup.Status, setup.RetentionExpiresAt)
	}
}

func TestEvaluateHostedSetupValidCredentialWithoutAllowmentIsActionRequired(t *testing.T) {
	t.Parallel()

	setup := EvaluateHostedSetup(HostedSetupInput{
		TenantID:    "ten_telegram",
		ConnectorID: "telegram-main",
		DisplayName: "Telegram Main",
		Credential:  CredentialValid,
		AccountBinding: AccountBinding{
			ConnectorAccountID:   "bot_redacted",
			ProviderAccountLabel: "agent_bot",
			PermissionState:      PermissionValid,
		},
		ValidatedAt: time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC),
	})

	if setup.TerminalState != TerminalActionRequired || setup.HostedReady {
		t.Fatalf("expected action-required without explicit allowment, got state=%s hostedReady=%v", setup.TerminalState, setup.HostedReady)
	}
	if setup.ReasonCode != "telegram_allowment_missing" {
		t.Fatalf("expected allowment remediation reason, got %q", setup.ReasonCode)
	}
}

func TestEvaluateHostedSetupTerminalStates(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		input      HostedSetupInput
		wantState  TerminalState
		wantReason string
	}{
		{
			name:       "missing credential",
			input:      HostedSetupInput{Credential: CredentialMissing},
			wantState:  TerminalActionRequired,
			wantReason: string(baseconnectors.DiagnosticAuthMissing),
		},
		{
			name:       "revoked credential",
			input:      HostedSetupInput{Credential: CredentialRevoked},
			wantState:  TerminalActionRequired,
			wantReason: string(baseconnectors.DiagnosticAuthMissing),
		},
		{
			name:       "provider unavailable",
			input:      HostedSetupInput{Credential: CredentialValid, AccountBinding: AccountBinding{PermissionState: PermissionProviderUnavailable}},
			wantState:  TerminalUnavailable,
			wantReason: string(baseconnectors.DiagnosticProviderUnavailable),
		},
		{
			name:       "cancelled",
			input:      HostedSetupInput{Cancelled: true, Credential: CredentialValid},
			wantState:  TerminalCancelled,
			wantReason: "user_cancelled",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.input.TenantID = "ten_telegram"
			tt.input.ConnectorID = "telegram-main"
			tt.input.DisplayName = "Telegram Main"
			tt.input.ValidatedAt = time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
			got := EvaluateHostedSetup(tt.input)
			if got.TerminalState != tt.wantState || got.ReasonCode != tt.wantReason || got.HostedReady {
				t.Fatalf("state=%s reason=%s hostedReady=%v, want state=%s reason=%s not ready", got.TerminalState, got.ReasonCode, got.HostedReady, tt.wantState, tt.wantReason)
			}
		})
	}
}

func TestEvaluateHostedSetupTimeoutProducesActionableDiagnostic(t *testing.T) {
	t.Parallel()

	started := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	setup := EvaluateHostedSetup(HostedSetupInput{
		TenantID:    "ten_telegram",
		ConnectorID: "telegram-main",
		DisplayName: "Telegram Main",
		Credential:  CredentialSubmitted,
		StartedAt:   started,
		ValidatedAt: started.Add(5*time.Minute + time.Second),
	})

	if setup.TerminalState != TerminalActionRequired || setup.ReasonCode != "telegram_setup_timeout" {
		t.Fatalf("expected timeout action-required diagnostic, got state=%s reason=%s", setup.TerminalState, setup.ReasonCode)
	}
}
