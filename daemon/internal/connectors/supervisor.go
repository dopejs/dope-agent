package connectors

import (
	"errors"
	"sync"
	"time"
)

var (
	ErrConnectorIDRequired      = errors.New("connector id is required")
	ErrConnectorKindRequired    = errors.New("connector kind is required")
	ErrConnectorNotFound        = errors.New("connector not found")
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
)

type Connector struct {
	ConnectorID       string     `json:"connectorId"`
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
	ConnectorID string `json:"connectorId"`
	Kind        string `json:"kind"`
	DisplayName string `json:"displayName"`
}

type ReportHealthInput struct {
	Status Status `json:"status"`
}

type ReportFailureInput struct {
	Reason string `json:"reason"`
}

type Supervisor struct {
	mu    sync.RWMutex
	byID  map[string]Connector
	order []string
}

func NewSupervisor() *Supervisor {
	return &Supervisor{
		byID: make(map[string]Connector),
	}
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
		existing.UpdatedAt = now
		s.byID[input.ConnectorID] = existing
		return existing, false, nil
	}

	connector := Connector{
		ConnectorID: input.ConnectorID,
		Kind:        input.Kind,
		DisplayName: input.DisplayName,
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

func (s *Supervisor) Get(connectorID string) (Connector, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	connector, ok := s.byID[connectorID]
	return connector, ok
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

func (s *Supervisor) Restore(items []Connector) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.byID = make(map[string]Connector, len(items))
	s.order = make([]string, 0, len(items))
	for _, item := range items {
		s.byID[item.ConnectorID] = item
		s.order = append(s.order, item.ConnectorID)
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
