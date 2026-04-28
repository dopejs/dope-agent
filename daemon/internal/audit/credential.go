package audit

import (
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/secrets"
)

const CredentialEventKind = "credential.audit_recorded"

type CredentialAuditInput struct {
	TenantID        string
	PrincipalID     string
	ResourceKind    secrets.ResourceKind
	ResourceID      string
	Action          secrets.AuditAction
	Outcome         string
	ReasonCode      string
	SecretRef       string
	SecretVersionID string
	SecretRefs      []string
	CreatedAt       time.Time
}

// BuildCredentialAuditEvent constructs a tenant audit event for
// credential-bearing behavior. The document intentionally includes only
// resource identifiers, status, versions, and redacted secret reference
// summaries; callers must not pass raw secret values through this surface.
func BuildCredentialAuditEvent(input CredentialAuditInput) identity.TenantAuditEvent {
	createdAt := input.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	outcome := input.Outcome
	if outcome == "" {
		outcome = identity.AuditOutcomeSucceeded
	}
	document := map[string]any{
		"resourceKind": string(input.ResourceKind),
		"action":       string(input.Action),
	}
	if input.ResourceID != "" {
		document["resourceId"] = input.ResourceID
	}
	if input.SecretRef != "" {
		document["secretRef"] = input.SecretRef
	}
	if input.SecretVersionID != "" {
		document["secretVersionId"] = input.SecretVersionID
	}
	if len(input.SecretRefs) > 0 {
		document["secretRefs"] = secrets.RedactSecretRefs(input.SecretRefs)
		document["secretRefCount"] = len(input.SecretRefs)
	}
	return identity.TenantAuditEvent{
		EventKind:   CredentialEventKind,
		TenantID:    input.TenantID,
		PrincipalID: input.PrincipalID,
		Outcome:     outcome,
		ReasonCode:  input.ReasonCode,
		CreatedAt:   createdAt.UTC(),
		Document:    document,
	}
}
