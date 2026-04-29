package opsreadiness

import (
	"testing"
	"time"
)

func TestSoakReportAcceptsPassingBaselineAndRejectsHardFailures(t *testing.T) {
	report := sampleSoakReport()
	assertValid(t, ValidateSoakReport(report))

	report.Duration = time.Hour
	assertInvalidContains(t, ValidateSoakReport(report), "shorter")

	report.TemporaryShorterDuration = true
	report.TemporaryDurationReason = "developer rehearsal"
	report.FollowUpFullRerun = true
	assertValid(t, ValidateSoakReport(report))

	report = sampleSoakReport()
	report.UnclassifiedFailures = []string{"unknown delivery failure"}
	assertInvalidContains(t, ValidateSoakReport(report), "unclassified")
}
