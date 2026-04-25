package identity

import (
	"context"
	"time"
)

const (
	AuditOutcomeSucceeded    = "succeeded"
	AuditOutcomeDenied       = "denied"
	AuditOutcomeFailedClosed = "failed_closed"
)

type AuditStore interface {
	AppendTenantAuditEvent(ctx context.Context, event TenantAuditEvent) (TenantAuditEvent, error)
}

type Auditor struct {
	store AuditStore
	now   func() time.Time
}

func NewAuditor(store AuditStore) *Auditor {
	return &Auditor{store: store, now: func() time.Time { return time.Now().UTC() }}
}

func (a *Auditor) Record(ctx context.Context, event TenantAuditEvent) (TenantAuditEvent, error) {
	if a == nil || a.store == nil {
		return event, nil
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = a.now().UTC()
	}
	return a.store.AppendTenantAuditEvent(ctx, event)
}

func (a *Auditor) Require(ctx context.Context, event TenantAuditEvent) (TenantAuditEvent, error) {
	recorded, err := a.Record(ctx, event)
	if err != nil {
		return TenantAuditEvent{}, ErrAuditWriteFailed
	}
	return recorded, nil
}
