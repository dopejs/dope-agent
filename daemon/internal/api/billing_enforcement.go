package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/dopejs/dope-agent/daemon/internal/billing"
	"github.com/dopejs/dope-agent/daemon/internal/config"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
)

type BillingEnforcementMatrixEntry struct {
	EntryPoint              string             `json:"entryPoint"`
	Categories              []billing.Category `json:"categories"`
	ReservationAmount       string             `json:"reservationAmount"`
	OperationKeySource      string             `json:"operationKeySource"`
	CommitRefundTransition  string             `json:"commitRefundTransition"`
	QuotaStateUnavailable   string             `json:"quotaStateUnavailable"`
	RequiredVerificationSet []string           `json:"requiredVerificationSet"`
}

func BillingEnforcementMatrix() []BillingEnforcementMatrixEntry {
	common := []string{"allowed", "denied", "retry", "restart_pending"}
	return []BillingEnforcementMatrixEntry{
		{
			EntryPoint:              "POST /v1/runs",
			Categories:              []billing.Category{billing.CategoryRunLaunches},
			ReservationAmount:       "1 run launch",
			OperationKeySource:      "client idempotency key when present, otherwise daemon run id",
			CommitRefundTransition:  "commit when run is persisted; release/refund before durable run creation",
			QuotaStateUnavailable:   "hosted denies with quota_denied:quota_state_unavailable; unlimited local allows",
			RequiredVerificationSet: append(common, "persistence_failure_refund", "concurrent_last_unit"),
		},
		{
			EntryPoint:              "POST /v1/runs/{runId}/workflows",
			Categories:              []billing.Category{billing.CategoryWorkflowLaunches},
			ReservationAmount:       "1 workflow launch",
			OperationKeySource:      "run id plus workflow client key or daemon workflow id",
			CommitRefundTransition:  "commit when workflow is persisted as planned; refund planning failures",
			QuotaStateUnavailable:   "hosted denies; unlimited local allows",
			RequiredVerificationSet: append(common, "planning_failure_refund", "concurrent_workflow_creation"),
		},
		{
			EntryPoint:              "POST /v1/runs/{runId}/workflows/{workflowId}/start",
			Categories:              []billing.Category{billing.CategoryWorkflowLaunches},
			ReservationAmount:       "1 workflow launch or 0 when reusing an existing reservation",
			OperationKeySource:      "run id plus workflow id",
			CommitRefundTransition:  "commit when workflow enters running; refund start failures before execution",
			QuotaStateUnavailable:   "hosted denies; unlimited local allows",
			RequiredVerificationSet: append(common, "start_failure_refund", "concurrent_start"),
		},
		{
			EntryPoint:              "POST /v1/runs/{runId}/steps/{stepId}/tool-calls",
			Categories:              []billing.Category{billing.CategoryRuntimeToolCalls, billing.CategoryIntegrationOperations},
			ReservationAmount:       "1 tool call plus domain amounts when integration-backed work is invoked",
			OperationKeySource:      "run id plus step id plus tool call id or client key",
			CommitRefundTransition:  "commit when tool call is accepted/running/completed; refund creation failures",
			QuotaStateUnavailable:   "hosted denies; unlimited local allows",
			RequiredVerificationSet: append(common, "tool_creation_failure_refund", "concurrent_tool_calls"),
		},
		{
			EntryPoint:              "Roadmap 38 live-validation preflight gate",
			Categories:              []billing.Category{billing.CategoryLiveValidationAttempts, billing.CategoryIntegrationOperations},
			ReservationAmount:       "1 live validation attempt plus integration amount when applicable",
			OperationKeySource:      "validation request id or client key",
			CommitRefundTransition:  "commit once live validation starts; refund preflight failures before live action",
			QuotaStateUnavailable:   "hosted denies before live side effects; unlimited local allows",
			RequiredVerificationSet: append(common, "fail_closed_unavailable", "preflight_refund", "no_roadmap_40_executor"),
		},
		{
			EntryPoint:              "Calendar/mail/integration operation routes",
			Categories:              []billing.Category{billing.CategoryIntegrationOperations, billing.CategoryArtifactStorageBytes},
			ReservationAmount:       "1 operation plus estimated artifact bytes",
			OperationKeySource:      "domain operation id or client key",
			CommitRefundTransition:  "commit operation after backend attempt; reconcile artifact actual bytes after write",
			QuotaStateUnavailable:   "hosted denies before backend call; unlimited local allows",
			RequiredVerificationSet: append(common, "backend_preflight_refund", "artifact_estimate_reconciliation", "concurrent_operations"),
		},
		{
			EntryPoint:              "Computer-use and other artifact write service",
			Categories:              []billing.Category{billing.CategoryArtifactStorageBytes},
			ReservationAmount:       "defensible byte estimate",
			OperationKeySource:      "artifact id, storage key, or client key",
			CommitRefundTransition:  "commit actual bytes; refund smaller actuals; commit over-limit actuals with future denial",
			QuotaStateUnavailable:   "hosted denies before write; unlimited local allows",
			RequiredVerificationSet: append(common, "actual_smaller_refund", "actual_larger_over_limit_commit", "future_denial_after_over_limit_commit", "write_failure_release"),
		},
		{
			EntryPoint:              "POST /v1/evaluation/replay-candidates/{candidateId}/attempts",
			Categories:              []billing.Category{billing.CategoryReplayEvaluationAttempts, billing.CategoryRunLaunches, billing.CategoryWorkflowLaunches},
			ReservationAmount:       "1 replay/evaluation attempt plus runtime categories when runtime work is created",
			OperationKeySource:      "candidate id plus attempt id or client key",
			CommitRefundTransition:  "commit when attempt is accepted or runtime replay is recorded; refund rejected candidates",
			QuotaStateUnavailable:   "hosted denies before replay/campaign work; unlimited local allows",
			RequiredVerificationSet: append(common, "unreplayable_preflight_refund", "concurrent_attempts"),
		},
	}
}

