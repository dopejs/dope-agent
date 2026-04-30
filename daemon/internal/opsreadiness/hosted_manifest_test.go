package opsreadiness

import "testing"

func sampleHostedManifest() HostedDeploymentManifest {
	run := sampleHostedRun()
	return HostedDeploymentManifest{
		ManifestID:           "manifest_hosted_1",
		RunID:                run.RunID,
		ProfileID:            run.ProfileID,
		CommitOrVersion:      run.CommitOrVersion,
		Branch:               "028-hosted-operational-profile",
		Host:                 run.Host,
		Operator:             run.Operator,
		StartedAt:            run.StartedAt,
		ConfigurationProfile: "test",
		DataDirectory:        "~/.dope-test",
		ArtifactDirectory:    run.ArtifactRoot,
		SupervisorMode:       HostedSupervisorModeRepoForeground,
		DaemonAddress:        "127.0.0.1:19192",
		LiveConnectorMode:    HostedLiveConnectorsDisabled,
		RedactionStatus:      HostedRedactionPassed,
		RetentionExpiresAt:   run.RetentionExpiresAt,
	}
}

func TestHostedManifestRequiresFieldsAndRedaction(t *testing.T) {
	manifest := sampleHostedManifest()
	assertValid(t, ValidateHostedDeploymentManifest(sampleHostedRun(), manifest))

	manifest.CommitOrVersion = ""
	assertInvalidContains(t, ValidateHostedDeploymentManifest(sampleHostedRun(), manifest), "commit")

	manifest = sampleHostedManifest()
	manifest.ConfigurationProfile = "Authorization: Bearer access_token"
	assertInvalidContains(t, ValidateHostedDeploymentManifest(sampleHostedRun(), manifest), "credential")
}
