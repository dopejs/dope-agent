package opsreadiness

import "fmt"

func ValidateRealAccountSmoke(statuses []RealAccountSmokeStatus) error {
	if len(statuses) == 0 {
		return fmt.Errorf("real-account smoke status is required for each supported domain")
	}
	for _, status := range statuses {
		if err := RequireNonEmpty("real-account smoke domain", status.Domain); err != nil {
			return err
		}
		if status.ContainsRawCredentialMaterial {
			return fmt.Errorf("real-account smoke for %s exposed raw credential material", status.Domain)
		}
		if !status.FakeBackendCoveragePassing {
			return fmt.Errorf("fake-backend coverage must pass for %s", status.Domain)
		}
		if status.SafeCredentialsAvailable && !status.Enabled {
			return fmt.Errorf("safe credentials available for %s but smoke is not enabled", status.Domain)
		}
		if status.SafeCredentialsAvailable && status.Enabled && status.Result != StatusPass {
			return fmt.Errorf("real-account smoke for %s must pass when safe credentials are used", status.Domain)
		}
		if !status.SafeCredentialsAvailable && status.SkipReason == "" {
			return fmt.Errorf("missing safe credentials for %s require explicit skip reason", status.Domain)
		}
	}
	return nil
}
