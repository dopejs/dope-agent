package reminders

import (
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/livevalidation"
)

func TestReminderLifecycleLiveValidationFakeOutcomes(t *testing.T) {
	manager := NewManager(Dependencies{})
	if result := manager.RunLiveValidationOutcome(livevalidation.FakeOutcomeDuplicateRetry); result.Outcome != livevalidation.LedgerOutcomeCompleted || !result.AutomaticRetryAllowed {
		t.Fatalf("duplicate retry result=%+v, want idempotent retry completion", result)
	}
	if result := manager.RunLiveValidationOutcome(livevalidation.FakeOutcomeSubmitUnknown); result.Outcome != livevalidation.LedgerOutcomeOperatorActionNeeded {
		t.Fatalf("submit unknown result=%+v", result)
	}
}
