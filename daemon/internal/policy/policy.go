package policy

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

var (
	ErrActionRequired     = errors.New("action is required")
	ErrReasonRequired     = errors.New("reason is required")
	ErrApprovalNotFound   = errors.New("approval not found")
	ErrApprovalNotPending = errors.New("approval is not pending")
	ErrInvalidResolution  = errors.New("invalid approval resolution")
)

type ApprovalStatus string

const (
	ApprovalStatusPending  ApprovalStatus = "pending"
	ApprovalStatusApproved ApprovalStatus = "approved"
	ApprovalStatusRejected ApprovalStatus = "rejected"
)

type DecisionOutcome string

const (
	DecisionOutcomeAllowed          DecisionOutcome = "allowed"
	DecisionOutcomeRequiresApproval DecisionOutcome = "requires_approval"
	DecisionOutcomeApproved         DecisionOutcome = "approved"
	DecisionOutcomeRejected         DecisionOutcome = "rejected"
)

type Approval struct {
	ApprovalID   string         `json:"approvalId"`
	Action       string         `json:"action"`
	ResourceKind string         `json:"resourceKind,omitempty"`
	ResourceID   string         `json:"resourceId,omitempty"`
	Reason       string         `json:"reason"`
	RequestedBy  string         `json:"requestedBy,omitempty"`
	Status       ApprovalStatus `json:"status"`
	CreatedAt    time.Time      `json:"createdAt"`
	UpdatedAt    time.Time      `json:"updatedAt"`
	ResolvedAt   *time.Time     `json:"resolvedAt,omitempty"`
	Resolution   string         `json:"resolution,omitempty"`
	Comment      string         `json:"comment,omitempty"`
}

type Decision struct {
	DecisionID   string          `json:"decisionId"`
	Action       string          `json:"action"`
	ResourceKind string          `json:"resourceKind,omitempty"`
	ResourceID   string          `json:"resourceId,omitempty"`
	Outcome      DecisionOutcome `json:"outcome"`
	Reason       string          `json:"reason"`
	ApprovalID   string          `json:"approvalId,omitempty"`
	CreatedAt    time.Time       `json:"createdAt"`
}

type RequestApprovalInput struct {
	Action       string `json:"action"`
	ResourceKind string `json:"resourceKind"`
	ResourceID   string `json:"resourceId"`
	Reason       string `json:"reason"`
	RequestedBy  string `json:"requestedBy"`
}

type ResolveApprovalInput struct {
	Resolution string `json:"resolution"`
	Comment    string `json:"comment"`
}

type Engine struct {
	mu            sync.RWMutex
	approvalsByID map[string]Approval
	approvalIDs   []string
	decisionsByID map[string]Decision
	decisionIDs   []string
}

func NewEngine() *Engine {
	return &Engine{
		approvalsByID: make(map[string]Approval),
		decisionsByID: make(map[string]Decision),
	}
}

func (e *Engine) RequestApproval(input RequestApprovalInput) (Approval, Decision, error) {
	if input.Action == "" {
		return Approval{}, Decision{}, ErrActionRequired
	}
	if input.Reason == "" {
		return Approval{}, Decision{}, ErrReasonRequired
	}

	now := time.Now().UTC()
	approval := Approval{
		ApprovalID:   newApprovalID(),
		Action:       input.Action,
		ResourceKind: input.ResourceKind,
		ResourceID:   input.ResourceID,
		Reason:       input.Reason,
		RequestedBy:  input.RequestedBy,
		Status:       ApprovalStatusPending,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	decision := Decision{
		DecisionID:   newDecisionID(),
		Action:       input.Action,
		ResourceKind: input.ResourceKind,
		ResourceID:   input.ResourceID,
		Outcome:      DecisionOutcomeRequiresApproval,
		Reason:       input.Reason,
		ApprovalID:   approval.ApprovalID,
		CreatedAt:    now,
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	e.approvalsByID[approval.ApprovalID] = approval
	e.approvalIDs = append(e.approvalIDs, approval.ApprovalID)
	e.decisionsByID[decision.DecisionID] = decision
	e.decisionIDs = append(e.decisionIDs, decision.DecisionID)

	return approval, decision, nil
}

func (e *Engine) ListApprovals(status ApprovalStatus) []Approval {
	e.mu.RLock()
	defer e.mu.RUnlock()

	items := make([]Approval, 0, len(e.approvalIDs))
	for _, approvalID := range e.approvalIDs {
		approval := e.approvalsByID[approvalID]
		if status != "" && approval.Status != status {
			continue
		}
		items = append(items, approval)
	}

	return items
}

func (e *Engine) GetApproval(approvalID string) (Approval, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	approval, ok := e.approvalsByID[approvalID]
	return approval, ok
}

func (e *Engine) ResolveApproval(approvalID string, input ResolveApprovalInput) (Approval, Decision, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	approval, ok := e.approvalsByID[approvalID]
	if !ok {
		return Approval{}, Decision{}, ErrApprovalNotFound
	}
	if approval.Status != ApprovalStatusPending {
		return Approval{}, Decision{}, ErrApprovalNotPending
	}

	var (
		status  ApprovalStatus
		outcome DecisionOutcome
	)
	switch input.Resolution {
	case string(ApprovalStatusApproved):
		status = ApprovalStatusApproved
		outcome = DecisionOutcomeApproved
	case string(ApprovalStatusRejected):
		status = ApprovalStatusRejected
		outcome = DecisionOutcomeRejected
	default:
		return Approval{}, Decision{}, ErrInvalidResolution
	}

	now := time.Now().UTC()
	approval.Status = status
	approval.Resolution = input.Resolution
	approval.Comment = input.Comment
	approval.UpdatedAt = now
	approval.ResolvedAt = &now
	e.approvalsByID[approvalID] = approval

	decision := Decision{
		DecisionID:   newDecisionID(),
		Action:       approval.Action,
		ResourceKind: approval.ResourceKind,
		ResourceID:   approval.ResourceID,
		Outcome:      outcome,
		Reason:       approval.Reason,
		ApprovalID:   approval.ApprovalID,
		CreatedAt:    now,
	}
	e.decisionsByID[decision.DecisionID] = decision
	e.decisionIDs = append(e.decisionIDs, decision.DecisionID)

	return approval, decision, nil
}

func newApprovalID() string {
	return "approval_" + randomSuffix("fallback")
}

func newDecisionID() string {
	return "decision_" + randomSuffix("fallback")
}

func randomSuffix(fallback string) string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return fallback
	}
	return hex.EncodeToString(buf)
}
