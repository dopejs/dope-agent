package opsreadiness

import "fmt"

func ValidateCredentialRemediation(states []string) error {
	if err := RequireItems("credential remediation states", states); err != nil {
		return err
	}
	for _, state := range states {
		if state != "reconnect_required" && state != "revalidation_required" && state != "blocked_until_reconnected" {
			return fmt.Errorf("credential remediation state %q does not block credential-bearing use", state)
		}
	}
	return nil
}