func writeBillingDenial(w http.ResponseWriter, result billing.ReserveResult, err error) {
	status := http.StatusServiceUnavailable
	if errors.Is(err, billing.ErrQuotaDenied) {
		status = http.StatusTooManyRequests
	}
	if result.Denial != nil {
		writeJSON(w, status, result.Denial)
		return
	}
	writeError(w, status, err.Error())
}

type billingReservationError struct {
	result billing.ReserveResult
	err    error
}

func (e billingReservationError) Error() string {
	return e.err.Error()
}

func (e billingReservationError) Unwrap() error {
	return e.err
}

func newBillingReservationError(result billing.ReserveResult, err error) error {
	return billingReservationError{result: result, err: err}
}

func writeBillingReservationError(w http.ResponseWriter, err error) bool {
	var reservationErr billingReservationError
	if errors.As(err, &reservationErr) {
		writeBillingDenial(w, reservationErr.result, reservationErr.err)
		return true
	}
	var carrier interface {
		BillingReserveResult() (billing.ReserveResult, error)
	}
	if errors.As(err, &carrier) {
		result, cause := carrier.BillingReserveResult()
		writeBillingDenial(w, result, cause)
		return true
	}
	return false
}

func releaseBillingReservation(ctx context.Context, manager *billing.Manager, reservation billing.UsageReservation, reason string) {
	if manager == nil || reservation.ReservationID == "" {
		return
	}
	_, _ = manager.Release(ctx, billing.ResolveInput{
		TenantID:     reservation.TenantID,
		Category:     reservation.Category,
		OperationKey: reservation.OperationKey,
		Amount:       reservation.AmountReserved,
		ReasonCode:   "billing.reservation_released",
		Reason:       reason,
	})
}

func commitBillingReservation(ctx context.Context, manager *billing.Manager, reservation billing.UsageReservation, reasonCode, reason string) error {
	if manager == nil || reservation.ReservationID == "" {
		return nil
	}
	_, err := manager.Commit(ctx, billing.ResolveInput{
		TenantID:     reservation.TenantID,
		Category:     reservation.Category,
		OperationKey: reservation.OperationKey,
		Amount:       reservation.AmountReserved,
		ReasonCode:   reasonCode,
		Reason:       reason,
	})
	return err
}

