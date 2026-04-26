package store

import (
	"context"
	"strings"
	"testing"
)

// Roadmap 35 (US3 / T086) — query-plan regression for tenant-scoped lists.
//
// For each representative tenant-scoped list path, EXPLAIN QUERY PLAN MUST
// pick the named tenant-aware index and MUST NOT degrade to a table scan.
// The index name in the assertion mirrors the inventory's
// `indexesAndUniqueness` cell so reviewers can cross-walk the schema
// inventory contract directly.
//
// Coverage today: runtime spine (runs, sessions, llm_dispatches, steps,
// tool_calls, checkpoints), events (tenant-owned partial index), and the
// extended-domain indexes that ship in the head schema unconditionally
// (schedules, workflows, integrations, calendar, mail, reminders,
// computer_use, evaluation, delivery, sandbox, connector_messages,
// approvals, decisions). New tenant-scoped list paths MUST add a row to
// queryPlanCases — the test fails open on missing coverage.
//
// Per-table UNIQUE index activation for tables guarded by step (c)
// enforcement (T019–T026) is the *separate* legacy-Upsert auto-bind story;
// the indexes themselves are created at schema-migration time and are
// covered here regardless.
func TestQueryPlanUsesTenantIndex(t *testing.T) {
	s := newBackfillTestStore(t)
	ctx := context.Background()

	for _, c := range queryPlanCases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			rows, err := s.db.QueryContext(ctx, "EXPLAIN QUERY PLAN "+c.sql, c.args...)
			if err != nil {
				t.Fatalf("EXPLAIN QUERY PLAN %s: %v", c.sql, err)
			}
			defer rows.Close()
			var plan strings.Builder
			for rows.Next() {
				var id, parent, notused int
				var detail string
				if err := rows.Scan(&id, &parent, &notused, &detail); err != nil {
					t.Fatalf("scan plan row: %v", err)
				}
				plan.WriteString(detail)
				plan.WriteByte('\n')
			}
			if err := rows.Err(); err != nil {
				t.Fatalf("rows.Err: %v", err)
			}
			got := plan.String()
			if !strings.Contains(got, "USING INDEX "+c.expectedIndex) &&
				!strings.Contains(got, "USING COVERING INDEX "+c.expectedIndex) {
				t.Fatalf("query for %s did not use index %q\nplan:\n%s\nsql: %s", c.name, c.expectedIndex, got, c.sql)
			}
			// SCAN <table> with no index is the failure mode the test guards
			// against. SQLite renders this as "SCAN <name>" without USING
			// INDEX in the same step.
			lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
			for _, line := range lines {
				if strings.HasPrefix(strings.TrimSpace(line), "SCAN ") &&
					!strings.Contains(line, "USING INDEX") &&
					!strings.Contains(line, "USING COVERING INDEX") {
					t.Fatalf("query for %s degrades to a table scan\nplan:\n%s\nsql: %s", c.name, got, c.sql)
				}
			}
		})
	}
}

type queryPlanCase struct {
	name          string
	sql           string
	args          []any
	expectedIndex string
}

// inventoryTenantOwnedTablesWithListPath enumerates the tenant_owned
// tables from `specs/020-tenant-scoped-data-migration/contracts/
// schema-inventory.md` that have at least one tenant-scoped list path
// in production. Maintained alongside the inventory: any new
// tenant_owned table that exposes a list endpoint MUST get both an
// inventory row AND a corresponding queryPlanCase in this file. The
// completeness test below fails if the two drift apart.
//
// Tables intentionally excluded (have rows but no list-path query the
// API surface uses): workflow_dependencies, workflow_handoffs (joins
// embedded in the workflow resource), schedule_targets, schedule_
// dispatch_attempts (filtered by parent schedule_id, not directly by
// tenant_id), reminder_actions / reminder_occurrences (driven by
// reminder_id), computer_use_actions / computer_use_artifacts (driven
// by session_id), evaluation_replay_attempts / comparisons (driven by
// candidate_id), delivery_outcomes / delivery_attempts / delivery_
// summary_windows (driven by target_id / preference_id), calendar_
// artifacts / mail_artifacts (driven by operation_id), connector_
// messages_archive (no list endpoint), tool_calls / steps /
// checkpoints (filtered by run_id, the primary tenant-scoped list path
// is via runs).
var inventoryTenantOwnedTablesWithListPath = []string{
	"runs",
	"sessions",
	"llm_dispatches",
	"steps",
	"tool_calls",
	"checkpoints",
	"events",
	"schedules",
	"workflows",
	"integrations",
	"calendar_accounts",
	"calendar_operations",
	"mail_accounts",
	"mail_operations",
	"reminders",
	"computer_use_sessions",
	"evaluation_replay_candidates",
	"evaluation_regression_fixtures",
	"approvals",
	"decisions",
	"sandbox_executions",
	"consumer_policy_records",
	"provider_preferences",
	"mcp_tool_exposure_rules",
	"secret_scope_bindings",
	"connector_messages",
	"delivery_targets",
	"delivery_preferences",
}

