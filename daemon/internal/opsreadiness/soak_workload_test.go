package opsreadiness

import "testing"

func TestSoakWorkloadRequiresAllRoadmapAreas(t *testing.T) {
	report := sampleSoakReport()
	assertValid(t, ValidateSoakWorkload(report.WorkloadCoverage))

	report.WorkloadCoverage["evaluation"] = false
	assertInvalidContains(t, ValidateSoakWorkload(report.WorkloadCoverage), "evaluation")
}
