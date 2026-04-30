package opsreadiness

import "testing"

func TestHostedFailureOwnerAllowsOnlyKnownClassifications(t *testing.T) {
	for _, owner := range []string{FailureOwnerDaemon, FailureOwnerHost, FailureOwnerNetwork, FailureOwnerProvider, FailureOwnerCredential, FailureOwnerQuota, FailureOwnerOperatorAction, FailureOwnerUnsupportedObservation, FailureOwnerUnknown} {
		assertValid(t, ValidateHostedFailureOwner(owner))
	}
	assertInvalidContains(t, ValidateHostedFailureOwner("storage"), "failure owner")
}
