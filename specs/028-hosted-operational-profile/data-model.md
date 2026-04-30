# Data Model: Hosted Operational Profile And Recovery

## Hosted Operational Profile

Represents the named hosted/test-host operating contract.

**Fields**:

- `profile_id`: stable profile identifier.
- `profile_name`: human-readable name.
- `environment`: `test` by default; production-like only with explicit operator choice.
- `host_class`: stable test host, VPS, developer laptop, or unsupported host class.
- `data_directory`: daemon state location.
- `log_directory`: log location.
- `artifact_directory`: generated evidence root.
- `backup_directory`: backup artifact root.
- `report_directory`: report and release-index root.
- `temporary_directory`: temporary work root.
- `live_connector_mode`: disabled by default or explicitly opted in.
- `retention_days`: defaults to 90 unless an authorized policy overrides it.

**Validation rules**:

- Default profile must not point at production user data.
- Directory paths must be stable and must not overwrite evidence from prior run ids.
- Developer laptop host class cannot satisfy release-readiness stable-host evidence.

## Hosted Run

Represents one execution of the hosted profile.

**Fields**:

- `run_id`: unique identity for a hosted profile run.
- `profile_id`: associated hosted profile.
- `commit_or_version`: reviewed commit, branch, or version.
- `host`: host identity.
- `operator`: operator identity.
- `started_at`: run start timestamp.
- `completed_at`: optional run completion timestamp.
- `supervisor_mode`: repo-owned foreground supervisor for this phase.
- `status`: provisioning, running, stopped, failed, completed, or expired.
- `artifact_root`: run-specific evidence root.
- `retention_expires_at`: normal-inspection expiry timestamp.

**Validation rules**:

- Release evidence must match `run_id`, `profile_id`, and `commit_or_version`.
- Expired runs are unavailable for normal inspection unless an authorized policy applies.

## Deployment Manifest

Structured identity for a hosted run.

**Fields**:

- `manifest_id`
- `run_id`
- `commit_or_version`
- `branch`
- `host`
- `operator`
- `started_at`
- `configuration_profile`
- `data_directory`
- `artifact_directory`
- `supervisor_mode`
- `daemon_address`
- `live_connector_mode`
- `redaction_status`

**Validation rules**:

- Must exclude raw environment dumps and credential-bearing values.
- Must be linked from the release evidence index.

## Supervisor Event

Represents process lifecycle evidence from the foreground supervisor.

**Fields**:

- `event_id`
- `run_id`
- `event_type`: start, stop, restart, status, health_check, crash_detected,
  reboot_recovery, manual_stop, failed_restart, repeated_crash.
- `requested_by`
- `started_at`
- `completed_at`
- `daemon_health`
- `recovery_seconds`
- `result`: passed, failed, blocked, unsupported, or operator_action_needed.
- `failure_owner`: daemon, host, network, provider, credential, quota,
  operator_action, unsupported_observation, or unknown.
- `evidence_path`

**State transitions**:

- start -> health_check -> running
- crash_detected -> restart -> health_check -> running or failed_restart
- reboot_recovery -> health_check -> running or failed_restart
- manual_stop -> stopped without crash recovery

**Validation rules**:

- Crash or reboot recovery over 5 minutes must be recorded as failed recovery.
- Manual stop must not be classified as crash recovery.

## Upgrade Evidence

Preflight and postflight evidence for hosted upgrades.

**Fields**:

- `upgrade_evidence_id`
- `run_id`
- `phase`: preflight or postflight.
- `deployment_identity`
- `data_location`
- `artifact_location`
- `backup_status`
- `configuration_readiness`
- `daemon_health`
- `tenant_data_verification`
- `migration_state`
- `credential_remediation_state`
- `quota_state`
- `operational_diagnostics`
- `blocking_findings`

**Validation rules**:

- Missing preflight or postflight evidence blocks release readiness.
- Blocking findings must appear in the release evidence index.

## Backup Artifact

Recoverable snapshot material for hosted recovery.

**Fields**:

- `backup_id`
- `run_id`
- `source_profile_id`
- `source_commit_or_version`
- `created_at`
- `artifact_path`
- `checksum`
- `tenant_summary`
- `included_material`
- `excluded_material`
- `compatibility_notes`
- `redaction_status`

**Validation rules**:

- Must exclude raw secrets, OAuth authorization codes, access tokens, refresh tokens,
  provider tokens, local CLI auth material, and derived credential material.
- Must identify included secret metadata and references only.

## Restore Rehearsal Result

Evidence that a backup restored into an alternate target.

**Fields**:

- `restore_result_id`
- `run_id`
- `backup_id`
- `target_profile_id`
- `target_data_directory`
- `started_at`
- `completed_at`
- `tenant_count`
- `tenant_state_result`
- `migration_state_result`
- `credential_remediation_result`
- `quota_state_result`
- `daemon_health_result`
- `cross_tenant_leakage`
- `raw_credential_scan_result`
- `result`

**Validation rules**:

- Must use an alternate directory or instance rather than overwriting the active profile.
- Must cover at least three tenants with distinct credential, quota, and work states.
- Any cross-tenant leakage or raw credential exposure fails the rehearsal.

## Rollback Decision Record

Operator-facing recovery decision after failed upgrade or verification.

**Fields**:

- `rollback_decision_id`
- `run_id`
- `trigger`
- `decision`: in_place_rollback, restore_from_backup_required, no_rollback_needed,
  or blocked.
- `rationale`
- `required_backup_id`
- `supporting_evidence_links`
- `operator`
- `decided_at`

**Validation rules**:

- Must state whether in-place rollback is safe or restore from backup is required.
- Must link supporting preflight, postflight, backup, and restore evidence where relevant.

## Operational Observation Report

Resource, health, backlog, connector, MCP, and diagnostic observations for hosted runs.

**Fields**:

- `observation_report_id`
- `run_id`
- `sample_window`
- `daemon_health`
- `database_size`
- `log_size`
- `memory`
- `goroutines`
- `file_descriptors`
- `queue_or_backlog`
- `connector_health`
- `mcp_health`
- `integration_diagnostic_state`
- `unsupported_fields`
- `monotonic_resource_growth`
- `failure_owner`

**Validation rules**:

- Required fields must be present or explicitly listed as unsupported.
- Queue/backlog and monotonic resource growth findings must be visible in release
  evidence.

## Release Evidence Index

Single review artifact for ship/no-ship decisions.

**Fields**:

- `release_index_id`
- `run_id`
- `profile_id`
- `commit_or_version`
- `generated_at`
- `review_target`
- `required_evidence_links`
- `missing_evidence`
- `mismatched_evidence`
- `failed_thresholds`
- `redaction_status`
- `retention_expires_at`
- `review_time_target_minutes`: 30.
- `decision`: ship, no_ship, or ship_with_recorded_skips.

**Validation rules**:

- Every required artifact must match the reviewed commit, profile, and run identity.
- Missing, stale, mismatched, failed, or secret-exposing evidence is a no-ship condition.
- Review must be completable in 30 minutes or less.
