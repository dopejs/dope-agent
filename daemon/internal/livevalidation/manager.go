package livevalidation

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/billing"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/tenantctx"
)

var (
	ErrLiveValidationDisabled = errors.New("live validation is disabled")
	ErrLiveValidationBlocked  = errors.New("live validation blocked")
)

type CandidateToolClassResolver func(ctx context.Context, candidateID string) ([]ToolClass, error)

type Dependencies struct {
	EnvironmentScope           string
	Store                      Store
	Enabled                    bool
	Billing                    *billing.Manager
	HostedBilling              bool
	Clock                      func() time.Time
	LedgerEventSink            LedgerEventSink
	CandidateToolClassResolver CandidateToolClassResolver
}

type Manager struct {
	environmentScope           string
	store                      Store
	enabled                    bool
	billingManager             *billing.Manager
	hostedBilling              bool
	clock                      func() time.Time
	ledgerEventSink            LedgerEventSink
	candidateToolClassResolver CandidateToolClassResolver
}

func NewManager(deps Dependencies) *Manager {
	clock := deps.Clock
	if clock == nil {
		clock = func() time.Time { return time.Now().UTC() }
	}
	environmentScope := deps.EnvironmentScope
	if environmentScope == "" {
		environmentScope = "test"
	}
	return &Manager{
		environmentScope:           environmentScope,
		store:                      deps.Store,
		enabled:                    deps.Enabled,
		billingManager:             deps.Billing,
		hostedBilling:              deps.HostedBilling,
		clock:                      clock,
		ledgerEventSink:            deps.LedgerEventSink,
		candidateToolClassResolver: deps.CandidateToolClassResolver,
	}
}

func (m *Manager) Enabled() bool {
	return m != nil && m.enabled
}

func (m *Manager) EnvironmentScope() string {
	if m == nil || m.environmentScope == "" {
		return "test"
	}
	return m.environmentScope
}

func (m *Manager) SupportMatrix() (Matrix, error) {
	return NewMatrix(DefaultMatrixRows())
}

type StartInput struct {
	ValidationID         string          `json:"validationId,omitempty"`
	CandidateID          string          `json:"candidateId"`
	SourceAttemptID      string          `json:"sourceAttemptId,omitempty"`
	CandidateToolClasses []ToolClass     `json:"candidateToolClasses,omitempty"`
	RequestedScope       SideEffectScope `json:"requestedScope"`
	FreshApprovals       []FreshApproval `json:"freshApprovals,omitempty"`
	ClientKey            string          `json:"clientKey,omitempty"`
	ChangeWindowLabel    string          `json:"changeWindowLabel,omitempty"`
}

type Denial struct {
	Gate       string `json:"gate"`
	ReasonCode string `json:"reasonCode"`
	Message    string `json:"message"`
	Reference  string `json:"reference,omitempty"`
}

type StartResult struct {
	Attempt Attempt  `json:"attempt"`
	Denials []Denial `json:"denials,omitempty"`
}

