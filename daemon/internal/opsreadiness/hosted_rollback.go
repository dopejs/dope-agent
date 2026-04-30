package opsreadiness

func ValidateHostedRollbackDecision(run HostedRun, decision HostedRollbackDecisionRecord) error {
	errs := []error{
		RequireNonEmpty("rollback decision id", decision.RollbackDecisionID),
		requireHostedRunIdentity(run, decision.RunID, "", ""),
		RequireNonEmpty("trigger", decision.Trigger),
		requireAllowed("rollback decision", decision.Decision, []string{HostedRollbackInPlace, HostedRollbackRestoreFromBackupRequired, HostedRollbackNoRollbackNeeded, HostedRollbackBlocked}),
		RequireNonEmpty("rationale", decision.Rationale),
		RequireItems("supporting evidence links", decision.SupportingEvidenceLinks),
		RequireNonEmpty("operator", decision.Operator),
		ValidateHostedRedaction("rollback decision", decision),
	}
	if decision.Decision == HostedRollbackRestoreFromBackupRequired {
		errs = append(errs, RequireNonEmpty("required backup id", decision.RequiredBackupID))
	}
	if decision.DecidedAt.IsZero() {
		errs = append(errs, requireGeneratedAt("rollback decision", decision.DecidedAt))
	}
	return JoinErrors(errs...)
}
