package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/billing"
	"github.com/dopejs/dope-agent/daemon/internal/config"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
)

type billingPlanAssignmentRequest struct {
	PlanKey         string                  `json:"planKey"`
	EnforcementMode billing.EnforcementMode `json:"enforcementMode"`
	Reason          string                  `json:"reason"`
}

type billingQuotaOverrideRequest struct {
	Category billing.Category `json:"category"`
	Limit    *int64           `json:"limit,omitempty"`
	Reason   string           `json:"reason"`
}

type billingManualAdjustmentRequest struct {
	Category      billing.Category `json:"category"`
	QuotaPeriodID string           `json:"quotaPeriodId"`
	AmountDelta   int64            `json:"amountDelta"`
	Reason        string           `json:"reason"`
}

type billingReservationResolutionRequest struct {
	Outcome billing.ReservationStatus `json:"outcome"`
	Reason  string                    `json:"reason"`
	Amount  int64                     `json:"amount,omitempty"`
}

func handleHostedBilling(cfg config.Config, manager *billing.Manager, w http.ResponseWriter, r *http.Request) {
	if manager == nil {
		writeError(w, http.StatusInternalServerError, "billing manager is not configured")
		return
	}
	tenantContext, err := RequirePermission(r.Context(), identity.PermissionBillingView)
	if err != nil {
		writeTenantDenial(w, http.StatusForbidden)
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	hosted := cfg.Environment == config.EnvironmentProd
	switch strings.Trim(strings.TrimPrefix(r.URL.Path, "/v1/billing/"), "/") {
	case "plan":
		plan, err := manager.ActivePlan(r.Context(), tenantContext.TenantID, hosted)
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, plan)
	case "usage":
		summary, err := manager.UsageSummary(r.Context(), tenantContext.TenantID, hosted)
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, summary)
	case "quotas":
		summary, err := manager.UsageSummary(r.Context(), tenantContext.TenantID, hosted)
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, ListResponse[billing.EffectiveQuota]{Items: summary.Quotas})
	case "denials":
		summary, err := manager.UsageSummary(r.Context(), tenantContext.TenantID, hosted)
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, ListResponse[billing.QuotaDenial]{Items: summary.Denials})
	default:
		http.NotFound(w, r)
	}
}

func handleHostedBillingAdmin(manager *billing.Manager, w http.ResponseWriter, r *http.Request) {
	if manager == nil {
		writeError(w, http.StatusInternalServerError, "billing manager is not configured")
		return
	}
	tenantContext, err := RequirePermission(r.Context(), identity.PermissionBillingManage)
	if err != nil {
		writeTenantDenial(w, http.StatusForbidden)
		return
	}
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/v1/admin/billing/tenants/"), "/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}
	targetTenantID := parts[0]
	if targetTenantID != tenantContext.TenantID {
		writeTenantDenial(w, http.StatusForbidden)
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	switch parts[1] {
	case "plan":
		handleBillingPlanAssignment(manager, tenantContext, targetTenantID, w, r)
	case "quota-overrides":
		handleBillingQuotaOverride(manager, tenantContext, targetTenantID, w, r)
	case "manual-adjustments":
		handleBillingManualAdjustment(manager, tenantContext, targetTenantID, w, r)
	case "reservations":
		if len(parts) != 4 || parts[2] == "" || parts[3] != "resolve" {
			http.NotFound(w, r)
			return
		}
		handleBillingReservationResolution(manager, tenantContext, targetTenantID, parts[2], w, r)
	default:
		http.NotFound(w, r)
	}
}

func handleBillingPlanAssignment(manager *billing.Manager, tenantContext identity.TenantContext, tenantID string, w http.ResponseWriter, r *http.Request) {
	var request billingPlanAssignmentRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	plan := billing.TenantPlan{
		PlanID:                "plan_" + tenantID + "_" + strings.ReplaceAll(strings.TrimSpace(request.PlanKey), " ", "_") + "_" + time.Now().UTC().Format("20060102150405"),
		TenantID:              tenantID,
		PlanKey:               strings.TrimSpace(request.PlanKey),
		Status:                billing.PlanStatusActive,
		EnforcementMode:       request.EnforcementMode,
		AssignedByPrincipalID: tenantContext.PrincipalID,
		AssignmentReason:      request.Reason,
		EffectiveAt:           time.Now().UTC(),
	}
	if plan.PlanKey == "" {
		writeError(w, http.StatusBadRequest, "planKey is required")
		return
	}
	if err := manager.AssignPlan(r.Context(), plan, tenantContext.PrincipalID, request.Reason); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, plan)
}

func handleBillingQuotaOverride(manager *billing.Manager, tenantContext identity.TenantContext, tenantID string, w http.ResponseWriter, r *http.Request) {
	var request billingQuotaOverrideRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	override := billing.QuotaOverride{
		QuotaOverrideID:      "quota_override_" + tenantID + "_" + string(request.Category) + "_" + time.Now().UTC().Format("20060102150405"),
		TenantID:             tenantID,
		Category:             request.Category,
		Limit:                request.Limit,
		EffectiveAt:          time.Now().UTC(),
		Reason:               request.Reason,
		CreatedByPrincipalID: tenantContext.PrincipalID,
	}
	if _, ok := billing.DefinitionFor(request.Category); !ok {
		writeError(w, http.StatusBadRequest, "unknown billing quota category")
		return
	}
	if err := manager.ApplyQuotaOverride(r.Context(), override); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, override)
}

func handleBillingManualAdjustment(manager *billing.Manager, tenantContext identity.TenantContext, tenantID string, w http.ResponseWriter, r *http.Request) {
	var request billingManualAdjustmentRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	adjustment := billing.ManualAdjustment{
		AdjustmentID:         "manual_adjustment_" + tenantID + "_" + string(request.Category) + "_" + time.Now().UTC().Format("20060102150405"),
		TenantID:             tenantID,
		Category:             request.Category,
		QuotaPeriodID:        request.QuotaPeriodID,
		AmountDelta:          request.AmountDelta,
		Reason:               request.Reason,
		CreatedByPrincipalID: tenantContext.PrincipalID,
		CreatedAt:            time.Now().UTC(),
	}
	if adjustment.QuotaPeriodID == "" {
		writeError(w, http.StatusBadRequest, "quotaPeriodId is required")
		return
	}
	if err := manager.ApplyManualAdjustment(r.Context(), adjustment); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, adjustment)
}

func handleBillingReservationResolution(manager *billing.Manager, tenantContext identity.TenantContext, tenantID, reservationID string, w http.ResponseWriter, r *http.Request) {
	var request billingReservationResolutionRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	reservation, err := manager.ResolveReservation(r.Context(), billing.ResolveReservationInput{
		TenantID:         tenantID,
		ReservationID:    reservationID,
		Outcome:          request.Outcome,
		Amount:           request.Amount,
		Reason:           request.Reason,
		ActorPrincipalID: tenantContext.PrincipalID,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, reservation)
}