func (m *Manager) Start(ctx context.Context, input StartInput) (StartResult, error) {
	if !m.Enabled() {
		return StartResult{}, ErrLiveValidationDisabled
	}
	now := m.clock()
	tenantContext, ok := tenantctx.FromContext(ctx)
	if !ok || tenantContext.TenantID == "" || tenantContext.PrincipalID == "" {
		attempt := m.newAttempt(input, identity.TenantContext{}, now)
		return m.block(ctx, attempt, "permission", "tenant_context_missing", "Tenant context is required.", "")
	}
	attempt := m.newAttempt(input, tenantContext, now)

	permission := identity.EvaluatePermission(tenantContext, identity.PermissionLiveValidationExecute)
	attempt.PermissionDecision = GateDecision{Allowed: permission.Allowed, ReasonCode: permission.ReasonCode, CheckedAt: now}
	if !permission.Allowed {
		return m.block(ctx, attempt, "permission", firstNonEmpty(permission.ReasonCode, "permission_missing"), "Missing live validation permission.", string(identity.PermissionLiveValidationExecute))
	}

	quotaDecision, quotaDenial, reservation, quotaErr := m.evaluateQuota(ctx, tenantContext.TenantID, attempt.ValidationID, input.ClientKey, now)
	attempt.QuotaDecision = quotaDecision
	if quotaErr != nil || !quotaDecision.Allowed {
		return m.block(ctx, attempt, "quota", firstNonEmpty(quotaDecision.ReasonCode, "quota_denied"), firstNonEmpty(quotaDenial.Message, "Live validation quota denied."), quotaDenial.OperationKey)
	}

	killSwitchDecision, killDenial, err := m.evaluateKillSwitch(ctx, tenantContext.TenantID, now)
	attempt.KillSwitchDecision = killSwitchDecision
	if err != nil {
		m.releaseQuota(ctx, reservation, "live validation kill-switch check failed before start")
		return StartResult{}, err
	}
	if !killSwitchDecision.Allowed {
		m.releaseQuota(ctx, reservation, "live validation blocked by kill switch before start")
		return m.block(ctx, attempt, "kill_switch", killSwitchDecision.ReasonCode, killDenial.Message, killDenial.Reference)
	}

	resolvedInput, err := m.resolveCandidateToolClasses(ctx, input)
	if err != nil {
		m.releaseQuota(ctx, reservation, "live validation candidate tool class resolution failed before start")
		return StartResult{}, err
	}
	input = resolvedInput
	if denial := m.evaluateSupport(input); denial != nil {
		m.releaseQuota(ctx, reservation, "live validation support check blocked before start")
		return m.block(ctx, attempt, denial.Gate, denial.ReasonCode, denial.Message, denial.Reference)
	}

	freshApprovals := normalizeFreshApprovals(input.FreshApprovals, attempt)
	approvalSummary := m.approvalSummary(attempt, input.RequestedScope, freshApprovals)
	attempt.ApprovalSummary = approvalSummary
	if approvalSummary.Denied > 0 {
		m.releaseQuota(ctx, reservation, "live validation approval denied before live start")
		return m.block(ctx, attempt, "approval", "live_validation.approval_denied", "A required fresh approval was denied.", "")
	}
	if approvalSummary.Expired > 0 {
		m.releaseQuota(ctx, reservation, "live validation approval expired before live start")
		return m.block(ctx, attempt, "approval", "live_validation.approval_expired", "A required fresh approval is expired.", "")
	}
	if approvalSummary.Pending > 0 {
		attempt.Status = AttemptStatusAwaitingApproval
		if err := m.persistAttempt(ctx, attempt); err != nil {
			m.releaseQuota(ctx, reservation, "live validation awaiting-approval attempt failed to persist")
			return StartResult{}, err
		}
		m.releaseQuota(ctx, reservation, "live validation awaits approval before live start")
		return StartResult{Attempt: attempt}, nil
	}

	attempt.Status = AttemptStatusRunning
	attempt.StartedAt = &now
	if err := m.persistAttempt(ctx, attempt); err != nil {
		m.releaseQuota(ctx, reservation, "live validation running attempt failed to persist")
		return StartResult{}, err
	}
	if err := m.commitQuota(ctx, reservation, "live_validation.started", "live validation started after gates passed"); err != nil {
		return StartResult{}, err
	}
	return StartResult{Attempt: attempt}, nil
}

func (m *Manager) evaluateQuota(ctx context.Context, tenantID, validationID, clientKey string, now time.Time) (GateDecision, billing.DenialPayload, billing.UsageReservation, error) {
	operationKey := billing.LiveValidationOperationKey(tenantID, validationID, clientKey)
	result, err := billing.ReserveLiveValidationPreflight(ctx, m.billingManager, tenantID, validationID, clientKey, m.hostedBilling)
	if err != nil {
		denial := billing.DenialPayload{}
		if result.Denial != nil {
			denial = *result.Denial
		}
		return GateDecision{Allowed: false, ReasonCode: firstNonEmpty(denial.ReasonCode, err.Error()), Reference: operationKey, CheckedAt: now}, denial, result.Reservation, err
	}
	return GateDecision{Allowed: result.Allowed, Reference: operationKey, CheckedAt: now}, billing.DenialPayload{}, result.Reservation, nil
}

