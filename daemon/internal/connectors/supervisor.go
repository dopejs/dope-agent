package connectors

import (
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/livevalidation"
	"github.com/dopejs/dope-agent/daemon/internal/secrets"
)

var (
	ErrConnectorIDRequired      = errors.New("connector id is required")
	ErrConnectorKindRequired    = errors.New("connector kind is required")
	ErrConnectorNotFound        = errors.New("connector not found")
	ErrConnectorDisabled        = errors.New("connector is disabled")
	ErrInvalidConnectorHealth   = errors.New("invalid connector health status")
	ErrConnectorFailureRequired = errors.New("connector failure reason is required")
)

type Status string

const (
	StatusRegistered Status = "registered"
	StatusHealthy    Status = "healthy"
	StatusDegraded   Status = "degraded"
	StatusBackingOff Status = "backing_off"
	StatusFailed     Status = "failed"
	StatusDisabled   Status = "disabled"
)

type Connector struct {
	TenantID          string                          `json:"tenantId,omitempty"`
	ConnectorID       string                          `json:"connectorId"`
	Kind              string                          `json:"kind"`
	DisplayName       string                          `json:"displayName"`
	Status            Status                          `json:"status"`
	DisabledReason    string                          `json:"disabledReason,omitempty"`
	SecretRefs        []string                        `json:"secretRefs,omitempty"`
	SecretSummary     []secrets.RedactedSecretSummary `json:"secretSummary,omitempty"`
	FailureCount      int                             `json:"failureCount"`
	RestartCount      int                             `json:"restartCount"`
	BackoffSeconds    int                             `json:"backoffSeconds"`
	NextRestartAt     *time.Time                      `json:"nextRestartAt,omitempty"`
	LastRestartAt     *time.Time                      `json:"lastRestartAt,omitempty"`
	LastHeartbeatAt   *time.Time                      `json:"lastHeartbeatAt,omitempty"`
	LastFailureReason string                          `json:"lastFailureReason,omitempty"`
	CapabilityProfile map[string]any                  `json:"capabilityProfile,omitempty"`
	DiagnosticState   map[string]any                  `json:"diagnosticState,omitempty"`
	ConformanceResult map[string]any                  `json:"conformanceResult,omitempty"`
	AccountBinding    map[string]any                  `json:"accountBinding,omitempty"`
	CreatedAt         time.Time                       `json:"createdAt"`
	UpdatedAt         time.Time                       `json:"updatedAt"`
}

type RegisterInput struct {
	TenantID    string   `json:"tenantId,omitempty"`
	ConnectorID string   `json:"connectorId"`
	Kind        string   `json:"kind"`
	DisplayName string   `json:"displayName"`
	SecretRefs  []string `json:"secretRefs,omitempty"`
}

type ReportHealthInput struct {
	Status Status `json:"status"`
}

type ReportFailureInput struct {
	Reason string `json:"reason"`
}

type Supervisor struct {
	mu            sync.RWMutex
	byID          map[string]Connector
	order         []string
	mutationLocks map[string]*sync.Mutex
}

func NewSupervisor() *Supervisor {
	return &Supervisor{
		byID:          make(map[string]Connector),
		mutationLocks: make(map[string]*sync.Mutex),
	}
}

func (s *Supervisor) RunLiveValidationOutcome(outcome livevalidation.FakeOutcome) livevalidation.FakeOutcomeResult {
	return livevalidation.FakeOutcomeResultFor(outcome, livevalidation.SafetyClassNonIdempotentMutation)
}

func (s *Supervisor) Register(input RegisterInput) (Connector, bool, error) {
	if input.ConnectorID == "" {
		return Connector{}, false, ErrConnectorIDRequired
	}
	if input.Kind == "" {
		return Connector{}, false, ErrConnectorKindRequired
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	existing, ok := s.byID[input.ConnectorID]
	if ok {
		existing.Kind = input.Kind
		existing.DisplayName = input.DisplayName
		if input.TenantID != "" {
			existing.TenantID = input.TenantID
		}
		if input.SecretRefs != nil {
			existing.SecretRefs = cleanStrings(input.SecretRefs)
		}
		existing.UpdatedAt = now
		s.byID[input.ConnectorID] = existing
		return existing, false, nil
	}

	connector := Connector{
		TenantID:    input.TenantID,
		ConnectorID: input.ConnectorID,
		Kind:        input.Kind,
		DisplayName: input.DisplayName,
		SecretRefs:  cleanStrings(input.SecretRefs),
		Status:      StatusRegistered,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	s.byID[connector.ConnectorID] = connector
	s.order = append(s.order, connector.ConnectorID)

	return connector, true, nil
}

func (s *Supervisor) List() []Connector {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]Connector, 0, len(s.order))
	for _, id := range s.order {
		items = append(items, s.byID[id])
	}
	return items
}

func (s *Supervisor) ListForTenant(tenantID string) []Connector {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]Connector, 0, len(s.order))
	for _, id := range s.order {
		connector := s.byID[id]
		if tenantID == "" || connector.TenantID == tenantID {
			items = append(items, connector)
		}
	}
	return items
}

func (s *Supervisor) Get(connectorID string) (Connector, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	connector, ok := s.byID[connectorID]
	return connector, ok
}

