package opsreadiness

import (
	"testing"
	"time"
)

func TestHostedRetentionExpiryAndAuthorizedLongerRetention(t *testing.T) {
	now := time.Now().UTC()
	assertInvalidContains(t, ValidateHostedRetention("manifest", now.Add(-time.Hour), ""), "expired")
	assertValid(t, ValidateHostedRetention("manifest", now.Add(-time.Hour), "legal_hold_2026"))
	assertValid(t, ValidateHostedRetention("manifest", now.AddDate(0, 0, 90), ""))
}
