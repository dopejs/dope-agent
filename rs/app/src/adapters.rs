//! Adapter implementations needed by the app wiring (port of the small adapter
//! structs defined in Go app.go / the api package).

use std::sync::Arc;

use dope_scheduler::WorkflowLauncher as _;
use dope_store::SQLiteStore;

// ---------------------------------------------------------------------------
// Identity store handle (Go store.SQLiteStore is the identity store; the
// Rust SQLiteStore is !Sync, so the API layer erases it behind the
// object-safe dope_identity::Store trait through this mutex handle).
// ---------------------------------------------------------------------------

#[derive(Debug, thiserror::Error)]
#[error("{0}")]
struct IdentityStoreError(String);

fn identity_store_err(message: String) -> dope_identity::IdentityError {
    dope_identity::IdentityError::Store(Box::new(IdentityStoreError(message)))
}

/// Send + Sync handle over the shared SQLite store implementing
/// [`dope_identity::Store`], mirroring the erased-store pattern used by the
/// api test suite (TestIdentityStore).
pub struct IdentityStoreHandle(pub Arc<parking_lot::Mutex<SQLiteStore>>);

impl dope_identity::ResolverStore for IdentityStoreHandle {
    fn get_principal(
        &self,
        principal_id: &str,
    ) -> Result<Option<dope_identity::Principal>, dope_identity::IdentityError> {
        self.0
            .lock()
            .get_principal(principal_id)
            .map_err(identity_store_err)
    }
    fn get_tenant(
        &self,
        tenant_id: &str,
    ) -> Result<Option<dope_identity::Tenant>, dope_identity::IdentityError> {
        self.0
            .lock()
            .get_tenant(tenant_id)
            .map_err(identity_store_err)
    }
    fn list_memberships(
        &self,
        filter: &dope_identity::MembershipFilter,
    ) -> Result<Vec<dope_identity::Membership>, dope_identity::IdentityError> {
        self.0
            .lock()
            .list_memberships(filter)
            .map_err(identity_store_err)
    }
    fn list_token_tenant_grants(
        &self,
        token_id: &str,
    ) -> Result<Vec<dope_identity::TokenTenantGrant>, dope_identity::IdentityError> {
        self.0
            .lock()
            .list_token_tenant_grants(token_id)
            .map_err(identity_store_err)
    }
}

impl dope_identity::AuditStore for IdentityStoreHandle {
    fn append_tenant_audit_event(
        &self,
        event: dope_identity::TenantAuditEvent,
    ) -> Result<dope_identity::TenantAuditEvent, dope_identity::IdentityError> {
        self.0
            .lock()
            .append_tenant_audit_event(&event)
            .map_err(identity_store_err)
    }
}

impl dope_identity::Store for IdentityStoreHandle {
    fn upsert_tenant(
        &self,
        tenant: &dope_identity::Tenant,
    ) -> Result<(), dope_identity::IdentityError> {
        self.0
            .lock()
            .upsert_tenant(tenant)
            .map_err(identity_store_err)
    }
    fn upsert_principal(
        &self,
        principal: &dope_identity::Principal,
    ) -> Result<(), dope_identity::IdentityError> {
        self.0
            .lock()
            .upsert_principal(principal)
            .map_err(identity_store_err)
    }
    fn upsert_membership(
        &self,
        membership: &dope_identity::Membership,
    ) -> Result<(), dope_identity::IdentityError> {
        self.0
            .lock()
            .upsert_membership(membership)
            .map_err(identity_store_err)
    }
    fn upsert_tenant_invitation(
        &self,
        invitation: &dope_identity::TenantInvitation,
    ) -> Result<(), dope_identity::IdentityError> {
        self.0
            .lock()
            .upsert_tenant_invitation(invitation)
            .map_err(identity_store_err)
    }
    fn upsert_token_tenant_grant(
        &self,
        grant: &dope_identity::TokenTenantGrant,
    ) -> Result<(), dope_identity::IdentityError> {
        self.0
            .lock()
            .upsert_token_tenant_grant(grant)
            .map_err(identity_store_err)
    }
    fn list_tenants(
        &self,
        filter: &dope_identity::TenantFilter,
    ) -> Result<Vec<dope_identity::Tenant>, dope_identity::IdentityError> {
        self.0
            .lock()
            .list_tenants(filter)
            .map_err(identity_store_err)
    }
    fn list_principals(
        &self,
        filter: &dope_identity::PrincipalFilter,
    ) -> Result<Vec<dope_identity::Principal>, dope_identity::IdentityError> {
        self.0
            .lock()
            .list_principals(filter)
            .map_err(identity_store_err)
    }
    fn list_tenant_invitations(
        &self,
        filter: &dope_identity::InvitationFilter,
    ) -> Result<Vec<dope_identity::TenantInvitation>, dope_identity::IdentityError> {
        self.0
            .lock()
            .list_tenant_invitations(filter)
            .map_err(identity_store_err)
    }
    fn list_token_authorities(
        &self,
    ) -> Result<Vec<dope_identity::TokenAuthority>, dope_identity::IdentityError> {
        self.0
            .lock()
            .list_token_authorities()
            .map_err(identity_store_err)
    }
}
// ---------------------------------------------------------------------------
// Workflow launcher (Go api.NewScheduleWorkflowLauncher): launches a run in
// the runtime manager for scheduled workflows and reminders.
// ---------------------------------------------------------------------------

