# Data Model: Production Operations Soak

This roadmap defines operational evidence objects. Implementation may persist them as
repo-managed fixtures, generated JSON reports, daemon test artifacts, or API/schema
resources if required by the chosen harness. If any object becomes a public daemon
resource, `schemas/`, contract tests, and docs must be updated together.

## Production Baseline

Represents the validated operating target for this phase.

Fields:

- `topology`: fixed to `tenant_scoped_single_node`.
- `environment`: `test` by default; live smoke requires explicit opt-in.
- `data_directory`: default test state path for local verification.
- `host_binding`: default test daemon binding.
- `out_of_scope`: multi-node rollout, clustering, distributed failover, payment launch,
  enterprise SSO, new integration domains, memory, and self-improvement.

Validation rules:

- Default validation must not use production user data.
- Any live connector or real-account use must be explicitly recorded.

## Install Runbook

Operator guidance for preparing, configuring, starting, and validating a new baseline
installation.

Fields:

- `prerequisites`
- `environment_setup`
- `configuration_inputs`
- `start_steps`
- `health_checks`
- `diagnostic_locations`
- `failure_modes`
- `rollback_or_cleanup`

Validation rules:

- A clean install must be confirmable in 60 minutes or less.
- Runbook must identify test vs production state paths and avoid implicit live connector
  use.

## Upgrade Runbook

Operator guidance for upgrading an existing baseline installation.

Fields:

- `source_version`
- `target_version`
- `preflight_checks`
- `backup_requirement`
- `migration_steps`
- `postflight_checks`
- `rollback_decision_points`
- `restore_required_conditions`

Validation rules:

- Upgrade verification must be complete in 90 minutes or less for a representative data
  set.
- Rollback guidance must state when restore from backup is the only safe path.
- Old binaries must not be pointed at incompatible newer schema versions.

## Backup Artifact

Recoverable snapshot material for tenant-scoped daemon state.

Fields:

- `artifact_id`
- `created_at`
- `source_version`
- `source_environment`
- `included_material`
- `excluded_material`
- `tenant_count`
- `tenant_state_summary`
- `integrity_checks`
- `compatibility_notes`

Validation rules:

- Must include at least three tenants with distinct credential, quota, and work states for
  representative verification.
- Must include secret metadata and references only.
- Must exclude raw credential material and derived credential material.
- Must include enough metadata to identify reconnect/revalidation work after restore.

## Restore Verification Result

Evidence that a backup was restored correctly.

Fields:

- `backup_artifact_id`
- `restore_environment`
- `restore_started_at`
- `restore_completed_at`
- `tenant_record_checks`
- `secret_reference_checks`
- `quota_state_checks`
- `work_state_checks`
- `cross_tenant_leakage_check`
- `credential_remediation_state`
- `result`

Validation rules:

- 100% of expected tenant-owned records must be available to the correct tenant.
- Cross-tenant leakage must be zero.
- Restored credential-bearing integrations must remain blocked until reconnect or
  revalidation.
- Invalid or incompatible backups must fail clearly before partial recovery is reported as
  success.

## Migration Verification Report

Preflight and postflight evidence for upgrade safety.

Fields:

- `source_version`
- `target_version`
- `preflight_checks`
- `postflight_checks`
- `migration_progress`
- `tenant_integrity_summary`
- `quota_accounting_summary`
- `rollback_path`
- `result`

Validation rules:

- Failed or incomplete migration steps must not be reported as successful upgrade.
- Orphaned or unbindable tenant state must be operator-visible.
- Backup-restore is the canonical rollback when persisted state cannot be reversed in
  place.

## Soak Scenario

Long-running workload definition.

Fields:

- `scenario_id`
- `duration_target`
- `environment`
- `tenant_set`
- `workload_mix`
- `restart_schedule`
- `fault_drill_set`
- `resource_observations`
- `report_destination`

Required workload coverage:

- runtime work
- scheduler dispatch
- integrations
- delivery
- approvals
- quota enforcement
- tenant switching
- evaluation/replay behavior

Validation rules:

- First baseline target duration is at least 24 hours.
- At least three daemon restarts are required.
- Production user data and live connectors are excluded unless explicitly opted in.

## Fault Drill

Controlled external-service failure case.

Fields:

- `fault_id`
- `domain`
- `fault_type`
- `injection_method`
- `expected_classification`
- `observed_classification`
- `retry_summary`
- `operator_action_needed_reason`

Allowed `fault_type` values:

- `transient_5xx`
- `rate_limit`
- `auth_expiry`
- `provider_unavailable`
- `slow_response`
- `malformed_response`

Allowed classifications:

- `recovered`
- `retry_exhausted`
- `operator_action_needed`

Validation rules:

- Every observed fault must be classified.
- Retry exhaustion without operator-action-needed state is a hard failure.

## Soak Report

Release evidence produced by a soak run.

Fields:

- `report_id`
- `started_at`
- `completed_at`
- `duration`
- `environment`
- `baseline_topology`
- `tenant_set_summary`
- `workload_coverage`
- `restart_events`
- `fault_drill_results`
- `recovery_time_summary`
- `retry_exhaustion_summary`
- `queue_backlog_summary`
- `resource_growth_summary`
- `cross_tenant_leakage_summary`
- `unclassified_failures`
- `real_account_smoke_summary`
- `overall_result`

Hard-fail rules:

- any cross-tenant leakage
- any unclassified failure
- restart recovery over 5 minutes
- retry exhaustion without operator-action-needed state
- queue backlog persisting over 30 minutes
- monotonic resource growth over the full run

## Real-Account Smoke Checklist

Opt-in evidence for supported integration domains.

Fields:

- `domain`
- `credential_source`
- `safety_review`
- `enabled`
- `skip_reason`
- `smoke_steps`
- `result`
- `redaction_check`

Validation rules:

- Fake-backend coverage remains mandatory.
- Missing safe credentials do not block readiness when fake-backend coverage passes and
  the skip reason is recorded.
- Smoke output must not expose credential material.

## Release Readiness Gate

Ship/no-ship decision record.

Fields:

- `install_result`
- `upgrade_result`
- `backup_restore_result`
- `migration_verification_result`
- `soak_report`
- `real_account_smoke_status`
- `roadmap_40_41_rerun_requirement`
- `operator_notes`
- `decision`

Validation rules:

- Missing required evidence blocks readiness.
- Failed hard thresholds block readiness.
- Roadmaps 40 and 41 cannot ship without rerunning the soak harness.
