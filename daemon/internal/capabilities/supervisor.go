package capabilities

import (
	"errors"
	"sync"
	"time"
)

var (
	ErrCapabilityIDRequired      = errors.New("capability id is required")
	ErrCapabilityKindRequired    = errors.New("capability kind is required")
	ErrCapabilityNotFound        = errors.New("capability not found")
	ErrInvalidCapabilityHealth   = errors.New("invalid capability health status")
	ErrCapabilityFailureRequired = errors.New("capability failure reason is required")
)

type Status string

const (
	StatusRegistered Status = "registered"
	StatusHealthy    Status = "healthy"
	StatusDegraded   Status = "degraded"
	StatusBackingOff Status = "backing_off"
	StatusFailed     Status = "failed"
)

type Capability struct {
	CapabilityID      string     `json:"capabilityId"`
	Kind              string     `json:"kind"`
	DisplayName       string     `json:"displayName"`
	Status            Status     `json:"status"`
	FailureCount      int        `json:"failureCount"`
	RestartCount      int        `json:"restartCount"`
	BackoffSeconds    int        `json:"backoffSeconds"`
	NextRestartAt     *time.Time `json:"nextRestartAt,omitempty"`
	LastRestartAt     *time.Time `json:"lastRestartAt,omitempty"`
	LastHeartbeatAt   *time.Time `json:"lastHeartbeatAt,omitempty"`
	LastFailureReason string     `json:"lastFailureReason,omitempty"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
}

type RegisterInput struct {
	CapabilityID string `json:"capabilityId"`
	Kind         string `json:"kind"`
	DisplayName  string `json:"displayName"`
}

type ReportHealthInput struct {
	Status Status `json:"status"`
}

type ReportFailureInput struct {
	Reason string `json:"reason"`
}

type Supervisor struct {
	mu    sync.RWMutex
	byID  map[string]Capability
	order []string
}

func NewSupervisor() *Supervisor {
	return &Supervisor{
		byID: make(map[string]Capability),
	}
}

func (s *Supervisor) Register(input RegisterInput) (Capability, bool, error) {
	if input.CapabilityID == "" {
		return Capability{}, false, ErrCapabilityIDRequired
	}
	if input.Kind == "" {
		return Capability{}, false, ErrCapabilityKindRequired
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	existing, ok := s.byID[input.CapabilityID]
	if ok {
		existing.Kind = input.Kind
		existing.DisplayName = input.DisplayName
		existing.UpdatedAt = now
		s.byID[input.CapabilityID] = existing
		return existing, false, nil
	}

	capability := Capability{
		CapabilityID: input.CapabilityID,
		Kind:         input.Kind,
		DisplayName:  input.DisplayName,
		Status:       StatusRegistered,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	s.byID[capability.CapabilityID] = capability
	s.order = append(s.order, capability.CapabilityID)

	return capability, true, nil
}

func (s *Supervisor) List() []Capability {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]Capability, 0, len(s.order))
	for _, id := range s.order {
		items = append(items, s.byID[id])
	}
	return items
}

func (s *Supervisor) Get(capabilityID string) (Capability, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	capability, ok := s.byID[capabilityID]
	return capability, ok
}

func (s *Supervisor) ReportHealth(capabilityID string, input ReportHealthInput) (Capability, error) {
	if input.Status != StatusHealthy && input.Status != StatusDegraded {
		return Capability{}, ErrInvalidCapabilityHealth
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	capability, ok := s.byID[capabilityID]
	if !ok {
		return Capability{}, ErrCapabilityNotFound
	}

	now := time.Now().UTC()
	capability.Status = input.Status
	capability.FailureCount = 0
	capability.BackoffSeconds = 0
	capability.NextRestartAt = nil
	capability.LastHeartbeatAt = &now
	capability.UpdatedAt = now
	s.byID[capabilityID] = capability

	return capability, nil
}

func (s *Supervisor) ReportFailure(capabilityID string, input ReportFailureInput) (Capability, error) {
	if input.Reason == "" {
		return Capability{}, ErrCapabilityFailureRequired
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	capability, ok := s.byID[capabilityID]
	if !ok {
		return Capability{}, ErrCapabilityNotFound
	}

	now := time.Now().UTC()
	capability.FailureCount++
	capability.LastFailureReason = input.Reason
	capability.UpdatedAt = now

	if capability.FailureCount >= 5 {
		capability.Status = StatusFailed
		capability.BackoffSeconds = 0
		capability.NextRestartAt = nil
		s.byID[capabilityID] = capability
		return capability, nil
	}

	backoffSeconds := minInt(5*(1<<(capability.FailureCount-1)), 300)
	nextRestartAt := now.Add(time.Duration(backoffSeconds) * time.Second)
	capability.Status = StatusBackingOff
	capability.BackoffSeconds = backoffSeconds
	capability.NextRestartAt = &nextRestartAt
	s.byID[capabilityID] = capability

	return capability, nil
}

func (s *Supervisor) Restart(capabilityID string) (Capability, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	capability, ok := s.byID[capabilityID]
	if !ok {
		return Capability{}, ErrCapabilityNotFound
	}

	now := time.Now().UTC()
	capability.Status = StatusRegistered
	capability.RestartCount++
	capability.BackoffSeconds = 0
	capability.NextRestartAt = nil
	capability.LastRestartAt = &now
	capability.UpdatedAt = now
	s.byID[capabilityID] = capability

	return capability, nil
}

func (s *Supervisor) Restore(items []Capability) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.byID = make(map[string]Capability, len(items))
	s.order = make([]string, 0, len(items))
	for _, item := range items {
		s.byID[item.CapabilityID] = item
		s.order = append(s.order, item.CapabilityID)
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
