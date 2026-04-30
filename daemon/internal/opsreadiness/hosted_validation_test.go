package opsreadiness

import (
	"testing"
	"time"
)

var hostedNow = time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)

func sampleHostedProfile() HostedOperationalProfile {
	return HostedOperationalProfile{
		ProfileID:          "profile_hosted_test",
		ProfileName:        "Hosted test profile",
		Environment:        EnvironmentTest,
		HostClass:          HostedHostClassStableTestHost,
		DataDirectory:      "~/.dope-test",
		LogDirectory:       "~/.dope-test/logs",
		ArtifactDirectory:  "~/.dope-test/artifacts",
		BackupDirectory:    "~/.dope-test/backups",
		ReportDirectory:    "~/.dope-test/reports",
		TemporaryDirectory: "~/.dope-test/tmp",
		LiveConnectorMode:  HostedLiveConnectorsDisabled,
		RetentionDays:      90,
	}
}

func sampleHostedRun() HostedRun {
	return HostedRun{
		RunID:              "hosted_run_20260430",
		ProfileID:          "profile_hosted_test",
		CommitOrVersion:    "028-hosted-operational-profile",
		Host:               "stable-test-host-1",
		Operator:           "operator@example.test",
		StartedAt:          hostedNow.Add(-10 * time.Minute),
		SupervisorMode:     HostedSupervisorModeRepoForeground,
		Status:             HostedRunStatusRunning,
		ArtifactRoot:       "~/.dope-test/artifacts/hosted_run_20260430",
		RetentionExpiresAt: hostedNow.AddDate(0, 0, 90),
	}
}

func TestHostedValidationRequiresIdentityRetentionAndSafeDefaults(t *testing.T) {
	profile := sampleHostedProfile()
	run := sampleHostedRun()
	assertValid(t, ValidateHostedProfile(profile))
	assertValid(t, ValidateHostedRun(profile, run, hostedNow))

	run.ProfileID = "other_profile"
	assertInvalidContains(t, ValidateHostedRun(profile, run, hostedNow), "profile")

	run = sampleHostedRun()
	run.RetentionExpiresAt = hostedNow.Add(-time.Minute)
	assertInvalidContains(t, ValidateHostedRun(profile, run, hostedNow), "expired")

	profile = sampleHostedProfile()
	profile.DataDirectory = "~/.dope"
	assertInvalidContains(t, ValidateHostedProfile(profile), "production")
}

func TestHostedStableHostRejectsDeveloperLaptopForReleaseEvidence(t *testing.T) {
	profile := sampleHostedProfile()
	profile.HostClass = HostedHostClassDeveloperLaptop
	assertInvalidContains(t, ValidateHostedStableHost(profile), "developer laptop")
}