pub struct WorkflowLauncherImpl {
    runtime: Arc<dope_runtime::Manager>,
}

impl WorkflowLauncherImpl {
    #[must_use]
    pub fn new(runtime: Arc<dope_runtime::Manager>) -> Self {
        Self { runtime }
    }

    fn launch_run(
        &self,
        entrypoint: &str,
        goal: &str,
        schedule_id: &str,
        schedule_attempt_id: &str,
        reminder_id: &str,
        reminder_occurrence_id: &str,
    ) -> Result<String, String> {
        let run = self
            .runtime
            .create_run(dope_runtime::CreateRunInput {
                run_id: String::new(),
                session_id: String::new(),
                schedule_id: schedule_id.to_string(),
                schedule_attempt_id: schedule_attempt_id.to_string(),
                reminder_id: reminder_id.to_string(),
                reminder_occurrence_id: reminder_occurrence_id.to_string(),
                entrypoint: if entrypoint.trim().is_empty() {
                    "operator".to_string()
                } else {
                    entrypoint.to_string()
                },
                goal: goal.to_string(),
            })
            .map_err(|err| format!("launch run: {err}"))?;
        Ok(run.run_id)
    }
}

impl dope_scheduler::WorkflowLauncher for WorkflowLauncherImpl {
    fn launch_scheduled_workflow(
        &self,
        target: &dope_scheduler::WorkflowTarget,
        schedule_id: &str,
        schedule_attempt_id: &str,
    ) -> Result<dope_scheduler::WorkflowLaunchResult, String> {
        let goal = if target.workflow_goal.trim().is_empty() {
            target.run_goal.clone()
        } else {
            target.workflow_goal.clone()
        };
        let run_id = self.launch_run(
            &target.entrypoint,
            &goal,
            schedule_id,
            schedule_attempt_id,
            "",
            "",
        )?;
        Ok(dope_scheduler::WorkflowLaunchResult {
            run_id,
            workflow_id: String::new(),
            downstream_status: dope_scheduler::DownstreamStatus::Running,
        })
    }
}

impl dope_reminders::WorkflowLauncher for WorkflowLauncherImpl {
    fn launch_reminder_workflow(
        &self,
        cfg: &dope_reminders::WorkflowLaunchConfig,
        reminder_id: &str,
        occurrence_id: &str,
    ) -> Result<dope_reminders::WorkflowLaunchResult, String> {
        let goal = if cfg.workflow_goal.trim().is_empty() {
            cfg.run_goal.clone()
        } else {
            cfg.workflow_goal.clone()
        };
        let run_id = self.launch_run(&cfg.entrypoint, &goal, "", "", reminder_id, occurrence_id)?;
        Ok(dope_reminders::WorkflowLaunchResult {
            run_id,
            workflow_id: String::new(),
        })
    }
}

// ---------------------------------------------------------------------------
// Webhook firer (Go webhookWorkflowFirer): fires a webhook target by
// launching a scheduled workflow through the launcher.
// ---------------------------------------------------------------------------

pub struct WebhookFirerImpl {
    launcher: Arc<WorkflowLauncherImpl>,
}

impl WebhookFirerImpl {
    #[must_use]
    pub fn new(launcher: Arc<WorkflowLauncherImpl>) -> Self {
        Self { launcher }
    }
}

impl dope_webhook::Firer for WebhookFirerImpl {
    fn fire(&self, endpoint: &dope_webhook::Endpoint, _payload: &[u8]) -> Result<String, String> {
        let target = dope_scheduler::WorkflowTarget {
            session_id: String::new(),
            entrypoint: "operator".to_string(),
            run_goal: String::new(),
            workflow_goal: endpoint.target_ref.clone(),
            calendar_action: None,
            mail_action: None,
        };
        let result = self.launcher.launch_scheduled_workflow(
            &target,
            &format!("webhook:{}", endpoint.webhook_id),
            "",
        )?;
        if result.run_id.is_empty() {
            Ok(result.workflow_id)
        } else {
            Ok(result.run_id)
        }
    }
}

