package opsreadiness

import "testing"

func TestRealAccountSmokeSkipsRequireReasonAndPassingFakeBackend(t *testing.T) {
	statuses := []RealAccountSmokeStatus{
		{Domain: "calendar", SafeCredentialsAvailable: false, SkipReason: "no safe account", FakeBackendCoveragePassing: true},
		{Domain: "mail", SafeCredentialsAvailable: true, Enabled: true, Result: StatusPass, FakeBackendCoveragePassing: true},
	}
	assertValid(t, ValidateRealAccountSmoke(statuses))

	statuses[0].SkipReason = ""
	assertInvalidContains(t, ValidateRealAccountSmoke(statuses), "skip reason")
}

func TestRealAccountSmokeWithSafeCredentialsRequiresPassingResult(t *testing.T) {
	statuses := []RealAccountSmokeStatus{
		{Domain: "calendar", SafeCredentialsAvailable: true, Enabled: true, Result: StatusFail, FakeBackendCoveragePassing: true},
	}
	assertInvalidContains(t, ValidateRealAccountSmoke(statuses), "must pass")
}
