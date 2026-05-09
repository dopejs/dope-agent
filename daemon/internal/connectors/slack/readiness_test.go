package slack

import (
	"testing"
	"time"

	baseconnectors "github.com/dopejs/dope-agent/daemon/internal/connectors"
)

func TestEvaluateHostedSetupReadyWithValidWorkspaceAndRoutePolicy(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	setup := EvaluateHostedSetup(HostedSetupInput{
		TenantID:          "ten_slack",
		ConnectorID:       "slack-main",
		DisplayName:       "Slack Main",
		OAuthState:        OAuthGrantValid,
		ProviderAvailable: true,
		NetworkAvailable:  true,
		ValidatedAt:       now,
		WorkspaceBinding: WorkspaceBinding{
			WorkspaceID:        "workspace_redacted",
			InstallationID:     "installation_redacted",
			OAuthGrantState:    "valid",
			RequiredScopeState: "valid",
		},
		RoutePolicy: RoutePolicy{
			ValidationState: RoutePolicyValid,
			SelectedChannels: []ConversationRoute{{
				ConversationID:       "channel_redacted",
				ConversationType:     ConversationChannel,
				SelectedChannelState: SelectedChannelSelected,
				ValidationState:      RoutePolicyValid,
			}},
		},
	})
	if setup.TerminalState != TerminalReady || setup.Status != baseconnectors.LifecycleStateHealthy || !setup.DeliveryEligible {
		t.Fatalf("expected ready hosted setup, got %+v", setup)
	}
	if setup.WorkspaceBinding.WorkspaceBindingID != "slack_workspace_slack-main" || setup.RoutePolicyState != RoutePolicyStateValid {
		t.Fatalf("expected normalized workspace binding and route policy, got %+v", setup)
	}
}

func TestEvaluateHostedSetupMapsOAuthTerminalStates(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	cases := []struct {
		name       string
		input      HostedSetupInput
		wantState  TerminalState
		wantOAuth  OAuthState
		wantReason string
	}{
		{name: "missing grant", input: HostedSetupInput{OAuthState: OAuthGrantMissing}, wantState: TerminalActionRequired, wantOAuth: OAuthGrantMissing, wantReason: string(baseconnectors.DiagnosticAuthMissing)},
		{name: "revoked", input: HostedSetupInput{OAuthState: OAuthRevoked}, wantState: TerminalActionRequired, wantOAuth: OAuthRevoked, wantReason: string(baseconnectors.DiagnosticAuthMissing)},
		{name: "scope missing", input: HostedSetupInput{OAuthState: OAuthScopeMissing}, wantState: TerminalActionRequired, wantOAuth: OAuthScopeMissing, wantReason: string(baseconnectors.DiagnosticPermissionMissing)},
		{name: "approval required", input: HostedSetupInput{OAuthState: OAuthApprovalRequired}, wantState: TerminalActionRequired, wantOAuth: OAuthApprovalRequired, wantReason: string(baseconnectors.DiagnosticPermissionMissing)},
		{name: "cancelled", input: HostedSetupInput{OAuthState: OAuthStarted, Cancelled: true}, wantState: TerminalCancelled, wantOAuth: OAuthStarted, wantReason: "user_cancelled"},
		{name: "provider unavailable", input: HostedSetupInput{OAuthState: OAuthGrantValid, ProviderAvailable: false, NetworkAvailable: true}, wantState: TerminalUnavailable, wantOAuth: OAuthGrantValid, wantReason: string(baseconnectors.DiagnosticProviderUnavailable)},
		{name: "network failed", input: HostedSetupInput{OAuthState: OAuthGrantValid, ProviderAvailable: true, NetworkAvailable: false}, wantState: TerminalUnavailable, wantOAuth: OAuthGrantValid, wantReason: string(baseconnectors.DiagnosticNetworkFailed)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := tc.input
			input.TenantID = "ten_slack"
			input.ConnectorID = "slack-main"
			input.DisplayName = "Slack Main"
			input.ValidatedAt = now
			setup := EvaluateHostedSetup(input)
			if setup.TerminalState != tc.wantState || setup.OAuthState != tc.wantOAuth || setup.ReasonCode != tc.wantReason || setup.DeliveryEligible {
				t.Fatalf("setup=%+v, want state=%s oauth=%s reason=%s and ineligible", setup, tc.wantState, tc.wantOAuth, tc.wantReason)
			}
		})
	}
}