// ---------------------------------------------------------------------------
// Routine scheduler adapter (Go *scheduler.Scheduler satisfies the routine
// builder Scheduler interface; the Rust scheduler crate does not implement
// dope_routine::Scheduler, so this local adapter performs the mapping).
// ---------------------------------------------------------------------------

pub struct RoutineSchedulerAdapter {
    inner: Arc<dope_scheduler::Scheduler>,
}

impl RoutineSchedulerAdapter {
    #[must_use]
    pub fn new(inner: Arc<dope_scheduler::Scheduler>) -> Self {
        Self { inner }
    }
}

fn to_scheduler_trigger_kind(
    kind: dope_routine::SchedulerTriggerKind,
) -> dope_scheduler::TriggerKind {
    match kind {
        dope_routine::SchedulerTriggerKind::Once => dope_scheduler::TriggerKind::Once,
        dope_routine::SchedulerTriggerKind::Cron => dope_scheduler::TriggerKind::Cron,
    }
}

fn to_routine_trigger_kind(
    kind: dope_scheduler::TriggerKind,
) -> dope_routine::SchedulerTriggerKind {
    match kind {
        dope_scheduler::TriggerKind::Once => dope_routine::SchedulerTriggerKind::Once,
        dope_scheduler::TriggerKind::Cron => dope_routine::SchedulerTriggerKind::Cron,
    }
}

fn to_scheduler_target_kind(kind: dope_routine::SchedulerTargetKind) -> dope_scheduler::TargetKind {
    match kind {
        dope_routine::SchedulerTargetKind::Run => dope_scheduler::TargetKind::Run,
        dope_routine::SchedulerTargetKind::Workflow => dope_scheduler::TargetKind::Workflow,
    }
}

fn to_routine_target_kind(kind: dope_scheduler::TargetKind) -> dope_routine::SchedulerTargetKind {
    match kind {
        dope_scheduler::TargetKind::Run => dope_routine::SchedulerTargetKind::Run,
        dope_scheduler::TargetKind::Workflow => dope_routine::SchedulerTargetKind::Workflow,
    }
}

fn to_scheduler_backoff(
    kind: dope_routine::SchedulerRetryBackoffKind,
) -> dope_scheduler::RetryBackoffKind {
    match kind {
        dope_routine::SchedulerRetryBackoffKind::Fixed => dope_scheduler::RetryBackoffKind::Fixed,
        dope_routine::SchedulerRetryBackoffKind::Exponential => {
            dope_scheduler::RetryBackoffKind::Exponential
        }
    }
}

fn to_routine_backoff(
    kind: dope_scheduler::RetryBackoffKind,
) -> dope_routine::SchedulerRetryBackoffKind {
    match kind {
        dope_scheduler::RetryBackoffKind::Fixed => dope_routine::SchedulerRetryBackoffKind::Fixed,
        dope_scheduler::RetryBackoffKind::Exponential => {
            dope_routine::SchedulerRetryBackoffKind::Exponential
        }
    }
}

fn to_scheduler_create_input(input: &dope_routine::CreateInput) -> dope_scheduler::CreateInput {
    dope_scheduler::CreateInput {
        trigger: dope_scheduler::Trigger {
            kind: to_scheduler_trigger_kind(input.trigger.kind),
            fire_at: input.trigger.fire_at,
            cron_expr: input.trigger.cron_expr.clone(),
            timezone: input.trigger.timezone.clone(),
            next_due_at: None,
        },
        target: dope_scheduler::Target {
            kind: to_scheduler_target_kind(input.target.kind),
            revision: 0,
            active: input.target.active,
            run: None,
            workflow: input.target.workflow.as_ref().map(|workflow| {
                dope_scheduler::WorkflowTarget {
                    session_id: String::new(),
                    entrypoint: workflow.entrypoint.clone(),
                    run_goal: String::new(),
                    workflow_goal: workflow.workflow_goal.clone(),
                    calendar_action: None,
                    mail_action: None,
                }
            }),
            summary: input.target.summary.clone(),
            updated_at: chrono::Utc::now(),
        },
        retry_policy: dope_scheduler::RetryPolicy {
            max_retries: input.retry_policy.max_retries,
            backoff_kind: to_scheduler_backoff(input.retry_policy.backoff_kind),
            base_delay_seconds: input.retry_policy.base_delay_seconds,
            max_delay_seconds: input.retry_policy.max_delay_seconds,
        },
    }
}

