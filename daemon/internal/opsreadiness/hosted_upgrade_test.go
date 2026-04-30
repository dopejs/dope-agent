package opsreadiness

import "testing"

func sampleHostedUpgradePreflight() HostedUpgradeEvidence {
	run := sampleHostedRun()
	return HostedUpgradeEvidence{
		UpgradeEvidenceID:      "upgrade_preflight_1",
		RunID:                  run.RunID,
		Phase:                  HostedUpgradePhasePreflight,
		DeploymentIdentity:     "manifest_hosted_1",
		ProfileIdentity:        run.ProfileID,
		DataLocation:           "~/.dope-test",
		ArtifactLocation:       run.ArtifactRoot,
		RequiredBackupState:    StatusPass,
		DaemonHealth:           StatusPass,
		ConfigurationReadiness: StatusPass,
		BlockingFindings:       nil,
		GeneratedAt:            hostedNow,
	}
}

func TestHostedUpgradePreflightRequiresIdentityBackupHealthAndReadiness(t *testing.T) {
	evidence := sampleHostedUpgradePreflight()
	assertValid(t, ValidateHostedUpgradeEvidence(sampleHostedRun(), evidence))

	evidence.RequiredBackupState = StatusFail
	evidence.BlockingFindings = []string{"backup missing"}
	assertInvalidContains(t, ValidateHostedUpgradeEvidence(sampleHostedRun(), evidence), "blocking")
}
