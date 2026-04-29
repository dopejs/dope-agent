package opsreadiness

import "fmt"

func ValidateSoakReport(report SoakReport) error {
	errs := []error{
		RequireNonEmpty("report id", report.ReportID),
		RequireNonEmpty("branch or version", report.BranchOrVersion),
		RequireNonEmpty("environment", report.Environment),
		RequireNonEmpty("data directory", report.DataDirectory),
		RequireNonEmpty("baseline topology", report.BaselineTopology),
		ValidateSoakWorkload(report.WorkloadCoverage),
		ValidateRestartRecovery(report.RestartEvents),
		ValidateFaultDrills(report.FaultDrillResults),
		ValidateResourceObservations(report.ResourceObservations),
		validateRepresentativeTenants(len(report.TenantSetSummary), report.TenantSetSummary),
	}
	if report.BaselineTopology != TopologyTenantScopedSingleNode {
		errs = append(errs, fmt.Errorf("baseline topology must be %s", TopologyTenantScopedSingleNode))
	}
	if report.Environment != EnvironmentTest {
		errs = append(errs, fmt.Errorf("default soak environment must be %s", EnvironmentTest))
	}
	if report.Duration < MinimumSoakDuration {
		if !report.TemporaryShorterDuration || report.TemporaryDurationReason == "" || !report.FollowUpFullRerun {
			errs = append(errs, fmt.Errorf("soak duration %s is shorter than %s without temporary threshold rationale and full rerun requirement", report.Duration, MinimumSoakDuration))
		}
	}
	if report.CrossTenantLeakage {
		errs = append(errs, fmt.Errorf("cross-tenant leakage observed"))
	}
	if len(report.UnclassifiedFailures) > 0 {
		errs = append(errs, fmt.Errorf("unclassified failures observed: %v", report.UnclassifiedFailures))
	}
	if report.FinalResult != StatusPass {
		errs = append(errs, fmt.Errorf("final result must be pass"))
	}
	return JoinErrors(errs...)
}

func ValidateFaultDrills(results []FaultDrillResult) error {
	coverage := map[string]bool{}
	for _, result := range results {
		coverage[result.FaultType] = true
		if err := requireAllowed("fault classification", result.ObservedClassification, []string{
			ClassificationRecovered,
			ClassificationRetryExhausted,
			ClassificationOperatorActionNeeded,
		}); err != nil {
			return err
		}
		if result.RetryExhausted && !result.OperatorActionNeeded {
			return fmt.Errorf("retry exhaustion for %s lacks operator-action-needed state", result.FaultType)
		}
		if result.ContainsRawCredentialMaterial {
			return fmt.Errorf("fault drill %s exposed raw credential material", result.FaultType)
		}
	}
	return requireCoverage("fault drills", coverage, RequiredFaultTypes)
}