fn to_routine_schedule(schedule: dope_scheduler::Schedule) -> dope_routine::Schedule {
    dope_routine::Schedule {
        schedule_id: schedule.schedule_id,
        environment_scope: schedule.environment_scope,
        tenant_id: schedule.tenant_id,
        kind: match schedule.kind {
            dope_scheduler::ScheduleKind::OneTime => dope_routine::ScheduleKind::OneTime,
            dope_scheduler::ScheduleKind::Recurring => dope_routine::ScheduleKind::Recurring,
        },
        status: match schedule.status {
            dope_scheduler::ScheduleStatus::Scheduled => dope_routine::ScheduleStatus::Scheduled,
            dope_scheduler::ScheduleStatus::Active => dope_routine::ScheduleStatus::Active,
            dope_scheduler::ScheduleStatus::Paused => dope_routine::ScheduleStatus::Paused,
            dope_scheduler::ScheduleStatus::Cancelled => dope_routine::ScheduleStatus::Cancelled,
            dope_scheduler::ScheduleStatus::Completed => dope_routine::ScheduleStatus::Completed,
            dope_scheduler::ScheduleStatus::DispatchFailed => {
                dope_routine::ScheduleStatus::DispatchFailed
            }
        },
        target_ref_id: schedule.target_ref_id,
        trigger: dope_routine::SchedulerTrigger {
            kind: to_routine_trigger_kind(schedule.trigger.kind),
            fire_at: schedule.trigger.fire_at,
            cron_expr: schedule.trigger.cron_expr,
            timezone: schedule.trigger.timezone,
        },
        target: dope_routine::SchedulerTarget {
            kind: to_routine_target_kind(schedule.target.kind),
            active: schedule.target.active,
            workflow: schedule.target.workflow.map(|workflow| {
                dope_routine::SchedulerWorkflowTarget {
                    entrypoint: workflow.entrypoint,
                    workflow_goal: workflow.workflow_goal,
                }
            }),
            summary: schedule.target.summary,
        },
        retry_policy: dope_routine::SchedulerRetryPolicy {
            max_retries: schedule.retry_policy.max_retries,
            backoff_kind: to_routine_backoff(schedule.retry_policy.backoff_kind),
            base_delay_seconds: schedule.retry_policy.base_delay_seconds,
            max_delay_seconds: schedule.retry_policy.max_delay_seconds,
        },
        created_at: schedule.created_at,
        updated_at: schedule.updated_at,
    }
}

impl dope_routine::Scheduler for RoutineSchedulerAdapter {
    fn create(&self, input: &dope_routine::CreateInput) -> Result<dope_routine::Schedule, String> {
        let schedule = self
            .inner
            .create(to_scheduler_create_input(input))
            .map_err(|err| err.to_string())?;
        Ok(to_routine_schedule(schedule))
    }

    fn pause(&self, schedule_id: &str) -> Result<(dope_routine::Schedule, bool), String> {
        match self
            .inner
            .pause(schedule_id)
            .map_err(|err| err.to_string())?
        {
            Some(schedule) => Ok((to_routine_schedule(schedule), true)),
            None => Err("routine schedule not found".to_string()),
        }
    }

    fn resume(&self, schedule_id: &str) -> Result<(dope_routine::Schedule, bool), String> {
        match self
            .inner
            .resume(schedule_id)
            .map_err(|err| err.to_string())?
        {
            Some(schedule) => Ok((to_routine_schedule(schedule), true)),
            None => Err("routine schedule not found".to_string()),
        }
    }

    fn cancel(&self, schedule_id: &str) -> Result<(dope_routine::Schedule, bool), String> {
        match self
            .inner
            .cancel(schedule_id)
            .map_err(|err| err.to_string())?
        {
            Some(schedule) => Ok((to_routine_schedule(schedule), true)),
            None => Err("routine schedule not found".to_string()),
        }
    }

    fn get(&self, schedule_id: &str) -> Result<(dope_routine::Schedule, bool), String> {
        match self.inner.get(schedule_id).map_err(|err| err.to_string())? {
            Some(schedule) => Ok((to_routine_schedule(schedule), true)),
            None => Ok((dope_routine::Schedule::default(), false)),
        }
    }
}

// ---------------------------------------------------------------------------
// Tenant migration gate (Go api.MigrationStatus): the Rust port has no
// migration runner yet, so the gate reports no in-flight migration.
// ---------------------------------------------------------------------------

pub struct NoMigrationInProgress;

impl dope_api::state::MigrationStatus for NoMigrationInProgress {
    fn in_progress(&self) -> bool {
        false
    }

    fn pending_steps(&self) -> Vec<String> {
        Vec::new()
    }
}
