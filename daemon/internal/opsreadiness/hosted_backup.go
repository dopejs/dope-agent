package opsreadiness

func ValidateHostedBackupEvidence(run HostedRun, backup HostedBackupEvidence) error {
	errs := []error{
		RequireNonEmpty("backup id", backup.BackupID),
		requireHostedRunIdentity(run, backup.RunID, backup.SourceProfileID, backup.SourceCommitOrVersion),
		RequireNonEmpty("artifact path", backup.ArtifactPath),
		RequireNonEmpty("checksum", backup.Checksum),
		RequireItems("included material", backup.IncludedMaterial),
		RequireItems("excluded material", backup.ExcludedMaterial),
		RequireItems("compatibility notes", backup.CompatibilityNotes),
		validateRepresentativeTenants(len(backup.TenantSummary), backup.TenantSummary),
		requireCredentialExclusions(backup.ExcludedMaterial),
		requireAllowed("redaction status", backup.RedactionStatus, []string{HostedRedactionPassed}),
		requireGeneratedAt("backup", backup.GeneratedAt),
		ValidateHostedRedaction("backup", backup),
	}
	for i, tenant := range backup.TenantSummary {
		errs = append(errs, validateTenantStateSummary("tenant summary", tenant))
		_ = i
	}
	return JoinErrors(errs...)
}
