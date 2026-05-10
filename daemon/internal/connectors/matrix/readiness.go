package matrix

import (
	"errors"
	"strings"
	"time"

	baseconnectors "github.com/dopejs/dope-agent/daemon/internal/connectors"
)

var ErrHomeserverBindingInvalid = errors.New("matrix homeserver binding is invalid")

func NormalizeHomeserverBinding(tenantID, connectorID string, binding HomeserverBinding) HomeserverBinding {
	binding.TenantID = coalesceString(binding.TenantID, tenantID)
	binding.ConnectorID = coalesceString(binding.ConnectorID, connectorID)
	if binding.HomeserverBindingID == "" && binding.ConnectorID != "" {
		binding.HomeserverBindingID = "matrix_homeserver_" + binding.ConnectorID
	}
	if binding.AuthorizationState == "" {
		binding.AuthorizationState = AuthorizationMissing
	}
	if binding.CapabilityState == "" {
		binding.CapabilityState = HomeserverCapabilityUnknown
	}
	if binding.ValidatedAt.IsZero() {
		binding.ValidatedAt = time.Now().UTC()
	}
	if binding.RedactionStatus == "" {
		binding.RedactionStatus = baseconnectors.RedactionStatusRedacted
	}
	return binding
}

func coalesceString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func ValidateHomeserverBinding(binding HomeserverBinding) error {
	if strings.TrimSpace(binding.TenantID) == "" ||
		strings.TrimSpace(binding.ConnectorID) == "" ||
		strings.TrimSpace(binding.HomeserverURL) == "" ||
		strings.TrimSpace(binding.BotUserID) == "" ||
		binding.AuthorizationState != AuthorizationValid ||
		binding.CapabilityState != HomeserverCapabilityValid {
		return ErrHomeserverBindingInvalid
	}
	return nil
}

func homeserverState(binding HomeserverBinding) HomeserverState {
	switch {
	case binding.CapabilityState == HomeserverCapabilityUnsupported:
		return HomeserverUnsupported
	case binding.CapabilityState == HomeserverCapabilityRateLimited:
		return HomeserverRateLimited
	case binding.AuthorizationState == AuthorizationProviderUnavailable:
		return HomeserverUnreachable
	case binding.AuthorizationState == AuthorizationNetworkFailed:
		return HomeserverNetworkFailed
	case binding.CapabilityState == HomeserverCapabilityValid:
		return HomeserverReachable
	default:
		return HomeserverUnknown
	}
}
