// Package webhook implements the webhook + external trigger plane (Roadmap 67): tenant-scoped
// webhook endpoints that safely trigger runs/workflows/routines with signature authentication,
// replay protection, bounded + redacted payloads, quota/permission gating, and audit linkage.
// Webhooks are trigger resources, NOT channel connectors, and never ingest payloads into memory.
package webhook

import "time"

// MaxPayloadBytes bounds an inbound webhook payload (Roadmap 67, FR payload bounding).
const MaxPayloadBytes = 64 * 1024

// TargetKind is what a webhook fires.
type TargetKind string

const (
	TargetKindRoutine  TargetKind = "routine"
	TargetKindWorkflow TargetKind = "workflow"
	TargetKindRun      TargetKind = "run"
)

// Status is the webhook endpoint lifecycle state.
type Status string

const (
	StatusActive   Status = "active"
	StatusDisabled Status = "disabled"
)

// Endpoint is a tenant-scoped webhook trigger resource. The signing secret is never stored on
// the projection — only a redacted fingerprint is surfaced.
type Endpoint struct {
	WebhookID         string     `json:"webhookId"`
	TenantID          string     `json:"tenantId"`
	EnvironmentScope  string     `json:"environmentScope"`
	Name              string     `json:"name"`
	TargetKind        TargetKind `json:"targetKind"`
	TargetRef         string     `json:"targetRef"` // routineId / workflow goal / run goal
	Status            Status     `json:"status"`
	SecretFingerprint string     `json:"secretFingerprint"` // redacted (sha256 prefix), never the secret
	SecretVersion     int        `json:"secretVersion"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
}

// TriggerStatus classifies the outcome of a webhook invocation.
type TriggerStatus string

const (
	TriggerStatusFired            TriggerStatus = "fired"
	TriggerStatusReplaySuppressed TriggerStatus = "replay_suppressed"
	TriggerStatusAuthFailed       TriggerStatus = "auth_failed"
	TriggerStatusPayloadTooLarge  TriggerStatus = "payload_too_large"
	TriggerStatusQuotaDenied      TriggerStatus = "quota_denied"
	TriggerStatusDisabled         TriggerStatus = "disabled"
)

// TriggerRecord is the audited outcome of one webhook invocation. It never carries payload
// content — only the byte size and the redacted outcome (FR payload redaction).
type TriggerRecord struct {
	TriggerID        string        `json:"triggerId"`
	WebhookID        string        `json:"webhookId"`
	TenantID         string        `json:"tenantId"`
	EnvironmentScope string        `json:"environmentScope"`
	IdempotencyKey   string        `json:"idempotencyKey,omitempty"`
	Status           TriggerStatus `json:"status"`
	PayloadBytes     int           `json:"payloadBytes"`
	ExecutionRef     string        `json:"executionRef,omitempty"`
	FailureReason    string        `json:"failureReason,omitempty"`
	CreatedAt        time.Time     `json:"createdAt"`
}

// CreateSecret is returned once when a webhook is created or its secret rotated. The plaintext
// secret is returned to the caller exactly once and never persisted in cleartext.
type CreateSecret struct {
	Endpoint Endpoint `json:"endpoint"`
	Secret   string   `json:"secret"`
}
