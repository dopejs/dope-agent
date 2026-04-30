package opsreadiness

import (
	"fmt"
	"strings"
	"time"
)

func ValidateHostedProfile(profile HostedOperationalProfile) error {
	errs := []error{
		RequireNonEmpty("profile id", profile.ProfileID),
		RequireNonEmpty("profile name", profile.ProfileName),
		requireAllowed("environment", profile.Environment, []string{EnvironmentTest}),
		requireAllowed("host class", profile.HostClass, []string{HostedHostClassStableTestHost, HostedHostClassVPS, HostedHostClassDeveloperLaptop, HostedHostClassUnsupported}),
		RequireNonEmpty("data directory", profile.DataDirectory),
		RequireNonEmpty("log directory", profile.LogDirectory),
		RequireNonEmpty("artifact directory", profile.ArtifactDirectory),
		RequireNonEmpty("backup directory", profile.BackupDirectory),
		RequireNonEmpty("report directory", profile.ReportDirectory),
		RequireNonEmpty("temporary directory", profile.TemporaryDirectory),
		requireAllowed("live connector mode", profile.LiveConnectorMode, []string{HostedLiveConnectorsDisabled, HostedLiveConnectorsLive}),
	}
	if strings.TrimSpace(profile.DataDirectory) == "~/.dope" {
		errs = append(errs, fmt.Errorf("hosted profile refuses production data directory without an explicit production recovery opt-in"))
	}
	if profile.LiveConnectorMode == HostedLiveConnectorsLive {
		errs = append(errs, fmt.Errorf("live connector mode requires explicit operator opt-in outside default hosted validation"))
	}
	if profile.RetentionDays < 90 {
		errs = append(errs, fmt.Errorf("retention days must be at least 90"))
	}
	return JoinErrors(errs...)
}

func ValidateHostedStableHost(profile HostedOperationalProfile) error {
	switch profile.HostClass {
	case HostedHostClassStableTestHost, HostedHostClassVPS:
		return nil
	case HostedHostClassDeveloperLaptop:
		return fmt.Errorf("developer laptop cannot satisfy hosted release-readiness stable-host evidence")
	default:
		return fmt.Errorf("unsupported host class %q cannot satisfy hosted release-readiness evidence", profile.HostClass)
	}
}

func ValidateHostedRun(profile HostedOperationalProfile, run HostedRun, now time.Time) error {
	errs := []error{
		RequireNonEmpty("run id", run.RunID),
		RequireNonEmpty("run profile id", run.ProfileID),
		RequireNonEmpty("commit or version", run.CommitOrVersion),
		RequireNonEmpty("host", run.Host),
		RequireNonEmpty("operator", run.Operator),
		RequireNonEmpty("artifact root", run.ArtifactRoot),
		requireAllowed("supervisor mode", run.SupervisorMode, []string{HostedSupervisorModeRepoForeground}),
		requireAllowed("run status", run.Status, []string{HostedRunStatusProvisioning, HostedRunStatusRunning, HostedRunStatusStopped, HostedRunStatusFailed, HostedRunStatusCompleted, HostedRunStatusExpired}),
		ValidateHostedRetention("run", run.RetentionExpiresAt, ""),
	}
	if profile.ProfileID != "" && run.ProfileID != profile.ProfileID {
		errs = append(errs, fmt.Errorf("run profile identity %q does not match hosted profile %q", run.ProfileID, profile.ProfileID))
	}
	if run.StartedAt.IsZero() {
		errs = append(errs, fmt.Errorf("run started at is required"))
	}
	if !now.IsZero() && !run.RetentionExpiresAt.IsZero() && !run.RetentionExpiresAt.After(now) {
		errs = append(errs, fmt.Errorf("run evidence expired at %s", run.RetentionExpiresAt.Format(time.RFC3339)))
	}
	return JoinErrors(errs...)
}

func ValidateHostedProvisioningElapsed(elapsed time.Duration) error {
	return RequireElapsedAtMost("hosted profile provisioning", elapsed, MaxInstallElapsed)
}

func requireHostedRunIdentity(run HostedRun, runID, profileID, commitOrVersion string) error {
	errs := []error{}
	if runID != run.RunID {
		errs = append(errs, fmt.Errorf("evidence identity run %q does not match %q", runID, run.RunID))
	}
	if profileID != "" && profileID != run.ProfileID {
		errs = append(errs, fmt.Errorf("evidence identity profile %q does not match %q", profileID, run.ProfileID))
	}
	if commitOrVersion != "" && commitOrVersion != run.CommitOrVersion {
		errs = append(errs, fmt.Errorf("evidence identity commit %q does not match %q", commitOrVersion, run.CommitOrVersion))
	}
	return JoinErrors(errs...)
}

func requireGeneratedAt(label string, value time.Time) error {
	if value.IsZero() {
		return fmt.Errorf("%s generated timestamp is required", label)
	}
	return nil
}

func requireStatusPass(label, value string) error {
	if value != StatusPass && value != HostedResultPassed {
		return fmt.Errorf("%s must pass, got %q", label, value)
	}
	return nil
}
