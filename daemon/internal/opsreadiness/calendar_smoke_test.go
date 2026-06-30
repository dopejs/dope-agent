package opsreadiness

import "testing"

// FR-011 / FR-012 / SC-006: the calendar real-account smoke runs against safe credentials when
// available and otherwise records an explicit structured skip that still validates.
func TestCalendarRealAccountSmoke(t *testing.T) {
	// Default CI path: no safe credentials -> explicit structured skip, readiness can still pass.
	skip := CalendarRealAccountSmoke(CalendarSmokeInput{FakeBackendCoveragePassing: true})
	if skip.Result != StatusSkip || skip.SkipReason == "" {
		t.Fatalf("unavailable credentials must skip with a reason: %+v", skip)
	}
	if err := ValidateRealAccountSmoke([]RealAccountSmokeStatus{skip}); err != nil {
		t.Fatalf("reasoned skip must validate: %v", err)
	}

	// Operator path: safe credentials, enabled, write rows exercised -> pass.
	pass := CalendarRealAccountSmoke(CalendarSmokeInput{
		SafeCredentialsAvailable:    true,
		Enabled:                     true,
		CreateUpdateCancelExercised: true,
		FakeBackendCoveragePassing:  true,
	})
	if pass.Result != StatusPass {
		t.Fatalf("exercised smoke must pass: %+v", pass)
	}
	if err := ValidateRealAccountSmoke([]RealAccountSmokeStatus{pass}); err != nil {
		t.Fatalf("passing smoke must validate: %v", err)
	}

	// Safety: raw credential exposure is a hard failure regardless of result.
	leaky := CalendarRealAccountSmoke(CalendarSmokeInput{
		SafeCredentialsAvailable:      true,
		Enabled:                       true,
		CreateUpdateCancelExercised:   true,
		FakeBackendCoveragePassing:    true,
		ContainsRawCredentialMaterial: true,
	})
	if err := ValidateRealAccountSmoke([]RealAccountSmokeStatus{leaky}); err == nil {
		t.Fatal("raw credential exposure must fail validation")
	}
}
