package profiles

import "testing"

func TestProfileDomainConstantsCoverRuntimeAndRollbackStates(t *testing.T) {
	if StatusActive != "active" || ChangeRolledBack != "rolled_back" || RollbackInvalidOverlay != "invalid_overlay" || OverlayPermissionDenied != "permission_denied" || RuntimeResourceHandoffDestination != "handoff_destination" {
		t.Fatalf("profile domain constants drifted")
	}
}

func TestBuildRuntimeProjectionUsesSafeProfileMetadata(t *testing.T) {
	projection := BuildRuntimeProjection(
		AgentProfile{TenantID: "ten_1", ProfileID: "prof_1", DisplayName: "Ops Agent", Persona: Persona{SafeSummary: "operator profile"}},
		ActiveSelection{SelectionID: "sel_1", ProfileVersionID: "profv_1", SelectionScope: SelectionScopeTenantDefault, SelectionReason: SelectionUserActivated},
		RuntimeProjectionInput{ResourceKind: RuntimeResourceRun, ResourceID: "run_1", RunID: "run_1"},
	)
	if projection.SafeDisplayName != "Ops Agent" || projection.SafeSummary != "operator profile" || projection.RedactionStatus != RedactionRedacted {
		t.Fatalf("unexpected runtime projection: %+v", projection)
	}
}