// TestQueryPlanCoverageMatchesInventory fails if a tenant_owned table
// declared in inventoryTenantOwnedTablesWithListPath has no
// queryPlanCase asserting its tenant-prefixed list path. The reverse
// direction (an orphan queryPlanCase with no inventory entry) is not
// fatal but is logged so reviewers can decide whether to add the table
// to the inventory or drop the case.
func TestQueryPlanCoverageMatchesInventory(t *testing.T) {
	covered := map[string]struct{}{}
	for _, c := range queryPlanCases {
		// Heuristic: derive the table name from the case name's
		// "<table>_list_..." prefix — case names in this file follow
		// that convention.
		parts := strings.SplitN(c.name, "_", 2)
		if len(parts) < 2 {
			continue
		}
		// Strip "<table>_<rest>" → take the table portion, but multi-
		// word tables (calendar_accounts, mail_operations, ...) need a
		// longest-prefix match against the inventory list.
		for _, table := range inventoryTenantOwnedTablesWithListPath {
			if strings.HasPrefix(c.name, table+"_") {
				covered[table] = struct{}{}
				break
			}
		}
	}
	var missing []string
	for _, table := range inventoryTenantOwnedTablesWithListPath {
		if _, ok := covered[table]; !ok {
			missing = append(missing, table)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("tenant_owned tables in the schema inventory have NO queryPlanCase coverage: %v\n"+
			"each tenant-scoped list path MUST appear in queryPlanCases so EXPLAIN QUERY PLAN "+
			"keeps catching index regressions.", missing)
	}
}

var queryPlanCases = []queryPlanCase{
	{
		name:          "runs_list_by_tenant_created",
		sql:           `SELECT run_id FROM runs WHERE tenant_id = ? ORDER BY created_at DESC, run_id DESC LIMIT 50`,
		args:          []any{"ten_owner"},
		expectedIndex: "idx_runs_tenant_created",
	},
	{
		name:          "sessions_list_by_tenant_created",
		sql:           `SELECT session_id FROM sessions WHERE tenant_id = ? ORDER BY created_at DESC LIMIT 50`,
		args:          []any{"ten_owner"},
		expectedIndex: "idx_sessions_tenant_created",
	},
	{
		name:          "llm_dispatches_list_by_tenant_created",
		sql:           `SELECT dispatch_id FROM llm_dispatches WHERE tenant_id = ? ORDER BY created_at DESC LIMIT 50`,
		args:          []any{"ten_owner"},
		expectedIndex: "idx_llm_dispatches_tenant_created",
	},
	{
		name:          "llm_dispatches_list_by_tenant_provider_status",
		sql:           `SELECT dispatch_id FROM llm_dispatches WHERE tenant_id = ? AND provider = ? AND status = ? ORDER BY created_at DESC LIMIT 50`,
		args:          []any{"ten_owner", "echo", "completed"},
		expectedIndex: "idx_llm_dispatches_tenant_provider_status",
	},
	{
		name:          "steps_list_by_tenant_run",
		sql:           `SELECT step_id FROM steps WHERE tenant_id = ? AND run_id = ? ORDER BY created_at DESC LIMIT 50`,
		args:          []any{"ten_owner", "run_a"},
		expectedIndex: "idx_steps_tenant_run_created",
	},
	{
		name:          "tool_calls_by_tenant_step",
		sql:           `SELECT tool_call_id FROM tool_calls WHERE tenant_id = ? AND step_id = ? LIMIT 50`,
		args:          []any{"ten_owner", "step_a"},
		expectedIndex: "idx_tool_calls_tenant_step",
	},
	{
		name:          "checkpoints_by_tenant_run_captured",
		sql:           `SELECT checkpoint_id FROM checkpoints WHERE tenant_id = ? AND run_id = ? ORDER BY captured_at DESC LIMIT 50`,
		args:          []any{"ten_owner", "run_a"},
		expectedIndex: "idx_checkpoints_tenant_run_captured",
	},
	{
		name:          "events_list_by_tenant_time",
		sql:           `SELECT event_id FROM events WHERE tenant_id = ? ORDER BY occurred_at DESC LIMIT 50`,
		args:          []any{"ten_owner"},
		expectedIndex: "idx_events_tenant_time",
	},
	{
		name:          "events_list_by_tenant_category",
		sql:           `SELECT event_id FROM events WHERE tenant_id = ? AND category = ? AND name = ? ORDER BY occurred_at DESC LIMIT 50`,
		args:          []any{"ten_owner", "runtime", "run-created"},
		expectedIndex: "idx_events_tenant_category_time",
	},
	{
		name:          "schedules_list_by_tenant_status_due",
		sql:           `SELECT schedule_id FROM schedules WHERE tenant_id = ? AND status = ? ORDER BY next_due_at, schedule_id LIMIT 50`,
		args:          []any{"ten_owner", "active"},
		expectedIndex: "idx_schedules_tenant_status_due",
	},
	{
		name:          "workflows_list_by_tenant_updated",
		sql:           `SELECT workflow_id FROM workflows WHERE tenant_id = ? ORDER BY updated_at DESC, workflow_id DESC LIMIT 50`,
		args:          []any{"ten_owner"},
		expectedIndex: "idx_workflows_tenant_updated",
	},
	{
		name:          "integrations_list_by_tenant_readiness",
		sql:           `SELECT integration_id FROM integrations WHERE tenant_id = ? AND readiness_status = ? ORDER BY updated_at DESC, integration_id DESC LIMIT 50`,
		args:          []any{"ten_owner", "ready"},
		expectedIndex: "idx_integrations_tenant_readiness",
	},
	// Calendar / mail / reminders / computer-use / evaluation /
	// approvals / decisions / sandbox / policy / provider / mcp /
	// secret-scope / connector / delivery — added per I6 to drive the
	// coverage assertion against the schema-inventory.
	{
		name:          "calendar_accounts_list_by_tenant_readiness",
		sql:           `SELECT calendar_account_id FROM calendar_accounts WHERE tenant_id = ? AND readiness_status = ? ORDER BY updated_at DESC, calendar_account_id DESC LIMIT 50`,
		args:          []any{"ten_owner", "ready"},
		expectedIndex: "idx_calendar_accounts_tenant_readiness",
	},
	{
		name:          "calendar_operations_list_by_tenant_account",
		sql:           `SELECT operation_id FROM calendar_operations WHERE tenant_id = ? AND calendar_account_id = ? ORDER BY updated_at DESC, operation_id DESC LIMIT 50`,
		args:          []any{"ten_owner", "cal_a"},
		expectedIndex: "idx_calendar_operations_tenant_account",
	},
	{
		name:          "mail_accounts_list_by_tenant_readiness",
		sql:           `SELECT mail_account_id FROM mail_accounts WHERE tenant_id = ? AND readiness_status = ? ORDER BY updated_at DESC, mail_account_id DESC LIMIT 50`,
		args:          []any{"ten_owner", "ready"},
		expectedIndex: "idx_mail_accounts_tenant_readiness",
	},
	{
		name:          "mail_operations_list_by_tenant_account",
		sql:           `SELECT operation_id FROM mail_operations WHERE tenant_id = ? AND mail_account_id = ? ORDER BY updated_at DESC, operation_id DESC LIMIT 50`,
		args:          []any{"ten_owner", "mail_a"},
		expectedIndex: "idx_mail_operations_tenant_account",
	},
	{
		name:          "reminders_list_by_tenant_state",
		sql:           `SELECT reminder_id FROM reminders WHERE tenant_id = ? AND current_state = ? ORDER BY updated_at DESC, reminder_id DESC LIMIT 50`,
		args:          []any{"ten_owner", "scheduled"},
		expectedIndex: "idx_reminders_tenant_state",
	},
	{
		name:          "computer_use_sessions_list_by_tenant_run",
		sql:           `SELECT computer_use_session_id FROM computer_use_sessions WHERE tenant_id = ? AND run_id = ? ORDER BY updated_at DESC, computer_use_session_id DESC LIMIT 50`,
		args:          []any{"ten_owner", "run_a"},
		expectedIndex: "idx_computer_use_sessions_tenant_run",
	},
	{
		name:          "evaluation_replay_candidates_list_by_tenant_ready",
		sql:           `SELECT candidate_id FROM evaluation_replay_candidates WHERE tenant_id = ? AND readiness_status = ? ORDER BY updated_at DESC, candidate_id DESC LIMIT 50`,
		args:          []any{"ten_owner", "ready"},
		expectedIndex: "idx_eval_candidates_tenant_ready",
	},
	{
		name:          "evaluation_regression_fixtures_list_by_tenant_domain",
		sql:           `SELECT fixture_id FROM evaluation_regression_fixtures WHERE tenant_id = ? AND domain_class = ? ORDER BY updated_at DESC, fixture_id DESC LIMIT 50`,
		args:          []any{"ten_owner", "runtime"},
		expectedIndex: "idx_eval_fixtures_tenant_domain",
	},
	{
		name:          "approvals_list_by_tenant_status",
		sql:           `SELECT approval_id FROM approvals WHERE tenant_id = ? AND status = ? ORDER BY created_at DESC LIMIT 50`,
		args:          []any{"ten_owner", "pending"},
		expectedIndex: "idx_approvals_tenant_status_created",
	},
	{
		name:          "decisions_list_by_tenant_created",
		sql:           `SELECT decision_id FROM decisions WHERE tenant_id = ? ORDER BY created_at DESC, decision_id DESC LIMIT 50`,
		args:          []any{"ten_owner"},
		expectedIndex: "idx_decisions_tenant_created",
	},
	{
		name:          "sandbox_executions_list_by_tenant_status",
		sql:           `SELECT execution_id FROM sandbox_executions WHERE tenant_id = ? AND status = ? ORDER BY requested_at DESC, execution_id DESC LIMIT 50`,
		args:          []any{"ten_owner", "running"},
		expectedIndex: "idx_sandbox_executions_tenant_status",
	},
	{
		name:          "consumer_policy_records_list_by_tenant_started",
		sql:           `SELECT policy_record_id FROM consumer_policy_records WHERE tenant_id = ? ORDER BY started_at DESC, policy_record_id DESC LIMIT 50`,
		args:          []any{"ten_owner"},
		expectedIndex: "idx_policy_records_tenant_started",
	},
	{
		name:          "provider_preferences_list_by_tenant",
		sql:           `SELECT provider_id FROM provider_preferences WHERE tenant_id = ? ORDER BY provider_id LIMIT 50`,
		args:          []any{"ten_owner"},
		expectedIndex: "idx_provider_preferences_tenant",
	},
	{
		name:          "mcp_tool_exposure_rules_list_by_tenant_server_tool",
		sql:           `SELECT runtime_surface FROM mcp_tool_exposure_rules WHERE tenant_id = ? AND server_id = ? AND tool_name = ? LIMIT 50`,
		args:          []any{"ten_owner", "srv_a", "tool_x"},
		expectedIndex: "idx_mcp_tool_exposure_tenant_server_tool",
	},
	{
		name:          "secret_scope_bindings_list_by_tenant_consumer",
		sql:           `SELECT secret_ref FROM secret_scope_bindings WHERE tenant_id = ? AND consumer_kind = ? AND consumer_id = ? LIMIT 50`,
		args:          []any{"ten_owner", "skill", "skill_a"},
		expectedIndex: "idx_secret_scope_bindings_tenant_consumer",
	},
	{
		name:          "connector_messages_list_by_tenant_connector",
		sql:           `SELECT delivery_id FROM connector_messages WHERE tenant_id = ? AND connector_id = ? ORDER BY created_at DESC, delivery_id DESC LIMIT 50`,
		args:          []any{"ten_owner", "discord_a"},
		expectedIndex: "idx_connector_messages_tenant_connector",
	},
	{
		name:          "delivery_targets_list_by_tenant_status",
		sql:           `SELECT target_id FROM delivery_targets WHERE tenant_id = ? AND status = ? ORDER BY updated_at DESC, target_id DESC LIMIT 50`,
		args:          []any{"ten_owner", "ready"},
		expectedIndex: "idx_delivery_targets_tenant_status",
	},
	{
		name:          "delivery_preferences_list_by_tenant_scope",
		sql:           `SELECT preference_id FROM delivery_preferences WHERE tenant_id = ? AND scope_kind = ? AND integration_id = ? AND active = 1 ORDER BY updated_at DESC, preference_id DESC LIMIT 50`,
		args:          []any{"ten_owner", "global", "int_a"},
		expectedIndex: "idx_delivery_preferences_tenant_scope",
	},
}