func (m *Manager) resolveCandidateToolClasses(ctx context.Context, input StartInput) (StartInput, error) {
	if len(input.CandidateToolClasses) > 0 || m == nil || m.candidateToolClassResolver == nil || input.CandidateID == "" {
		return input, nil
	}
	classes, err := m.candidateToolClassResolver(ctx, input.CandidateID)
	if err != nil {
		return input, err
	}
	input.CandidateToolClasses = dedupeToolClasses(classes)
	return input, nil
}

func (m *Manager) releaseQuota(ctx context.Context, reservation billing.UsageReservation, reason string) {
	if m == nil || m.billingManager == nil || reservation.ReservationID == "" {
		return
	}
	_, _ = m.billingManager.Release(ctx, billing.ResolveInput{
		TenantID:     reservation.TenantID,
		Category:     reservation.Category,
		OperationKey: reservation.OperationKey,
		Amount:       reservation.AmountReserved,
		ReasonCode:   "live_validation.preflight_released",
		Reason:       reason,
	})
}

func (m *Manager) commitQuota(ctx context.Context, reservation billing.UsageReservation, reasonCode, reason string) error {
	if m == nil || m.billingManager == nil || reservation.ReservationID == "" {
		return nil
	}
	_, err := m.billingManager.Commit(ctx, billing.ResolveInput{
		TenantID:     reservation.TenantID,
		Category:     reservation.Category,
		OperationKey: reservation.OperationKey,
		Amount:       reservation.AmountReserved,
		ReasonCode:   reasonCode,
		Reason:       reason,
	})
	return err
}

func (m *Manager) evaluateSupport(input StartInput) *Denial {
	matrix, err := m.SupportMatrix()
	if err != nil {
		return &Denial{Gate: "support_matrix", ReasonCode: "live_validation.support_matrix_invalid", Message: err.Error()}
	}
	scope := input.RequestedScope
	reachableClasses := input.CandidateToolClasses
	if len(reachableClasses) == 0 {
		return &Denial{Gate: "support_matrix", ReasonCode: "live_validation.candidate_tool_classes_required", Message: "Live validation requires explicit candidate tool classes."}
	}
	readiness := EvaluateCandidateReadiness(matrix, CandidateReadinessInput{
		CandidateID:          input.CandidateID,
		ReachableToolClasses: reachableClasses,
		RequestedScope:       scope,
	})
	if readiness.Status == ReadinessStatusBlocked {
		reference := ""
		if len(readiness.UnsupportedClasses) > 0 {
			reference = string(readiness.UnsupportedClasses[0])
		}
		return &Denial{Gate: "support_matrix", ReasonCode: "live_validation.unsupported_tool_class", Message: "Unsupported candidate tool classes must be explicitly excluded from live validation scope.", Reference: reference}
	}
	for _, toolClass := range scope.IncludedToolClasses {
		if _, err := matrix.Lookup(toolClass); err != nil {
			return &Denial{Gate: "support_matrix", ReasonCode: "live_validation.unsupported_tool_class", Message: fmt.Sprintf("Tool class %s is unsupported for live validation.", toolClass), Reference: string(toolClass)}
		}
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func dedupeToolClasses(items []ToolClass) []ToolClass {
	if len(items) == 0 {
		return nil
	}
	deduped := make([]ToolClass, 0, len(items))
	seen := map[ToolClass]bool{}
	for _, item := range items {
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		deduped = append(deduped, item)
	}
	return deduped
}

func newID(prefix string) string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
	}
	return prefix + "_" + hex.EncodeToString(b[:])
}
