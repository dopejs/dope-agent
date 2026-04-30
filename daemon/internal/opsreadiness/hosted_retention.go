package opsreadiness

import (
	"fmt"
	"strings"
	"time"
)

func ValidateHostedRetention(label string, expiresAt time.Time, authorizedPolicy string) error {
	if expiresAt.IsZero() {
		return fmt.Errorf("%s retention expiry is required", label)
	}
	if !expiresAt.After(time.Now().UTC()) && strings.TrimSpace(authorizedPolicy) == "" {
		return fmt.Errorf("%s evidence expired for normal inspection", label)
	}
	return nil
}
