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

func (scope ThreadAccessScope) AllowsConversationShape(evidence threads.ConversationShapeEvidence) bool {
	return scope.Allows(evidence.TenantID)
}

func (scope ThreadAccessScope) AllowsParticipationDecision(decision threads.ParticipationDecision) bool {
	return scope.Allows(decision.TenantID)
}

func (scope ThreadAccessScope) AllowsResetEvent(event threads.ResetEvent) bool {
	return scope.Allows(event.TenantID)
}

func (scope ThreadAccessScope) AllowsHandoffLink(link threads.HandoffLink) bool {
	return scope.Allows(link.TenantID)
}

func (scope ThreadAccessScope) AllowsHandoffSourceReference(reference threads.HandoffSourceReference) bool {
	return scope.Allows(reference.TenantID)
}
