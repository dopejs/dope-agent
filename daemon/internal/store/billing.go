package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/billing"
)

func (s *SQLiteStore) ActivePlan(ctx context.Context, tenantID string) (billing.TenantPlan, bool, error) {
	if s == nil {
		return billing.TenantPlan{}, false, nil
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT plan_id, tenant_id, plan_key, status, enforcement_mode, effective_at, superseded_at,
		       assigned_by_principal_id, assignment_reason, document_json
		FROM billing_tenant_plans
		WHERE tenant_id = ? AND status = ? AND effective_at <= ?
		ORDER BY effective_at DESC, plan_id DESC
		LIMIT 1
	`, tenantID, billing.PlanStatusActive, time.Now().UTC().Format(time.RFC3339Nano))
	plan, err := scanBillingTenantPlan(row)
	if err == sql.ErrNoRows {
		return billing.TenantPlan{}, false, nil
	}
	if err != nil {
		return billing.TenantPlan{}, false, err
	}
	return plan, true, nil
}

func (s *SQLiteStore) SavePlan(ctx context.Context, plan billing.TenantPlan) error {
	if s == nil {
		return nil
	}
	if plan.PlanID == "" {
		plan.PlanID = newBillingID("plan")
	}
	if plan.Status == "" {
		plan.Status = billing.PlanStatusActive
	}
	if plan.EnforcementMode == "" {
		plan.EnforcementMode = billing.EnforcementModeEnforced
	}
	if plan.EffectiveAt.IsZero() {
		plan.EffectiveAt = time.Now().UTC()
	}
	documentJSON, err := marshalJSON(plan.Document)
	if err != nil {
		return fmt.Errorf("encode billing tenant plan %s document: %w", plan.PlanID, err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin billing plan save: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if plan.Status == billing.PlanStatusActive {
		if _, err := tx.ExecContext(ctx, `
			UPDATE billing_tenant_plans
			SET status = ?, superseded_at = ?
			WHERE tenant_id = ? AND status = ? AND plan_id <> ?
		`, billing.PlanStatusSuperseded, plan.EffectiveAt.UTC().Format(time.RFC3339Nano), plan.TenantID, billing.PlanStatusActive, plan.PlanID); err != nil {
			return fmt.Errorf("supersede billing tenant plans for tenant %s: %w", plan.TenantID, err)
		}
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO billing_tenant_plans (
			plan_id, tenant_id, plan_key, status, enforcement_mode, effective_at, superseded_at,
			assigned_by_principal_id, assignment_reason, document_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(plan_id) DO UPDATE SET
			tenant_id = excluded.tenant_id,
			plan_key = excluded.plan_key,
			status = excluded.status,
			enforcement_mode = excluded.enforcement_mode,
			effective_at = excluded.effective_at,
			superseded_at = excluded.superseded_at,
			assigned_by_principal_id = excluded.assigned_by_principal_id,
			assignment_reason = excluded.assignment_reason,
			document_json = excluded.document_json
	`, plan.PlanID, plan.TenantID, plan.PlanKey, plan.Status, plan.EnforcementMode,
		plan.EffectiveAt.UTC().Format(time.RFC3339Nano), nullableTimeString(plan.SupersededAt),
		nullString(plan.AssignedByPrincipalID), nullString(plan.AssignmentReason), documentJSON); err != nil {
		return fmt.Errorf("save billing tenant plan %s: %w", plan.PlanID, err)
	}
	return tx.Commit()
}