func reserveRuntimeToolCallQuota(ctx context.Context, manager *billing.Manager, tenantID, runID, stepID, toolCallID, entryPoint string, hosted bool) (billing.ReserveResult, error) {
	if manager == nil {
		if hosted {
			operationKey := billing.ToolCallOperationKey(tenantID, runID, stepID, toolCallID, "")
			denial := billing.NewQuotaStateUnavailableDenial(tenantID, operationKey).Payload
			return billing.ReserveResult{Allowed: false, Denial: &denial}, billing.ErrQuotaStateUnavailable
		}
		return billing.ReserveResult{Allowed: true}, nil
	}
	return manager.Reserve(ctx, billing.ReserveInput{
		TenantID:          tenantID,
		Category:          billing.CategoryRuntimeToolCalls,
		Amount:            1,
		OperationKey:      billing.ToolCallOperationKey(tenantID, runID, stepID, toolCallID, ""),
		ReservationPoint:  entryPoint + " before runtime.CreateToolCall",
		GuardedEntryPoint: entryPoint,
		Hosted:            hosted,
	})
}

func maybeCommitRuntimeToolCallQuota(ctx context.Context, manager *billing.Manager, tenantID, runID, stepID string, toolCall runtime.ToolCall, reasonCode, reason string) error {
	if manager == nil || tenantID == "" || toolCall.ToolCallID == "" {
		return nil
	}
	_, err := manager.Commit(ctx, billing.ResolveInput{
		TenantID:     tenantID,
		Category:     billing.CategoryRuntimeToolCalls,
		OperationKey: billing.ToolCallOperationKey(tenantID, runID, stepID, toolCall.ToolCallID, ""),
		Amount:       1,
		ReasonCode:   reasonCode,
		Reason:       reason,
	})
	if errors.Is(err, billing.ErrReservationNotFound) {
		return nil
	}
	return err
}

func reserveIntegrationOperationQuota(ctx context.Context, manager *billing.Manager, tenantID, domain, operationID, clientKey, entryPoint string, hosted bool) (billing.ReserveResult, error) {
	operationKey := billing.IntegrationOperationKey(tenantID, domain, operationID, clientKey)
	if manager == nil {
		if hosted {
			denial := billing.NewQuotaStateUnavailableDenial(tenantID, operationKey).Payload
			return billing.ReserveResult{Allowed: false, Denial: &denial}, billing.ErrQuotaStateUnavailable
		}
		return billing.ReserveResult{Allowed: true}, nil
	}
	return manager.Reserve(ctx, billing.ReserveInput{
		TenantID:          tenantID,
		Category:          billing.CategoryIntegrationOperations,
		Amount:            1,
		OperationKey:      operationKey,
		ReservationPoint:  entryPoint + " before integration backend operation",
		GuardedEntryPoint: entryPoint,
		Hosted:            hosted,
	})
}

func beginIntegrationOperationQuota(ctx context.Context, cfg config.Config, manager *billing.Manager, domain, operationID, entryPoint string, w http.ResponseWriter, r *http.Request) (billing.UsageReservation, bool) {
	tenantContext, ok := tenantContextFromContext(ctx)
	if !ok || tenantContext.TenantID == "" {
		return billing.UsageReservation{}, true
	}
	result, err := reserveIntegrationOperationQuota(ctx, manager, tenantContext.TenantID, domain, operationID, r.Header.Get("Idempotency-Key"), entryPoint, cfg.Environment == config.EnvironmentProd)
	if err != nil {
		writeBillingDenial(w, result, err)
		return billing.UsageReservation{}, false
	}
	return result.Reservation, true
}

func reserveLiveValidationPreflight(ctx context.Context, manager *billing.Manager, tenantID, validationID, clientKey string, hosted bool) (billing.ReserveResult, error) {
	if manager == nil {
		if hosted {
			operationKey := billing.LiveValidationOperationKey(tenantID, validationID, clientKey)
			denial := billing.NewQuotaStateUnavailableDenial(tenantID, operationKey).Payload
			return billing.ReserveResult{Allowed: false, Denial: &denial}, billing.ErrQuotaStateUnavailable
		}
		return billing.ReserveResult{Allowed: true}, nil
	}
	return manager.Reserve(ctx, billing.ReserveInput{
		TenantID:          tenantID,
		Category:          billing.CategoryLiveValidationAttempts,
		Amount:            1,
		OperationKey:      billing.LiveValidationOperationKey(tenantID, validationID, clientKey),
		ReservationPoint:  "Roadmap 38 live-validation preflight gate",
		GuardedEntryPoint: "Roadmap 38 live-validation preflight gate",
		Hosted:            hosted,
	})
}