func TestEvaluateHostedSetupRequiresValidatedRoutePolicy(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	base := HostedSetupInput{
		TenantID:          "ten_slack",
		ConnectorID:       "slack-main",
		DisplayName:       "Slack Main",
		OAuthState:        OAuthGrantValid,
		ProviderAvailable: true,
		NetworkAvailable:  true,
		ValidatedAt:       now,
		WorkspaceBinding: WorkspaceBinding{
			WorkspaceID:        "workspace_redacted",
			InstallationID:     "installation_redacted",
			OAuthGrantState:    "valid",
			RequiredScopeState: "valid",
		},
	}
	setup := EvaluateHostedSetup(base)
	if setup.TerminalState != TerminalActionRequired || setup.RoutePolicyState != RoutePolicyStateNone || setup.DeliveryEligible {
		t.Fatalf("valid OAuth without selected route must stay action-required, got %+v", setup)
	}
	base.RoutePolicy = RoutePolicy{
		ValidationState: RoutePolicyBlocked,
		AllowedDMUsers:  []string{"user_hash_1"},
	}
	setup = EvaluateHostedSetup(base)
	if setup.TerminalState != TerminalActionRequired || setup.DeliveryEligible {
		t.Fatalf("blocked route policy with DM allowment must not be ready, got %+v", setup)
	}
}

func TestEvaluateHostedSetupWorkspaceBindingCardinalityAndTimeout(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 8, 10, 6, 0, 0, time.UTC)
	setup := EvaluateHostedSetup(HostedSetupInput{
		TenantID:            "ten_slack",
		ConnectorID:         "slack-main",
		DisplayName:         "Slack Main",
		OAuthState:          OAuthGrantValid,
		ProviderAvailable:   true,
		NetworkAvailable:    true,
		ExpectedWorkspaceID: "workspace_expected",
		StartedAt:           now.Add(-6 * time.Minute),
		ValidatedAt:         now,
		WorkspaceBinding: WorkspaceBinding{
			WorkspaceID:        "workspace_other",
			InstallationID:     "installation_redacted",
			OAuthGrantState:    "valid",
			RequiredScopeState: "valid",
		},
		RoutePolicy: RoutePolicy{
			ValidationState: RoutePolicyValid,
			AllowedDMUsers:  []string{"user_hash_1"},
		},
	})
	if setup.TerminalState != TerminalUnavailable || setup.ReasonCode != "setup_timeout" {
		t.Fatalf("expected timeout to fail with actionable terminal state before readiness, got %+v", setup)
	}

	first := EvaluateHostedSetup(HostedSetupInput{TenantID: "ten_slack", ConnectorID: "slack-east", ValidatedAt: now})
	second := EvaluateHostedSetup(HostedSetupInput{TenantID: "ten_slack", ConnectorID: "slack-west", ValidatedAt: now})
	if first.WorkspaceBindingID == second.WorkspaceBindingID {
		t.Fatalf("multiple connectors for one tenant need distinct workspace bindings: %s", first.WorkspaceBindingID)
	}

	mismatch := EvaluateHostedSetup(HostedSetupInput{
		TenantID:            "ten_slack",
		ConnectorID:         "slack-main",
		OAuthState:          OAuthGrantValid,
		ProviderAvailable:   true,
		NetworkAvailable:    true,
		ExpectedWorkspaceID: "workspace_expected",
		ValidatedAt:         now,
		WorkspaceBinding: WorkspaceBinding{
			WorkspaceID:        "workspace_other",
			InstallationID:     "installation_redacted",
			OAuthGrantState:    "valid",
			RequiredScopeState: "valid",
		},
		RoutePolicy: RoutePolicy{
			ValidationState: RoutePolicyValid,
			AllowedDMUsers:  []string{"user_hash_1"},
		},
	})
	if mismatch.TerminalState != TerminalActionRequired || mismatch.ReasonCode != "workspace_mismatch" || mismatch.DeliveryEligible {
		t.Fatalf("expected workspace mismatch to fail closed, got %+v", mismatch)
	}
}
