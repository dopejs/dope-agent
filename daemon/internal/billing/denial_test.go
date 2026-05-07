package billing

import (
	"context"
	"testing"
	"time"
)

func TestStableQuotaDenialPayloads(t *testing.T) {
	period := QuotaPeriod{
		QuotaPeriodID: "period_1",
		TenantID:      fixtureTenantA,
		Category:      CategoryRunLaunches,
		PeriodStart:   time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
		PeriodEnd:     time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
	}
	denial := NewQuotaExhaustedDenial(fixtureTenantA, CategoryRunLaunches, "op_1", 1, 0, period).Payload
	if denial.Code != "quota_denied" || denial.ReasonCode != "quota_denied:run_launches_exhausted" {
		t.Fatalf("unexpected denial: %+v", denial)
	}
	if denial.Message == "" || denial.PeriodStart == "" || denial.PeriodEnd == "" {
		t.Fatalf("denial missing stable projection fields: %+v", denial)
	}
	unavailable := NewQuotaStateUnavailableDenial(fixtureTenantA, "op_2").Payload
	if unavailable.ReasonCode != ReasonQuotaStateUnavailable || unavailable.Category != "" {
		t.Fatalf("unexpected unavailable denial: %+v", unavailable)
	}
}

func TestProjectDenialDetailClassifiesQuotaAbuseUnavailableAndOperatorStates(t *testing.T) {
	tests := []struct {
		name       string
		denial     QuotaDenial
		wantClass  DenialClassification
		wantAction RecoveryAction
	}{
		{
			name: "quota exhaustion",
			denial: QuotaDenial{
				DenialID: "denial_quota", TenantID: fixtureTenantA, Category: CategoryRunLaunches,
				OperationKey: "tenant:ten_a:run:client_1", ReasonCode: "quota_denied:run_launches_exhausted",
				RequestedAmount: 1, RemainingAmount: 0, GuardedEntryPoint: "POST /v1/runs", CreatedAt: time.Now().UTC(),
			},
			wantClass:  DenialClassificationQuotaExhaustion,
			wantAction: RecoveryActionWait,
		},
		{
			name: "abuse restriction",
			denial: QuotaDenial{
				DenialID: "denial_abuse", TenantID: fixtureTenantA, Category: CategoryRuntimeToolCalls,
				OperationKey: "tenant:ten_a:tool_call:1", ReasonCode: "abuse_restriction:temporary",
				RequestedAmount: 1, RemainingAmount: 0, GuardedEntryPoint: "tool call creation", CreatedAt: time.Now().UTC(),
			},
			wantClass:  DenialClassificationAbuseRestriction,
			wantAction: RecoveryActionContactSupport,
		},
		{
			name: "quota state unavailable",
			denial: QuotaDenial{
				DenialID: "denial_unavailable", TenantID: fixtureTenantA, OperationKey: "tenant:ten_a:run:missing",
				ReasonCode: ReasonQuotaStateUnavailable, GuardedEntryPoint: "POST /v1/runs", CreatedAt: time.Now().UTC(),
			},
			wantClass:  DenialClassificationQuotaStateUnavailable,
			wantAction: RecoveryActionOperatorResolutionRequired,
		},
		{
			name: "operator action needed",
			denial: QuotaDenial{
				DenialID: "denial_operator", TenantID: fixtureTenantA, Category: CategoryRunLaunches,
				OperationKey: "tenant:ten_a:run:pending", ReasonCode: "quota_denied:operator_action_needed",
				RequestedAmount: 1, RemainingAmount: 0, GuardedEntryPoint: "POST /v1/runs", CreatedAt: time.Now().UTC(),
			},
			wantClass:  DenialClassificationOperatorActionNeeded,
			wantAction: RecoveryActionOperatorResolutionRequired,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detail := ProjectDenialDetail(tt.denial, nil)
			if detail.Classification != tt.wantClass {
				t.Fatalf("classification=%s, want %s: %+v", detail.Classification, tt.wantClass, detail)
			}
			if len(detail.RecoveryActions) == 0 || detail.RecoveryActions[0] != tt.wantAction {
				t.Fatalf("expected first recovery action %s, got %+v", tt.wantAction, detail.RecoveryActions)
			}
			if detail.OperationRef == "" || detail.OperationKey == "" {
				t.Fatalf("expected safe operation references, got %+v", detail)
			}
		})
	}
}

