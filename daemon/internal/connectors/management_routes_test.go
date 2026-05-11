package connectors

import (
	"testing"
	"time"
)

func TestDefaultAndNormalizedRoutePolicyAreRedactedAndFutureEligible(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	policy := DefaultRoutePolicy("ten_channels", "slack-main", now)
	if !policy.BackgroundDeliveryEligible || policy.ValidationState != "valid" || policy.RedactionStatus != RedactionStatusRedacted {
		t.Fatalf("unexpected default route policy: %+v", policy)
	}

	normalized := NormalizeRoutePolicy(RoutePolicy{TenantID: "ten_channels", ConnectorID: "slack-main"}, now)
	if normalized.ValidatedAt.IsZero() || normalized.ValidationState != "valid" || normalized.RedactionStatus != RedactionStatusRedacted {
		t.Fatalf("unexpected normalized route policy: %+v", normalized)
	}
}
