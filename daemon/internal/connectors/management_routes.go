package connectors

import "time"

func DefaultRoutePolicy(tenantID, connectorID string, now time.Time) RoutePolicy {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return RoutePolicy{
		TenantID:                   tenantID,
		ConnectorID:                connectorID,
		BackgroundDeliveryEligible: true,
		ValidationState:            "valid",
		ValidatedAt:                now,
		RedactionStatus:            RedactionStatusRedacted,
	}
}

func NormalizeRoutePolicy(policy RoutePolicy, now time.Time) RoutePolicy {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if policy.ValidatedAt.IsZero() {
		policy.ValidatedAt = now
	}
	if policy.ValidationState == "" {
		policy.ValidationState = "valid"
	}
	if policy.RedactionStatus == "" {
		policy.RedactionStatus = RedactionStatusRedacted
	}
	return policy
}
