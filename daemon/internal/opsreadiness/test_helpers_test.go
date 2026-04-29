package opsreadiness

import (
	"strings"
	"testing"
	"time"
)

func assertValid(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("expected valid evidence: %v", err)
	}
}

func assertInvalidContains(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected validation error containing %q", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("expected validation error containing %q, got %v", want, err)
	}
}

func sampleTenantState() []TenantStateSummary {
	return []TenantStateSummary{
		{TenantID: "ten_ops_alpha", CredentialRefs: []string{"secretref_calendar_alpha"}, QuotaState: "finite_usage_10_of_100", WorkState: "runtime_and_delivery_completed"},
		{TenantID: "ten_ops_beta", CredentialRefs: []string{"secretref_mail_beta"}, QuotaState: "finite_usage_40_of_100", WorkState: "scheduled_work_pending"},
		{TenantID: "ten_ops_gamma", CredentialRefs: []string{"secretref_gamma_reconnect"}, QuotaState: "finite_usage_95_of_100", WorkState: "operator_action_needed", ReconnectRequired: true, OperatorActionNeeded: true},
	}
}

func sampleSoakReport() SoakReport {
	return SoakReport{
		ReportID:         "soak_r39",
		BranchOrVersion:  "024-production-ops-soak",
		Environment:      EnvironmentTest,
		DataDirectory:    "~/.dope-test",
		BaselineTopology: TopologyTenantScopedSingleNode,
		StartedAt:        time.Date(2026, 4, 29, 0, 0, 0, 0, time.UTC),
		CompletedAt:      time.Date(2026, 4, 30, 0, 1, 0, 0, time.UTC),
		Duration:         24*time.Hour + time.Minute,
		TenantSetSummary: sampleTenantState(),
		WorkloadCoverage: WorkloadCoverage{
			"runtime": true, "scheduler": true, "integrations": true, "delivery": true,
			"approvals": true, "quotas": true, "tenant_switching": true, "evaluation": true,
		},
		RestartEvents: []RestartEvent{
			{RestartID: "restart_1", UnfinishedWork: "run_1", Classification: ClassificationRecovered, RecoveryTime: time.Minute},
			{RestartID: "restart_2", UnfinishedWork: "schedule_1", Classification: ClassificationRetried, RecoveryTime: 2 * time.Minute},
			{RestartID: "restart_3", UnfinishedWork: "approval_1", Classification: ClassificationOperatorActionNeeded, RecoveryTime: 3 * time.Minute},
		},
		FaultDrillResults: []FaultDrillResult{
			{FaultType: "transient_5xx", Domain: "calendar", ObservedClassification: ClassificationRecovered},
			{FaultType: "rate_limit", Domain: "mail", ObservedClassification: ClassificationRecovered},
			{FaultType: "auth_expiry", Domain: "calendar", ObservedClassification: ClassificationOperatorActionNeeded},
			{FaultType: "provider_unavailable", Domain: "mail", ObservedClassification: ClassificationRetryExhausted, RetryExhausted: true, OperatorActionNeeded: true},
			{FaultType: "slow_response", Domain: "calendar", ObservedClassification: ClassificationRecovered},
			{FaultType: "malformed_response", Domain: "mail", ObservedClassification: ClassificationOperatorActionNeeded},
		},
		ResourceObservations: []ResourceObservation{
			{Category: "logs", Available: true, OperatorVisibility: "file size recorded"},
			{Category: "stored_data_size", Available: true, OperatorVisibility: "sqlite bytes recorded"},
			{Category: "active_work_or_queue_backlog", Available: true, QueueBacklogAge: 10 * time.Minute, OperatorVisibility: "queue backlog sampled"},
			{Category: "memory", Available: true, OperatorVisibility: "rss sampled"},
			{Category: "open_handles", Available: true, OperatorVisibility: "lsof sampled"},
			{Category: "goroutines", Available: true, OperatorVisibility: "runtime sampled"},
		},
		FinalResult: StatusPass,
	}
}
