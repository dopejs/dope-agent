package activation

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/identity"
)

func (s *Service) recordAudit(ctx context.Context, record auditRecord) error {
	if s == nil || s.audit == nil {
		return nil
	}
	now := s.now()
	document := map[string]any{
		"activationId":     record.ActivationID,
		"environmentScope": s.environmentScope,
		"stage":            string(record.Stage),
		"fromStatus":       string(record.FromStatus),
		"toStatus":         string(record.ToStatus),
		"reasonCode":       string(record.ReasonCode),
		"retryable":        record.Retryable,
		"transitionedAt":   now,
	}
	if record.RemediationOwner != "" {
		document["remediationOwner"] = string(record.RemediationOwner)
	}
	if record.TestChat != nil {
		document["testChat"] = record.TestChat
	}
	if len(record.CompletedStepIDs) > 0 {
		document["completedStepIds"] = append([]string(nil), record.CompletedStepIDs...)
	}
	if len(record.ReadinessItemIDs) > 0 {
		document["readinessItemIds"] = append([]string(nil), record.ReadinessItemIDs...)
	}
	if record.QuotaBaselineStat != "" {
		document["quotaBaselineStatus"] = record.QuotaBaselineStat
	}
	event, err := s.audit.AppendTenantAuditEvent(ctx, identity.TenantAuditEvent{
		AuditEventID: randomActivationAuditID(),
		EventKind:    record.EventKind,
		TenantID:     record.TenantID,
		PrincipalID:  record.PrincipalID,
		TokenID:      record.TokenID,
		Outcome:      record.Outcome,
		ReasonCode:   string(record.ReasonCode),
		CreatedAt:    now,
		Document:     document,
	})
	if err != nil {
		return activationError(ReasonAuditWriteFailed, FailureStageAudit, true, RemediationOwnerOperator, err.Error())
	}
	_ = event
	return nil
}

func randomActivationAuditID() string {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "audit_activation_" + time.Now().UTC().Format("20060102150405")
	}
	return "audit_activation_" + hex.EncodeToString(buf[:])
}
