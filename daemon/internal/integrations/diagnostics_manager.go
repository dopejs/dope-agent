package integrations

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
)

type DiagnosticManager struct{}

func NewDiagnosticManager() *DiagnosticManager {
	return &DiagnosticManager{}
}

func (m *DiagnosticManager) Inspect(input DiagnosticInspectionInput) DiagnosticResult {
	now := input.CheckedAt.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	capability := strings.TrimSpace(input.Capability)
	if capability == "" {
		capability = "integration.readiness"
	}
	reason := DiagnosticProjectionReason(input.Resource)
	if input.ForceGeneric {
		reason = ReasonRedactionFailedClosed
	}
	status, owner, retrySafety := DiagnosticDefaults(reason)
	redaction := RedactDiagnosticSummary(input.EvidenceText)
	if input.ForceGeneric || redaction.Status == RedactionStatusFailedClosed {
		redaction = FailClosedDiagnosticRedaction()
		reason = ReasonRedactionFailedClosed
		status, owner, retrySafety = DiagnosticDefaults(reason)
	}
	resultID := diagnosticID("diag_result", input.Resource.TenantID, input.Resource.IntegrationID, capability, input.RunID, now.Format(time.RFC3339Nano))
	return DiagnosticResult{
		DiagnosticResultID:   resultID,
		TenantID:             strings.TrimSpace(input.Resource.TenantID),
		IntegrationID:        strings.TrimSpace(input.Resource.IntegrationID),
		IntegrationAccountID: strings.TrimSpace(input.Resource.AccountBinding.AccountKey),
		DomainKind:           strings.TrimSpace(input.Resource.DomainKind),
		ProviderKind:         providerKind(input.Resource),
		Capability:           capability,
		Status:               status,
		ReasonCode:           reason,
		RemediationOwner:     owner,
		RemediationHint:      DiagnosticRemediationHint(reason),
		RetrySafety:          retrySafety,
		CheckedAt:            now,
		StaleAfter:           now.Add(DiagnosticStaleAfter),
		FreshnessState:       FreshnessStateFresh,
		RunID:                strings.TrimSpace(input.RunID),
		RedactionStatus:      redaction.Status,
		EvidenceSummary:      redaction.Summary,
		RetentionExpiresAt:   DiagnosticRetentionExpiry(now),
		CreatedAt:            now,
		UpdatedAt:            now,
	}
}

func (m *DiagnosticManager) CreateRun(input DiagnosticRunInput) DiagnosticRun {
	now := input.StartedAt.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	clientKey := strings.TrimSpace(input.ClientKey)
	runID := "diag_run_" + clientKey
	if clientKey == "" {
		runID = diagnosticID("diag_run", input.Resource.TenantID, input.Resource.IntegrationID, input.RequestedBy, now.Format(time.RFC3339Nano))
	}
	capabilities := normalizeDiagnosticCapabilities(input.Capabilities)
	trigger := strings.TrimSpace(input.Trigger)
	if trigger == "" {
		trigger = "operator_inspection"
	}
	return DiagnosticRun{
		DiagnosticRunID:      runID,
		TenantID:             strings.TrimSpace(input.Resource.TenantID),
		IntegrationID:        strings.TrimSpace(input.Resource.IntegrationID),
		IntegrationAccountID: strings.TrimSpace(input.Resource.AccountBinding.AccountKey),
		DomainKind:           strings.TrimSpace(input.Resource.DomainKind),
		ProviderKind:         providerKind(input.Resource),
		RequestedBy:          strings.TrimSpace(input.RequestedBy),
		Trigger:              trigger,
		Status:               DiagnosticRunRunning,
		StartedAt:            now,
		CheckedCapabilities:  capabilities,
		ResultIDs:            []string{},
		RedactionStatus:      RedactionStatusRedacted,
		RetentionExpiresAt:   DiagnosticRetentionExpiry(now),
		IdempotencyKey:       clientKey,
	}
}

func CompleteDiagnosticRun(run DiagnosticRun, results []DiagnosticResult, completedAt time.Time) DiagnosticRun {
	if completedAt.IsZero() {
		completedAt = time.Now().UTC()
	}
	run.Status = DiagnosticRunCompleted
	run.CompletedAt = &completedAt
	run.ResultIDs = make([]string, 0, len(results))
	for _, result := range results {
		run.ResultIDs = append(run.ResultIDs, result.DiagnosticResultID)
		if result.RedactionStatus == RedactionStatusFailedClosed {
			run.RedactionStatus = RedactionStatusFailedClosed
			run.FailureReasonCode = ReasonRedactionFailedClosed
		}
	}
	return run
}

func RefreshDiagnosticResultFreshness(result DiagnosticResult, now time.Time) DiagnosticResult {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	result.FreshnessState = DiagnosticFreshness(now.UTC(), result.StaleAfter)
	return result
}

func normalizeDiagnosticCapabilities(capabilities []string) []string {
	seen := map[string]struct{}{}
	normalized := make([]string, 0, len(capabilities))
	for _, capability := range capabilities {
		capability = strings.TrimSpace(capability)
		if capability == "" {
			continue
		}
		if _, ok := seen[capability]; ok {
			continue
		}
		seen[capability] = struct{}{}
		normalized = append(normalized, capability)
	}
	if len(normalized) == 0 {
		return []string{"integration.readiness"}
	}
	return normalized
}

func providerKind(resource Resource) string {
	value := strings.TrimSpace(string(resource.BackendBinding.BackendKind))
	if value == "" {
		return "unknown"
	}
	return value
}

func diagnosticID(prefix string, parts ...string) string {
	hash := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return prefix + "_" + hex.EncodeToString(hash[:])[:24]
}
