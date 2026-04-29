package contracts_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	contracts "github.com/dopejs/dope-agent/daemon/internal/contracts"
	"github.com/dopejs/dope-agent/daemon/internal/livevalidation"
)

func liveValidationContractFixtures() map[string]string {
	return map[string]string{
		"schemas/api/live-validation-attempt-resource.schema.json":                 `{"validationId":"lv_1","tenantId":"ten_1","candidateId":"candidate_1","requestedBy":"prn_1","environmentScope":"test","requestedScope":{"scopeId":"scope_1","validationId":"lv_1","includedToolClasses":["daemon.inspection.read"],"approvalMode":"scope_level","declaredBy":"prn_1","declaredAt":"2026-04-29T10:00:00Z"},"status":"queued","permissionDecision":{"allowed":true,"checkedAt":"2026-04-29T10:00:00Z"},"quotaDecision":{"allowed":true,"checkedAt":"2026-04-29T10:00:00Z"},"killSwitchDecision":{"allowed":true,"checkedAt":"2026-04-29T10:00:00Z"},"approvalSummary":{"required":1,"approved":1,"denied":0,"expired":0,"pending":0},"ledgerSummary":{"attempted":1,"completed":1},"createdAt":"2026-04-29T10:00:00Z","updatedAt":"2026-04-29T10:00:01Z"}`,
		"schemas/api/live-validation-attempt-list.response.schema.json":            `{"tenantId":"ten_1","environmentScope":"test","items":[{"validationId":"lv_1","tenantId":"ten_1","candidateId":"candidate_1","requestedBy":"prn_1","environmentScope":"test","requestedScope":{"scopeId":"scope_1","validationId":"lv_1","approvalMode":"scope_level","declaredBy":"prn_1","declaredAt":"2026-04-29T10:00:00Z"},"status":"queued","permissionDecision":{"allowed":true,"checkedAt":"2026-04-29T10:00:00Z"},"quotaDecision":{"allowed":true,"checkedAt":"2026-04-29T10:00:00Z"},"killSwitchDecision":{"allowed":true,"checkedAt":"2026-04-29T10:00:00Z"},"approvalSummary":{"required":1,"approved":1,"denied":0,"expired":0,"pending":0},"ledgerSummary":{},"createdAt":"2026-04-29T10:00:00Z","updatedAt":"2026-04-29T10:00:01Z"}]}`,
		"schemas/api/live-validation-denial.schema.json":                           `{"reasonCode":"live_validation.permission_denied","gate":"permission","message":"Missing live validation permission.","validationId":"lv_1","checkedAt":"2026-04-29T10:00:00Z"}`,
		"schemas/api/live-validation-ledger-resource.schema.json":                  `{"ledgerEntryId":"ledger_1","validationId":"lv_1","tenantId":"ten_1","candidateId":"candidate_1","sourceRef":"tool_call_1","toolClass":"daemon.inspection.read","safetyClass":"read_only","actionRef":"action_1","outcome":"completed","attemptedAt":"2026-04-29T10:00:00Z","completedAt":"2026-04-29T10:00:01Z","updatedAt":"2026-04-29T10:00:01Z","evidenceRefs":["event_1"],"retryCount":0,"ambiguousCommit":false}`,
		"schemas/api/live-validation-ledger-list.response.schema.json":             `{"validationId":"lv_1","tenantId":"ten_1","items":[{"ledgerEntryId":"ledger_1","validationId":"lv_1","tenantId":"ten_1","candidateId":"candidate_1","sourceRef":"tool_call_1","toolClass":"daemon.inspection.read","safetyClass":"read_only","actionRef":"action_1","outcome":"completed","updatedAt":"2026-04-29T10:00:01Z","retryCount":0,"ambiguousCommit":false}]}`,
		"schemas/api/live-validation-support-matrix-resource.schema.json":          `{"toolClass":"mail.send","safetyClass":"non_idempotent_mutation","permission":"live_validation.execute","approval":"per_action","approvalAction":"live_validation.approve","idempotency":"message idempotency key where supported","retryPolicy":"no_retry","ambiguousCommitBehavior":"operator_action_needed","compensation":"manual_confirmation","ledgerEvents":["attempted","completed","failed","aborted","denied","operator_action_needed"],"testCase":"fake mail send ambiguous commit test","version":"v1"}`,
		"schemas/api/live-validation-support-matrix.response.schema.json":          `{"environmentScope":"test","version":"v1","items":[{"toolClass":"mcp.tool_call","safetyClass":"unsupported","approval":"unsupported","idempotency":"not available","retryPolicy":"no_retry","ambiguousCommitBehavior":"unsupported validation state","compensation":"unsupported","ledgerEvents":["skipped","denied"],"testCase":"MCP unsupported completeness test","version":"v1"}]}`,
		"schemas/api/live-validation-kill-switch-resource.schema.json":             `{"killSwitchId":"kill_1","scope":"tenant","tenantId":"ten_1","enabled":true,"reason":"operator containment","changedBy":"prn_owner","changedAt":"2026-04-29T10:00:00Z"}`,
		"schemas/api/live-validation-reconciliation-resource.schema.json":          `{"reconciliationId":"rec_1","ambiguousCommitId":"amb_1","tenantId":"ten_1","resolvedBy":"prn_owner","resolution":"confirmed_committed","reason":"provider state verified","evidenceRefs":["provider:event_1"],"resolvedAt":"2026-04-29T10:10:00Z"}`,
		"schemas/api/live-validation-retention-resource.schema.json":               `{"policyId":"ret_1","tenantId":"ten_1","appliesTo":"all","mode":"indefinite","createdByPrincipalId":"prn_owner","createdAt":"2026-04-29T10:00:00Z"}`,
		"schemas/api/live-validation-comparison-resource.schema.json":              `{"comparisonId":"cmp_1","validationId":"lv_1","candidateId":"candidate_1","baselineRef":"attempt_1","terminalStatus":"matched","ledgerSummary":{"completed":1},"unsupportedClasses":[],"denials":[],"ambiguousCommits":[],"driftFindings":[],"generatedAt":"2026-04-29T10:01:00Z"}`,
		"schemas/api/resolve-live-validation-reconciliation.request.schema.json":   `{"resolution":"confirmed_committed","reason":"provider state verified","evidenceRefs":["provider:event_1"]}`,
		"schemas/api/create-live-validation-comparison.response.schema.json":       `{"comparisonId":"cmp_1","validationId":"lv_1","candidateId":"candidate_1","baselineRef":"attempt_1","terminalStatus":"matched","ledgerSummary":{"completed":1},"generatedAt":"2026-04-29T10:01:00Z"}`,
		"schemas/api/update-live-validation-kill-switch.request.schema.json":       `{"scope":"tenant","tenantId":"ten_1","enabled":true,"reason":"containment"}`,
		"schemas/api/create-live-validation.request.schema.json":                   `{"validationId":"lv_1","candidateId":"candidate_1","candidateToolClasses":["daemon.inspection.read"],"requestedScope":{"scopeId":"scope_1","validationId":"lv_1","includedToolClasses":["daemon.inspection.read"],"approvalMode":"scope_level","declaredBy":"prn_1","declaredAt":"2026-04-29T10:00:00Z"}}`,
		"schemas/api/create-live-validation.response.schema.json":                  `{"attempt":{"validationId":"lv_1","candidateId":"candidate_1","requestedBy":"prn_1","environmentScope":"test","requestedScope":{"scopeId":"scope_1","validationId":"lv_1","approvalMode":"scope_level","declaredBy":"prn_1","declaredAt":"2026-04-29T10:00:00Z"},"status":"awaiting_approval","permissionDecision":{"allowed":true,"checkedAt":"2026-04-29T10:00:00Z"},"quotaDecision":{"allowed":true,"checkedAt":"2026-04-29T10:00:00Z"},"killSwitchDecision":{"allowed":true,"checkedAt":"2026-04-29T10:00:00Z"},"approvalSummary":{"required":1,"approved":0,"denied":0,"expired":0,"pending":1},"ledgerSummary":{},"createdAt":"2026-04-29T10:00:00Z","updatedAt":"2026-04-29T10:00:00Z"}}`,
		"schemas/events/live-validation-started.event.schema.json":                 `{"eventId":"evt_lv_1","sequence":1,"category":"evaluation","name":"live_validation.started","occurredAt":"2026-04-29T10:00:00Z","scope":{},"resource":{"kind":"live_validation","id":"lv_1"},"payload":{"validationId":"lv_1","tenantId":"ten_1","candidateId":"candidate_1","environmentScope":"test","status":"queued"}}`,
		"schemas/events/live-validation-blocked.event.schema.json":                 `{"eventId":"evt_lv_2","sequence":2,"category":"evaluation","name":"live_validation.blocked","occurredAt":"2026-04-29T10:00:00Z","scope":{},"resource":{"kind":"live_validation","id":"lv_1"},"payload":{"validationId":"lv_1","tenantId":"ten_1","candidateId":"candidate_1","gate":"permission","reasonCode":"live_validation.permission_denied","denials":[{"reasonCode":"live_validation.permission_denied","gate":"permission","message":"Missing live validation permission."}]}}`,
		"schemas/events/live-validation-awaiting-approval.event.schema.json":       `{"eventId":"evt_lv_awaiting","sequence":3,"category":"evaluation","name":"live_validation.awaiting_approval","occurredAt":"2026-04-29T10:00:00Z","scope":{},"resource":{"kind":"live_validation","id":"lv_1"},"payload":{"validationId":"lv_1","tenantId":"ten_1","candidateId":"candidate_1","environmentScope":"test","status":"awaiting_approval"}}`,
		"schemas/events/live-validation-side-effect-recorded.event.schema.json":    `{"eventId":"evt_lv_3","sequence":3,"category":"evaluation","name":"live_validation.side_effect_recorded","occurredAt":"2026-04-29T10:00:01Z","scope":{},"resource":{"kind":"live_validation_ledger_entry","id":"ledger_1"},"payload":{"validationId":"lv_1","tenantId":"ten_1","ledgerEntryId":"ledger_1","toolClass":"daemon.inspection.read","actionRef":"action_1","outcome":"completed","ambiguousCommit":false}}`,
		"schemas/events/live-validation-reconciliation-resolved.event.schema.json": `{"eventId":"evt_lv_4","sequence":4,"category":"evaluation","name":"live_validation.reconciliation_resolved","occurredAt":"2026-04-29T10:10:00Z","scope":{},"resource":{"kind":"live_validation_reconciliation","id":"rec_1"},"payload":{"validationId":"lv_1","tenantId":"ten_1","ambiguousCommitId":"amb_1","reconciliationId":"rec_1","resolution":"confirmed_committed","resolvedBy":"prn_owner"}}`,
		"schemas/events/live-validation-operator-action-needed.event.schema.json":  `{"eventId":"evt_lv_5","sequence":5,"category":"evaluation","name":"live_validation.operator_action_needed","occurredAt":"2026-04-29T10:10:00Z","scope":{},"resource":{"kind":"live_validation_ledger_entry","id":"ledger_1"},"payload":{"validationId":"lv_1","tenantId":"ten_1","ledgerEntryId":"ledger_1","reasonCode":"live_validation.ambiguous_commit"}}`,
		"schemas/events/live-validation-completed.event.schema.json":               `{"eventId":"evt_lv_6","sequence":6,"category":"evaluation","name":"live_validation.completed","occurredAt":"2026-04-29T10:10:00Z","scope":{},"resource":{"kind":"live_validation","id":"lv_1"},"payload":{"validationId":"lv_1","tenantId":"ten_1","status":"completed"}}`,
		"schemas/events/live-validation-comparison-completed.event.schema.json":    `{"eventId":"evt_lv_7","sequence":7,"category":"evaluation","name":"live_validation.comparison_completed","occurredAt":"2026-04-29T10:10:00Z","scope":{},"resource":{"kind":"live_validation_comparison","id":"cmp_1"},"payload":{"validationId":"lv_1","comparisonId":"cmp_1","terminalStatus":"matched"}}`,
		"schemas/events/live-validation-aborted.event.schema.json":                 `{"eventId":"evt_lv_8","sequence":8,"category":"evaluation","name":"live_validation.aborted","occurredAt":"2026-04-29T10:10:00Z","scope":{},"resource":{"kind":"live_validation","id":"lv_1"},"payload":{"validationId":"lv_1","tenantId":"ten_1","status":"aborted","reasonCode":"live_validation.kill_switch_aborted"}}`,
	}
}

