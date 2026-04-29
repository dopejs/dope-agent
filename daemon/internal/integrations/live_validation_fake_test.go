package integrations

import (
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/livevalidation"
)

func TestFakeBackendLiveValidationOutcomes(t *testing.T) {
	backend := FakeBackend{}
	cases := map[livevalidation.FakeOutcome]livevalidation.LedgerOutcome{
		livevalidation.FakeOutcomeCompleted:          livevalidation.LedgerOutcomeCompleted,
		livevalidation.FakeOutcomeFailed:             livevalidation.LedgerOutcomeFailed,
		livevalidation.FakeOutcomeTimeoutAfterSubmit: livevalidation.LedgerOutcomeOperatorActionNeeded,
		livevalidation.FakeOutcomeDuplicateRetry:     livevalidation.LedgerOutcomeCompleted,
	}
	for outcome, want := range cases {
		got := backend.RunLiveValidationOutcome(outcome)
		if got.Outcome != want {
			t.Fatalf("%s outcome=%s, want %s", outcome, got.Outcome, want)
		}
	}
}
