package connectors

import (
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/livevalidation"
)

func TestConnectorMessageSendLiveValidationOutcomes(t *testing.T) {
	supervisor := NewSupervisor()
	result := supervisor.RunLiveValidationOutcome(livevalidation.FakeOutcomeSubmitUnknown)
	if result.Outcome != livevalidation.LedgerOutcomeOperatorActionNeeded || !result.AmbiguousCommit || result.AutomaticRetryAllowed {
		t.Fatalf("result=%+v, want ambiguous no retry", result)
	}
}