func TestProjectDenialDetailMapsSafeOperationReferencesForAllGuardedCategories(t *testing.T) {
	tests := map[Category]string{
		CategoryRunLaunches:              "tenant:ten_a:run:client_1",
		CategoryWorkflowLaunches:         "tenant:ten_a:workflow:run_1:workflow_1",
		CategoryRuntimeToolCalls:         "tenant:ten_a:tool_call:run_1:step_1:tool_1",
		CategoryLiveValidationAttempts:   "tenant:ten_a:live_validation:validation_1",
		CategoryIntegrationOperations:    "tenant:ten_a:integration:calendar:operation_1",
		CategoryArtifactStorageBytes:     "tenant:ten_a:artifact:artifact_1",
		CategoryReplayEvaluationAttempts: "tenant:ten_a:evaluation:candidate_1:attempt_1",
	}
	for category, operationKey := range tests {
		t.Run(string(category), func(t *testing.T) {
			definition, _ := DefinitionFor(category)
			detail := ProjectDenialDetail(QuotaDenial{
				DenialID:          "denial_" + string(category),
				TenantID:          fixtureTenantA,
				Category:          category,
				OperationKey:      operationKey,
				ReasonCode:        definition.DenialReasonCode,
				RequestedAmount:   1,
				RemainingAmount:   0,
				GuardedEntryPoint: definition.ReservationRule,
				CreatedAt:         time.Now().UTC(),
			}, nil)
			if detail.OperationRef == "" || detail.OperationRef == operationKey || detail.TenantID == "" {
				t.Fatalf("expected tenant-safe operation ref for %s, got %+v", category, detail)
			}
			if detail.Classification != DenialClassificationQuotaExhaustion {
				t.Fatalf("expected quota exhaustion for %s, got %s", category, detail.Classification)
			}
		})
	}
}

func TestBuildEvidenceExportRedactsSensitiveMetadata(t *testing.T) {
	denial := ProjectDenialDetail(QuotaDenial{
		DenialID: "denial_export", TenantID: fixtureTenantA, Category: CategoryRunLaunches,
		OperationKey: "tenant:ten_a:run:client_1", ReasonCode: "quota_denied:run_launches_exhausted",
		RequestedAmount: 1, RemainingAmount: 0, GuardedEntryPoint: "POST /v1/runs", CreatedAt: time.Now().UTC(),
	}, nil)
	export := BuildEvidenceExport("prn_support", denial, []QuotaStatusItem{{Category: CategoryRunLaunches}}, map[string]any{
		"secret":           "sk-live",
		"connectorPayload": map[string]any{"token": "raw"},
		"events":           []any{map[string]any{"accessToken": "raw-token"}},
		"safe":             "kept",
	})
	if export.SchemaVersion == "" || export.ExportID == "" || export.Denial.DenialID != denial.DenialID {
		t.Fatalf("expected structured export, got %+v", export)
	}
	if len(export.Redactions) < 2 {
		t.Fatalf("expected secret and connector payload redaction records, got %+v", export.Redactions)
	}
	if _, ok := export.EffectiveLimitState["safe"]; !ok {
		t.Fatalf("expected safe metadata to remain, got %+v", export.EffectiveLimitState)
	}
	if _, ok := export.EffectiveLimitState["secret"]; ok {
		t.Fatalf("secret field was not redacted: %+v", export.EffectiveLimitState)
	}
	if events, ok := export.EffectiveLimitState["events"].([]any); ok {
		if first, ok := events[0].(map[string]any); ok {
			if _, leaked := first["accessToken"]; leaked {
				t.Fatalf("nested token field was not redacted: %+v", export.EffectiveLimitState)
			}
		}
	}
}

