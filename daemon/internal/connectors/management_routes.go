package connectors

import (
	"strings"
	"time"
)

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

func RoutePolicyIsValid(policy RoutePolicy) bool {
	return strings.TrimSpace(policy.ValidationState) == "valid"
}

func RoutePolicyAllowsConversation(policy RoutePolicy, conversationID string) bool {
	if !RoutePolicyIsValid(policy) {
		return false
	}
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return false
	}
	if len(policy.EligibleConversations) == 0 && len(policy.EligibleRooms) == 0 && len(policy.EligibleChannels) == 0 {
		return false
	}
	return containsRoutePolicyValue(policy.EligibleConversations, conversationID) ||
		containsRoutePolicyValue(policy.EligibleRooms, conversationID) ||
		containsRoutePolicyValue(policy.EligibleChannels, conversationID)
}

func RoutePolicyAllowsSender(policy RoutePolicy, senderID string) bool {
	if !RoutePolicyIsValid(policy) {
		return false
	}
	if len(policy.EligibleSenders) == 0 {
		return true
	}
	return containsRoutePolicyValue(policy.EligibleSenders, strings.TrimSpace(senderID))
}

func containsRoutePolicyValue(values []string, target string) bool {
	target = strings.TrimSpace(target)
	if target == "" {
		return false
	}
	for _, value := range values {
		if strings.TrimSpace(value) == target {
			return true
		}
	}
	return false
}
