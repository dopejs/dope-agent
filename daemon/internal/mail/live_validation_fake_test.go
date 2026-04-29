package mail

import (
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/livevalidation"
)

func TestMailFakeBackendNonIdempotentSendReplyForwardOutcomes(t *testing.T) {
	backend := NewFakeBackend()
	for _, outcome := range []livevalidation.FakeOutcome{livevalidation.FakeOutcomeCompleted, livevalidation.FakeOutcomeFailed, livevalidation.FakeOutcomeTimeoutAfterSubmit} {
		result := backend.RunLiveValidationOutcome(outcome)
		if outcome == livevalidation.FakeOutcomeTimeoutAfterSubmit && (!result.AmbiguousCommit || result.AutomaticRetryAllowed) {
			t.Fatalf("timeout result=%+v, want ambiguous no retry", result)
		}
	}
}
