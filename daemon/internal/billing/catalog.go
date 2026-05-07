package billing

import "time"

const (
	PeriodAnchorUTC = "UTC"

	ReasonQuotaStateUnavailable = "quota_denied:quota_state_unavailable"
)

type CatalogEntry struct {
	Definition         QuotaDefinition `json:"definition"`
	OperationKeyShape  string          `json:"operationKeyShape"`
	ConcurrencyGuard   string          `json:"concurrencyGuard"`
	RequiredTests      []string        `json:"requiredTests"`
	ReservationPoint   string          `json:"reservationPoint"`
	CommitPoint        string          `json:"commitPoint"`
	RefundPoint        string          `json:"refundPoint"`
	OverLimitCommit    bool            `json:"overLimitCommit,omitempty"`
	FutureDenialOnOver bool            `json:"futureDenialOnOver,omitempty"`
}

type CatalogExport struct {
	Categories []CatalogEntry `json:"categories"`
}

func ExportCatalog(now time.Time) CatalogExport {
	return CatalogExport{Categories: InitialCatalog(now)}
}

func InitialCatalog(now time.Time) []CatalogEntry {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	commonTests := []string{"allowed", "denied", "retry", "restart_pending"}
	return []CatalogEntry{
		catalogEntry(CategoryRunLaunches, UnitCount, PeriodMonthly, 1, "POST /v1/runs before runtime.CreateRun", "run persisted", "route denial or failure before run persisted", "tenant:{tenantId}:run:{clientKey|runId}", "quota_denied:run_launches_exhausted", append(commonTests, "concurrent_last_unit"), now),
		catalogEntry(CategoryWorkflowLaunches, UnitCount, PeriodMonthly, 1, "workflow create/start before execution", "workflow planned/running", "planning/start denial or cancellation before execution", "tenant:{tenantId}:workflow:{runId}:{workflowId|clientKey}", "quota_denied:workflow_launches_exhausted", append(commonTests, "concurrent_start"), now),
		catalogEntry(CategoryRuntimeToolCalls, UnitCount, PeriodDaily, 1, "tool call creation before invocation", "tool call accepted/running/completed", "denial, failed creation, cancellation before invocation", "tenant:{tenantId}:tool_call:{runId}:{stepId}:{toolCallId|clientKey}", "quota_denied:runtime_tool_calls_exhausted", append(commonTests, "concurrent_tool_calls"), now),
		catalogEntry(CategoryLiveValidationAttempts, UnitAttempts, PeriodDaily, 1, "Roadmap 38 live-validation preflight gate", "validation starts or no executor is mounted", "denial or unsafe preflight failure before live action", "tenant:{tenantId}:live_validation:{validationId|clientKey}", "quota_denied:live_validation_attempts_exhausted", append(commonTests, "fail_closed_unavailable", "no_roadmap_40_executor"), now),
		catalogEntry(CategoryIntegrationOperations, UnitCount, PeriodMonthly, 1, "integration operation handlers before backend operation", "operation record persisted after backend attempt", "denial or failed preflight before backend attempt", "tenant:{tenantId}:integration:{domain}:{operationId|clientKey}", "quota_denied:integration_operations_exhausted", append(commonTests, "concurrent_operations"), now),
		func() CatalogEntry {
			entry := catalogEntry(CategoryArtifactStorageBytes, UnitBytes, PeriodMonthly, 0, "artifact write service before writing bytes using estimate", "actual bytes known after write", "write failure before consumption or smaller actual refund", "tenant:{tenantId}:artifact:{artifactId|storageKey|clientKey}", "quota_denied:artifact_storage_bytes_exhausted", append(commonTests, "actual_smaller_refund", "actual_larger_over_limit_commit", "future_denial_after_over_limit_commit"), now)
			entry.OverLimitCommit = true
			entry.FutureDenialOnOver = true
			entry.Definition.Document = map[string]any{"artifactWriteReservationEstimateBytes": int64(4096)}
			return entry
		}(),
		catalogEntry(CategoryReplayEvaluationAttempts, UnitAttempts, PeriodMonthly, 1, "replay/evaluation attempt creation before work starts", "attempt persisted as accepted/started/completed", "denial or preflight unreplayable before attempt consumption", "tenant:{tenantId}:evaluation:{candidateId}:{attemptId|clientKey}", "quota_denied:replay_evaluation_attempts_exhausted", append(commonTests, "concurrent_attempt"), now),
	}
}

func InitialDefinitions(now time.Time) []QuotaDefinition {
	entries := InitialCatalog(now)
	out := make([]QuotaDefinition, 0, len(entries))
	for _, entry := range entries {
		out = append(out, entry.Definition)
	}
	return out
}

func DefinitionFor(category Category) (QuotaDefinition, bool) {
	for _, item := range InitialDefinitions(time.Now().UTC()) {
		if item.Category == category {
			return item, true
		}
	}
	return QuotaDefinition{}, false
}

func RequiredCategories() []Category {
	return []Category{
		CategoryRunLaunches,
		CategoryWorkflowLaunches,
		CategoryRuntimeToolCalls,
		CategoryLiveValidationAttempts,
		CategoryIntegrationOperations,
		CategoryArtifactStorageBytes,
		CategoryReplayEvaluationAttempts,
	}
}

func catalogEntry(category Category, unit Unit, period PeriodKind, defaultLimit int64, reservationPoint, commitPoint, refundPoint, operationKeyShape, denialReason string, tests []string, now time.Time) CatalogEntry {
	return CatalogEntry{
		Definition: QuotaDefinition{
			QuotaDefinitionID: "quota_def_" + string(category),
			Category:          category,
			Unit:              unit,
			PeriodKind:        period,
			PeriodAnchor:      PeriodAnchorUTC,
			DefaultLimit:      defaultLimit,
			ReservationRule:   reservationPoint,
			CommitRule:        commitPoint,
			RefundRule:        refundPoint,
			DenialReasonCode:  denialReason,
			Active:            true,
			CreatedAt:         now.UTC(),
			UpdatedAt:         now.UTC(),
		},
		OperationKeyShape: operationKeyShape,
		ConcurrencyGuard:  "single durable transaction over tenant/category/period counter and reservation row",
		RequiredTests:     append([]string(nil), tests...),
		ReservationPoint:  reservationPoint,
		CommitPoint:       commitPoint,
		RefundPoint:       refundPoint,
	}
}
