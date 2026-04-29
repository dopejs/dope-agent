package opsreadiness

func ValidateSoakWorkload(coverage WorkloadCoverage) error {
	return requireCoverage("soak workload", map[string]bool(coverage), RequiredWorkloadAreas)
}
