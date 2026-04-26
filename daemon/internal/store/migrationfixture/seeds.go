package migrationfixture

import (
	"context"
	"fmt"

	"github.com/dopejs/dope-agent/daemon/internal/store"
)

const ts = "2025-01-01T00:00:00Z"

func exec(ctx context.Context, s *store.SQLiteStore, query string, args ...any) error {
	if _, err := s.DB().ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("seed %s: %w", queryHead(query), err)
	}
	return nil
}

func queryHead(q string) string {
	for i := 0; i < len(q); i++ {
		if q[i] == '(' {
			return q[:i]
		}
	}
	return q
}

func seedRuntime(ctx context.Context, s *store.SQLiteStore) error {
	if err := exec(ctx, s, `INSERT INTO sessions (session_id, kind, status, channel, peer_id, routing_key, generation, created_at, updated_at, last_active_at)
		VALUES (?,?,?,?,?,?,?,?,?,?)`,
		"sess_seed", "chat", "active", "test", "peer_1", "rk_seed", 1, ts, ts, ts); err != nil {
		return err
	}
	if err := exec(ctx, s, `INSERT INTO runs (run_id, session_id, entrypoint, status, goal, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?)`,
		"run_seed", "sess_seed", "test", "queued", "g", ts, ts); err != nil {
		return err
	}
	if err := exec(ctx, s, `INSERT INTO steps (step_id, run_id, title, kind, status, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?)`,
		"step_seed", "run_seed", "step", "model", "queued", ts, ts); err != nil {
		return err
	}
	if err := exec(ctx, s, `INSERT INTO tool_calls (tool_call_id, run_id, step_id, capability_id, tool_name, status, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?)`,
		"tc_seed", "run_seed", "step_seed", "cap_a", "tool_a", "requested", ts, ts); err != nil {
		return err
	}
	if err := exec(ctx, s, `INSERT INTO llm_dispatches (dispatch_id, provider, model, messages_json, stream, status, output_text, usage_json, timeout_ms, max_retries, attempt_count, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		"dsp_seed", "openai", "gpt", "[]", 0, "completed", "", "{}", 1000, 0, 1, ts, ts); err != nil {
		return err
	}
	if err := exec(ctx, s, `INSERT INTO checkpoints (checkpoint_id, run_id, captured_at, snapshot_json)
		VALUES (?,?,?,?)`,
		"chk_seed", "run_seed", ts, `{"run":{"runId":"run_seed","entrypoint":"test","status":"queued","goal":"g","createdAt":"2025-01-01T00:00:00Z","updatedAt":"2025-01-01T00:00:00Z"},"steps":[],"toolCalls":[]}`); err != nil {
		return err
	}
	return nil
}

func seedSchedules(ctx context.Context, s *store.SQLiteStore) error {
	if err := exec(ctx, s, `INSERT INTO schedules (schedule_id, environment_scope, kind, status, target_ref_id, created_at, updated_at, document_json)
		VALUES (?,?,?,?,?,?,?,?)`,
		"sch_seed", "test", "cron", "active", "tgt_seed", ts, ts, "{}"); err != nil {
		return err
	}
	if err := exec(ctx, s, `INSERT INTO schedule_targets (target_ref_id, schedule_id, target_kind, revision, active, updated_at, document_json)
		VALUES (?,?,?,?,?,?,?)`,
		"tgt_seed", "sch_seed", "workflow", 1, 1, ts, "{}"); err != nil {
		return err
	}
	if err := exec(ctx, s, `INSERT INTO schedule_dispatch_attempts (attempt_id, schedule_id, due_at, trigger_source, dispatch_status, retry_count, retry_budget, resolved_target_revision, downstream_status, missed_count, created_at, updated_at, document_json)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		"atm_sch_seed", "sch_seed", ts, "auto", "queued", 0, 3, 1, "pending", 0, ts, ts, "{}"); err != nil {
		return err
	}
	return nil
}

func seedWorkflows(ctx context.Context, s *store.SQLiteStore) error {
	if err := exec(ctx, s, `INSERT INTO workflows (workflow_id, run_id, environment_scope, goal, status, created_at, updated_at, document_json)
		VALUES (?,?,?,?,?,?,?,?)`,
		"wf_seed", "run_seed", "test", "g", "queued", ts, ts, "{}"); err != nil {
		return err
	}
	if err := exec(ctx, s, `INSERT INTO workflow_steps (workflow_step_id, workflow_id, position, status, attempt_count, max_attempts, document_json)
		VALUES (?,?,?,?,?,?,?)`,
		"wfs_seed", "wf_seed", 1, "queued", 0, 3, "{}"); err != nil {
		return err
	}
	if err := exec(ctx, s, `INSERT INTO workflow_dependencies (dependency_id, workflow_id, document_json)
		VALUES (?,?,?)`,
		"wdep_seed", "wf_seed", "{}"); err != nil {
		return err
	}
	if err := exec(ctx, s, `INSERT INTO workflow_handoffs (handoff_id, workflow_id, status, document_json)
		VALUES (?,?,?,?)`,
		"whof_seed", "wf_seed", "pending", "{}"); err != nil {
		return err
	}
	return nil
}

