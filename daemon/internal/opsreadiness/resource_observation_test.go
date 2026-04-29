package opsreadiness

import (
	"testing"
	"time"
)

func TestResourceObservationsCoverRequiredSignalsAndRejectGrowth(t *testing.T) {
	report := sampleSoakReport()
	assertValid(t, ValidateResourceObservations(report.ResourceObservations))

	report.ResourceObservations[0].MonotonicGrowth = true
	assertInvalidContains(t, ValidateResourceObservations(report.ResourceObservations), "monotonically")

	report = sampleSoakReport()
	report.ResourceObservations[2].QueueBacklogAge = 31 * time.Minute
	assertInvalidContains(t, ValidateResourceObservations(report.ResourceObservations), "queue backlog")
}