func TestBuildEvidenceExportSupportsOrdinaryAndAbuseRestrictionDenials(t *testing.T) {
	ordinary := ProjectDenialDetail(QuotaDenial{
		DenialID: "denial_ordinary", TenantID: fixtureTenantA, Category: CategoryRunLaunches,
		OperationKey: "tenant:ten_a:run:client_1", ReasonCode: "quota_denied:run_launches_exhausted",
		RequestedAmount: 1, RemainingAmount: 0, GuardedEntryPoint: "POST /v1/runs", CreatedAt: time.Now().UTC(),
	}, nil)
	restriction := &AbuseRestrictionSummary{
		RestrictionID:     "restriction_1",
		Status:            AbuseRestrictionStatusActive,
		AffectedCategory:  CategoryRuntimeToolCalls,
		RecoveryAction:    RecoveryActionContactSupport,
		VisibleReasonCode: "abuse_restriction:temporary",
		SourceAuditRef:    "audit_abuse_1",
	}
	abuse := ProjectDenialDetail(QuotaDenial{
		DenialID: "denial_abuse_export", TenantID: fixtureTenantA, Category: CategoryRuntimeToolCalls,
		OperationKey: "tenant:ten_a:tool_call:1", ReasonCode: "abuse_restriction:temporary",
		RequestedAmount: 1, RemainingAmount: 0, GuardedEntryPoint: "tool call creation", CreatedAt: time.Now().UTC(),
	}, restriction)
	for _, denial := range []QuotaDenialDetail{ordinary, abuse} {
		export := BuildEvidenceExport("prn_support", denial, []QuotaStatusItem{{Category: denial.Category}}, map[string]any{"safe": "value"})
		if export.TenantID != fixtureTenantA || export.Denial.DenialID != denial.DenialID || len(export.UsageSnapshot) != 1 {
			t.Fatalf("unexpected export for %s: %+v", denial.DenialID, export)
		}
		if len(export.Redactions) == 0 {
			t.Fatalf("expected explicit redaction records for %s, got none", denial.DenialID)
		}
		if denial.Classification == DenialClassificationAbuseRestriction && len(export.AuditRefs) == 0 {
			t.Fatalf("expected abuse restriction audit ref, got %+v", export)
		}
	}
}

func TestManagerDenialDetailHydratesExplicitAbuseRestrictionRecord(t *testing.T) {
	now := time.Date(2026, 5, 7, 10, 0, 0, 0, time.UTC)
	expiresAt := now.Add(time.Hour)
	repo := newFixtureRepo(t, now.Add(-time.Hour))
	repo.denials = append(repo.denials, QuotaDenial{
		DenialID:          "denial_abuse_hydrate",
		TenantID:          fixtureTenantA,
		Category:          CategoryRuntimeToolCalls,
		OperationKey:      "tenant:ten_finite:tool_call:run_1:step_1:tool_1",
		ReasonCode:        "abuse_restriction:temporary",
		RequestedAmount:   1,
		RemainingAmount:   0,
		GuardedEntryPoint: "tool call creation",
		CreatedAt:         now,
	})
	repo.restrictions = append(repo.restrictions, AbuseRestrictionRecord{
		RestrictionID:         "restriction_runtime",
		TenantID:              fixtureTenantA,
		Status:                AbuseRestrictionStatusActive,
		AffectedCategory:      CategoryRuntimeToolCalls,
		RecoveryAction:        RecoveryActionContactSupport,
		VisibleReasonCode:     "abuse_restriction:temporary",
		SourceAuditRef:        "audit_runtime_restriction",
		SupportContactAllowed: true,
		StartedAt:             now.Add(-time.Minute),
		ExpiresAt:             &expiresAt,
		Document:              map[string]any{"detectionSignals": "not visible"},
	})
	manager := NewManagerWithClock(repo, func() time.Time { return now })

	detail, found, err := manager.DenialDetail(context.Background(), fixtureTenantA, "denial_abuse_hydrate")
	if err != nil || !found {
		t.Fatalf("DenialDetail found=%v err=%v", found, err)
	}
	if detail.Restriction == nil || detail.Restriction.RestrictionID != "restriction_runtime" || detail.Restriction.SourceAuditRef != "audit_runtime_restriction" || detail.Restriction.ExpiresAt == nil {
		t.Fatalf("expected hydrated explicit restriction, got %+v", detail.Restriction)
	}
}

