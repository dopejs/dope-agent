package contracts_test

import (
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/opsreadiness"
)

func TestHostedUpgradeEvidenceBlocksReleaseWhenMissingFailedOrMismatched(t *testing.T) {
	run := hostedContractRun()
	preflight := opsreadiness.HostedUpgradeEvidence{
		UpgradeEvidenceID:      "preflight_contract_1",
		RunID:                  run.RunID,
		Phase:                  opsreadiness.HostedUpgradePhasePreflight,
		DeploymentIdentity:     "manifest_contract_1",
		ProfileIdentity:        run.ProfileID,
		DataLocation:           "/tmp/dope-test",
		ArtifactLocation:       run.ArtifactRoot,
		RequiredBackupState:    opsreadiness.StatusPass,
		DaemonHealth:           opsreadiness.StatusPass,
		ConfigurationReadiness: opsreadiness.StatusPass,
		GeneratedAt:            hostedContractNow,
	}
	if err := opsreadiness.ValidateHostedUpgradeEvidence(run, preflight); err != nil {
		t.Fatalf("passing upgrade preflight invalid: %v", err)
	}
	preflight.RunID = "old_run"
	if err := opsreadiness.ValidateHostedUpgradeEvidence(run, preflight); err == nil {
		t.Fatalf("expected mismatched upgrade evidence to fail")
	}
}