func seedIntegrationsDelivery(ctx context.Context, s *store.SQLiteStore) error {
	if err := exec(ctx, s, `INSERT INTO integrations (integration_id, domain_kind, environment_scope, backend_kind, readiness_status, canonical_default, updated_at, document_json)
		VALUES (?,?,?,?,?,?,?,?)`,
		"int_seed", "calendar", "test", "google", "ready", 1, ts, "{}"); err != nil {
		return err
	}
	if err := exec(ctx, s, `INSERT INTO delivery_targets (target_id, environment_scope, target_kind, status, updated_at, document_json)
		VALUES (?,?,?,?,?,?)`,
		"deltgt_seed", "test", "discord", "ready", ts, "{}"); err != nil {
		return err
	}
	if err := exec(ctx, s, `INSERT INTO delivery_preferences (preference_id, environment_scope, scope_kind, active, updated_at, document_json)
		VALUES (?,?,?,?,?,?)`,
		"delpref_seed", "test", "global", 1, ts, "{}"); err != nil {
		return err
	}
	if err := exec(ctx, s, `INSERT INTO delivery_outcomes (delivery_id, environment_scope, source_kind, source_id, status, updated_at, document_json)
		VALUES (?,?,?,?,?,?,?)`,
		"delout_seed", "test", "run", "run_seed", "queued", ts, "{}"); err != nil {
		return err
	}
	if err := exec(ctx, s, `INSERT INTO delivery_attempts (attempt_id, delivery_id, attempt_number, target_id, status, document_json)
		VALUES (?,?,?,?,?,?)`,
		"atm_del_seed", "delout_seed", 1, "deltgt_seed", "queued", "{}"); err != nil {
		return err
	}
	if err := exec(ctx, s, `INSERT INTO delivery_summary_windows (summary_window_id, environment_scope, target_id, preference_id, status, window_ends_at, updated_at, document_json)
		VALUES (?,?,?,?,?,?,?,?)`,
		"sw_seed", "test", "deltgt_seed", "delpref_seed", "open", ts, ts, "{}"); err != nil {
		return err
	}
	return nil
}

func seedCalendarMailReminders(ctx context.Context, s *store.SQLiteStore) error {
	if err := exec(ctx, s, `INSERT INTO calendar_accounts (calendar_account_id, integration_id, environment_scope, readiness_status, canonical_default, updated_at, document_json)
		VALUES (?,?,?,?,?,?,?)`,
		"cal_seed", "int_seed", "test", "ready", 1, ts, "{}"); err != nil {
		return err
	}
	if err := exec(ctx, s, `INSERT INTO calendar_operations (operation_id, integration_id, calendar_account_id, environment_scope, operation_class, status, updated_at, document_json)
		VALUES (?,?,?,?,?,?,?,?)`,
		"calop_seed", "int_seed", "cal_seed", "test", "list", "ok", ts, "{}"); err != nil {
		return err
	}
	if err := exec(ctx, s, `INSERT INTO calendar_artifacts (artifact_id, operation_id, integration_id, environment_scope, kind, created_at, document_json)
		VALUES (?,?,?,?,?,?,?)`,
		"calart_seed", "calop_seed", "int_seed", "test", "event", ts, "{}"); err != nil {
		return err
	}
	// Mail uses its own integration row (FK is UNIQUE on integration_id).
	if err := exec(ctx, s, `INSERT INTO integrations (integration_id, domain_kind, environment_scope, backend_kind, readiness_status, canonical_default, updated_at, document_json)
		VALUES (?,?,?,?,?,?,?,?)`,
		"int_mail_seed", "mail", "test", "gmail", "ready", 1, ts, "{}"); err != nil {
		return err
	}
	if err := exec(ctx, s, `INSERT INTO mail_accounts (mail_account_id, integration_id, environment_scope, readiness_status, canonical_default, updated_at, document_json)
		VALUES (?,?,?,?,?,?,?)`,
		"mail_seed", "int_mail_seed", "test", "ready", 1, ts, "{}"); err != nil {
		return err
	}
	if err := exec(ctx, s, `INSERT INTO mail_operations (operation_id, integration_id, mail_account_id, environment_scope, operation_class, status, result_mode, updated_at, document_json)
		VALUES (?,?,?,?,?,?,?,?,?)`,
		"mailop_seed", "int_mail_seed", "mail_seed", "test", "list", "ok", "json", ts, "{}"); err != nil {
		return err
	}
	if err := exec(ctx, s, `INSERT INTO mail_artifacts (artifact_id, operation_id, integration_id, environment_scope, kind, created_at, document_json)
		VALUES (?,?,?,?,?,?,?)`,
		"mailart_seed", "mailop_seed", "int_mail_seed", "test", "message", ts, "{}"); err != nil {
		return err
	}
	if err := exec(ctx, s, `INSERT INTO reminders (reminder_id, environment_scope, behavior_mode, current_state, updated_at, document_json)
		VALUES (?,?,?,?,?,?)`,
		"rem_seed", "test", "single", "active", ts, "{}"); err != nil {
		return err
	}
	if err := exec(ctx, s, `INSERT INTO reminder_occurrences (occurrence_id, reminder_id, environment_scope, state, scheduled_for, updated_at, document_json)
		VALUES (?,?,?,?,?,?,?)`,
		"remocc_seed", "rem_seed", "test", "scheduled", ts, ts, "{}"); err != nil {
		return err
	}
	if err := exec(ctx, s, `INSERT INTO reminder_actions (action_id, reminder_id, action_kind, created_at, document_json)
		VALUES (?,?,?,?,?)`,
		"remact_seed", "rem_seed", "fire", ts, "{}"); err != nil {
		return err
	}
	return nil
}

