package opsreadiness

import "testing"

func TestHostedResourceGrowthAndBacklogAreBlockingFindings(t *testing.T) {
	report := sampleHostedObservationReport()
	report.MonotonicResourceGrowth = true
	report.FailureOwner = FailureOwnerDaemon
	report.BlockingFindings = []string{"monotonic resource growth"}
	assertInvalidContains(t, ValidateHostedObservationReport(sampleHostedRun(), report), "resource growth")

	report = sampleHostedObservationReport()
	report.QueueOrBacklog = HostedObservation{Value: "backlog_persisted"}
	report.FailureOwner = FailureOwnerDaemon
	report.BlockingFindings = []string{"queue backlog persisted"}
	assertInvalidContains(t, ValidateHostedObservationReport(sampleHostedRun(), report), "backlog")
}
