package opsreadiness

import "fmt"

func ValidateHostedDeploymentManifest(run HostedRun, manifest HostedDeploymentManifest) error {
	errs := []error{
		RequireNonEmpty("manifest id", manifest.ManifestID),
		RequireNonEmpty("commit or version", manifest.CommitOrVersion),
		requireHostedRunIdentity(run, manifest.RunID, manifest.ProfileID, manifest.CommitOrVersion),
		RequireNonEmpty("branch", manifest.Branch),
		RequireNonEmpty("host", manifest.Host),
		RequireNonEmpty("operator", manifest.Operator),
		RequireNonEmpty("configuration profile", manifest.ConfigurationProfile),
		RequireNonEmpty("data directory", manifest.DataDirectory),
		RequireNonEmpty("artifact directory", manifest.ArtifactDirectory),
		requireAllowed("supervisor mode", manifest.SupervisorMode, []string{HostedSupervisorModeRepoForeground}),
		RequireNonEmpty("daemon address", manifest.DaemonAddress),
		requireAllowed("live connector mode", manifest.LiveConnectorMode, []string{HostedLiveConnectorsDisabled}),
		requireAllowed("redaction status", manifest.RedactionStatus, []string{HostedRedactionPassed}),
		ValidateHostedRetention("manifest", manifest.RetentionExpiresAt, ""),
		ValidateHostedRedaction("manifest", manifest),
	}
	if manifest.StartedAt.IsZero() {
		errs = append(errs, fmt.Errorf("manifest started at is required"))
	}
	return JoinErrors(errs...)
}
