package connectors

import (
	"errors"
	"testing"
	"time"
)

func TestRunMatrixCaseRecordsCoreInvariantPassAndFailureEvidence(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 7, 10, 0, 0, 0, time.UTC)
	input := MatrixCase{
		ScenarioID:    "fake.core.failure",
		TenantID:      "ten_033",
		ConnectorID:   "connector_fake",
		ConnectorKind: "fake",
		Now:           now,
		CoreInvariantResults: map[ConformanceArea]ConformanceResultStatus{
			ConformanceAreaTenantOwnership:            ConformanceResultPass,
			ConformanceAreaPermissionGating:           ConformanceResultPass,
			ConformanceAreaRedaction:                  ConformanceResultFail,
			ConformanceAreaActiveTenantAccountBinding: ConformanceResultPass,
			ConformanceAreaInboundIdentity:            ConformanceResultPass,
			ConformanceAreaDurableDedupe:              ConformanceResultPass,
			ConformanceAreaStableRouting:              ConformanceResultPass,
			ConformanceAreaMinimumForegroundReply:     ConformanceResultPass,
			ConformanceAreaDiagnostics:                ConformanceResultPass,
			ConformanceAreaDeliverySeparation:         ConformanceResultPass,
		},
	}

	results, profile, err := RunMatrixCase(input)
	if err != nil {
		t.Fatalf("RunMatrixCase returned error: %v", err)
	}
	if len(results) != len(CoreInvariantAreas()) {
		t.Fatalf("result count=%d, want %d", len(results), len(CoreInvariantAreas()))
	}
	if err := ValidateCapabilityProfile(profile); !errors.Is(err, ErrCoreInvariantFailed) {
		t.Fatalf("ValidateCapabilityProfile error=%v, want ErrCoreInvariantFailed", err)
	}

	var redactionFailure ConformanceResult
	for _, result := range results {
		if result.Area == string(ConformanceAreaRedaction) {
			redactionFailure = result
			break
		}
	}
	if redactionFailure.Result != ConformanceResultFail {
		t.Fatalf("redaction result=%s, want fail", redactionFailure.Result)
	}
	if redactionFailure.ReasonCode != "core_invariant_failed" {
		t.Fatalf("redaction failure reason=%q, want core_invariant_failed", redactionFailure.ReasonCode)
	}
	if got, want := redactionFailure.RetentionExpiresAt, now.Add(90*24*time.Hour); !got.Equal(want) {
		t.Fatalf("retention=%s, want %s", got, want)
	}
}

func TestRunMatrixCaseRequiresEquivalentDurableIdentityRuleDetails(t *testing.T) {
	t.Parallel()

	_, _, err := RunMatrixCase(MatrixCase{
		ScenarioID:                      "fake.equivalent_identity.missing_rule",
		ConnectorID:                     "connector_fake",
		ConnectorKind:                   "fake",
		CoreInvariantResults:            passingCoreInvariantResults(),
		EquivalentDurableIdentityRuleID: "provider_alias",
	})
	if !errors.Is(err, ErrEquivalentIdentityRequired) {
		t.Fatalf("RunMatrixCase error=%v, want ErrEquivalentIdentityRequired", err)
	}
}

func TestRunMatrixCaseDegradesUnsafeIncrementalUpdates(t *testing.T) {
	t.Parallel()

	_, profile, err := RunMatrixCase(MatrixCase{
		ScenarioID:                      "fake.incremental.degraded",
		ConnectorID:                     "connector_fake",
		ConnectorKind:                   "fake",
		CoreInvariantResults:            passingCoreInvariantResults(),
		UnsafeIncrementalUpdateDegraded: true,
	})
	if err != nil {
		t.Fatalf("RunMatrixCase returned error: %v", err)
	}
	if got := profile.ProviderSurfaceResults["incremental_visible_updates"]; got != SurfaceLimited {
		t.Fatalf("incremental_visible_updates=%s, want limited", got)
	}
}

func passingCoreInvariantResults() map[ConformanceArea]ConformanceResultStatus {
	results := map[ConformanceArea]ConformanceResultStatus{}
	for _, area := range CoreInvariantAreas() {
		results[area] = ConformanceResultPass
	}
	return results
}