func (s *SQLiteStore) SaveQuotaDefinition(ctx context.Context, definition billing.QuotaDefinition) error {
	if s == nil {
		return nil
	}
	if definition.QuotaDefinitionID == "" {
		definition.QuotaDefinitionID = "quota_definition_" + string(definition.Category)
	}
	if definition.CreatedAt.IsZero() {
		definition.CreatedAt = time.Now().UTC()
	}
	if definition.UpdatedAt.IsZero() {
		definition.UpdatedAt = definition.CreatedAt
	}
	documentJSON, err := marshalJSON(definition.Document)
	if err != nil {
		return fmt.Errorf("encode billing quota definition %s document: %w", definition.QuotaDefinitionID, err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO billing_quota_definitions (
			quota_definition_id, category, unit, period_kind, period_anchor, default_limit,
			carryover_enabled, carryover_max, reservation_rule, commit_rule, refund_rule,
			denial_reason_code, active, created_at, updated_at, document_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(category) DO UPDATE SET
			unit = excluded.unit,
			period_kind = excluded.period_kind,
			period_anchor = excluded.period_anchor,
			default_limit = excluded.default_limit,
			carryover_enabled = excluded.carryover_enabled,
			carryover_max = excluded.carryover_max,
			reservation_rule = excluded.reservation_rule,
			commit_rule = excluded.commit_rule,
			refund_rule = excluded.refund_rule,
			denial_reason_code = excluded.denial_reason_code,
			active = excluded.active,
			updated_at = excluded.updated_at,
			document_json = excluded.document_json
	`, definition.QuotaDefinitionID, definition.Category, definition.Unit, definition.PeriodKind,
		definition.PeriodAnchor, definition.DefaultLimit, boolToInt(definition.CarryoverEnabled),
		definition.CarryoverMax, definition.ReservationRule, definition.CommitRule, definition.RefundRule,
		definition.DenialReasonCode, boolToInt(definition.Active), definition.CreatedAt.UTC().Format(time.RFC3339Nano),
		definition.UpdatedAt.UTC().Format(time.RFC3339Nano), documentJSON)
	if err != nil {
		return fmt.Errorf("save billing quota definition %s: %w", definition.QuotaDefinitionID, err)
	}
	return nil
}

func (s *SQLiteStore) QuotaOverride(ctx context.Context, tenantID string, category billing.Category, at time.Time) (*billing.QuotaOverride, error) {
	if s == nil {
		return nil, nil
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT quota_override_id, tenant_id, category, limit_amount, carryover_enabled, carryover_max,
		       effective_at, expires_at, reason, created_by_principal_id
		FROM billing_quota_overrides
		WHERE tenant_id = ? AND category = ? AND effective_at <= ? AND (expires_at IS NULL OR expires_at > ?)
		ORDER BY effective_at DESC, quota_override_id DESC
		LIMIT 1
	`, tenantID, category, at.UTC().Format(time.RFC3339Nano), at.UTC().Format(time.RFC3339Nano))
	override, err := scanBillingQuotaOverride(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &override, nil
}

func (s *SQLiteStore) SaveQuotaOverride(ctx context.Context, override billing.QuotaOverride) error {
	if s == nil {
		return nil
	}
	if override.QuotaOverrideID == "" {
		override.QuotaOverrideID = newBillingID("quota_override")
	}
	if override.EffectiveAt.IsZero() {
		override.EffectiveAt = time.Now().UTC()
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO billing_quota_overrides (
			quota_override_id, tenant_id, category, limit_amount, carryover_enabled, carryover_max,
			effective_at, expires_at, reason, created_by_principal_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(quota_override_id) DO UPDATE SET
			tenant_id = excluded.tenant_id,
			category = excluded.category,
			limit_amount = excluded.limit_amount,
			carryover_enabled = excluded.carryover_enabled,
			carryover_max = excluded.carryover_max,
			effective_at = excluded.effective_at,
			expires_at = excluded.expires_at,
			reason = excluded.reason,
			created_by_principal_id = excluded.created_by_principal_id
	`, override.QuotaOverrideID, override.TenantID, override.Category, nullableInt64(override.Limit),
		nullableBoolInt(override.CarryoverEnabled), nullableInt64(override.CarryoverMax),
		override.EffectiveAt.UTC().Format(time.RFC3339Nano), nullableTimeString(override.ExpiresAt),
		override.Reason, nullString(override.CreatedByPrincipalID))
	if err != nil {
		return fmt.Errorf("save billing quota override %s: %w", override.QuotaOverrideID, err)
	}
	return nil
}

func (s *SQLiteStore) ReserveUsage(ctx context.Context, input billing.ReserveInput, now time.Time) (billing.ReserveResult, error) {
	if s == nil {
		if input.Hosted {
			operationKey := strings.TrimSpace(input.OperationKey)
			denial := billing.NewQuotaStateUnavailableDenial(input.TenantID, operationKey).Payload
			return billing.ReserveResult{Allowed: false, Denial: &denial}, billing.ErrQuotaStateUnavailable
		}
		return billing.ReserveResult{Allowed: true}, nil
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	now = now.UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return billing.ReserveResult{}, fmt.Errorf("begin billing reserve transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	result, err := reserveUsageTx(ctx, tx, input, now)
	if err != nil {
		return result, err
	}
	if err := tx.Commit(); err != nil {
		return billing.ReserveResult{}, fmt.Errorf("commit billing reserve transaction: %w", err)
	}
	return result, nil
}

func (s *SQLiteStore) ReserveAllUsage(ctx context.Context, inputs []billing.ReserveInput, now time.Time) (billing.ReserveAllResult, error) {
	if s == nil {
		return billing.ReserveAllResult{Allowed: true}, nil
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	now = now.UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return billing.ReserveAllResult{}, fmt.Errorf("begin billing reserve-all transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `SAVEPOINT billing_reserve_all`); err != nil {
		return billing.ReserveAllResult{}, fmt.Errorf("create billing reserve-all savepoint: %w", err)
	}
	results := make([]billing.ReserveResult, 0, len(inputs))
	for _, input := range inputs {
		result, err := reserveUsageTx(ctx, tx, input, now)
		results = append(results, result)
		if err == nil && result.Allowed {
			continue
		}
		if _, rollbackErr := tx.ExecContext(ctx, `ROLLBACK TO billing_reserve_all`); rollbackErr != nil {
			return billing.ReserveAllResult{}, fmt.Errorf("rollback billing reserve-all savepoint: %w", rollbackErr)
		}
		if _, releaseErr := tx.ExecContext(ctx, `RELEASE billing_reserve_all`); releaseErr != nil {
			return billing.ReserveAllResult{}, fmt.Errorf("release billing reserve-all savepoint: %w", releaseErr)
		}
		if errors.Is(err, billing.ErrQuotaDenied) || errors.Is(err, billing.ErrOperatorActionRequired) {
			deniedResult, deniedErr := reserveUsageTx(ctx, tx, input, now)
			if deniedErr != nil {
				if commitErr := tx.Commit(); commitErr != nil {
					return billing.ReserveAllResult{}, fmt.Errorf("commit denied billing reserve-all transaction: %w", commitErr)
				}
				return billing.ReserveAllResult{Allowed: false, Results: []billing.ReserveResult{deniedResult}, Denial: deniedResult.Denial}, deniedErr
			}
			result = deniedResult
		}
		if commitErr := tx.Commit(); commitErr != nil {
			return billing.ReserveAllResult{}, fmt.Errorf("commit billing reserve-all denial transaction: %w", commitErr)
		}
		return billing.ReserveAllResult{Allowed: false, Results: []billing.ReserveResult{result}, Denial: result.Denial}, err
	}
	if _, err := tx.ExecContext(ctx, `RELEASE billing_reserve_all`); err != nil {
		return billing.ReserveAllResult{}, fmt.Errorf("release billing reserve-all savepoint: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return billing.ReserveAllResult{}, fmt.Errorf("commit billing reserve-all transaction: %w", err)
	}
	return billing.ReserveAllResult{Allowed: true, Results: results}, nil
}

func reserveUsageTx(ctx context.Context, tx *sql.Tx, input billing.ReserveInput, now time.Time) (billing.ReserveResult, error) {
	amount := input.Amount
	if amount <= 0 {
		amount = 1
	}
	definition, ok := billing.DefinitionFor(input.Category)
	if !ok {
		return billing.ReserveResult{}, fmt.Errorf("unknown quota category %q", input.Category)
	}
	operationKey := strings.TrimSpace(input.OperationKey)
	plan, ok, err := activePlanTx(ctx, tx, input.TenantID, now)
	if err != nil {
		return billing.ReserveResult{}, err
	}
	if !ok {
		if input.Hosted {
			denial := billing.NewQuotaStateUnavailableDenial(input.TenantID, operationKey).Payload
			return billing.ReserveResult{Allowed: false, Denial: &denial}, billing.ErrQuotaStateUnavailable
		}
		plan = billing.DevelopmentPlan(input.TenantID, now)
	}
	if plan.EnforcementMode == billing.EnforcementModeUnlimited {
		return billing.ReserveResult{Allowed: true}, nil
	}
	period, err := openPeriodTx(ctx, tx, input.TenantID, definition, now)
	if err != nil {
		if input.Hosted {
			denial := billing.NewQuotaStateUnavailableDenial(input.TenantID, operationKey).Payload
			return billing.ReserveResult{Allowed: false, Denial: &denial}, billing.ErrQuotaStateUnavailable
		}
		return billing.ReserveResult{}, err
	}
	if existing, ok, err := reservationByOperationTx(ctx, tx, input.TenantID, input.Category, operationKey); err != nil {
		return billing.ReserveResult{}, err
	} else if ok {
		if existing.Status == billing.ReservationStatusOperatorActionNeeded {
			denial := billing.NewQuotaStateUnavailableDenial(input.TenantID, operationKey).Payload
			return billing.ReserveResult{Allowed: false, Reservation: existing, Denial: &denial}, billing.ErrOperatorActionRequired
		}
		if existing.Status == billing.ReservationStatusDenied {
			denialErr := billing.NewQuotaExhaustedDenial(input.TenantID, input.Category, operationKey, amount, 0, period)
			payload := denialErr.Payload
			return billing.ReserveResult{Allowed: false, Reservation: existing, Denial: &payload}, billing.ErrQuotaDenied
		}
		return billing.ReserveResult{Allowed: existing.Status != billing.ReservationStatusDenied, Reservation: existing}, nil
	}
	counter, ok, err := usageCounterTx(ctx, tx, input.TenantID, input.Category, period.QuotaPeriodID)
	if err != nil {
		return billing.ReserveResult{}, err
	}
	if !ok {
		counter = billing.UsageCounter{
			UsageCounterID: "usage_counter_" + input.TenantID + "_" + string(input.Category) + "_" + period.QuotaPeriodID,
			TenantID:       input.TenantID,
			Category:       input.Category,
			QuotaPeriodID:  period.QuotaPeriodID,
			UpdatedAt:      now,
		}
	}
	override, err := quotaOverrideTx(ctx, tx, input.TenantID, input.Category, now)
	if err != nil {
		return billing.ReserveResult{}, err
	}
	quota := billing.ProjectQuota(plan, definition, period, counter, override)
	if quota.RemainingAmount < amount {
		denialErr := billing.NewQuotaExhaustedDenial(input.TenantID, input.Category, operationKey, amount, quota.RemainingAmount, period)
		reservation := billing.UsageReservation{
			ReservationID:    "reservation_" + operationKey,
			TenantID:         input.TenantID,
			Category:         input.Category,
			QuotaPeriodID:    period.QuotaPeriodID,
			OperationKey:     operationKey,
			AmountReserved:   amount,
			Status:           billing.ReservationStatusDenied,
			ReservationPoint: input.ReservationPoint,
			CreatedAt:        now,
			UpdatedAt:        now,
		}
		if err := saveReservationTx(ctx, tx, reservation); err != nil {
			return billing.ReserveResult{}, err
		}
		denial := billing.QuotaDenial{
			DenialID:          "denial_" + operationKey,
			TenantID:          input.TenantID,
			Category:          input.Category,
			QuotaPeriodID:     period.QuotaPeriodID,
			OperationKey:      operationKey,
			ReasonCode:        denialErr.Payload.ReasonCode,
			RequestedAmount:   amount,
			RemainingAmount:   quota.RemainingAmount,
			GuardedEntryPoint: input.GuardedEntryPoint,
			CreatedAt:         now,
		}
		if err := appendQuotaDenialTx(ctx, tx, denial); err != nil {
			return billing.ReserveResult{}, err
		}
		_ = appendUsageEventTx(ctx, tx, billing.UsageEvent{
			UsageEventID:  "usage_event_denial_" + operationKey,
			TenantID:      input.TenantID,
			Category:      input.Category,
			QuotaPeriodID: period.QuotaPeriodID,
			OperationKey:  operationKey,
			EventKind:     billing.UsageEventDenial,
			Amount:        amount,
			ReasonCode:    denial.ReasonCode,
			Outcome:       "denied",
			CreatedAt:     now,
		})
		payload := denialErr.Payload
		return billing.ReserveResult{Allowed: false, Reservation: reservation, Denial: &payload, Quota: quota}, billing.ErrQuotaDenied
	}
	reservation := billing.UsageReservation{
		ReservationID:    "reservation_" + operationKey,
		TenantID:         input.TenantID,
		Category:         input.Category,
		QuotaPeriodID:    period.QuotaPeriodID,
		OperationKey:     operationKey,
		AmountReserved:   amount,
		Status:           billing.ReservationStatusReserved,
		ReservationPoint: input.ReservationPoint,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	counter.ReservedAmount += amount
	counter.UpdatedAt = now
	if err := saveUsageCounterTx(ctx, tx, counter); err != nil {
		return billing.ReserveResult{}, err
	}
	if err := saveReservationTx(ctx, tx, reservation); err != nil {
		return billing.ReserveResult{}, err
	}
	if err := appendUsageEventTx(ctx, tx, billing.UsageEvent{
		UsageEventID:     "usage_event_reserved_" + operationKey,
		TenantID:         input.TenantID,
		Category:         input.Category,
		QuotaPeriodID:    period.QuotaPeriodID,
		OperationKey:     operationKey,
		EventKind:        billing.UsageEventReservation,
		Amount:           amount,
		ReasonCode:       "usage_reserved",
		ActorPrincipalID: input.ActorPrincipalID,
		Outcome:          "reserved",
		CreatedAt:        now,
	}); err != nil {
		return billing.ReserveResult{}, err
	}
	return billing.ReserveResult{Allowed: true, Reservation: reservation, Quota: quota}, nil
}

func (s *SQLiteStore) ResolveUsage(ctx context.Context, input billing.ResolveInput, status billing.ReservationStatus, eventKind billing.UsageEventKind, now time.Time) (billing.UsageReservation, error) {
	if s == nil {
		return billing.UsageReservation{}, nil
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	now = now.UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return billing.UsageReservation{}, fmt.Errorf("begin billing resolve transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	reservation, err := resolveUsageTx(ctx, tx, input, status, eventKind, now)
	if err != nil {
		return billing.UsageReservation{}, err
	}
	if err := tx.Commit(); err != nil {
		return billing.UsageReservation{}, fmt.Errorf("commit billing resolve transaction: %w", err)
	}
	return reservation, nil
}

func resolveUsageTx(ctx context.Context, tx *sql.Tx, input billing.ResolveInput, status billing.ReservationStatus, eventKind billing.UsageEventKind, now time.Time) (billing.UsageReservation, error) {
	reservation, ok, err := reservationByOperationTx(ctx, tx, input.TenantID, input.Category, input.OperationKey)
	if err != nil {
		return billing.UsageReservation{}, err
	}
	if !ok {
		return billing.UsageReservation{}, fmt.Errorf("%w for %s", billing.ErrReservationNotFound, input.OperationKey)
	}
	if reservation.Status == status {
		return reservation, nil
	}
	counter, ok, err := usageCounterTx(ctx, tx, input.TenantID, input.Category, reservation.QuotaPeriodID)
	if err != nil {
		return billing.UsageReservation{}, err
	}
	if !ok {
		return billing.UsageReservation{}, fmt.Errorf("counter not found for reservation %s", reservation.ReservationID)
	}
	amount := input.Amount
	if amount <= 0 {
		amount = reservation.AmountReserved
	}
	switch status {
	case billing.ReservationStatusCommitted:
		delta := amount - reservation.AmountCommitted
		reservedRelease := minInt64(counter.ReservedAmount, reservation.AmountReserved-reservation.AmountRefunded)
		counter.ReservedAmount -= reservedRelease
		counter.CommittedAmount += delta
		reservation.AmountCommitted = amount
		if amount < reservation.AmountReserved {
			reservation.AmountRefunded += reservation.AmountReserved - amount
		}
	case billing.ReservationStatusRefunded, billing.ReservationStatusReleased:
		refund := minInt64(counter.ReservedAmount, reservation.AmountReserved-reservation.AmountRefunded)
		counter.ReservedAmount -= refund
		reservation.AmountRefunded += refund
	case billing.ReservationStatusOperatorActionNeeded:
		reservation.RecoveryReason = input.Reason
	}
	counter.UpdatedAt = now
	reservation.Status = status
	reservation.UpdatedAt = now
	if err := saveUsageCounterTx(ctx, tx, counter); err != nil {
		return billing.UsageReservation{}, err
	}
	if err := saveReservationTx(ctx, tx, reservation); err != nil {
		return billing.UsageReservation{}, err
	}
	reasonCode := input.ReasonCode
	if reasonCode == "" {
		reasonCode = string(eventKind)
	}
	if err := appendUsageEventTx(ctx, tx, billing.UsageEvent{
		UsageEventID:     fmt.Sprintf("usage_event_%s_%s", eventKind, input.OperationKey),
		TenantID:         input.TenantID,
		Category:         input.Category,
		QuotaPeriodID:    reservation.QuotaPeriodID,
		OperationKey:     input.OperationKey,
		EventKind:        eventKind,
		Amount:           amount,
		ReasonCode:       reasonCode,
		Reason:           input.Reason,
		ActorPrincipalID: input.ActorPrincipalID,
		Outcome:          string(status),
		CreatedAt:        now,
	}); err != nil {
		return billing.UsageReservation{}, err
	}
	return reservation, nil
}

func activePlanTx(ctx context.Context, tx *sql.Tx, tenantID string, now time.Time) (billing.TenantPlan, bool, error) {
	row := tx.QueryRowContext(ctx, `
		SELECT plan_id, tenant_id, plan_key, status, enforcement_mode, effective_at, superseded_at,
		       assigned_by_principal_id, assignment_reason, document_json
		FROM billing_tenant_plans
		WHERE tenant_id = ? AND status = ? AND effective_at <= ?
		ORDER BY effective_at DESC, plan_id DESC
		LIMIT 1
	`, tenantID, billing.PlanStatusActive, now.UTC().Format(time.RFC3339Nano))
	plan, err := scanBillingTenantPlan(row)
	if err == sql.ErrNoRows {
		return billing.TenantPlan{}, false, nil
	}
	if err != nil {
		return billing.TenantPlan{}, false, err
	}
	return plan, true, nil
}

func openPeriodTx(ctx context.Context, tx *sql.Tx, tenantID string, definition billing.QuotaDefinition, at time.Time) (billing.QuotaPeriod, error) {
	start, end := billing.PeriodFor(definition.PeriodKind, at)
	periodID := "quota_period_" + tenantID + "_" + string(definition.Category) + "_" + start.Format("20060102")
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO billing_quota_periods (
			quota_period_id, tenant_id, category, period_kind, period_start, period_end,
			carryover_from_period_id, status
		) VALUES (?, ?, ?, ?, ?, ?, NULL, ?)
		ON CONFLICT(tenant_id, category, period_start) DO NOTHING
	`, periodID, tenantID, definition.Category, definition.PeriodKind,
		start.UTC().Format(time.RFC3339Nano), end.UTC().Format(time.RFC3339Nano), "open"); err != nil {
		return billing.QuotaPeriod{}, fmt.Errorf("open billing quota period %s: %w", periodID, err)
	}
	row := tx.QueryRowContext(ctx, `
		SELECT quota_period_id, tenant_id, category, period_kind, period_start, period_end,
		       carryover_from_period_id, status
		FROM billing_quota_periods
		WHERE tenant_id = ? AND category = ? AND period_start = ?
	`, tenantID, definition.Category, start.UTC().Format(time.RFC3339Nano))
	return scanBillingQuotaPeriod(row)
}

func reservationByOperationTx(ctx context.Context, tx *sql.Tx, tenantID string, category billing.Category, operationKey string) (billing.UsageReservation, bool, error) {
	row := tx.QueryRowContext(ctx, `
		SELECT reservation_id, tenant_id, category, quota_period_id, operation_key, amount_reserved,
		       amount_committed, amount_refunded, status, reservation_point, commit_point, refund_point,
		       created_at, updated_at, expires_at, recovery_reason
		FROM billing_usage_reservations
		WHERE tenant_id = ? AND category = ? AND operation_key = ?
	`, tenantID, category, operationKey)
	reservation, err := scanBillingUsageReservation(row)
	if err == sql.ErrNoRows {
		return billing.UsageReservation{}, false, nil
	}
	if err != nil {
		return billing.UsageReservation{}, false, err
	}
	return reservation, true, nil
}

func usageCounterTx(ctx context.Context, tx *sql.Tx, tenantID string, category billing.Category, quotaPeriodID string) (billing.UsageCounter, bool, error) {
	row := tx.QueryRowContext(ctx, `
		SELECT usage_counter_id, tenant_id, category, quota_period_id, committed_amount, reserved_amount,
		       adjusted_amount, carryover_amount, updated_at
		FROM billing_usage_counters
		WHERE tenant_id = ? AND category = ? AND quota_period_id = ?
	`, tenantID, category, quotaPeriodID)
	counter, err := scanBillingUsageCounter(row)
	if err == sql.ErrNoRows {
		return billing.UsageCounter{}, false, nil
	}
	if err != nil {
		return billing.UsageCounter{}, false, err
	}
	return counter, true, nil
}

func quotaOverrideTx(ctx context.Context, tx *sql.Tx, tenantID string, category billing.Category, at time.Time) (*billing.QuotaOverride, error) {
	row := tx.QueryRowContext(ctx, `
		SELECT quota_override_id, tenant_id, category, limit_amount, carryover_enabled, carryover_max,
		       effective_at, expires_at, reason, created_by_principal_id
		FROM billing_quota_overrides
		WHERE tenant_id = ? AND category = ? AND effective_at <= ? AND (expires_at IS NULL OR expires_at > ?)
		ORDER BY effective_at DESC, quota_override_id DESC
		LIMIT 1
	`, tenantID, category, at.UTC().Format(time.RFC3339Nano), at.UTC().Format(time.RFC3339Nano))
	override, err := scanBillingQuotaOverride(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &override, nil
}

func saveUsageCounterTx(ctx context.Context, tx *sql.Tx, counter billing.UsageCounter) error {
	if counter.UsageCounterID == "" {
		counter.UsageCounterID = newBillingID("usage_counter")
	}
	if counter.UpdatedAt.IsZero() {
		counter.UpdatedAt = time.Now().UTC()
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO billing_usage_counters (
			usage_counter_id, tenant_id, category, quota_period_id, committed_amount, reserved_amount,
			adjusted_amount, carryover_amount, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(tenant_id, category, quota_period_id) DO UPDATE SET
			committed_amount = excluded.committed_amount,
			reserved_amount = excluded.reserved_amount,
			adjusted_amount = excluded.adjusted_amount,
			carryover_amount = excluded.carryover_amount,
			updated_at = excluded.updated_at
	`, counter.UsageCounterID, counter.TenantID, counter.Category, counter.QuotaPeriodID,
		counter.CommittedAmount, counter.ReservedAmount, counter.AdjustedAmount, counter.CarryoverAmount,
		counter.UpdatedAt.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("save billing usage counter %s: %w", counter.UsageCounterID, err)
	}
	return nil
}

func saveReservationTx(ctx context.Context, tx *sql.Tx, reservation billing.UsageReservation) error {
	if reservation.ReservationID == "" {
		reservation.ReservationID = newBillingID("reservation")
	}
	now := time.Now().UTC()
	if reservation.CreatedAt.IsZero() {
		reservation.CreatedAt = now
	}
	if reservation.UpdatedAt.IsZero() {
		reservation.UpdatedAt = now
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO billing_usage_reservations (
			reservation_id, tenant_id, category, quota_period_id, operation_key, amount_reserved,
			amount_committed, amount_refunded, status, reservation_point, commit_point, refund_point,
			created_at, updated_at, expires_at, recovery_reason
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(tenant_id, category, operation_key) DO UPDATE SET
			quota_period_id = excluded.quota_period_id,
			amount_reserved = excluded.amount_reserved,
			amount_committed = excluded.amount_committed,
			amount_refunded = excluded.amount_refunded,
			status = excluded.status,
			reservation_point = excluded.reservation_point,
			commit_point = excluded.commit_point,
			refund_point = excluded.refund_point,
			updated_at = excluded.updated_at,
			expires_at = excluded.expires_at,
			recovery_reason = excluded.recovery_reason
	`, reservation.ReservationID, reservation.TenantID, reservation.Category, reservation.QuotaPeriodID,
		reservation.OperationKey, reservation.AmountReserved, reservation.AmountCommitted, reservation.AmountRefunded,
		reservation.Status, nullString(reservation.ReservationPoint), nullString(reservation.CommitPoint),
		nullString(reservation.RefundPoint), reservation.CreatedAt.UTC().Format(time.RFC3339Nano),
		reservation.UpdatedAt.UTC().Format(time.RFC3339Nano), nullableTimeString(reservation.ExpiresAt),
		nullString(reservation.RecoveryReason))
	if err != nil {
		return fmt.Errorf("save billing usage reservation %s: %w", reservation.ReservationID, err)
	}
	return nil
}

func appendUsageEventTx(ctx context.Context, tx *sql.Tx, event billing.UsageEvent) error {
	if event.UsageEventID == "" {
		event.UsageEventID = newBillingID("usage_event")
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	documentJSON, err := marshalJSON(event.Document)
	if err != nil {
		return fmt.Errorf("encode billing usage event %s document: %w", event.UsageEventID, err)
	}
	_, err = tx.ExecContext(ctx, `
		INSERT OR IGNORE INTO billing_usage_events (
			usage_event_id, tenant_id, category, quota_period_id, operation_key, event_kind,
			amount, reason_code, reason, actor_principal_id, outcome, created_at, document_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, event.UsageEventID, event.TenantID, nullString(string(event.Category)), nullString(event.QuotaPeriodID),
		nullString(event.OperationKey), event.EventKind, event.Amount, event.ReasonCode, nullString(event.Reason),
		nullString(event.ActorPrincipalID), event.Outcome, event.CreatedAt.UTC().Format(time.RFC3339Nano), documentJSON)
	if err != nil {
		return fmt.Errorf("append billing usage event %s: %w", event.UsageEventID, err)
	}
	return nil
}

func appendQuotaDenialTx(ctx context.Context, tx *sql.Tx, denial billing.QuotaDenial) error {
	if denial.DenialID == "" {
		denial.DenialID = newBillingID("denial")
	}
	if denial.CreatedAt.IsZero() {
		denial.CreatedAt = time.Now().UTC()
	}
	_, err := tx.ExecContext(ctx, `
		INSERT OR IGNORE INTO billing_quota_denials (
			denial_id, tenant_id, category, quota_period_id, operation_key, reason_code,
			requested_amount, remaining_amount, guarded_entry_point, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, denial.DenialID, denial.TenantID, nullString(string(denial.Category)), nullString(denial.QuotaPeriodID),
		denial.OperationKey, denial.ReasonCode, denial.RequestedAmount, denial.RemainingAmount,
		denial.GuardedEntryPoint, denial.CreatedAt.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("append billing quota denial %s: %w", denial.DenialID, err)
	}
	return nil
}

func (s *SQLiteStore) OpenPeriod(ctx context.Context, tenantID string, definition billing.QuotaDefinition, at time.Time) (billing.QuotaPeriod, error) {
	if s == nil {
		return billing.QuotaPeriod{}, nil
	}
	start, end := billing.PeriodFor(definition.PeriodKind, at)
	periodID := "quota_period_" + tenantID + "_" + string(definition.Category) + "_" + start.Format("20060102")
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO billing_quota_periods (
			quota_period_id, tenant_id, category, period_kind, period_start, period_end,
			carryover_from_period_id, status
		) VALUES (?, ?, ?, ?, ?, ?, NULL, ?)
		ON CONFLICT(tenant_id, category, period_start) DO NOTHING
	`, periodID, tenantID, definition.Category, definition.PeriodKind,
		start.UTC().Format(time.RFC3339Nano), end.UTC().Format(time.RFC3339Nano), "open")
	if err != nil {
		return billing.QuotaPeriod{}, fmt.Errorf("open billing quota period %s: %w", periodID, err)
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT quota_period_id, tenant_id, category, period_kind, period_start, period_end,
		       carryover_from_period_id, status
		FROM billing_quota_periods
		WHERE tenant_id = ? AND category = ? AND period_start = ?
	`, tenantID, definition.Category, start.UTC().Format(time.RFC3339Nano))
	return scanBillingQuotaPeriod(row)
}

func (s *SQLiteStore) UsageCounter(ctx context.Context, tenantID string, category billing.Category, quotaPeriodID string) (billing.UsageCounter, bool, error) {
	if s == nil {
		return billing.UsageCounter{}, false, nil
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT usage_counter_id, tenant_id, category, quota_period_id, committed_amount, reserved_amount,
		       adjusted_amount, carryover_amount, updated_at
		FROM billing_usage_counters
		WHERE tenant_id = ? AND category = ? AND quota_period_id = ?
	`, tenantID, category, quotaPeriodID)
	counter, err := scanBillingUsageCounter(row)
	if err == sql.ErrNoRows {
		return billing.UsageCounter{}, false, nil
	}
	if err != nil {
		return billing.UsageCounter{}, false, err
	}
	return counter, true, nil
}

func (s *SQLiteStore) SaveUsageCounter(ctx context.Context, counter billing.UsageCounter) error {
	if s == nil {
		return nil
	}
	if counter.UsageCounterID == "" {
		counter.UsageCounterID = newBillingID("usage_counter")
	}
	if counter.UpdatedAt.IsZero() {
		counter.UpdatedAt = time.Now().UTC()
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO billing_usage_counters (
			usage_counter_id, tenant_id, category, quota_period_id, committed_amount, reserved_amount,
			adjusted_amount, carryover_amount, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(tenant_id, category, quota_period_id) DO UPDATE SET
			committed_amount = excluded.committed_amount,
			reserved_amount = excluded.reserved_amount,
			adjusted_amount = excluded.adjusted_amount,
			carryover_amount = excluded.carryover_amount,
			updated_at = excluded.updated_at
	`, counter.UsageCounterID, counter.TenantID, counter.Category, counter.QuotaPeriodID,
		counter.CommittedAmount, counter.ReservedAmount, counter.AdjustedAmount, counter.CarryoverAmount,
		counter.UpdatedAt.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("save billing usage counter %s: %w", counter.UsageCounterID, err)
	}
	return nil
}

func (s *SQLiteStore) ReservationByOperation(ctx context.Context, tenantID string, category billing.Category, operationKey string) (billing.UsageReservation, bool, error) {
	if s == nil {
		return billing.UsageReservation{}, false, nil
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT reservation_id, tenant_id, category, quota_period_id, operation_key, amount_reserved,
		       amount_committed, amount_refunded, status, reservation_point, commit_point, refund_point,
		       created_at, updated_at, expires_at, recovery_reason
		FROM billing_usage_reservations
		WHERE tenant_id = ? AND category = ? AND operation_key = ?
	`, tenantID, category, operationKey)
	reservation, err := scanBillingUsageReservation(row)
	if err == sql.ErrNoRows {
		return billing.UsageReservation{}, false, nil
	}
	if err != nil {
		return billing.UsageReservation{}, false, err
	}
	return reservation, true, nil
}

func (s *SQLiteStore) ReservationByID(ctx context.Context, tenantID string, reservationID string) (billing.UsageReservation, bool, error) {
	if s == nil {
		return billing.UsageReservation{}, false, nil
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT reservation_id, tenant_id, category, quota_period_id, operation_key, amount_reserved,
		       amount_committed, amount_refunded, status, reservation_point, commit_point, refund_point,
		       created_at, updated_at, expires_at, recovery_reason
		FROM billing_usage_reservations
		WHERE tenant_id = ? AND reservation_id = ?
	`, tenantID, reservationID)
	reservation, err := scanBillingUsageReservation(row)
	if err == sql.ErrNoRows {
		return billing.UsageReservation{}, false, nil
	}
	if err != nil {
		return billing.UsageReservation{}, false, err
	}
	return reservation, true, nil
}

func (s *SQLiteStore) SaveReservation(ctx context.Context, reservation billing.UsageReservation) error {
	if s == nil {
		return nil
	}
	if reservation.ReservationID == "" {
		reservation.ReservationID = newBillingID("reservation")
	}
	now := time.Now().UTC()
	if reservation.CreatedAt.IsZero() {
		reservation.CreatedAt = now
	}
	if reservation.UpdatedAt.IsZero() {
		reservation.UpdatedAt = now
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO billing_usage_reservations (
			reservation_id, tenant_id, category, quota_period_id, operation_key, amount_reserved,
			amount_committed, amount_refunded, status, reservation_point, commit_point, refund_point,
			created_at, updated_at, expires_at, recovery_reason
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(tenant_id, category, operation_key) DO UPDATE SET
			quota_period_id = excluded.quota_period_id,
			amount_reserved = excluded.amount_reserved,
			amount_committed = excluded.amount_committed,
			amount_refunded = excluded.amount_refunded,
			status = excluded.status,
			reservation_point = excluded.reservation_point,
			commit_point = excluded.commit_point,
			refund_point = excluded.refund_point,
			updated_at = excluded.updated_at,
			expires_at = excluded.expires_at,
			recovery_reason = excluded.recovery_reason
	`, reservation.ReservationID, reservation.TenantID, reservation.Category, reservation.QuotaPeriodID,
		reservation.OperationKey, reservation.AmountReserved, reservation.AmountCommitted, reservation.AmountRefunded,
		reservation.Status, nullString(reservation.ReservationPoint), nullString(reservation.CommitPoint),
		nullString(reservation.RefundPoint), reservation.CreatedAt.UTC().Format(time.RFC3339Nano),
		reservation.UpdatedAt.UTC().Format(time.RFC3339Nano), nullableTimeString(reservation.ExpiresAt),
		nullString(reservation.RecoveryReason))
	if err != nil {
		return fmt.Errorf("save billing usage reservation %s: %w", reservation.ReservationID, err)
	}
	return nil
}

func (s *SQLiteStore) AppendUsageEvent(ctx context.Context, event billing.UsageEvent) error {
	if s == nil {
		return nil
	}
	if event.UsageEventID == "" {
		event.UsageEventID = newBillingID("usage_event")
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	documentJSON, err := marshalJSON(event.Document)
	if err != nil {
		return fmt.Errorf("encode billing usage event %s document: %w", event.UsageEventID, err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO billing_usage_events (
			usage_event_id, tenant_id, category, quota_period_id, operation_key, event_kind,
			amount, reason_code, reason, actor_principal_id, outcome, created_at, document_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, event.UsageEventID, event.TenantID, nullString(string(event.Category)), nullString(event.QuotaPeriodID),
		nullString(event.OperationKey), event.EventKind, event.Amount, event.ReasonCode, nullString(event.Reason),
		nullString(event.ActorPrincipalID), event.Outcome, event.CreatedAt.UTC().Format(time.RFC3339Nano), documentJSON)
	if err != nil {
		return fmt.Errorf("append billing usage event %s: %w", event.UsageEventID, err)
	}
	return nil
}

func (s *SQLiteStore) AppendQuotaDenial(ctx context.Context, denial billing.QuotaDenial) error {
	if s == nil {
		return nil
	}
	if denial.DenialID == "" {
		denial.DenialID = newBillingID("denial")
	}
	if denial.CreatedAt.IsZero() {
		denial.CreatedAt = time.Now().UTC()
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO billing_quota_denials (
			denial_id, tenant_id, category, quota_period_id, operation_key, reason_code,
			requested_amount, remaining_amount, guarded_entry_point, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, denial.DenialID, denial.TenantID, nullString(string(denial.Category)), nullString(denial.QuotaPeriodID),
		denial.OperationKey, denial.ReasonCode, denial.RequestedAmount, denial.RemainingAmount,
		denial.GuardedEntryPoint, denial.CreatedAt.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("append billing quota denial %s: %w", denial.DenialID, err)
	}
	return nil
}

func (s *SQLiteStore) SaveManualAdjustment(ctx context.Context, adjustment billing.ManualAdjustment) error {
	if s == nil {
		return nil
	}
	if adjustment.AdjustmentID == "" {
		adjustment.AdjustmentID = newBillingID("manual_adjustment")
	}
	if adjustment.CreatedAt.IsZero() {
		adjustment.CreatedAt = time.Now().UTC()
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO billing_manual_adjustments (
			adjustment_id, tenant_id, category, quota_period_id, amount_delta,
			reason, created_by_principal_id, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(adjustment_id) DO UPDATE SET
			tenant_id = excluded.tenant_id,
			category = excluded.category,
			quota_period_id = excluded.quota_period_id,
			amount_delta = excluded.amount_delta,
			reason = excluded.reason,
			created_by_principal_id = excluded.created_by_principal_id,
			created_at = excluded.created_at
	`, adjustment.AdjustmentID, adjustment.TenantID, adjustment.Category, adjustment.QuotaPeriodID,
		adjustment.AmountDelta, adjustment.Reason, nullString(adjustment.CreatedByPrincipalID),
		adjustment.CreatedAt.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("save billing manual adjustment %s: %w", adjustment.AdjustmentID, err)
	}
	return nil
}

func (s *SQLiteStore) ListPendingReservations(ctx context.Context) ([]billing.UsageReservation, error) {
	if s == nil {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT reservation_id, tenant_id, category, quota_period_id, operation_key, amount_reserved,
		       amount_committed, amount_refunded, status, reservation_point, commit_point, refund_point,
		       created_at, updated_at, expires_at, recovery_reason
		FROM billing_usage_reservations
		WHERE status = ?
		ORDER BY updated_at ASC, reservation_id ASC
	`, billing.ReservationStatusReserved)
	if err != nil {
		return nil, fmt.Errorf("list pending billing reservations: %w", err)
	}
	defer rows.Close()
	items := make([]billing.UsageReservation, 0)
	for rows.Next() {
		item, err := scanBillingUsageReservation(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *SQLiteStore) ListQuotaDenials(ctx context.Context, tenantID string, limit int) ([]billing.QuotaDenial, error) {
	if s == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT denial_id, tenant_id, category, quota_period_id, operation_key, reason_code,
		       requested_amount, remaining_amount, guarded_entry_point, created_at
		FROM billing_quota_denials
		WHERE tenant_id = ?
		ORDER BY created_at DESC, denial_id DESC
		LIMIT ?
	`, tenantID, limit)
	if err != nil {
		return nil, fmt.Errorf("list billing quota denials: %w", err)
	}
	defer rows.Close()
	items := make([]billing.QuotaDenial, 0)
	for rows.Next() {
		item, err := scanBillingQuotaDenial(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *SQLiteStore) ListManualAdjustments(ctx context.Context, tenantID string, limit int) ([]billing.ManualAdjustment, error) {
	if s == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT adjustment_id, tenant_id, category, quota_period_id, amount_delta,
		       reason, created_by_principal_id, created_at
		FROM billing_manual_adjustments
		WHERE tenant_id = ?
		ORDER BY created_at DESC, adjustment_id DESC
		LIMIT ?
	`, tenantID, limit)
	if err != nil {
		return nil, fmt.Errorf("list billing manual adjustments: %w", err)
	}
	defer rows.Close()
	items := make([]billing.ManualAdjustment, 0)
	for rows.Next() {
		item, err := scanBillingManualAdjustment(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func scanBillingTenantPlan(scanner interface{ Scan(dest ...any) error }) (billing.TenantPlan, error) {
	var item billing.TenantPlan
	var effectiveAt string
	var supersededAt sql.NullString
	var assignedBy sql.NullString
	var reason sql.NullString
	var documentJSON sql.NullString
	if err := scanner.Scan(&item.PlanID, &item.TenantID, &item.PlanKey, &item.Status, &item.EnforcementMode,
		&effectiveAt, &supersededAt, &assignedBy, &reason, &documentJSON); err != nil {
		return billing.TenantPlan{}, err
	}
	parsed, err := time.Parse(time.RFC3339Nano, effectiveAt)
	if err != nil {
		return billing.TenantPlan{}, fmt.Errorf("parse billing tenant plan effective_at: %w", err)
	}
	item.EffectiveAt = parsed
	if err := assignOptionalTime(&item.SupersededAt, supersededAt); err != nil {
		return billing.TenantPlan{}, fmt.Errorf("parse billing tenant plan superseded_at: %w", err)
	}
	item.AssignedByPrincipalID = assignedBy.String
	item.AssignmentReason = reason.String
	if err := unmarshalNullableJSON(documentJSON, &item.Document); err != nil {
		return billing.TenantPlan{}, fmt.Errorf("decode billing tenant plan document: %w", err)
	}
	return item, nil
}

func scanBillingQuotaOverride(scanner interface{ Scan(dest ...any) error }) (billing.QuotaOverride, error) {
	var item billing.QuotaOverride
	var limit sql.NullInt64
	var carryoverEnabled sql.NullInt64
	var carryoverMax sql.NullInt64
	var effectiveAt string
	var expiresAt sql.NullString
	var createdBy sql.NullString
	if err := scanner.Scan(&item.QuotaOverrideID, &item.TenantID, &item.Category, &limit, &carryoverEnabled, &carryoverMax,
		&effectiveAt, &expiresAt, &item.Reason, &createdBy); err != nil {
		return billing.QuotaOverride{}, err
	}
	if limit.Valid {
		value := limit.Int64
		item.Limit = &value
	}
	if carryoverEnabled.Valid {
		value := carryoverEnabled.Int64 != 0
		item.CarryoverEnabled = &value
	}
	if carryoverMax.Valid {
		value := carryoverMax.Int64
		item.CarryoverMax = &value
	}
	parsed, err := time.Parse(time.RFC3339Nano, effectiveAt)
	if err != nil {
		return billing.QuotaOverride{}, fmt.Errorf("parse billing quota override effective_at: %w", err)
	}
	item.EffectiveAt = parsed
	if err := assignOptionalTime(&item.ExpiresAt, expiresAt); err != nil {
		return billing.QuotaOverride{}, fmt.Errorf("parse billing quota override expires_at: %w", err)
	}
	item.CreatedByPrincipalID = createdBy.String
	return item, nil
}

func scanBillingQuotaPeriod(scanner interface{ Scan(dest ...any) error }) (billing.QuotaPeriod, error) {
	var item billing.QuotaPeriod
	var periodStart string
	var periodEnd string
	var carryoverFrom sql.NullString
	if err := scanner.Scan(&item.QuotaPeriodID, &item.TenantID, &item.Category, &item.PeriodKind,
		&periodStart, &periodEnd, &carryoverFrom, &item.Status); err != nil {
		return billing.QuotaPeriod{}, err
	}
	parsedStart, err := time.Parse(time.RFC3339Nano, periodStart)
	if err != nil {
		return billing.QuotaPeriod{}, fmt.Errorf("parse billing quota period start: %w", err)
	}
	parsedEnd, err := time.Parse(time.RFC3339Nano, periodEnd)
	if err != nil {
		return billing.QuotaPeriod{}, fmt.Errorf("parse billing quota period end: %w", err)
	}
	item.PeriodStart = parsedStart
	item.PeriodEnd = parsedEnd
	item.CarryoverFromPeriodID = carryoverFrom.String
	return item, nil
}

func scanBillingUsageCounter(scanner interface{ Scan(dest ...any) error }) (billing.UsageCounter, error) {
	var item billing.UsageCounter
	var updatedAt string
	if err := scanner.Scan(&item.UsageCounterID, &item.TenantID, &item.Category, &item.QuotaPeriodID,
		&item.CommittedAmount, &item.ReservedAmount, &item.AdjustedAmount, &item.CarryoverAmount, &updatedAt); err != nil {
		return billing.UsageCounter{}, err
	}
	parsed, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return billing.UsageCounter{}, fmt.Errorf("parse billing usage counter updated_at: %w", err)
	}
	item.UpdatedAt = parsed
	return item, nil
}

func scanBillingUsageReservation(scanner interface{ Scan(dest ...any) error }) (billing.UsageReservation, error) {
	var item billing.UsageReservation
	var reservationPoint sql.NullString
	var commitPoint sql.NullString
	var refundPoint sql.NullString
	var createdAt string
	var updatedAt string
	var expiresAt sql.NullString
	var recoveryReason sql.NullString
	if err := scanner.Scan(&item.ReservationID, &item.TenantID, &item.Category, &item.QuotaPeriodID,
		&item.OperationKey, &item.AmountReserved, &item.AmountCommitted, &item.AmountRefunded, &item.Status,
		&reservationPoint, &commitPoint, &refundPoint, &createdAt, &updatedAt, &expiresAt, &recoveryReason); err != nil {
		return billing.UsageReservation{}, err
	}
	parsedCreated, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return billing.UsageReservation{}, fmt.Errorf("parse billing reservation created_at: %w", err)
	}
	parsedUpdated, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return billing.UsageReservation{}, fmt.Errorf("parse billing reservation updated_at: %w", err)
	}
	item.CreatedAt = parsedCreated
	item.UpdatedAt = parsedUpdated
	item.ReservationPoint = reservationPoint.String
	item.CommitPoint = commitPoint.String
	item.RefundPoint = refundPoint.String
	item.RecoveryReason = recoveryReason.String
	if err := assignOptionalTime(&item.ExpiresAt, expiresAt); err != nil {
		return billing.UsageReservation{}, fmt.Errorf("parse billing reservation expires_at: %w", err)
	}
	return item, nil
}

func scanBillingQuotaDenial(scanner interface{ Scan(dest ...any) error }) (billing.QuotaDenial, error) {
	var item billing.QuotaDenial
	var category sql.NullString
	var quotaPeriodID sql.NullString
	var createdAt string
	if err := scanner.Scan(&item.DenialID, &item.TenantID, &category, &quotaPeriodID, &item.OperationKey,
		&item.ReasonCode, &item.RequestedAmount, &item.RemainingAmount, &item.GuardedEntryPoint, &createdAt); err != nil {
		return billing.QuotaDenial{}, err
	}
	item.Category = billing.Category(category.String)
	item.QuotaPeriodID = quotaPeriodID.String
	parsed, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return billing.QuotaDenial{}, fmt.Errorf("parse billing quota denial created_at: %w", err)
	}
	item.CreatedAt = parsed
	return item, nil
}

func scanBillingManualAdjustment(scanner interface{ Scan(dest ...any) error }) (billing.ManualAdjustment, error) {
	var item billing.ManualAdjustment
	var createdBy sql.NullString
	var createdAt string
	if err := scanner.Scan(&item.AdjustmentID, &item.TenantID, &item.Category, &item.QuotaPeriodID,
		&item.AmountDelta, &item.Reason, &createdBy, &createdAt); err != nil {
		return billing.ManualAdjustment{}, err
	}
	item.CreatedByPrincipalID = createdBy.String
	parsed, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return billing.ManualAdjustment{}, fmt.Errorf("parse billing manual adjustment created_at: %w", err)
	}
	item.CreatedAt = parsed
	return item, nil
}

func nullableInt64(value *int64) sql.NullInt64 {
	if value == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *value, Valid: true}
}

func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func nullableBoolInt(value *bool) sql.NullInt64 {
	if value == nil {
		return sql.NullInt64{}
	}
	if *value {
		return sql.NullInt64{Int64: 1, Valid: true}
	}
	return sql.NullInt64{Int64: 0, Valid: true}
}

func newBillingID(prefix string) string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return prefix + "_fallback"
	}
	return strings.TrimSpace(prefix) + "_" + hex.EncodeToString(buf)
}