func TestLiveValidationSchemasAcceptCanonicalFixtures(t *testing.T) {
	t.Parallel()

	validator := contracts.NewValidator(schemaRootDir(t))
	mustValidateFixtures(t, validator, liveValidationContractFixtures())
}

func TestLiveValidationPlanningContractsCoverSafetyRules(t *testing.T) {
	t.Parallel()

	root := contractRepoRoot(t)
	checks := map[string][]string{
		"specs/025-live-validation-replay/contracts/replay-support-matrix.md": {
			"Missing rows are treated as `unsupported`",
			"no automatic retry after submit-unknown",
		},
		"specs/025-live-validation-replay/contracts/side-effect-ledger.md": {
			"operator_action_needed",
			"tenant owner/admin or explicit reconciliation permission",
		},
		"specs/025-live-validation-replay/contracts/live-validation-surfaces.md": {
			"live validation",
			"kill switch",
		},
	}
	for rel, needles := range checks {
		body, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		text := string(body)
		for _, needle := range needles {
			if !strings.Contains(text, needle) {
				t.Fatalf("%s missing contract phrase %q", rel, needle)
			}
		}
	}
}

func TestLiveValidationDefaultSupportMatrixEnforcesContractInvariants(t *testing.T) {
	t.Parallel()

	matrix, err := livevalidation.NewMatrix(livevalidation.DefaultMatrixRows())
	if err != nil {
		t.Fatalf("NewMatrix(DefaultMatrixRows): %v", err)
	}
	if _, err := matrix.Lookup("unknown.tool"); !errors.Is(err, livevalidation.ErrMatrixRowMissing) {
		t.Fatalf("missing matrix row err=%v, want ErrMatrixRowMissing", err)
	}
	if _, err := matrix.Lookup(livevalidation.ToolClassMCPToolCall); !errors.Is(err, livevalidation.ErrMatrixRowUnsupported) {
		t.Fatalf("unsupported matrix row err=%v, want ErrMatrixRowUnsupported", err)
	}
	for _, row := range matrix.Rows() {
		if row.SafetyClass == livevalidation.SafetyClassNonIdempotentMutation && row.RetryPolicy == livevalidation.RetryPolicyAutomatic {
			t.Fatalf("non-idempotent row %s must not allow automatic retry", row.ToolClass)
		}
		if row.TestCase == "" {
			t.Fatalf("matrix row %s missing proving test", row.ToolClass)
		}
	}
}
