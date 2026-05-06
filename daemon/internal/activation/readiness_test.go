package activation

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/billing"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
)

func TestServiceActivateProjectsQuotaBaselineReadiness(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 5, 6, 10, 0, 0, 0, time.UTC)
	repo := newMemoryIdentityRepository()
	repo.principals["prn_quota"] = activePrincipal("prn_quota", now)
	stateStore := newMemoryStateStore()
	svc := NewService(Dependencies{
		StateStore:       stateStore,
		Identity:         repo,
		Billing:          staticBillingProjector{summary: quotaUsageSummary("ten_personal_prn_quota")},
		Audit:            &fakeAuditSink{},
		Now:              func() time.Time { return now },
		EnvironmentScope: "test",
		Hosted:           true,
	})

	state, err := svc.Activate(ctx, ActivateInput{Token: identity.TokenAuthority{TokenID: "tok_quota", PrincipalID: "prn_quota", Status: identity.StatusActive}})
	if err != nil {
		t.Fatalf("Activate returned error: %v", err)
	}

	if state.Status != StatusActive {
		t.Fatalf("expected active state, got %#v", state)
	}
	if state.QuotaBaseline == nil || state.QuotaBaseline.PlanKey != "hosted-free" || state.QuotaBaseline.Status != QuotaBaselineStatusAvailable {
		t.Fatalf("expected projected quota baseline, got %#v", state.QuotaBaseline)
	}
	if len(state.QuotaBaseline.Quotas) != 1 || state.QuotaBaseline.Quotas[0].Category != string(billing.CategoryRunLaunches) {
		t.Fatalf("expected run launch quota projection, got %#v", state.QuotaBaseline.Quotas)
	}
	if !state.FirstAction.Available || len(state.BlockingReasonCodes) != 0 {
		t.Fatalf("expected available test chat action without blockers, got action=%#v blockers=%#v", state.FirstAction, state.BlockingReasonCodes)
	}
	if !hasReadiness(state, "quota-baseline", ReadinessStatusReady, "") {
		t.Fatalf("expected ready quota readiness item, got %#v", state.ReadinessItems)
	}
}

func TestServiceActivateBlocksWhenQuotaBaselineUnavailable(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 5, 6, 10, 0, 0, 0, time.UTC)
	repo := newMemoryIdentityRepository()
	repo.principals["prn_blocked"] = activePrincipal("prn_blocked", now)
	stateStore := newMemoryStateStore()
	svc := NewService(Dependencies{
		StateStore:       stateStore,
		Identity:         repo,
		Billing:          staticBillingProjector{err: billing.ErrQuotaStateUnavailable},
		Audit:            &fakeAuditSink{},
		Now:              func() time.Time { return now },
		EnvironmentScope: "prod",
		Hosted:           true,
	})

	state, err := svc.Activate(ctx, ActivateInput{Token: identity.TokenAuthority{TokenID: "tok_blocked", PrincipalID: "prn_blocked", Status: identity.StatusActive}})
	if err != nil {
		t.Fatalf("Activate returned error for retryable quota blocker: %v", err)
	}

	if state.Status != StatusBlocked || state.CurrentStepID != StepQuotaBaseline {
		t.Fatalf("expected quota-blocked activation, got %#v", state)
	}
	if state.QuotaBaseline == nil || state.QuotaBaseline.Status != QuotaBaselineStatusUnavailable || state.QuotaBaseline.ReasonCode != ReasonQuotaBaselineUnavailable {
		t.Fatalf("expected unavailable quota baseline, got %#v", state.QuotaBaseline)
	}
	if state.FirstAction.Available || len(state.FirstAction.BlockingItemIDs) != 1 || state.FirstAction.BlockingItemIDs[0] != "quota-baseline" {
		t.Fatalf("expected unavailable test chat action blocked by quota baseline, got %#v", state.FirstAction)
	}
	if state.FailureReason == nil || state.FailureReason.ReasonCode != ReasonQuotaBaselineUnavailable || !state.FailureReason.Retryable {
		t.Fatalf("expected retryable quota failure reason, got %#v", state.FailureReason)
	}
	if !hasReadiness(state, "quota-baseline", ReadinessStatusBlocked, ReasonQuotaBaselineUnavailable) {
		t.Fatalf("expected blocked quota readiness item, got %#v", state.ReadinessItems)
	}
	persisted, ok, err := stateStore.GetActivationStateForPrincipalTenant(ctx, "prn_blocked", state.TenantID)
	if err != nil || !ok {
		t.Fatalf("expected persisted blocked activation, ok=%v err=%v", ok, err)
	}
	if persisted.Status != StatusBlocked {
		t.Fatalf("expected persisted blocked state, got %#v", persisted)
	}
}

func TestServiceActivatePropagatesUnexpectedQuotaProjectionFailures(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 5, 6, 10, 0, 0, 0, time.UTC)
	repo := newMemoryIdentityRepository()
	repo.principals["prn_quota_error"] = activePrincipal("prn_quota_error", now)
	svc := NewService(Dependencies{
		StateStore:       newMemoryStateStore(),
		Identity:         repo,
		Billing:          staticBillingProjector{err: errors.New("billing database unavailable")},
		Audit:            &fakeAuditSink{},
		Now:              func() time.Time { return now },
		EnvironmentScope: "prod",
		Hosted:           true,
	})

	_, err := svc.Activate(ctx, ActivateInput{Token: identity.TokenAuthority{TokenID: "tok_quota_error", PrincipalID: "prn_quota_error", Status: identity.StatusActive}})
	if got := ReasonCodeFromError(err); got != ReasonQuotaBaselineUnavailable {
		t.Fatalf("expected stable quota reason for projection failure, got %q err=%v", got, err)
	}
}

type staticBillingProjector struct {
	summary billing.UsageSummary
	err     error
}

func (p staticBillingProjector) UsageSummary(context.Context, string, bool) (billing.UsageSummary, error) {
	if p.err != nil {
		return billing.UsageSummary{}, p.err
	}
	return p.summary, nil
}

func activePrincipal(principalID string, now time.Time) identity.Principal {
	return identity.Principal{
		PrincipalID:   principalID,
		PrincipalKind: identity.PrincipalKindUser,
		DisplayName:   "Hosted User",
		Status:        identity.StatusActive,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

func quotaUsageSummary(tenantID string) billing.UsageSummary {
	limit := int64(10)
	used := int64(2)
	return billing.UsageSummary{
		TenantID:        tenantID,
		PlanKey:         "hosted-free",
		EnforcementMode: billing.EnforcementModeEnforced,
		Quotas: []billing.EffectiveQuota{
			{
				TenantID:         tenantID,
				PlanKey:          "hosted-free",
				Category:         billing.CategoryRunLaunches,
				Unit:             billing.UnitCount,
				Limit:            limit,
				ConsumedAmount:   used,
				RemainingAmount:  limit - used,
				EnforcementMode:  billing.EnforcementModeEnforced,
				PeriodStart:      time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
				PeriodEnd:        time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
				PeriodAnchor:     billing.PeriodAnchorUTC,
				DenialReasonCode: "quota_denied:run_launches_exhausted",
			},
		},
	}
}

func hasReadiness(state State, itemID string, status ReadinessStatus, reason ReasonCode) bool {
	for _, item := range state.ReadinessItems {
		if item.ItemID != itemID || item.Status != status {
			continue
		}
		if reason != "" && item.ReasonCode != reason {
			continue
		}
		return true
	}
	return false
}
