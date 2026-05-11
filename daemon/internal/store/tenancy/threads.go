package tenancy

import "github.com/dopejs/dope-agent/daemon/internal/threads"

type ThreadAccessScope struct {
	TenantID string
}

func (scope ThreadAccessScope) Allows(threadTenantID string) bool {
	return scope.TenantID != "" && scope.TenantID == threadTenantID
}

func (scope ThreadAccessScope) AllowsContinuityTurn(turn threads.ContinuityTurn) bool {
	return scope.Allows(turn.TenantID)
}

func (scope ThreadAccessScope) AllowsContinuityPreview(preview threads.ContinuityPreview) bool {
	return scope.Allows(preview.TenantID)
}
