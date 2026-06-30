package calendar

import "testing"

// FR-005: when the provider cannot honor the requested attendee notification behavior, the
// recorded outcome is explicitly unsupported (not silently applied or dropped).
func TestBuildAttendeeOutcomeUnsupported(t *testing.T) {
	details := []Attendee{
		{Email: "a@example.com", InvitationStatus: InvitationStatusUnsupported},
	}
	outcome := buildAttendeeOutcome(true, details)
	if outcome == nil {
		t.Fatal("expected an attendee outcome")
	}
	if !outcome.Unsupported || outcome.UnsupportedReason == "" {
		t.Fatalf("outcome should be explicitly unsupported: %+v", outcome)
	}
	if outcome.NotificationBehavior != NotificationBehaviorUnsupported {
		t.Fatalf("behavior = %q, want unsupported", outcome.NotificationBehavior)
	}
}

// FR-002: a silent (notify=false) attendee write records notification behavior as silent with
// no invitation sent.
func TestBuildAttendeeOutcomeSilent(t *testing.T) {
	details := []Attendee{{Email: "a@example.com", InvitationStatus: InvitationStatusNotRequested}}
	outcome := buildAttendeeOutcome(false, details)
	if outcome == nil || outcome.NotificationRequested {
		t.Fatalf("silent outcome wrong: %+v", outcome)
	}
	if outcome.NotificationBehavior != NotificationBehaviorSilent {
		t.Fatalf("behavior = %q, want silent", outcome.NotificationBehavior)
	}
}
