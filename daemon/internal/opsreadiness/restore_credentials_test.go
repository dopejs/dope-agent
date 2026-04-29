package opsreadiness

import "testing"

func TestCredentialRemediationBlocksCredentialBearingUse(t *testing.T) {
	assertValid(t, ValidateCredentialRemediation([]string{"reconnect_required", "blocked_until_reconnected"}))
	assertInvalidContains(t, ValidateCredentialRemediation([]string{"authorized"}), "does not block")
}
