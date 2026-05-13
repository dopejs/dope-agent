package events

import (
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/profiles"
)

func TestAgentProfileEventsUseSafeMetadata(t *testing.T) {
	lifecycle := AgentProfileLifecycleEvent(AgentProfileLifecycleInput{TenantID: "ten_1", ProfileID: "prof_1", EventName: "agent_profile.created", Outcome: "succeeded", ReasonCode: "user_created_profile", SafeSummary: "safe"})
	if lifecycle.Category != "agent_profile" || lifecycle.Payload["safeSummary"] != "safe" || lifecycle.Payload["redactionStatus"] != string(profiles.RedactionRedacted) {
		t.Fatalf("unexpected lifecycle event: %+v", lifecycle)
	}
	projection := AgentProfileRuntimeProjectedEvent(profiles.RuntimeProjection{TenantID: "ten_1", RuntimeProfileProjectionID: "rpp_1", ProfileID: "prof_1", ProfileVersionID: "profv_1", SelectionID: "sel_1", ResourceKind: profiles.RuntimeResourceRun, ResourceID: "run_1", SafeDisplayName: "Agent", SafeSummary: "safe", RedactionStatus: profiles.RedactionRedacted})
	if projection.Name != "agent_profile.runtime_projected" || projection.Resource.Kind != "run" {
		t.Fatalf("unexpected runtime projection event: %+v", projection)
	}
}

func TestAgentProfileLifecycleEventNamesCoverFailureRetirementAndRollbackOutcomes(t *testing.T) {
	cases := []struct {
		name       string
		eventName  string
		outcome    string
		reasonCode string
	}{
		{"validation failure", "agent_profile.validation_failed", "denied", "profile_validation_failed"},
		{"permission denial", "agent_profile.permission_denied", "denied", "permission_denied"},
		{"archive", "agent_profile.archived", "succeeded", "operator_retired_profile"},
		{"disable", "agent_profile.disabled", "succeeded", "operator_retired_profile"},
		{"retirement denial", "agent_profile.retirement_denied", "denied", "profile_not_found"},
		{"safe fallback", "agent_profile.safe_default_fallback", "succeeded", "current_default_retired"},
		{"rollback requested", "agent_profile.rollback_requested", "requested", "operator_reverted_persona"},
		{"rollback succeeded", "agent_profile.rolled_back", "succeeded", "operator_reverted_persona"},
		{"rollback denied", "agent_profile.rollback_denied", "denied", "profile_not_activatable"},
		{"audit failed closed", "agent_profile.audit_failed_closed", "failed_closed", "audit_write_failed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			event := AgentProfileLifecycleEvent(AgentProfileLifecycleInput{
				TenantID:         "ten_1",
				ProfileID:        "prof_1",
				ProfileVersionID: "profv_1",
				EventName:        tc.eventName,
				Outcome:          tc.outcome,
				ReasonCode:       tc.reasonCode,
				PermissionGate:   "profiles.manage",
				SafeSummary:      "safe metadata",
			})
			if event.Name != tc.eventName || event.Payload["outcome"] != tc.outcome || event.Payload["reasonCode"] != tc.reasonCode {
				t.Fatalf("unexpected lifecycle event: %+v", event)
			}
			if event.Payload["safeSummary"] != "safe metadata" || event.Payload["redactionStatus"] != string(profiles.RedactionRedacted) {
				t.Fatalf("event did not retain safe redacted metadata: %+v", event)
			}
		})
	}
}
