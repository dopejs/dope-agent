package calendar

import (
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/livevalidation"
)

func TestCalendarFakeBackendLiveValidationAmbiguousCommit(t *testing.T) {
	result := NewFakeBackend().RunLiveValidationOutcome(livevalidation.FakeOutcomeSubmitUnknown)
	if result.Outcome != livevalidation.LedgerOutcomeOperatorActionNeeded || !result.AmbiguousCommit || result.AutomaticRetryAllowed {
		t.Fatalf("result=%+v, want ambiguous operator-action-needed without retry", result)
	}
}
