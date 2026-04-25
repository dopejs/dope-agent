package identity

import (
	"context"
	"errors"
	"testing"
)

func TestAuditorRequireFailsClosed(t *testing.T) {
	store := newMemoryStore()
	store.auditErr = errors.New("disk full")
	auditor := NewAuditor(store)

	_, err := auditor.Require(context.Background(), TenantAuditEvent{EventKind: "tenant.access_denied"})
	if !errors.Is(err, ErrAuditWriteFailed) {
		t.Fatalf("expected ErrAuditWriteFailed, got %v", err)
	}
}

func (s *memoryStore) AppendTenantAuditEvent(_ context.Context, event TenantAuditEvent) (TenantAuditEvent, error) {
	if s.auditErr != nil {
		return TenantAuditEvent{}, s.auditErr
	}
	return event, nil
}
