package opsreadiness

func ValidateHostedFailureOwner(owner string) error {
	return requireAllowed("failure owner", owner, []string{
		FailureOwnerDaemon,
		FailureOwnerHost,
		FailureOwnerNetwork,
		FailureOwnerProvider,
		FailureOwnerCredential,
		FailureOwnerQuota,
		FailureOwnerOperatorAction,
		FailureOwnerUnsupportedObservation,
		FailureOwnerUnknown,
	})
}
