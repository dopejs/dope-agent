package opsreadiness

import "testing"

// FR-009 (spec 048): the mail real-account smoke runs against safe credentials when available
// and otherwise records an explicit structured skip that still validates.
func TestMailRealAccountSmoke(t *testing.T) {
	skip := MailRealAccountSmoke(MailSmokeInput{FakeBackendCoveragePassing: true})
	if skip.Result != StatusSkip || skip.SkipReason == "" {
		t.Fatalf("unavailable credentials must skip with a reason: %+v", skip)
	}
	if err := ValidateRealAccountSmoke([]RealAccountSmokeStatus{skip}); err != nil {
		t.Fatalf("reasoned skip must validate: %v", err)
	}

	pass := MailRealAccountSmoke(MailSmokeInput{
		SafeCredentialsAvailable:   true,
		Enabled:                    true,
		SendReplyForwardExercised:  true,
		FakeBackendCoveragePassing: true,
	})
	if pass.Result != StatusPass {
		t.Fatalf("exercised smoke must pass: %+v", pass)
	}
	if err := ValidateRealAccountSmoke([]RealAccountSmokeStatus{pass}); err != nil {
		t.Fatalf("passing smoke must validate: %v", err)
	}

	leaky := MailRealAccountSmoke(MailSmokeInput{
		SafeCredentialsAvailable:      true,
		Enabled:                       true,
		SendReplyForwardExercised:     true,
		FakeBackendCoveragePassing:    true,
		ContainsRawCredentialMaterial: true,
	})
	if err := ValidateRealAccountSmoke([]RealAccountSmokeStatus{leaky}); err == nil {
		t.Fatal("raw credential exposure must fail validation")
	}
}