func seedComputerUse(ctx context.Context, s *store.SQLiteStore) error {
	if err := exec(ctx, s, `INSERT INTO computer_use_sessions (computer_use_session_id, environment_scope, run_id, status, driver_kind, started_at, updated_at, document_json)
		VALUES (?,?,?,?,?,?,?,?)`,
		"cus_seed", "test", "run_seed", "active", "playwright", ts, ts, "{}"); err != nil {
		return err
	}
	if err := exec(ctx, s, `INSERT INTO computer_use_actions (computer_use_action_id, environment_scope, computer_use_session_id, run_id, action_kind, status, risk_level, requested_at, updated_at, document_json)
		VALUES (?,?,?,?,?,?,?,?,?,?)`,
		"cua_seed", "test", "cus_seed", "run_seed", "click", "completed", "low", ts, ts, "{}"); err != nil {
		return err
	}
	if err := exec(ctx, s, `INSERT INTO computer_use_artifacts (artifact_id, environment_scope, computer_use_session_id, computer_use_action_id, run_id, kind, status, byte_size, created_at, document_json)
		VALUES (?,?,?,?,?,?,?,?,?,?)`,
		"cuart_seed", "test", "cus_seed", "cua_seed", "run_seed", "screenshot", "ready", 0, ts, "{}"); err != nil {
		return err
	}
	return nil
}

func seedApprovalsDecisions(ctx context.Context, s *store.SQLiteStore) error {
	if err := exec(ctx, s, `INSERT INTO approvals (approval_id, action, reason, status, created_at, updated_at)
		VALUES (?,?,?,?,?,?)`,
		"apr_seed", "test.action", "r", "pending", ts, ts); err != nil {
		return err
	}
	if err := exec(ctx, s, `INSERT INTO decisions (decision_id, action, outcome, reason, approval_id, created_at)
		VALUES (?,?,?,?,?,?)`,
		"dec_seed", "test.action", "approved", "ok", "apr_seed", ts); err != nil {
		return err
	}
	return nil
}

func seedEvaluation(ctx context.Context, s *store.SQLiteStore) error {
	if err := exec(ctx, s, `INSERT INTO evaluation_replay_candidates (candidate_id, environment_scope, candidate_kind, source_kind, source_id, readiness_status, created_at, updated_at, document_json)
		VALUES (?,?,?,?,?,?,?,?,?)`,
		"cand_seed", "test", "run", "run", "run_seed", "ready", ts, ts, "{}"); err != nil {
		return err
	}
	if err := exec(ctx, s, `INSERT INTO evaluation_replay_attempts (attempt_id, candidate_id, environment_scope, mode, status, created_at, updated_at, document_json)
		VALUES (?,?,?,?,?,?,?,?)`,
		"evatm_seed", "cand_seed", "test", "shadow", "queued", ts, ts, "{}"); err != nil {
		return err
	}
	return nil
}