func (s *Supervisor) GetForTenant(connectorID, tenantID string) (Connector, bool) {
	connector, ok := s.Get(connectorID)
	if !ok {
		return Connector{}, false
	}
	if tenantID != "" && connector.TenantID != tenantID {
		return Connector{}, false
	}
	return connector, true
}

func (s *Supervisor) RequireInboundReady(connectorID, tenantID string) (Connector, error) {
	connector, ok := s.GetForTenant(connectorID, tenantID)
	if !ok {
		return Connector{}, ErrConnectorNotFound
	}
	if connector.Status == StatusDisabled {
		return Connector{}, ErrConnectorDisabled
	}
	return connector, nil
}

func (s *Supervisor) WithConnectorMutation(connectorID string, fn func() error) error {
	lock := s.connectorMutationLock(connectorID)
	lock.Lock()
	defer lock.Unlock()
	return fn()
}

func (s *Supervisor) connectorMutationLock(connectorID string) *sync.Mutex {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.mutationLocks == nil {
		s.mutationLocks = make(map[string]*sync.Mutex)
	}
	lock, ok := s.mutationLocks[connectorID]
	if !ok {
		lock = &sync.Mutex{}
		s.mutationLocks[connectorID] = lock
	}
	return lock
}

func (s *Supervisor) ReportHealth(connectorID string, input ReportHealthInput) (Connector, error) {
	if input.Status != StatusHealthy && input.Status != StatusDegraded {
		return Connector{}, ErrInvalidConnectorHealth
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	connector, ok := s.byID[connectorID]
	if !ok {
		return Connector{}, ErrConnectorNotFound
	}
	if connector.Status == StatusDisabled {
		return Connector{}, ErrConnectorDisabled
	}

	now := time.Now().UTC()
	connector.Status = input.Status
	connector.FailureCount = 0
	connector.BackoffSeconds = 0
	connector.NextRestartAt = nil
	connector.LastHeartbeatAt = &now
	connector.UpdatedAt = now
	s.byID[connectorID] = connector

	return connector, nil
}

func (s *Supervisor) ReportFailure(connectorID string, input ReportFailureInput) (Connector, error) {
	if input.Reason == "" {
		return Connector{}, ErrConnectorFailureRequired
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	connector, ok := s.byID[connectorID]
	if !ok {
		return Connector{}, ErrConnectorNotFound
	}
	if connector.Status == StatusDisabled {
		return Connector{}, ErrConnectorDisabled
	}

	now := time.Now().UTC()
	connector.FailureCount++
	connector.LastFailureReason = input.Reason
	connector.UpdatedAt = now

	if connector.FailureCount >= 5 {
		connector.Status = StatusFailed
		connector.BackoffSeconds = 0
		connector.NextRestartAt = nil
		s.byID[connectorID] = connector
		return connector, nil
	}

	backoffSeconds := minInt(5*(1<<(connector.FailureCount-1)), 300)
	nextRestartAt := now.Add(time.Duration(backoffSeconds) * time.Second)
	connector.Status = StatusBackingOff
	connector.BackoffSeconds = backoffSeconds
	connector.NextRestartAt = &nextRestartAt
	s.byID[connectorID] = connector

	return connector, nil
}

func (s *Supervisor) Restart(connectorID string) (Connector, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	connector, ok := s.byID[connectorID]
	if !ok {
		return Connector{}, ErrConnectorNotFound
	}
	if connector.Status == StatusDisabled {
		return Connector{}, ErrConnectorDisabled
	}

	now := time.Now().UTC()
	connector.Status = StatusRegistered
	connector.RestartCount++
	connector.BackoffSeconds = 0
	connector.NextRestartAt = nil
	connector.LastRestartAt = &now
	connector.UpdatedAt = now
	s.byID[connectorID] = connector

	return connector, nil
}

func (s *Supervisor) Disable(connectorID string, reason string) (Connector, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	connector, ok := s.byID[connectorID]
	if !ok {
		return Connector{}, ErrConnectorNotFound
	}
	now := time.Now().UTC()
	connector.Status = StatusDisabled
	connector.DisabledReason = reason
	connector.BackoffSeconds = 0
	connector.NextRestartAt = nil
	connector.UpdatedAt = now
	s.byID[connectorID] = connector
	return connector, nil
}

func (s *Supervisor) ReEnable(connectorID string) (Connector, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	connector, ok := s.byID[connectorID]
	if !ok {
		return Connector{}, ErrConnectorNotFound
	}
	now := time.Now().UTC()
	connector.Status = StatusRegistered
	connector.DisabledReason = ""
	connector.UpdatedAt = now
	s.byID[connectorID] = connector
	return connector, nil
}

func (s *Supervisor) Restore(items []Connector) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.byID = make(map[string]Connector, len(items))
	s.order = make([]string, 0, len(items))
	if s.mutationLocks == nil {
		s.mutationLocks = make(map[string]*sync.Mutex)
	}
	for _, item := range items {
		s.byID[item.ConnectorID] = item
		s.order = append(s.order, item.ConnectorID)
	}
}

func cleanStrings(values []string) []string {
	items := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		items = append(items, trimmed)
	}
	return items
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
