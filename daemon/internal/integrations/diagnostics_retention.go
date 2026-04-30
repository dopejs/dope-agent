package integrations

import "time"

type DiagnosticRetentionState string

const (
	DiagnosticRetentionActive  DiagnosticRetentionState = "active"
	DiagnosticRetentionExpired DiagnosticRetentionState = "expired"
	DiagnosticRetentionPurged  DiagnosticRetentionState = "purged"
)

type DiagnosticRetentionRecord struct {
	RetentionRecordID  string                   `json:"retentionRecordId"`
	TenantID           string                   `json:"tenantId"`
	TargetKind         string                   `json:"targetKind"`
	TargetID           string                   `json:"targetId"`
	PolicyRef          string                   `json:"policyRef,omitempty"`
	DefaultExpiresAt   time.Time                `json:"defaultExpiresAt"`
	EffectiveExpiresAt time.Time                `json:"effectiveExpiresAt"`
	RetentionState     DiagnosticRetentionState `json:"retentionState"`
	AppliedAt          *time.Time               `json:"appliedAt,omitempty"`
	CreatedAt          time.Time                `json:"createdAt"`
	UpdatedAt          time.Time                `json:"updatedAt"`
}

func NewDiagnosticRetentionRecord(tenantID, targetKind, targetID string, createdAt time.Time) DiagnosticRetentionRecord {
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	expiresAt := DiagnosticRetentionExpiry(createdAt)
	return DiagnosticRetentionRecord{
		RetentionRecordID:  diagnosticID("diag_retention", tenantID, targetKind, targetID, createdAt.Format(time.RFC3339Nano)),
		TenantID:           tenantID,
		TargetKind:         targetKind,
		TargetID:           targetID,
		DefaultExpiresAt:   expiresAt,
		EffectiveExpiresAt: expiresAt,
		RetentionState:     DiagnosticRetentionActive,
		CreatedAt:          createdAt.UTC(),
		UpdatedAt:          createdAt.UTC(),
	}
}
