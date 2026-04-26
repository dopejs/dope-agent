// Package migrationfixture builds reproducible SQLite fixtures for the
// Roadmap 35 US2 migration regression suite (T066). Build a fresh
// pre-tenant (schema v21) database via BuildPreTenantV21Fixture and
// then run the head migrations via store.SQLiteStore.ApplyHeadMigrations
// to exercise the backfill in T078–T081b.
//
// The fixture is built programmatically rather than checked in as a
// binary blob so it stays reproducible: any future schema or seeding
// change is reviewable as a code diff. The seeding intentionally
// covers at least one parent + one child per parent-child pair so the
// backfill drivers are exercised end-to-end.
package migrationfixture

import (
	"context"
	"fmt"

	"github.com/dopejs/dope-agent/daemon/internal/store"
)

// PreTenantSchemaVersion is the head schema version BEFORE any
// Roadmap 35 tenant migration applied. v22+ added tenant_id columns;
// v21 is the last "pre-tenant" head.
const PreTenantSchemaVersion = 21

// BuildPreTenantV21Fixture creates a fresh SQLite database in dataDir
// at schema version 21 and seeds at least one parent + one child row
// in every in-scope tenant_owned table. The returned store is open at
// v21 — callers run ApplyHeadMigrations on it to exercise the
// migrations + backfills.
//
// The seed set intentionally covers:
//   - sessions, runs, steps, tool_calls, llm_dispatches, checkpoints
//   - schedules + schedule_targets + schedule_dispatch_attempts
//   - workflows + workflow_steps + workflow_dependencies + workflow_handoffs
//   - integrations + delivery_targets + delivery_outcomes + delivery_attempts
//   - calendar/mail/reminders/computer-use accounts + ops + artifacts
//   - approvals + decisions
//   - evaluation_replay_candidates + replay_attempts
//   - mcp_servers + mcp_tools + mcp_tool_exposure_rules
//   - sandbox_executions, consumer_policy_records, provider_preferences,
//     secret_scope_bindings
//   - connector_messages
//   - events (one per category with appropriate parent FK)
//
// Schemas not yet present at v21 (added v22+) are silently skipped —
// the seeder only writes columns that exist at v21.
func BuildPreTenantV21Fixture(dataDir string) (*store.SQLiteStore, error) {
	s, err := store.NewSQLiteStoreAtVersion(dataDir, PreTenantSchemaVersion)
	if err != nil {
		return nil, fmt.Errorf("open v21 store: %w", err)
	}
	ctx := context.Background()
	if err := seedRuntime(ctx, s); err != nil {
		_ = s.Close()
		return nil, err
	}
	if err := seedSchedules(ctx, s); err != nil {
		_ = s.Close()
		return nil, err
	}
	if err := seedWorkflows(ctx, s); err != nil {
		_ = s.Close()
		return nil, err
	}
	if err := seedIntegrationsDelivery(ctx, s); err != nil {
		_ = s.Close()
		return nil, err
	}
	if err := seedCalendarMailReminders(ctx, s); err != nil {
		_ = s.Close()
		return nil, err
	}
	if err := seedComputerUse(ctx, s); err != nil {
		_ = s.Close()
		return nil, err
	}
	if err := seedApprovalsDecisions(ctx, s); err != nil {
		_ = s.Close()
		return nil, err
	}
	if err := seedEvaluation(ctx, s); err != nil {
		_ = s.Close()
		return nil, err
	}
	if err := seedHarness(ctx, s); err != nil {
		_ = s.Close()
		return nil, err
	}
	if err := seedConnectorMessages(ctx, s); err != nil {
		_ = s.Close()
		return nil, err
	}
	if err := seedEvents(ctx, s); err != nil {
		_ = s.Close()
		return nil, err
	}
	return s, nil
}

// SeedRowCounts captures the per-table row count snapshot used by the
// migration regression suite to assert pre/post counts are equal.
type SeedRowCounts map[string]int

// CountSeededRows returns the row count for every table the fixture
// seeded. Used by T078 to assert migration is loss-less.
func CountSeededRows(ctx context.Context, s *store.SQLiteStore) (SeedRowCounts, error) {
	tables := []string{
		"sessions", "runs", "steps", "tool_calls", "llm_dispatches", "checkpoints",
		"schedules", "schedule_targets", "schedule_dispatch_attempts",
		"workflows", "workflow_steps", "workflow_dependencies", "workflow_handoffs",
		"integrations", "delivery_targets", "delivery_outcomes", "delivery_attempts",
		"calendar_accounts", "calendar_operations", "calendar_artifacts",
		"mail_accounts", "mail_operations", "mail_artifacts",
		"reminders", "reminder_occurrences", "reminder_actions",
		"computer_use_sessions", "computer_use_actions", "computer_use_artifacts",
		"approvals", "decisions",
		"evaluation_replay_candidates", "evaluation_replay_attempts",
		"consumer_policy_records", "provider_preferences", "secret_scope_bindings", "sandbox_executions",
		"connector_messages", "events",
	}
	out := SeedRowCounts{}
	for _, t := range tables {
		var n int
		if err := s.DB().QueryRowContext(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s`, t)).Scan(&n); err != nil {
			return nil, fmt.Errorf("count %s: %w", t, err)
		}
		out[t] = n
	}
	return out, nil
}
