package integrations

import (
	"errors"
	"strings"
)

var (
	ErrIntegrationIDRequired = errors.New("integration id is required")
	ErrDomainKindRequired    = errors.New("domain kind is required")
	ErrDisplayNameRequired   = errors.New("display name is required")
	ErrBackendKindRequired   = errors.New("backend kind is required")
	ErrIntegrationNotFound   = errors.New("integration not found")
	ErrProbeUnsupported      = errors.New("integration probe unsupported")
	ErrProbeBlocked          = errors.New("integration probe blocked")
)

type Backend interface {
	RunProbe(resource Resource, probeKind ProbeKind, input map[string]any) (ProbeResult, error)
	SupportedDomainKinds() []string
}

func BackendKindSupportsDomain(kind BackendKind, domainKind string) bool {
	trimmed := strings.TrimSpace(domainKind)
	switch kind {
	case BackendKindFakeLocal:
		return FakeBackend{}.supportsDomainKind(trimmed)
	default:
		return false
	}
}

func normalizeBackendBinding(binding BackendBinding) BackendBinding {
	binding.BackendRefID = strings.TrimSpace(binding.BackendRefID)
	binding.BackendDisplayName = strings.TrimSpace(binding.BackendDisplayName)
	binding.SourceKind = strings.TrimSpace(binding.SourceKind)
	if binding.SourceKind == "" {
		binding.SourceKind = string(binding.BackendKind)
	}
	return binding
}
