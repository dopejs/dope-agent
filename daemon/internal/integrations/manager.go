package integrations

import (
	"errors"
	"strings"
	"sync"
	"time"
)

type Manager struct {
	mu       sync.RWMutex
	byID     map[string]Resource
	order    []string
	backends map[BackendKind]Backend
	env      string
}

func NewManager(environmentScope string) *Manager {
	return &Manager{
		byID:  make(map[string]Resource),
		order: make([]string, 0),
		backends: map[BackendKind]Backend{
			BackendKindFakeLocal:  FakeBackend{},
			BackendKindFeishuLark: FeishuLarkDiagnosticBackend{},
		},
		env: strings.TrimSpace(environmentScope),
	}
}

func (m *Manager) Restore(items []Resource) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.byID = make(map[string]Resource, len(items))
	m.order = make([]string, 0, len(items))
	for _, item := range items {
		m.byID[item.IntegrationID] = item
		m.order = append(m.order, item.IntegrationID)
	}
}

func (m *Manager) List() []Resource {
	m.mu.RLock()
	defer m.mu.RUnlock()
	items := make([]Resource, 0, len(m.order))
	for _, id := range m.order {
		items = append(items, m.byID[id])
	}
	return items
}

func (m *Manager) ListForTenant(tenantID string) []Resource {
	m.mu.RLock()
	defer m.mu.RUnlock()
	tenantID = strings.TrimSpace(tenantID)
	items := make([]Resource, 0, len(m.order))
	for _, id := range m.order {
		item := m.byID[id]
		if tenantID == "" || strings.TrimSpace(item.TenantID) == tenantID {
			items = append(items, item)
		}
	}
	return items
}

func (m *Manager) Get(integrationID string) (Resource, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	item, ok := m.byID[strings.TrimSpace(integrationID)]
	return item, ok
}

func (m *Manager) GetForTenant(integrationID, tenantID string) (Resource, bool) {
	item, ok := m.Get(integrationID)
	if !ok {
		return Resource{}, false
	}
	if strings.TrimSpace(tenantID) != "" && strings.TrimSpace(item.TenantID) != strings.TrimSpace(tenantID) {
		return Resource{}, false
	}
	return item, true
}