func TestManagerEvidenceExportIncludesEffectiveLimitOverrideAndRestrictionState(t *testing.T) {
	now := time.Date(2026, 5, 7, 10, 0, 0, 0, time.UTC)
	expiresAt := now.Add(time.Hour)
	overrideLimit := int64(3)
	repo := newFixtureRepo(t, now.Add(-time.Hour))
	definition, _ := DefinitionFor(CategoryRuntimeToolCalls)
	period, _ := repo.OpenPeriod(context.Background(), fixtureTenantA, definition, now)
	repo.overrides[fixtureTenantA+":"+string(CategoryRuntimeToolCalls)] = QuotaOverride{
		QuotaOverrideID: "override_runtime",
		TenantID:        fixtureTenantA,
		Category:        CategoryRuntimeToolCalls,
		Limit:           &overrideLimit,
		Reason:          "temporary lowered limit",
		EffectiveAt:     now.Add(-time.Minute),
	}
	repo.counters[fixtureTenantA+":"+string(CategoryRuntimeToolCalls)+":"+period.QuotaPeriodID] = UsageCounter{
		TenantID:        fixtureTenantA,
		Category:        CategoryRuntimeToolCalls,
		QuotaPeriodID:   period.QuotaPeriodID,
		CommittedAmount: 3,
		UpdatedAt:       now,
	}
	repo.denials = append(repo.denials, QuotaDenial{
		DenialID:          "denial_export_state",
		TenantID:          fixtureTenantA,
		Category:          CategoryRuntimeToolCalls,
		QuotaPeriodID:     period.QuotaPeriodID,
		OperationKey:      "tenant:ten_finite:tool_call:run_1:step_1:tool_1",
		ReasonCode:        "abuse_restriction:temporary",
		RequestedAmount:   1,
		RemainingAmount:   0,
		GuardedEntryPoint: "tool call creation",
		CreatedAt:         now,
	})
	repo.restrictions = append(repo.restrictions, AbuseRestrictionRecord{
		RestrictionID:         "restriction_runtime",
		TenantID:              fixtureTenantA,
		Status:                AbuseRestrictionStatusActive,
		AffectedCategory:      CategoryRuntimeToolCalls,
		RecoveryAction:        RecoveryActionContactSupport,
		VisibleReasonCode:     "abuse_restriction:temporary",
		SourceAuditRef:        "audit_runtime_restriction",
		SupportContactAllowed: true,
		StartedAt:             now.Add(-time.Minute),
		ExpiresAt:             &expiresAt,
	})
	repo.events = append(repo.events, UsageEvent{
		UsageEventID:  "usage_event_denial",
		TenantID:      fixtureTenantA,
		Category:      CategoryRuntimeToolCalls,
		QuotaPeriodID: period.QuotaPeriodID,
		OperationKey:  "tenant:ten_finite:tool_call:run_1:step_1:tool_1",
		EventKind:     UsageEventDenial,
		ReasonCode:    "abuse_restriction:temporary",
		CreatedAt:     now,
	})
	manager := NewManagerWithClock(repo, func() time.Time { return now })

	export, found, err := manager.EvidenceExport(context.Background(), fixtureTenantA, "denial_export_state", "prn_support", true)
	if err != nil || !found {
		t.Fatalf("EvidenceExport found=%v err=%v", found, err)
	}
	quotaState, ok := export.EffectiveLimitState["quota"].(map[string]any)
	if !ok {
		t.Fatalf("expected quota effective limit state, got %+v", export.EffectiveLimitState)
	}
	if quotaState["category"] != string(CategoryRuntimeToolCalls) || quotaState["baseLimit"] == nil || quotaState["effectiveLimit"] == nil || quotaState["override"] == nil || quotaState["restriction"] == nil {
		t.Fatalf("incomplete quota effective limit state: %+v", quotaState)
	}
	if len(export.AuditRefs) < 3 {
		t.Fatalf("expected denial, restriction, and usage refs, got %+v", export.AuditRefs)
	}
}
