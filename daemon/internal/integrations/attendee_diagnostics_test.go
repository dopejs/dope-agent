package integrations

import "testing"

// FR-005 (spec 046): unsupported provider RSVP/notification features classify to the explicit
// unsupported diagnostic reason, never a silent drop.
func TestUnsupportedAttendeeFeatureClassifiesExplicitly(t *testing.T) {
	for _, code := range []string{"rsvp_unsupported", "attendee_notification_unsupported"} {
		evidence := ProviderDiagnosticEvidence{
			ProviderKind:       string(BackendKindFeishuLark),
			DomainKind:         "calendar",
			OperationClass:     "update_attendees",
			ProviderErrorClass: code,
			SideEffecting:      true,
		}
		classification := ClassifyProviderEvidence(evidence)
		if classification.ReasonCode != ReasonUnsupportedDiagnostic {
			t.Fatalf("code %q classified as %q, want %q", code, classification.ReasonCode, ReasonUnsupportedDiagnostic)
		}
	}
}
