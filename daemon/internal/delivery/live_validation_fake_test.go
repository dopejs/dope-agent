package delivery

import (
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/livevalidation"
)

func TestDeliveryTestSinkLiveValidationOutcomes(t *testing.T) {
	sink := NewTestSinkAdapter()
	if result := sink.RunLiveValidationOutcome(livevalidation.FakeOutcomeCompleted); result.Outcome != livevalidation.LedgerOutcomeCompleted {
		t.Fatalf("completed result=%+v", result)
	}
	if result := sink.RunLiveValidationOutcome(livevalidation.FakeOutcomeSubmitUnknown); result.Outcome != livevalidation.LedgerOutcomeOperatorActionNeeded || !result.AmbiguousCommit {
		t.Fatalf("submit unknown result=%+v, want ambiguous", result)
	}
}
