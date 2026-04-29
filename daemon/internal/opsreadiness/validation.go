package opsreadiness

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	MaxInstallElapsed         = 60 * time.Minute
	MaxUpgradeElapsed         = 90 * time.Minute
	MaxReleaseReviewElapsed   = 30 * time.Minute
	MaxRestartRecoveryElapsed = 5 * time.Minute
	MaxQueueBacklogAge        = 30 * time.Minute
	MinimumSoakDuration       = 24 * time.Hour
	MinimumRestartCount       = 3
	MinimumTenantCount        = 3
)

var RawCredentialMarkers = []string{
	"raw_secret",
	"access_token",
	"refresh_token",
	"oauth_code",
	"provider_token",
	"R37_FAKE_SECRET",
	"R37_FAKE_TOKEN",
	"R39_RAW_SECRET",
	"DO_NOT_LEAK",
}

func RequireNonEmpty(label, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s is required", label)
	}
	return nil
}

func RequireItems(label string, values []string) error {
	if len(values) == 0 {
		return fmt.Errorf("%s requires at least one item", label)
	}
	for i, value := range values {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s[%d] is empty", label, i)
		}
	}
	return nil
}

func RequireElapsedAtMost(label string, elapsed, max time.Duration) error {
	if elapsed <= 0 {
		return fmt.Errorf("%s elapsed time is required", label)
	}
	if elapsed > max {
		return fmt.Errorf("%s elapsed time %s exceeds %s", label, elapsed, max)
	}
	return nil
}

func ContainsRawCredentialMaterial(value string) bool {
	lower := strings.ToLower(value)
	for _, marker := range RawCredentialMarkers {
		if strings.Contains(lower, strings.ToLower(marker)) {
			return true
		}
	}
	return false
}

func ContainsAnyRawCredentialMaterial(values []string) bool {
	for _, value := range values {
		if ContainsRawCredentialMaterial(value) {
			return true
		}
	}
	return false
}

func JoinErrors(errs ...error) error {
	filtered := make([]error, 0, len(errs))
	for _, err := range errs {
		if err != nil {
			filtered = append(filtered, err)
		}
	}
	return errors.Join(filtered...)
}

func requireAllowed(label, value string, allowed []string) error {
	for _, item := range allowed {
		if value == item {
			return nil
		}
	}
	return fmt.Errorf("%s %q is not one of %v", label, value, allowed)
}

func requireCoverage(label string, coverage map[string]bool, required []string) error {
	var missing []string
	for _, key := range required {
		if !coverage[key] {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("%s missing required coverage: %s", label, strings.Join(missing, ", "))
	}
	return nil
}

func requireNoRawCredentialSlice(label string, values []string) error {
	if ContainsAnyRawCredentialMaterial(values) {
		return fmt.Errorf("%s contains raw credential material", label)
	}
	return nil
}
