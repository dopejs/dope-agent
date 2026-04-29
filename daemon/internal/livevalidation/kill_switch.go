package livevalidation

import (
	"context"
	"errors"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/tenantctx"
)

var ErrKillSwitchPermissionDenied = errors.New("live validation kill switch permission denied")

func (m *Manager) SetKillSwitch(ctx context.Context, item KillSwitch) (KillSwitch, error) {
	tenantContext, ok := tenantctx.FromContext(ctx)
	if !ok || !identity.CanResolveLiveValidationReconciliation(tenantContext) {
		return KillSwitch{}, ErrKillSwitchPermissionDenied
	}
	now := m.clock()
	if item.Scope == "" {
		item.Scope = KillSwitchScopeTenant
	}
	if item.Scope == KillSwitchScopeTenant && item.TenantID == "" {
		item.TenantID = tenantContext.TenantID
	}
	if item.KillSwitchID == "" {
		item.KillSwitchID = "live_validation_kill_switch_" + string(item.Scope) + "_" + firstNonEmpty(item.TenantID, "global")
	}
	if item.ChangedBy == "" {
		item.ChangedBy = tenantContext.PrincipalID
	}
	if item.ChangedAt.IsZero() {
		item.ChangedAt = now
	}
	if m.store != nil {
		if err := m.store.UpsertLiveValidationKillSwitch(ctx, item); err != nil {
			return KillSwitch{}, err
		}
	}
	if item.Enabled {
		_ = m.AbortPendingForKillSwitch(ctx, item)
	}
	return item, nil
}

func (m *Manager) ListKillSwitches(ctx context.Context, filter KillSwitchFilter) ([]KillSwitch, error) {
	if m.store == nil {
		return nil, nil
	}
	return m.store.ListLiveValidationKillSwitches(ctx, filter)
}

func (m *Manager) AbortPendingForKillSwitch(ctx context.Context, item KillSwitch) error {
	if m.store == nil {
		return nil
	}
	attempts, err := m.store.ListLiveValidationAttempts(ctx, AttemptFilter{TenantID: item.TenantID})
	if err != nil {
		return err
	}
	now := m.clock()
	for _, attempt := range attempts {
		if item.Scope == KillSwitchScopeTenant && attempt.TenantID != item.TenantID {
			continue
		}
		if attempt.Status != AttemptStatusRunning && attempt.Status != AttemptStatusAwaitingApproval && attempt.Status != AttemptStatusQueued {
			continue
		}
		attempt.Status = AttemptStatusAborted
		attempt.CompletedAt = &now
		attempt.UpdatedAt = now
		if err := m.store.UpsertLiveValidationAttempt(ctx, attempt); err != nil {
			return err
		}
		entries, err := m.store.ListLiveValidationLedgerEntries(ctx, LedgerFilter{TenantID: attempt.TenantID, ValidationID: attempt.ValidationID})
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if IsTerminalLedgerOutcome(entry.Outcome) {
				continue
			}
			if err := m.store.UpdateLiveValidationLedgerEntryOutcome(ctx, entry.LedgerEntryID, LedgerOutcomeAborted, "live_validation.kill_switch_aborted"); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *Manager) evaluateKillSwitch(ctx context.Context, tenantID string, now time.Time) (GateDecision, Denial, error) {
	if m.store == nil {
		return GateDecision{Allowed: true, CheckedAt: now}, Denial{}, nil
	}
	enabled := true
	switches, err := m.store.ListLiveValidationKillSwitches(ctx, KillSwitchFilter{Enabled: &enabled})
	if err != nil {
		return GateDecision{}, Denial{}, err
	}
	for _, item := range switches {
		if item.ExpiresAt != nil && item.ExpiresAt.Before(now) {
			continue
		}
		if item.Scope == KillSwitchScopeGlobal || (item.Scope == KillSwitchScopeTenant && item.TenantID == tenantID) {
			reason := firstNonEmpty(item.Reason, "live validation kill switch enabled")
			return GateDecision{Allowed: false, ReasonCode: "live_validation.kill_switch_enabled", Reference: item.KillSwitchID, CheckedAt: now}, Denial{
				Gate:       "kill_switch",
				ReasonCode: "live_validation.kill_switch_enabled",
				Message:    reason,
				Reference:  item.KillSwitchID,
			}, nil
		}
	}
	return GateDecision{Allowed: true, CheckedAt: now}, Denial{}, nil
}