func (m *Manager) Create(input CreateInput) (Resource, error) {
	if strings.TrimSpace(input.IntegrationID) == "" {
		return Resource{}, ErrIntegrationIDRequired
	}
	if strings.TrimSpace(input.DomainKind) == "" {
		return Resource{}, ErrDomainKindRequired
	}
	if strings.TrimSpace(input.DisplayName) == "" {
		return Resource{}, ErrDisplayNameRequired
	}
	if strings.TrimSpace(string(input.BackendBinding.BackendKind)) == "" {
		return Resource{}, ErrBackendKindRequired
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now().UTC()
	resource := Resource{
		TenantID:         strings.TrimSpace(input.TenantID),
		IntegrationID:    strings.TrimSpace(input.IntegrationID),
		DomainKind:       strings.TrimSpace(input.DomainKind),
		DisplayName:      strings.TrimSpace(input.DisplayName),
		EnvironmentScope: firstNonEmpty(strings.TrimSpace(input.EnvironmentScope), m.env),
		ReadinessStatus:  ReadinessStatusNotConfigured,
		AuthState:        AuthStateNotStarted,
		HealthState:      HealthStateUnknown,
		CanonicalDefault: input.CanonicalDefault,
		AccountBinding:   input.AccountBinding,
		BackendBinding:   normalizeBackendBinding(input.BackendBinding),
		Provenance: Provenance{
			EnvironmentScope: firstNonEmpty(strings.TrimSpace(input.EnvironmentScope), m.env),
			BackedBy:         string(input.BackendBinding.BackendKind),
		},
		CreatedAt:        now,
		UpdatedAt:        now,
		LastTransitionAt: now,
	}
	if _, exists := m.byID[resource.IntegrationID]; !exists {
		m.order = append(m.order, resource.IntegrationID)
	}
	m.byID[resource.IntegrationID] = resource
	if resource.CanonicalDefault {
		m.demoteSiblingDefaultsLocked(resource)
		m.byID[resource.IntegrationID] = resource
	}
	return resource, nil
}

func (m *Manager) UpdateReadiness(integrationID string, input UpdateReadinessInput) (Resource, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	resource, ok := m.byID[strings.TrimSpace(integrationID)]
	if !ok {
		return Resource{}, ErrIntegrationNotFound
	}
	now := time.Now().UTC()
	resource.ReadinessStatus = input.ReadinessStatus
	if input.AuthState != "" {
		resource.AuthState = input.AuthState
	}
	if input.HealthState != "" {
		resource.HealthState = input.HealthState
	}
	resource.ReadinessReason = strings.TrimSpace(input.ReadinessReason)
	resource.RequiredOperatorAction = strings.TrimSpace(input.RequiredOperatorAction)
	if strings.TrimSpace(input.AccountBinding.AccountKey) != "" || strings.TrimSpace(input.AccountBinding.AccountLabel) != "" || strings.TrimSpace(input.AccountBinding.ExternalAccountID) != "" {
		resource.AccountBinding = input.AccountBinding
	}
	if resource.ReadinessStatus == ReadinessStatusHealthy {
		resource.LastReadyAt = &now
	}
	resource.UpdatedAt = now
	resource.LastTransitionAt = now
	resource = updateProvenance(resource, input)
	m.byID[resource.IntegrationID] = resource
	return resource, nil
}

func (m *Manager) UpdateReadinessForTenant(integrationID, tenantID string, input UpdateReadinessInput) (Resource, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	resource, ok := m.byID[strings.TrimSpace(integrationID)]
	if !ok || strings.TrimSpace(resource.TenantID) != strings.TrimSpace(tenantID) {
		return Resource{}, ErrIntegrationNotFound
	}
	return m.updateReadinessLocked(resource, input), nil
}

func (m *Manager) SetCanonicalDefault(integrationID string) (Resource, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	resource, ok := m.byID[strings.TrimSpace(integrationID)]
	if !ok {
		return Resource{}, ErrIntegrationNotFound
	}
	resource.CanonicalDefault = true
	resource.UpdatedAt = time.Now().UTC()
	m.demoteSiblingDefaultsLocked(resource)
	m.byID[resource.IntegrationID] = resource
	return resource, nil
}

func (m *Manager) SetCanonicalDefaultForTenant(integrationID, tenantID string) (Resource, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	resource, ok := m.byID[strings.TrimSpace(integrationID)]
	if !ok || strings.TrimSpace(resource.TenantID) != strings.TrimSpace(tenantID) {
		return Resource{}, ErrIntegrationNotFound
	}
	resource.CanonicalDefault = true
	resource.UpdatedAt = time.Now().UTC()
	m.demoteSiblingDefaultsLocked(resource)
	m.byID[resource.IntegrationID] = resource
	return resource, nil
}

func (m *Manager) Disconnect(integrationID, reason string) (Resource, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	resource, ok := m.byID[strings.TrimSpace(integrationID)]
	if !ok {
		return Resource{}, ErrIntegrationNotFound
	}
	return m.disconnectLocked(resource, reason), nil
}

func (m *Manager) DisconnectForTenant(integrationID, tenantID, reason string) (Resource, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	resource, ok := m.byID[strings.TrimSpace(integrationID)]
	if !ok || strings.TrimSpace(resource.TenantID) != strings.TrimSpace(tenantID) {
		return Resource{}, ErrIntegrationNotFound
	}
	return m.disconnectLocked(resource, reason), nil
}

func (m *Manager) BindingSummary(integrationID string, capturedAt time.Time) (BindingSummary, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	resource, ok := m.byID[strings.TrimSpace(integrationID)]
	if !ok {
		return BindingSummary{}, ErrIntegrationNotFound
	}
	return resourceBindingSummary(resource, capturedAt), nil
}

func (m *Manager) updateReadinessLocked(resource Resource, input UpdateReadinessInput) Resource {
	now := time.Now().UTC()
	resource.ReadinessStatus = input.ReadinessStatus
	if input.AuthState != "" {
		resource.AuthState = input.AuthState
	}
	if input.HealthState != "" {
		resource.HealthState = input.HealthState
	}
	resource.ReadinessReason = strings.TrimSpace(input.ReadinessReason)
	resource.RequiredOperatorAction = strings.TrimSpace(input.RequiredOperatorAction)
	if strings.TrimSpace(input.AccountBinding.AccountKey) != "" || strings.TrimSpace(input.AccountBinding.AccountLabel) != "" || strings.TrimSpace(input.AccountBinding.ExternalAccountID) != "" {
		resource.AccountBinding = input.AccountBinding
	}
	if resource.ReadinessStatus == ReadinessStatusHealthy {
		resource.LastReadyAt = &now
	}
	resource.UpdatedAt = now
	resource.LastTransitionAt = now
	resource = updateProvenance(resource, input)
	m.byID[resource.IntegrationID] = resource
	return resource
}

func (m *Manager) disconnectLocked(resource Resource, reason string) Resource {
	now := time.Now().UTC()
	resource.ReadinessStatus = ReadinessStatusUnavailable
	resource.AuthState = AuthStateRevoked
	resource.HealthState = HealthStateUnavailable
	resource.DisabledReason = strings.TrimSpace(reason)
	resource.RequiredOperatorAction = "reconnect integration"
	resource.CanonicalDefault = false
	resource.UpdatedAt = now
	resource.LastTransitionAt = now
	m.byID[resource.IntegrationID] = resource
	return resource
}

func (m *Manager) RunProbe(integrationID string, probeKind ProbeKind, input map[string]any) (Resource, ProbeResult, BindingSummary, error) {
	m.mu.RLock()
	resource, ok := m.byID[strings.TrimSpace(integrationID)]
	if !ok {
		m.mu.RUnlock()
		return Resource{}, ProbeResult{}, BindingSummary{}, ErrIntegrationNotFound
	}
	backend := m.backends[resource.BackendBinding.BackendKind]
	m.mu.RUnlock()
	if backend == nil {
		return Resource{}, ProbeResult{}, BindingSummary{}, ErrProbeUnsupported
	}
	if resource.ReadinessStatus == ReadinessStatusUnavailable {
		return resource, ProbeResult{}, BindingSummary{}, ErrProbeBlocked
	}
	if probeKind == ProbeKindInspect && !resource.BackendBinding.SupportsProbeRead {
		return resource, ProbeResult{}, BindingSummary{}, ErrProbeUnsupported
	}
	if probeKind == ProbeKindMutate && !resource.BackendBinding.SupportsProbeMutation {
		return resource, ProbeResult{}, BindingSummary{}, ErrProbeUnsupported
	}
	result, err := backend.RunProbe(resource, probeKind, input)
	if err != nil {
		return resource, ProbeResult{}, BindingSummary{}, err
	}
	summary := resourceBindingSummary(resource, time.Now().UTC())
	return resource, result, summary, nil
}

func (m *Manager) demoteSiblingDefaultsLocked(selected Resource) {
	for id, item := range m.byID {
		if id == selected.IntegrationID {
			continue
		}
		if sameBindingGroup(item, selected) && item.CanonicalDefault {
			item.CanonicalDefault = false
			item.UpdatedAt = time.Now().UTC()
			m.byID[id] = item
		}
	}
}

func sameBindingGroup(left, right Resource) bool {
	return strings.TrimSpace(left.TenantID) == strings.TrimSpace(right.TenantID) &&
		strings.TrimSpace(left.DomainKind) == strings.TrimSpace(right.DomainKind) &&
		strings.TrimSpace(left.EnvironmentScope) == strings.TrimSpace(right.EnvironmentScope) &&
		strings.TrimSpace(left.AccountBinding.AccountKey) == strings.TrimSpace(right.AccountBinding.AccountKey)
}

func resourceBindingSummary(resource Resource, capturedAt time.Time) BindingSummary {
	return BindingSummary{
		TenantID:              resource.TenantID,
		IntegrationID:         resource.IntegrationID,
		DomainKind:            resource.DomainKind,
		DisplayName:           resource.DisplayName,
		AccountKey:            resource.AccountBinding.AccountKey,
		CanonicalDefault:      resource.CanonicalDefault,
		ReadinessAtInvocation: resource.ReadinessStatus,
		BackendKind:           resource.BackendBinding.BackendKind,
		SecretResolution:      resource.Provenance.SecretResolution,
		EnvironmentScope:      resource.EnvironmentScope,
		CapturedAt:            capturedAt.UTC(),
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func IsUnavailableProbeError(err error) bool {
	return errors.Is(err, ErrProbeBlocked)
}