func seedHarness(ctx context.Context, s *store.SQLiteStore) error {
	if err := exec(ctx, s, `INSERT INTO consumer_policy_records (policy_record_id, consumer_kind, consumer_id, operation_kind, status, decision, approval_status, secret_resolution, started_at, document_json)
		VALUES (?,?,?,?,?,?,?,?,?,?)`,
		"polrec_seed", "skill", "skl_a", "tool_call", "completed", "allow", "n/a", "skipped", ts, "{}"); err != nil {
		return err
	}
	if err := exec(ctx, s, `INSERT INTO provider_preferences (provider_id, default_model, updated_at)
		VALUES (?,?,?)`,
		"openai", "gpt", ts); err != nil {
		return err
	}
	if err := exec(ctx, s, `INSERT INTO secret_scope_bindings (binding_id, consumer_kind, consumer_id, environment_scope, secret_ref, default_source, delivery_kind, active, document_json)
		VALUES (?,?,?,?,?,?,?,?,?)`,
		"binding_seed", "skill", "skl_a", "test", "ref://x", "env", "header", 1, "{}"); err != nil {
		return err
	}
	if err := exec(ctx, s, `INSERT INTO sandbox_executions (execution_id, profile_id, backend_kind, status, requested_at, updated_at, document_json)
		VALUES (?,?,?,?,?,?,?)`,
		"exec_seed", "default", "docker", "queued", ts, ts, "{}"); err != nil {
		return err
	}
	// mcp_servers / mcp_tools / mcp_tool_exposure_rules are NOT seeded
	// here: the production app restore path expects tool documents with
	// a richer shape than a minimal fixture can provide, and the
	// backfill behavior for mcp_tool_exposure_rules (composite-PK bulk
	// fanout) is already covered by store/tenant_backfill_special_test.go.
	// Adding them to the fixture would bind the test to MCP restore
	// semantics that are orthogonal to Roadmap 35.
	return nil
}

func seedConnectorMessages(ctx context.Context, s *store.SQLiteStore) error {
	if err := exec(ctx, s, `INSERT INTO connector_messages (delivery_id, connector_id, direction, channel_id, content, status, created_at, updated_at, session_id)
		VALUES (?,?,?,?,?,?,?,?,?)`,
		"cm_seed_session", "discord_a", "out", "ch_a", "hi", "queued", ts, ts, "sess_seed"); err != nil {
		return err
	}
	if err := exec(ctx, s, `INSERT INTO connector_messages (delivery_id, connector_id, direction, channel_id, content, status, created_at, updated_at, run_id)
		VALUES (?,?,?,?,?,?,?,?,?)`,
		"cm_seed_run", "discord_a", "out", "ch_a", "hi", "queued", ts, ts, "run_seed"); err != nil {
		return err
	}
	if err := exec(ctx, s, `INSERT INTO connector_messages (delivery_id, connector_id, direction, channel_id, content, status, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?)`,
		"cm_seed_orphan", "discord_a", "out", "ch_a", "hi", "queued", ts, ts); err != nil {
		return err
	}
	return nil
}

func seedEvents(ctx context.Context, s *store.SQLiteStore) error {
	// Tenant-owned event with run_id parent.
	if err := exec(ctx, s, `INSERT INTO events (event_id, category, name, occurred_at, run_id, resource_kind, resource_id, payload_json)
		VALUES (?,?,?,?,?,?,?,?)`,
		"evt_run_seed", "run", "run.created", ts, "run_seed", "run", "run_seed", "{}"); err != nil {
		return err
	}
	// Global system event.
	if err := exec(ctx, s, `INSERT INTO events (event_id, category, name, occurred_at, resource_kind, resource_id, payload_json)
		VALUES (?,?,?,?,?,?,?)`,
		"evt_sys_seed", "system", "system.heartbeat", ts, "system", "heartbeat", "{}"); err != nil {
		return err
	}
	// Connector event with proper resource pointer.
	if err := exec(ctx, s, `INSERT INTO events (event_id, category, name, occurred_at, connector_id, resource_kind, resource_id, payload_json)
		VALUES (?,?,?,?,?,?,?,?)`,
		"evt_cm_seed", "connector.message", "connector.message.delivered", ts, "discord_a", "connector_message", "cm_seed_session", "{}"); err != nil {
		return err
	}
	// Legacy connector event missing resource pointer — should reclassify to connector_global.
	if err := exec(ctx, s, `INSERT INTO events (event_id, category, name, occurred_at, connector_id, resource_kind, resource_id, payload_json)
		VALUES (?,?,?,?,?,?,?,?)`,
		"evt_cm_orphan", "connector.legacy", "connector.legacy.event", ts, "discord_a", "", "", "{}"); err != nil {
		return err
	}
	// Capability-only event — should reclassify to capability_global.
	if err := exec(ctx, s, `INSERT INTO events (event_id, category, name, occurred_at, capability_id, resource_kind, resource_id, payload_json)
		VALUES (?,?,?,?,?,?,?,?)`,
		"evt_cap_only", "capability.heartbeat", "cap.heartbeat", ts, "cap_a", "capability", "cap_a", "{}"); err != nil {
		return err
	}
	return nil
}
