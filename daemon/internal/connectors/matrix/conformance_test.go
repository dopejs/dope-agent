package matrix

import (
	"testing"
	"time"

	baseconnectors "github.com/dopejs/dope-agent/daemon/internal/connectors"
)

func TestConformanceProfileDeclaresMatrixSurfaces(t *testing.T) {
	t.Parallel()

	profile := ConformanceProfile(Config{
		ConnectorID:          "matrix-main",
		SelectedRoomIDs:      []string{"!room:example.org"},
		AllowedDirectUserIDs: []string{"@user:example.org"},
	}, time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC))

	if profile.ConnectorKind != ConnectorKind {
		t.Fatalf("ConnectorKind = %q, want %q", profile.ConnectorKind, ConnectorKind)
	}
	if profile.EquivalentDurableIdentityRuleID != "matrix_homeserver_conversation_event_id" {
		t.Fatalf("unexpected identity rule id: %q", profile.EquivalentDurableIdentityRuleID)
	}
	for _, area := range baseconnectors.CoreInvariantAreas() {
		if got := profile.CoreInvariantResults[area]; got != baseconnectors.ConformanceResultPass {
			t.Fatalf("core invariant %s = %s, want pass", area, got)
		}
	}
	for surface, want := range map[string]baseconnectors.SurfaceSupport{
		"tenant_provided_bot_setup":    baseconnectors.SurfaceSupported,
		"dopeagent_hosted_homeserver":  baseconnectors.SurfaceUnsupported,
		"matrix_account_provisioning":  baseconnectors.SurfaceUnsupported,
		"direct_message":               baseconnectors.SurfaceSupported,
		"allowed_room_mention":         baseconnectors.SurfaceSupported,
		"allowed_room_command":         baseconnectors.SurfaceSupported,
		"unencrypted_text":             baseconnectors.SurfaceSupported,
		"encrypted_rooms":              baseconnectors.SurfaceUnsupported,
		"undecryptable_events":         baseconnectors.SurfaceUnsupported,
		"e2ee_key_session_management":  baseconnectors.SurfaceUnsupported,
		"final_only_foreground_reply":  baseconnectors.SurfaceSupported,
		"connector_backed_delivery":    baseconnectors.SurfaceSupported,
		"whatsapp":                     baseconnectors.SurfaceUnsupported,
		"bridge_automation":            baseconnectors.SurfaceUnsupported,
		"media":                        baseconnectors.SurfaceUnsupported,
		"voice":                        baseconnectors.SurfaceUnsupported,
		"calls":                        baseconnectors.SurfaceUnsupported,
		"thinking_visibility":          baseconnectors.SurfaceUnsupported,
		"incremental_visible_updates":  baseconnectors.SurfaceUnsupported,
		"blocked_route_classification": baseconnectors.SurfaceSupported,
	} {
		if got := profile.ProviderSurfaceResults[surface]; got != want {
			t.Fatalf("surface %s = %s, want %s", surface, got, want)
		}
	}
}
