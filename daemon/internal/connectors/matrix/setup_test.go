package matrix

import (
	"testing"
	"time"

	baseconnectors "github.com/dopejs/dope-agent/daemon/internal/connectors"
)

func TestEvaluateHostedSetupReadyRequiresBotHomeserverBindingRouteAndConformance(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	setup := EvaluateHostedSetup(HostedSetupInput{
		TenantID:           "ten_matrix",
		ConnectorID:        "matrix-main",
		DisplayName:        "Matrix Main",
		BotCredentialState: BotCredentialValid,
		HomeserverBinding: HomeserverBinding{
			HomeserverBindingID: "matrix_hs_1",
			HomeserverURL:       "https://matrix.example.org",
			BotUserID:           "@bot:example.org",
			AuthorizationState:  AuthorizationValid,
			CapabilityState:     HomeserverCapabilityValid,
		},
		RoutePolicy: RoutePolicy{
			SelectedRooms: []ConversationRoute{{
				ConversationID:     "!room:example.org",
				ConversationType:   ConversationRoom,
				RoomSelectionState: RoomSelected,
				ValidationState:    RoutePolicyValid,
			}},
			ValidationState: RoutePolicyValid,
		},
		ProviderAvailable: true,
		NetworkAvailable:  true,
		ConformancePassed: true,
		StartedAt:         now.Add(-4 * time.Minute),
		ValidatedAt:       now,
	})

	if setup.TerminalState != TerminalReady || setup.Status != baseconnectors.LifecycleStateHealthy || !setup.DeliveryEligible {
		t.Fatalf("expected ready healthy setup, got %+v", setup)
	}
}

func TestEvaluateHostedSetupReturnsActionableBoundedTerminalState(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	setup := EvaluateHostedSetup(HostedSetupInput{
		TenantID:           "ten_matrix",
		ConnectorID:        "matrix-main",
		DisplayName:        "Matrix Main",
		BotCredentialState: BotCredentialInvalid,
		ProviderAvailable:  true,
		NetworkAvailable:   true,
		StartedAt:          now.Add(-6 * time.Minute),
		SetupTimeout:       5 * time.Minute,
		ValidatedAt:        now,
	})

	if setup.TerminalState != TerminalActionRequired {
		t.Fatalf("TerminalState = %s, want action-required", setup.TerminalState)
	}
	if setup.ReasonCode != string(baseconnectors.DiagnosticAuthMissing) {
		t.Fatalf("ReasonCode = %q, want auth_missing", setup.ReasonCode)
	}
	if setup.SetupCompletedWithin != 5*time.Minute {
		t.Fatalf("SetupCompletedWithin = %s, want 5m", setup.SetupCompletedWithin)
	}
}

func TestEvaluateHostedSetupRejectsHostedHomeserverProvisioning(t *testing.T) {
	t.Parallel()

	setup := EvaluateHostedSetup(HostedSetupInput{
		TenantID:                  "ten_matrix",
		ConnectorID:               "matrix-main",
		DisplayName:               "Matrix Main",
		RequestedHostedHomeserver: true,
		RequestedAccountProvision: true,
		ProviderAvailable:         true,
		NetworkAvailable:          true,
		ValidatedAt:               time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC),
	})
	if setup.TerminalState != TerminalActionRequired || setup.ReasonCode != string(baseconnectors.DiagnosticUnsupportedCapability) {
		t.Fatalf("expected unsupported setup terminal state, got %+v", setup)
	}
}
