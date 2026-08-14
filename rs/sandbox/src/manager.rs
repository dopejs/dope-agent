//! The sandbox manager: profiles, capability detection, execution lifecycle,
//! policy evaluation, approval orchestration, persistence, and event fan-out.
//!
//! Ported from daemon/internal/sandbox/manager.go. Contexts are replaced by
//! sync Rust (tenant context is read from the dope-identity task-local carrier);
//! Go context.CancelFunc is a CancellationToken (AtomicBool + child.kill()).

use std::collections::{HashMap, HashSet};
use std::sync::Arc;
use std::time::Duration;

use chrono::{DateTime, Utc};
use uuid::Uuid;

use crate::execution::{
    awaits_managed_provider_finalization, docker_image_for_execution,
    docker_mounts_for_execution, docker_network_mode, merge_backend_metadata,
    started_backend_metadata, CancellationToken, LaunchSpec, EVENT_CATEGORY,
    MANAGED_PROVIDER_PENDING_FINALIZATION_KEY, RESOURCE_KIND_EXECUTION,
    SANDBOX_APPROVAL_ACTION, SANDBOX_RESOURCE_KIND,
};
use crate::redaction::collect_secret_redaction_values;
use crate::{
    is_terminal, AccessRequest, ApprovalMode, BackendAvailabilityStatus, BackendCapabilityProfile,
    BackendHostStatus, BackendKind, BackendSelectionOutcome, ConsumerContractView, Decision,
    DecisionApprovalStatus, DecisionResolution, EnvironmentMode, ErrorClass, Execution,
    ExecutionFinalization, ExecutionRequest, ExecutionStatus, FilesystemMode, NetworkMode,
    PolicyRecordStatus, Profile, PROFILE_ID_DOCKER_DEFAULT, PROFILE_ID_MANAGED_PROVIDER_CLAUDE,
    PROFILE_ID_MANAGED_PROVIDER_CODEX, PROFILE_ID_SUBPROCESS_DEFAULT,
    Result as SandboxResult, SecretResolution, SecretScopeOutcome, Source,
};

/// Manager validation/lookup failures. The Go package surfaces sentinel errors
/// (ErrCommandRequired, ErrExecutionNotFound); this crate exposes a typed enum
/// in the style of dope-runtime.
#[derive(Debug, thiserror::Error, Clone, PartialEq, Eq)]
pub enum SandboxError {
    #[error("sandbox command is required")]
    CommandRequired,
    #[error("sandbox execution not found")]
    ExecutionNotFound,
    #[error("wait execution timed out")]
    WaitTimeout,
    #[error("attached docker sandbox execution is not supported")]
    AttachedDockerUnsupported,
    #[error("resolve sandbox cwd: {0}")]
    ResolveCwd(String),
    #[error("resolve sandbox read roots: {0}")]
    ResolveReadRoots(String),
    #[error("resolve sandbox write roots: {0}")]
    ResolveWriteRoots(String),
    #[error("request sandbox approval: {0}")]
    RequestApproval(String),
    #[error("marshal sandbox execution {0}: {1}")]
    MarshalExecution(String, String),
    #[error("marshal {0}: {1}")]
    Marshal(String, String),
    #[error("persist sandbox approval {0}: {1}")]
    PersistApproval(String, String),
    #[error("persist sandbox approval decision {0}: {1}")]
    PersistDecision(String, String),
    #[error("append sandbox event {0}: {1}")]
    AppendEvent(String, String),
    #[error("list sandbox executions: {0}")]
    ListExecutions(String),
    #[error("decode sandbox execution {0}: {1}")]
    DecodeExecution(String, String),
    #[error("persist sandbox execution {0}: {1}")]
    PersistExecution(String, String),
    #[error("persist consumer contract: {0}")]
    PersistConsumerContract(String),
    #[error("upsert secret scope binding {0}: {1}")]
    UpsertSecretScopeBinding(String, String),
    #[error("upsert consumer policy record {0}: {1}")]
    UpsertConsumerPolicyRecord(String, String),
    #[error("append tenant audit event: {0}")]
    AppendAuditEvent(String),
}

/// The outcome of `prepare`: the execution, its decision, the launch spec
/// (None when the decision denies before launch), the approval id, and the
/// policy decision created for a fresh approval request.
#[derive(Debug, Clone)]
pub(crate) struct PrepareOutcome {
    pub(crate) execution: Execution,
    pub(crate) decision: Decision,
    pub(crate) launch: Option<LaunchSpec>,
    pub(crate) approval_id: String,
    pub(crate) created_decision: Option<dope_policy::Decision>,
}

#[derive(Default)]
pub(crate) struct ManagerInner {
    pub(crate) profiles: HashMap<String, Profile>,
    pub(crate) profile_ids: Vec<String>,
    pub(crate) capabilities: HashMap<BackendKind, BackendCapabilityProfile>,
    pub(crate) executions: HashMap<String, Execution>,
    pub(crate) execution_ids: Vec<String>,
    pub(crate) cancels: HashMap<String, CancellationToken>,
    pub(crate) pending_final: HashSet<String>,
    pub(crate) secrets: Option<Arc<dope_secrets::Manager>>,
}

/// Thread-safe sandbox manager mirroring the Go `Manager`: an in-memory
/// insertion-ordered execution ledger plus profile/capability state guarded by
/// `parking_lot::RwLock`. Cloneable (Arc-shared, like the Go pointer) so the
/// process-execution threads can hold a handle.
#[derive(Clone)]
pub struct Manager {
    cfg: Arc<dope_config::Config>,
    store: Option<Arc<std::sync::Mutex<dope_store::SQLiteStore>>>,
    event_bus: dope_events::Bus,
    policy: Arc<dope_policy::Engine>,
    pub(crate) inner: Arc<parking_lot::RwLock<ManagerInner>>,
}

impl Manager {
    /// Go `NewManager`: builds the manager and loads the builtin profiles
    /// with detected backend capabilities.
    #[must_use]
    pub fn new(
        cfg: dope_config::Config,
        store: Option<Arc<std::sync::Mutex<dope_store::SQLiteStore>>>,
        event_bus: dope_events::Bus,
        policy: dope_policy::Engine,
    ) -> Manager {
        let manager = Manager {
            cfg: Arc::new(cfg),
            store,
            event_bus,
            policy: Arc::new(policy),
            inner: Arc::new(parking_lot::RwLock::new(ManagerInner::default())),
        };
        manager.reload_builtins();
        manager
    }

    /// Go `SetSecretManager`.
    pub fn set_secret_manager(&self, secret_manager: dope_secrets::Manager) {
        self.inner.write().secrets = Some(Arc::new(secret_manager));
    }

    /// Go `Reload`: reloads the builtin profiles/capabilities and returns the
    /// resulting profile list.
    pub fn reload(&self) -> Vec<Profile> {
        self.inner.write().reload_builtins_locked(&self.cfg);
        self.list_profiles_locked()
    }

    /// Go `ListProfiles`.
    pub fn list_profiles(&self) -> Vec<Profile> {
        self.list_profiles_locked()
    }

    /// Go `GetProfile`.
    #[must_use]
    pub fn get_profile(&self, profile_id: &str) -> Option<Profile> {
        let profile_id = normalize_profile_id(profile_id);
        self.inner.read().profiles.get(&profile_id).cloned()
    }

    /// Go `BackendCapabilities`: capabilities in a stable backend order.
    pub fn backend_capabilities(&self) -> Vec<BackendCapabilityProfile> {
        let inner = self.inner.read();
        let order = [BackendKind::Subprocess, BackendKind::Docker, BackendKind::Ssh, BackendKind::Remote];
        let mut items = Vec::with_capacity(inner.capabilities.len());
        for kind in order {
            if let Some(capability) = inner.capabilities.get(&kind) {
                items.push(capability.clone());
            }
        }
        items
    }

    /// Go `ListExecutions`: insertion-ordered execution list.
    pub fn list_executions(&self) -> Vec<Execution> {
        let inner = self.inner.read();
        inner.execution_ids.iter().filter_map(|id| inner.executions.get(id)).cloned().collect()
    }

    /// Go `GetExecution`.
    #[must_use]
    pub fn get_execution(&self, execution_id: &str) -> Option<Execution> {
        let execution_id = execution_id.trim();
        self.inner.read().executions.get(execution_id).cloned()
    }

    /// Go `Explain`: evaluates a request without creating an approval or
    /// launching anything.
    pub fn explain(&self, request: ExecutionRequest) -> Result<Decision, SandboxError> {
        let outcome = self.prepare(request, false)?;
        Ok(outcome.decision)
    }

    /// Go `EvaluateAccess`: evaluates an access request against a profile
    /// without an execution.
    #[must_use]
    pub fn evaluate_access(&self, profile_id: &str, cwd: &str, access: AccessRequest) -> Decision {
        let profile_id = first_non_empty(&[profile_id, PROFILE_ID_SUBPROCESS_DEFAULT]);
        let Some(profile) = self.get_profile(&profile_id) else {
            return Decision {
                decision_id: new_id("sandbox_decision"),
                resolution: DecisionResolution::Deny,
                matched_rules: vec!["profile:not_found".to_string()],
                approval_status: DecisionApprovalStatus::NotApplicable,
                effective_profile_id: profile_id.trim().to_string(),
                effective_backend_kind: BackendKind::Subprocess,
                explanation: "sandbox profile was not found".to_string(),
                ..Decision::default()
            };
        };
        evaluate_access_decision(&profile, cwd.trim(), &access)
    }

    /// Go `StartExecution`: prepares, evaluates, persists, publishes, and
    /// launches the execution on a worker thread.
    pub fn start_execution(&self, request: ExecutionRequest) -> Result<Execution, SandboxError> {
        let outcome = self.prepare(request, true)?;
        let mut execution = outcome.execution;
        let decision = outcome.decision;
        let launch = outcome.launch;
        let approval = outcome.approval_id;
        let created_decision = outcome.created_decision;

        execution.decision = decision.clone();
        execution.approval_id = approval.clone();
        execution.status = decision_to_status(&decision);
        execution.result.status = execution.status;
        synchronize_execution_consumer_state(&mut execution);
        if execution.status == ExecutionStatus::Denied || execution.status == ExecutionStatus::Unsupported {
            execution.result.error_class = decision_error_class(&decision).as_str().to_string();
            execution.result.error_code = decision_error_code(&decision);
            execution.result.error = decision.explanation.clone();
        }

        self.store_execution(&execution);

        if let Some(created_decision) = created_decision {
            self.persist_approval_artifacts(&approval, &created_decision)?;
            self.publish_approval_requested(&approval, &created_decision)?;
        }
        self.persist_consumer_contract(execution.consumer.as_ref())?;
        self.persist_execution(&execution)?;
        self.publish_execution_requested(&execution)?;
        self.publish_decision_recorded(&execution)?;
        if execution.status == ExecutionStatus::Denied || execution.status == ExecutionStatus::Unsupported {
            self.publish_execution_terminal(&execution)?;
            return Ok(execution);
        }

        let cancel = CancellationToken::new();
        {
            let mut inner = self.inner.write();
            inner.cancels.insert(execution.execution_id.clone(), cancel.clone());
        }
        let manager = self.clone();
        std::thread::spawn(move || manager.run_execution(cancel, execution.clone(), launch));
        Ok(execution)
    }

    /// Go `StartAttachedExecution`: prepares and spawns the subprocess,
    /// returning live stdin/stdout pipes and a streaming runner thread.
    pub fn start_attached_execution(
        &self,
        request: ExecutionRequest,
    ) -> Result<(Execution, Option<crate::execution::AttachedExecution>), SandboxError> {
        let outcome = self.prepare(request, true)?;
        let mut execution = outcome.execution;
        let decision = outcome.decision;
        let launch = outcome.launch;
        let approval = outcome.approval_id;
        let created_decision = outcome.created_decision;

        execution.decision = decision.clone();
        execution.approval_id = approval.clone();
        execution.status = decision_to_status(&decision);
        execution.result.status = execution.status;
        synchronize_execution_consumer_state(&mut execution);
        if execution.status == ExecutionStatus::Denied || execution.status == ExecutionStatus::Unsupported {
            execution.result.error_class = decision_error_class(&decision).as_str().to_string();
            execution.result.error_code = decision_error_code(&decision);
            execution.result.error = decision.explanation.clone();
        }

        self.store_execution(&execution);

        if let Some(created_decision) = created_decision {
            self.persist_approval_artifacts(&approval, &created_decision)?;
            self.publish_approval_requested(&approval, &created_decision)?;
        }
        self.persist_consumer_contract(execution.consumer.as_ref())?;
        self.persist_execution(&execution)?;
        self.publish_execution_requested(&execution)?;
        self.publish_decision_recorded(&execution)?;
        if execution.status == ExecutionStatus::Denied || execution.status == ExecutionStatus::Unsupported {
            self.publish_execution_terminal(&execution)?;
            return Ok((execution, None));
        }
        if execution.backend_kind == BackendKind::Docker {
            return Err(SandboxError::AttachedDockerUnsupported);
        }
        let Some(launch) = launch else {
            // Unreachable in practice (Go would nil-deref); keep the execution
            // ledger consistent by returning it as-is.
            return Ok((execution, None));
        };

        let cancel = CancellationToken::new();
        let mut command = std::process::Command::new(&launch.command);
        command.args(&launch.args);
        if !launch.cwd.is_empty() {
            command.current_dir(&launch.cwd);
        }
        command.env_clear();
        for item in &launch.env {
            if let Some((key, value)) = item.split_once('=') {
                command.env(key, value);
            }
        }
        command.stdin(std::process::Stdio::piped());
        command.stdout(std::process::Stdio::piped());
        command.stderr(std::process::Stdio::piped());

        let mut child = match command.spawn() {
            Ok(child) => child,
            Err(err) => {
                let now = Utc::now();
                execution.status = ExecutionStatus::Failed;
                execution.updated_at = now;
                execution.completed_at = Some(now);
                execution.result = SandboxResult {
                    execution_id: execution.execution_id.clone(),
                    status: ExecutionStatus::Failed,
                    completed_at: Some(now),
                    error_class: ErrorClass::LaunchFailed.as_str().to_string(),
                    error_code: "sandbox_launch_failed".to_string(),
                    error: err.to_string(),
                    consumer: execution.consumer.clone(),
                    ..SandboxResult::default()
                };
                synchronize_execution_consumer_state(&mut execution);
                self.store_execution(&execution);
                let _ = self.persist_consumer_contract(execution.consumer.as_ref());
                if self.persist_execution(&execution).is_ok() {
                    let _ = self.publish_execution_terminal(&execution);
                }
                return Ok((execution, None));
            }
        };
        let pid = child.id();
        let stderr_capture = Arc::new(std::sync::Mutex::new(crate::execution::CaptureBuffer::new(launch.max_output_bytes)));
        let stdin = child.stdin.take();
        let stdout = child.stdout.take();
        if let Some(stderr) = child.stderr.take() {
            let capture = Arc::clone(&stderr_capture);
            std::thread::spawn(move || crate::execution::read_pipe_into(stderr, capture));
        }
        cancel.register_child(child);

        {
            let mut inner = self.inner.write();
            let now = Utc::now();
            execution.status = ExecutionStatus::Running;
            execution.started_at = Some(now);
            execution.updated_at = now;
            execution.result.status = ExecutionStatus::Running;
            execution.result.started_at = Some(now);
            let mut extra = serde_json::Map::new();
            extra.insert("pid".to_string(), serde_json::json!(pid));
            execution.result.backend_metadata = merge_backend_metadata(&started_backend_metadata(&execution, Some(&extra)), &execution);
            synchronize_execution_consumer_state(&mut execution);
            inner.executions.insert(execution.execution_id.clone(), execution.clone());
            inner.cancels.insert(execution.execution_id.clone(), cancel.clone());
        }
        if self.persist_consumer_contract(execution.consumer.as_ref()).is_ok()
            && self.persist_execution(&execution).is_ok()
        {
            let _ = self.publish_execution_started(&execution);
        }

        let manager = self.clone();
        let attached_execution = execution.clone();
        std::thread::spawn(move || {
            manager.run_attached_execution(cancel, execution.clone(), stderr_capture, pid, launch.timeout)
        });

        Ok((
            attached_execution.clone(),
            Some(crate::execution::AttachedExecution {
                execution: attached_execution,
                stdin,
                stdout,
            }),
        ))
    }

    /// Go `CancelExecution`: cancels the execution's token (killing the
    /// registered child) unless it is already terminal. Returns the execution
    /// and whether it was already terminal.
    pub fn cancel_execution(&self, execution_id: &str) -> Result<(Execution, bool), SandboxError> {
        let execution_id = execution_id.trim();
        let (execution, cancel) = {
            let inner = self.inner.read();
            let Some(execution) = inner.executions.get(execution_id).cloned() else {
                return Err(SandboxError::ExecutionNotFound);
            };
            let cancel = inner.cancels.get(execution_id).cloned();
            (execution, cancel)
        };
        if is_terminal(execution.status) {
            return Ok((execution, true));
        }
        if let Some(cancel) = cancel {
            cancel.cancel();
        }
        Ok((execution, false))
    }

    /// Go `WaitExecution`: polls until the execution is terminal or the
    /// timeout expires.
    pub fn wait_execution(&self, execution_id: &str, timeout: Duration) -> Result<Execution, SandboxError> {
        let deadline = std::time::Instant::now() + timeout;
        loop {
            match self.get_execution(execution_id) {
                Some(execution) if is_terminal(execution.status) => return Ok(execution),
                Some(_) => {}
                None => return Err(SandboxError::ExecutionNotFound),
            }
            if std::time::Instant::now() >= deadline {
                return Err(SandboxError::WaitTimeout);
            }
            std::thread::sleep(Duration::from_millis(25));
        }
    }

    /// Go `PersistConsumerView`: persists a consumer contract view.
    pub fn persist_consumer_view(&self, view: &ConsumerContractView) -> Result<(), SandboxError> {
        let cloned = clone_consumer_contract_view(Some(view));
        self.persist_consumer_contract(cloned.as_ref())
    }

    /// Go `FinalizeExecution`: applies managed-provider finalization to a
    /// completed execution that deferred its terminal event.
    pub fn finalize_execution(&self, execution_id: &str, finalization: ExecutionFinalization) -> Result<Execution, SandboxError> {
        let execution_id = execution_id.trim();
        let execution = {
            let mut inner = self.inner.write();
            let Some(mut execution) = inner.executions.get(execution_id).cloned() else {
                return Err(SandboxError::ExecutionNotFound);
            };
            if !inner.pending_final.remove(execution_id) {
                return Ok(execution);
            }
            let mut status = finalization.status.unwrap_or(execution.status);
            if status != ExecutionStatus::Completed && status != ExecutionStatus::Failed {
                status = ExecutionStatus::Failed;
            }
            let now = Utc::now();
            execution.status = status;
            execution.updated_at = now;
            execution.completed_at = Some(now);
            execution.result.status = status;
            execution.result.completed_at = Some(now);
            execution.result.error_class = finalization.error_class;
            execution.result.error_code = finalization.error_code.trim().to_string();
            execution.result.error = finalization.error.trim().to_string();
            execution.metadata.remove(MANAGED_PROVIDER_PENDING_FINALIZATION_KEY);
            if status == ExecutionStatus::Completed {
                execution.result.error_class = String::new();
                execution.result.error_code = String::new();
                execution.result.error = String::new();
            }
            synchronize_execution_consumer_state(&mut execution);
            inner.executions.insert(execution_id.to_string(), execution.clone());
            execution
        };
        self.persist_consumer_contract(execution.consumer.as_ref())?;
        self.persist_execution(&execution)?;
        self.publish_execution_terminal(&execution)?;
        Ok(execution)
    }

    /// Go `Close`: cancels all executions and waits (up to 2s) for them to
    /// reach a terminal state.
    pub fn close(&self) -> Result<(), SandboxError> {
        let execution_ids = { self.inner.read().execution_ids.clone() };
        let mut first_error: Option<SandboxError> = None;
        for execution_id in execution_ids {
            match self.cancel_execution(&execution_id) {
                Ok(_) => {}
                Err(SandboxError::ExecutionNotFound) => {}
                Err(err) => {
                    if first_error.is_none() {
                        first_error = Some(err);
                    }
                }
            }
        }
        let deadline = std::time::Instant::now() + Duration::from_secs(2);
        loop {
            if !self.has_active_executions() {
                return Ok(());
            }
            if std::time::Instant::now() >= deadline {
                return match first_error {
                    Some(err) => Err(err),
                    None => Ok(()),
                };
            }
            std::thread::sleep(Duration::from_millis(25));
        }
    }

    /// Go `Restore`: reloads executions from the store, cancelling any that
    /// were interrupted by a restart (deferring the special managed-provider
    /// finalization recovery message).
    pub fn restore(&self) -> Result<(), SandboxError> {
        let Some(store) = &self.store else { return Ok(()); };
        let records = store.lock().unwrap().list_sandbox_executions().map_err(SandboxError::ListExecutions)?;

        {
            let mut inner = self.inner.write();
            inner.executions = HashMap::new();
            inner.execution_ids = Vec::new();
            inner.cancels = HashMap::new();
            inner.pending_final = HashSet::new();
            for record in &records {
                let execution: Execution = serde_json::from_str(&record.document)
                    .map_err(|err| SandboxError::DecodeExecution(record.execution_id.clone(), err.to_string()))?;
                inner.executions.insert(execution.execution_id.clone(), execution.clone());
                inner.execution_ids.push(execution.execution_id.clone());
            }
        }

        for record in &records {
            let Some(mut execution) = self.get_execution(&record.execution_id) else { continue; };
            if is_terminal(execution.status) && !awaits_managed_provider_finalization(&execution) {
                continue;
            }
            let now = Utc::now();
            execution.status = ExecutionStatus::Cancelled;
            execution.result.status = ExecutionStatus::Cancelled;
            execution.result.error_class = ErrorClass::Cancelled.as_str().to_string();
            execution.result.error_code = "daemon_restarted".to_string();
            execution.result.error = "execution was interrupted by daemon restart recovery".to_string();
            if awaits_managed_provider_finalization(&execution) {
                execution.result.error_code = "daemon_restarted_before_consumer_finalization".to_string();
                execution.result.error =
                    "execution completed at subprocess layer but daemon restarted before managed-provider finalization".to_string();
                execution.metadata.remove(MANAGED_PROVIDER_PENDING_FINALIZATION_KEY);
            }
            execution.result.completed_at = Some(now);
            execution.completed_at = Some(now);
            execution.updated_at = now;
            synchronize_execution_consumer_state(&mut execution);
            self.store_execution(&execution);
            self.persist_consumer_contract(execution.consumer.as_ref())?;
            self.persist_execution(&execution)?;
            self.publish_execution_terminal(&execution)?;
        }
        Ok(())
    }
}
impl Manager {
    /// Go `prepare`: resolves the profile, normalizes access roots, resolves
    /// the tenant secret scope, evaluates the decision, and builds the launch
    /// spec (or a denied/unsupported execution).
    fn prepare(&self, request: ExecutionRequest, create_approval: bool) -> Result<PrepareOutcome, SandboxError> {
        let profile = self.get_profile(&first_non_empty(&[request.profile_id.as_str(), PROFILE_ID_SUBPROCESS_DEFAULT]));
        let Some(profile) = profile else {
            let now = Utc::now();
            let execution_id = new_id("sandbox_exec");
            let decision = Decision {
                decision_id: new_id("sandbox_decision"),
                execution_id: execution_id.clone(),
                resolution: DecisionResolution::Deny,
                matched_rules: vec!["profile:not_found".to_string()],
                approval_status: DecisionApprovalStatus::NotApplicable,
                effective_profile_id: request.profile_id.trim().to_string(),
                effective_backend_kind: BackendKind::Subprocess,
                explanation: "sandbox profile was not found".to_string(),
                consumer: clone_consumer_contract_view(request.consumer.as_ref()),
                ..Decision::default()
            };
            let mut execution = Execution {
                execution_id,
                profile_id: request.profile_id.trim().to_string(),
                backend_kind: BackendKind::Subprocess,
                command: request.command.trim().to_string(),
                args: clone_strings(&request.args),
                cwd: request.cwd.trim().to_string(),
                env_keys: sorted_keys(&request.env),
                stdin_provided: !request.stdin.is_empty(),
                timeout_ms: request.timeout_ms,
                requested_by: request.requested_by.trim().to_string(),
                resource_kind: request.resource_kind.trim().to_string(),
                resource_id: request.resource_id.trim().to_string(),
                scope: request.scope.trim().to_string(),
                reason: request.reason.trim().to_string(),
                metadata: clone_string_map(&request.metadata),
                access: clone_access(&request.access),
                requested_at: now,
                updated_at: now,
                result: SandboxResult {
                    status: ExecutionStatus::Denied,
                    error_class: ErrorClass::InvalidProfile.as_str().to_string(),
                    error_code: "sandbox_profile_not_found".to_string(),
                    error: "sandbox profile was not found".to_string(),
                    partial: false,
                    output_truncated: false,
                    ..SandboxResult::default()
                },
                consumer: clone_consumer_contract_view(request.consumer.as_ref()),
                ..Execution::default()
            };
            execution.result.consumer = clone_consumer_contract_view(request.consumer.as_ref());
            return Ok(PrepareOutcome {
                execution,
                decision,
                launch: None,
                approval_id: String::new(),
                created_decision: None,
            });
        };
        if request.command.trim().is_empty() {
            return Err(SandboxError::CommandRequired);
        }

        let execution_id = new_id("sandbox_exec");
        let now = Utc::now();
        let cwd = normalize_path(&profile.default_work_dir, &request.cwd)
            .map_err(SandboxError::ResolveCwd)?;
        let read_roots = normalize_paths(&cwd, &request.access.read_roots)
            .map_err(SandboxError::ResolveReadRoots)?;
        let write_roots = normalize_paths(&cwd, &request.access.write_roots)
            .map_err(SandboxError::ResolveWriteRoots)?;
        let timeout_ms = effective_timeout(&profile, request.timeout_ms);
        let mut request_env = clone_string_map(&request.env);
        if request_env.is_empty() {
            request_env = HashMap::new();
        }
        let env = build_environment(&profile, &request_env);

        let mut execution = Execution {
            execution_id: execution_id.clone(),
            profile_id: profile.profile_id.clone(),
            backend_kind: profile.backend_kind,
            command: request.command.trim().to_string(),
            args: clone_strings(&request.args),
            cwd: cwd.clone(),
            env_keys: sorted_keys(&request.env),
            stdin_provided: !request.stdin.is_empty(),
            timeout_ms,
            requested_by: request.requested_by.trim().to_string(),
            resource_kind: request.resource_kind.trim().to_string(),
            resource_id: request.resource_id.trim().to_string(),
            scope: request.scope.trim().to_string(),
            approval_id: request.approval_id.trim().to_string(),
            reason: request.reason.trim().to_string(),
            metadata: clone_string_map(&request.metadata),
            access: AccessRequest {
                read_roots,
                write_roots,
                network_mode: request.access.network_mode,
                allowed_hosts: clone_strings(&request.access.allowed_hosts),
                allowed_ports: request.access.allowed_ports.clone(),
                allow_loopback: request.access.allow_loopback,
            },
            status: ExecutionStatus::Pending,
            requested_at: now,
            updated_at: now,
            result: SandboxResult {
                status: ExecutionStatus::Pending,
                output_truncated: false,
                partial: false,
                ..SandboxResult::default()
            },
            consumer: clone_consumer_contract_view(request.consumer.as_ref()),
            ..Execution::default()
        };
        if let Some(consumer) = &mut execution.consumer {
            if let Some(policy_record) = &mut consumer.policy_record {
                policy_record.sandbox_execution_id = execution.execution_id.clone();
            }
        }
        let (denied_rule, denied_reason) = self.resolve_tenant_secret_scope(execution.consumer.as_mut(), &mut request_env);
        if !denied_reason.is_empty() {
            let mut decision = evaluate_access_decision(&profile, &execution.cwd, &execution.access);
            decision.execution_id = execution_id;
            decision.consumer = clone_consumer_contract_view(execution.consumer.as_ref());
            mark_decision_denied(&mut decision, &denied_rule, &denied_reason, "secret_resolution_denied");
            execution.decision = decision.clone();
            execution.result.consumer = clone_consumer_contract_view(execution.consumer.as_ref());
            if let Some(consumer) = &mut execution.consumer {
                if let Some(policy_record) = &mut consumer.policy_record {
                    policy_record.secret_resolution = secret_resolution_from_consumer(consumer);
                }
            }
            return Ok(PrepareOutcome {
                execution,
                decision,
                launch: None,
                approval_id: String::new(),
                created_decision: None,
            });
        }
        let env = build_environment(&profile, &request_env);
        let (mut decision, approval_id, created_decision) = self.evaluate(&profile, &mut execution, create_approval)?;
        decision.execution_id = execution_id.clone();
        decision.consumer = clone_consumer_contract_view(execution.consumer.as_ref());
        execution.decision = decision.clone();
        execution.approval_id = approval_id.clone();
        execution.result.consumer = clone_consumer_contract_view(execution.consumer.as_ref());

        let mut launch = LaunchSpec {
            backend_kind: execution.backend_kind,
            command: execution.command.clone(),
            args: clone_strings(&execution.args),
            cwd: execution.cwd.clone(),
            env,
            secret_values: collect_secret_redaction_values(
                &request_env,
                execution.consumer.as_ref().unwrap_or(&ConsumerContractView::default()),
            ),
            stdin: request.stdin.clone(),
            timeout: Duration::from_millis(timeout_ms.max(0) as u64),
            kill_grace: Duration::from_millis(profile.process_policy.kill_grace_ms.max(0) as u64),
            capture_stdout: profile.process_policy.capture_stdout,
            capture_stderr: profile.process_policy.capture_stderr,
            max_output_bytes: profile.process_policy.max_output_bytes,
            docker_image: String::new(),
            docker_network: String::new(),
            docker_mounts: Vec::new(),
        };
        if execution.backend_kind == BackendKind::Docker {
            launch.docker_image = docker_image_for_execution();
            launch.docker_network = docker_network_mode(&request.access);
            launch.docker_mounts = docker_mounts_for_execution(&execution.cwd, &request.access);
        }
        Ok(PrepareOutcome {
            execution,
            decision,
            launch: Some(launch),
            approval_id,
            created_decision,
        })
    }

    /// Go `evaluate`: applies the access decision, backend requirement
    /// checks, command-approval rules, and the approval orchestration.
    fn evaluate(
        &self,
        profile: &Profile,
        execution: &mut Execution,
        create_approval: bool,
    ) -> Result<(Decision, String, Option<dope_policy::Decision>), SandboxError> {
        let mut decision = evaluate_access_decision(profile, &execution.cwd, &execution.access);
        if let Some(declaration) = execution.consumer.as_ref().and_then(|consumer| consumer.declaration.as_ref()) {
            decision.required_backend_kind = required_backend_kind(declaration);
            let required_strength = declaration.required_enforcement_strength.trim().to_string();
            if backend_requirement_unsupported(profile.backend_kind, decision.required_backend_kind, &required_strength) {
                mark_decision_unsupported(
                    &mut decision,
                    "enforcement:unsupported",
                    "sandbox declaration requires stronger guarantees than the selected backend can provide",
                    "unsupported_backend_guarantee",
                );
                if let Some(consumer) = &mut execution.consumer {
                    if let Some(policy_record) = &mut consumer.policy_record {
                        policy_record.status = PolicyRecordStatus::Unsupported;
                        policy_record.decision = DecisionResolution::Deny;
                        policy_record.approval_status = DecisionApprovalStatus::NotApplicable;
                        policy_record.failure_class = "unsupported_backend_guarantee".to_string();
                        policy_record.enforcement_strength = required_strength;
                    }
                }
                return Ok((decision, String::new(), None));
            }
        }

        let requested_approval = execution.approval_id.trim().to_string();
        if decision.resolution == DecisionResolution::Deny {
            return Ok((decision, String::new(), None));
        }

        let mut approval_required = decision.approval_required;
        let mut reasons: Vec<String> = decision.matched_rules.iter().skip(1).cloned().collect();

        let rule = command_approval_rule(profile, &execution.command);
        if !rule.is_empty() {
            approval_required = true;
            reasons.push(rule);
        }

        if profile.approval_policy.mode == ApprovalMode::Deny && approval_required {
            decision.resolution = DecisionResolution::Deny;
            decision.matched_rules.extend(reasons);
            decision.explanation = "sandbox profile denies requested escalation".to_string();
            return Ok((decision, String::new(), None));
        }
        if profile.approval_policy.mode == ApprovalMode::Allow {
            approval_required = false;
            reasons.clear();
        }

        if !approval_required {
            return Ok((decision, String::new(), None));
        }

        decision.approval_required = true;
        decision.resolution = DecisionResolution::Ask;
        decision.matched_rules.extend(reasons);
        decision.explanation = "sandbox execution requires approval".to_string();

        if !requested_approval.is_empty() {
            if let Some(approval) = self.policy.get_approval(&requested_approval) {
                if approval_matches_execution(&approval, execution, profile) {
                    match approval.status {
                        dope_policy::ApprovalStatus::Approved => {
                            decision.resolution = DecisionResolution::Allow;
                            decision.approval_status = DecisionApprovalStatus::Approved;
                            decision.explanation =
                                "sandbox execution is allowed by approved escalation".to_string();
                            return Ok((decision, requested_approval, None));
                        }
                        dope_policy::ApprovalStatus::Rejected => {
                            decision.resolution = DecisionResolution::Deny;
                            decision.approval_status = DecisionApprovalStatus::Rejected;
                            decision.explanation =
                                "sandbox execution was rejected by approval policy".to_string();
                            return Ok((decision, requested_approval, None));
                        }
                        dope_policy::ApprovalStatus::Pending => {
                            decision.approval_status = DecisionApprovalStatus::Pending;
                            return Ok((decision, requested_approval, None));
                        }
                    }
                }
            }
        }

        if !create_approval {
            decision.approval_status = DecisionApprovalStatus::Pending;
            return Ok((decision, String::new(), None));
        }

        let (approval, created_decision) = self
            .policy
            .request_approval(dope_policy::RequestApprovalInput {
                action: SANDBOX_APPROVAL_ACTION.to_string(),
                resource_kind: SANDBOX_RESOURCE_KIND.to_string(),
                resource_id: profile.profile_id.clone(),
                reason: first_non_empty(&[execution.reason.as_str(), "sandbox execution requires approval"]),
                requested_by: first_non_empty(&[execution.requested_by.as_str(), "sandbox"]),
                integration_bindings: Vec::new(),
            })
            .map_err(|err| SandboxError::RequestApproval(err.to_string()))?;
        decision.approval_status = DecisionApprovalStatus::Pending;
        Ok((decision, approval.approval_id, Some(created_decision)))
    }

    /// Go `resolveTenantSecretScope`: resolves the consumer's secret scope
    /// against the active tenant (dope-identity task-local carrier), injecting
    /// values into the request environment and failing closed without one.
    fn resolve_tenant_secret_scope(
        &self,
        consumer: &mut ConsumerContractView,
        env: &mut HashMap<String, String>,
    ) -> (String, String) {
        let secrets = match self.inner.read().secrets.clone() {
            Some(secrets) => secrets,
            None => return (String::new(), String::new()),
        };
        if consumer.secret_scope.is_empty() {
            return (String::new(), String::new());
        }
        let tenant_context = match dope_identity::tenantctx::from_context() {
            Some(context) => context,
            None => {
                for item in &mut consumer.secret_scope {
                    if !item.secret_ref.trim().is_empty() {
                        item.resolution = SecretResolution::Denied;
                    }
                }
                return (
                    "secret_scope:missing_tenant".to_string(),
                    "tenant context is required to resolve sandbox secrets".to_string(),
                );
            }
        };
        if tenant_context.tenant_id.trim().is_empty() {
            for item in &mut consumer.secret_scope {
                if !item.secret_ref.trim().is_empty() {
                    item.resolution = SecretResolution::Denied;
                }
            }
            return (
                "secret_scope:missing_tenant".to_string(),
                "tenant context is required to resolve sandbox secrets".to_string(),
            );
        }
        for item in &mut consumer.secret_scope {
            let secret_ref = item.secret_ref.trim().to_string();
            if secret_ref.is_empty() {
                continue;
            }
            let resolved = futures::executor::block_on(secrets.resolve(dope_secrets::ResolveInput {
                tenant_id: tenant_context.tenant_id.trim().to_string(),
                secret_ref: secret_ref.clone(),
            }));
            match resolved {
                Ok(secret) => {
                    if secret.value.trim().is_empty() {
                        item.resolution = SecretResolution::Unavailable;
                        return (
                            "secret_scope:unavailable".to_string(),
                            "required sandbox secret is unavailable".to_string(),
                        );
                    }
                    env.insert(secret_ref.clone(), secret.value);
                    item.resolution = SecretResolution::Resolved;
                }
                Err(dope_secrets::SecretsError::SecretDisabled) => {
                    item.resolution = SecretResolution::Denied;
                    return (
                        "secret_scope:disabled".to_string(),
                        "required sandbox secret is disabled".to_string(),
                    );
                }
                Err(dope_secrets::SecretsError::SecretNotFound)
                | Err(dope_secrets::SecretsError::SecretVersionNotFound) => {
                    item.resolution = SecretResolution::Unavailable;
                    return (
                        "secret_scope:unavailable".to_string(),
                        "required sandbox secret is unavailable".to_string(),
                    );
                }
                Err(_) => {
                    item.resolution = SecretResolution::Unavailable;
                    return (
                        "secret_scope:unavailable".to_string(),
                        "required sandbox secret could not be resolved".to_string(),
                    );
                }
            }
        }
        if let Some(store) = &self.store {
            let secret_refs: Vec<String> = consumer
                .secret_scope
                .iter()
                .filter(|item| !item.secret_ref.trim().is_empty())
                .map(|item| item.secret_ref.trim().to_string())
                .collect();
            let audit_event = dope_audit::build_credential_audit_event(&dope_audit::CredentialAuditInput {
                tenant_id: tenant_context.tenant_id.trim().to_string(),
                principal_id: tenant_context.principal_id.trim().to_string(),
                resource_kind: dope_secrets::ResourceKind::SandboxPolicy,
                resource_id: { let resource_id = consumer_id(consumer); first_non_empty(&[resource_id.as_str(), "sandbox"]) },
                action: dope_secrets::AuditAction::SecretUse,
                outcome: dope_identity::AUDIT_OUTCOME_SUCCEEDED.to_string(),
                reason_code: "sandbox_secret_scope_prepared".to_string(),
                secret_ref: String::new(),
                secret_version_id: String::new(),
                secret_refs,
                created_at: Utc::now(),
            });
            let _ = store.lock().unwrap().append_tenant_audit_event(&audit_event);
        }
        (String::new(), String::new())
    }

    pub(crate) fn persist_consumer_contract(&self, view: Option<&ConsumerContractView>) -> Result<(), SandboxError> {
        let Some(store) = &self.store else { return Ok(()); };
        let Some(view) = view else { return Ok(()); };
        let lock = store.lock().unwrap();
        for item in &view.secret_scope {
            let document = serde_json::to_string(item).map_err(|err| {
                SandboxError::Marshal(
                    format!("marshal secret scope binding {}/{}: {}", item.consumer_kind, item.secret_ref, err),
                )
            })?;
            lock.upsert_secret_scope_binding(&dope_store::SecretScopeBindingRecord {
                binding_id: secret_scope_binding_record_id(item),
                consumer_kind: item.consumer_kind.as_str().to_string(),
                consumer_id: item.consumer_id.clone(),
                environment_scope: item.environment_scope.as_str().to_string(),
                secret_ref: item.secret_ref.clone(),
                default_source: item
                    .default_source
                    .map(|source| source.as_str().to_string())
                    .unwrap_or_default(),
                delivery_kind: item.delivery_kind.clone(),
                active: true,
                document,
            })
            .map_err(|err| SandboxError::UpsertSecretScopeBinding(item.secret_ref.clone(), err))?;
        }
        if let Some(record) = &view.policy_record {
            let document = serde_json::to_string(view).map_err(|err| {
                SandboxError::Marshal(
                    format!("marshal consumer policy view {}: {}", record.policy_record_id, err),
                )
            })?;
            lock.upsert_consumer_policy_record(&dope_store::ConsumerPolicyRecordRecord {
                policy_record_id: record.policy_record_id.clone(),
                consumer_kind: record.consumer_kind.as_str().to_string(),
                consumer_id: record.consumer_id.clone(),
                operation_kind: record.operation_kind.clone(),
                declaration_id: record.declaration_id.clone(),
                status: record.status.as_str().to_string(),
                decision: record.decision.as_str().to_string(),
                approval_status: record.approval_status.as_str().to_string(),
                secret_resolution: record.secret_resolution.as_str().to_string(),
                requested_by: record.requested_by.clone(),
                sandbox_execution_id: record.sandbox_execution_id.clone(),
                tool_call_id: record.tool_call_id.clone(),
                provider_operation_id: record.provider_operation_id.clone(),
                started_at: record.started_at,
                completed_at: record.completed_at,
                document,
            })
            .map_err(|err| SandboxError::UpsertConsumerPolicyRecord(record.policy_record_id.clone(), err))?;
        }
        Ok(())
    }

    pub(crate) fn persist_execution(&self, execution: &Execution) -> Result<(), SandboxError> {
        let Some(store) = &self.store else { return Ok(()); };
        let document = serde_json::to_string(execution)
            .map_err(|err| SandboxError::MarshalExecution(execution.execution_id.clone(), err.to_string()))?;
        store
            .lock()
            .unwrap()
            .upsert_sandbox_execution(&dope_store::SandboxExecutionRecord {
                execution_id: execution.execution_id.clone(),
                profile_id: execution.profile_id.clone(),
                backend_kind: execution.backend_kind.as_str().to_string(),
                status: execution.status.as_str().to_string(),
                approval_id: execution.approval_id.clone(),
                requested_at: execution.requested_at,
                updated_at: execution.updated_at,
                started_at: execution.started_at,
                completed_at: execution.completed_at,
                document,
            })
            .map_err(|err| SandboxError::PersistExecution(execution.execution_id.clone(), err))
    }

    fn persist_approval_artifacts(&self, approval_id: &str, decision: &dope_policy::Decision) -> Result<(), SandboxError> {
        let Some(store) = &self.store else { return Ok(()); };
        if approval_id.is_empty() {
            return Ok(());
        }
        let Some(approval) = self.policy.get_approval(approval_id) else {
            return Ok(());
        };
        let lock = store.lock().unwrap();
        lock.upsert_approval(&approval)
            .map_err(|err| SandboxError::PersistApproval(approval_id.to_string(), err))?;
        lock.upsert_decision(decision)
            .map_err(|err| SandboxError::PersistDecision(decision.decision_id.clone(), err))?;
        Ok(())
    }

    fn publish_approval_requested(&self, approval_id: &str, decision: &dope_policy::Decision) -> Result<(), SandboxError> {
        let Some(approval) = self.policy.get_approval(approval_id) else {
            return Ok(());
        };
        let mut approval_payload = serde_json::Map::new();
        approval_payload.insert("action".to_string(), serde_json::json!(approval.action));
        approval_payload.insert("resourceKind".to_string(), serde_json::json!(approval.resource_kind));
        approval_payload.insert("resourceId".to_string(), serde_json::json!(approval.resource_id));
        approval_payload.insert("reason".to_string(), serde_json::json!(approval.reason));
        approval_payload.insert("requestedBy".to_string(), serde_json::json!(approval.requested_by));
        approval_payload.insert("status".to_string(), serde_json::json!(approval.status.as_str()));
        self.publish_event(dope_events::Event {
            category: "policy".to_string(),
            name: "policy.approval_requested".to_string(),
            resource: dope_events::Resource {
                kind: "approval".to_string(),
                id: approval.approval_id.clone(),
            },
            payload: approval_payload,
/// Go `approvalMatchesExecution`.
#[must_use]
pub fn approval_matches_execution(approval: &dope_policy::Approval, execution: &Execution, profile: &Profile) -> bool {
    if approval.action == SANDBOX_APPROVAL_ACTION
        && approval.resource_kind == SANDBOX_RESOURCE_KIND
        && approval.resource_id == profile.profile_id
    {
        return true;
    }
    approval.action == "tool_call.execute"
        && approval.resource_kind == execution.resource_kind.trim()
        && approval.resource_id == execution.resource_id.trim()
}

/// Go `evaluateAccessDecision`.
#[must_use]
pub fn evaluate_access_decision(profile: &Profile, cwd: &str, access: &AccessRequest) -> Decision {
    let mut decision = Decision {
        decision_id: new_id("sandbox_decision"),
        resolution: DecisionResolution::Allow,
        selection_outcome: Some(BackendSelectionOutcome::Selected),
        matched_rules: vec![format!("profile:{}", profile.profile_id)],
        approval_status: DecisionApprovalStatus::NotApplicable,
        effective_profile_id: profile.profile_id.clone(),
        effective_backend_kind: profile.backend_kind,
        host_status: Some(backend_host_status(&profile.backend_capability)),
        explanation: "execution is allowed by sandbox profile".to_string(),
        ..Decision::default()
    };
    let mut approval_required = false;
    let mut reasons: Vec<String> = Vec::with_capacity(4);

    match profile.backend_kind {
        BackendKind::Subprocess => {}
        BackendKind::Docker => {
            if profile.backend_capability.availability_status != BackendAvailabilityStatus::Available {
                mark_decision_unsupported(
                    &mut decision,
                    "backend:unsupported",
                    &first_non_empty(&[
                        profile.backend_capability.availability_reason.as_str(),
                        "docker backend is not available on this host",
                    ]),
                    "backend_unavailable",
                );
                return decision;
            }
            let mismatch = docker_access_mismatch(access);
            if !mismatch.is_empty() {
                mark_decision_unsupported(&mut decision, "backend:mismatch", &mismatch, "backend_capability_mismatch");
                return decision;
            }
        }
        _ => {
            if profile.approval_policy.required_for_unknown_backends {
                approval_required = true;
                reasons.push("backend:approval_required".to_string());
            } else {
                mark_decision_unsupported(
                    &mut decision,
                    "backend:unsupported",
                    "sandbox backend is not available",
                    "backend_unavailable",
                );
                return decision;
            }
        }
    }

    let (fs_decision, fs_rule) = evaluate_filesystem(profile, cwd, access);
    if fs_decision == DecisionResolution::Deny {
        decision.resolution = DecisionResolution::Deny;
        decision.matched_rules.push(fs_rule);
        decision.explanation = "filesystem access is denied by sandbox profile".to_string();
        return decision;
    }
    if fs_decision == DecisionResolution::Ask {
        approval_required = true;
        reasons.push(fs_rule);
    }

    let (net_decision, net_rule) = evaluate_network(profile, access);
    if net_decision == DecisionResolution::Deny {
        decision.resolution = DecisionResolution::Deny;
        decision.matched_rules.push(net_rule);
        decision.explanation = "network access is denied by sandbox profile".to_string();
        return decision;
    }
    if net_decision == DecisionResolution::Ask {
        approval_required = true;
        reasons.push(net_rule);
    }

    if approval_required {
        decision.approval_required = true;
        decision.resolution = DecisionResolution::Ask;
        decision.matched_rules.extend(reasons);
        decision.explanation = "sandbox execution requires approval".to_string();
    }
    decision
}

/// Go `requiredBackendKind`.
#[must_use]
pub fn required_backend_kind(declaration: &ConsumerRequirementDeclaration) -> Option<BackendKind> {
    if declaration.allowed_backend_kinds.len() == 1 {
        return Some(declaration.allowed_backend_kinds[0]);
    }
    None
}

/// Go `backendRequirementUnsupported`.
#[must_use]
pub fn backend_requirement_unsupported(profile_backend: BackendKind, required_backend: Option<BackendKind>, required_strength: &str) -> bool {
    if let Some(required) = required_backend {
        if required != profile_backend {
            return true;
        }
    }
    match required_strength.trim() {
        "" | "declared_only" | "subprocess" => false,
        "containerized" | "docker" => profile_backend != BackendKind::Docker,
        _ => true,
    }
}

/// Go `backendHostStatus`.
#[must_use]
pub fn backend_host_status(capability: &BackendCapabilityProfile) -> BackendHostStatus {
    match capability.availability_status {
        BackendAvailabilityStatus::Available => BackendHostStatus::Ready,
        BackendAvailabilityStatus::Unavailable => BackendHostStatus::MissingPrerequisite,
        BackendAvailabilityStatus::Degraded => BackendHostStatus::RuntimeUnavailable,
    }
}

/// Go `dockerAccessMismatch`.
#[must_use]
pub fn docker_access_mismatch(access: &AccessRequest) -> String {
    if access.network_mode == Some(NetworkMode::AllowList)
        || !access.allowed_hosts.is_empty()
        || !access.allowed_ports.is_empty()
    {
        return "docker backend cannot yet enforce host or port allow-lists for this request".to_string();
    }
    if access.allow_loopback {
        return "docker backend cannot yet provide explicit loopback-only guarantees for this request".to_string();
    }
    String::new()
}

/// Go `markDecisionUnsupported`.
pub fn mark_decision_unsupported(decision: &mut Decision, rule: &str, explanation: &str, mismatch_reason: &str) {
    decision.resolution = DecisionResolution::Deny;
    decision.selection_outcome = Some(BackendSelectionOutcome::Unsupported);
    decision.approval_required = false;
    decision.approval_status = DecisionApprovalStatus::NotApplicable;
    decision.matched_rules.push(rule.to_string());
    decision.explanation = explanation.to_string();
    decision.mismatch_reason = mismatch_reason.to_string();
}

/// Go `markDecisionDenied`.
pub fn mark_decision_denied(decision: &mut Decision, rule: &str, explanation: &str, mismatch_reason: &str) {
    decision.resolution = DecisionResolution::Deny;
    decision.selection_outcome = Some(BackendSelectionOutcome::Denied);
    decision.approval_required = false;
    decision.approval_status = DecisionApprovalStatus::NotApplicable;
    decision.matched_rules.push(rule.to_string());
    decision.explanation = explanation.to_string();
    decision.mismatch_reason = mismatch_reason.to_string();
}

/// Go `consumerID`.
#[must_use]
pub fn consumer_id(view: &ConsumerContractView) -> String {
    if let Some(record) = &view.policy_record {
        if !record.consumer_id.trim().is_empty() {
            return record.consumer_id.trim().to_string();
        }
    }
    if let Some(declaration) = &view.declaration {
        return declaration.consumer_id.trim().to_string();
    }
    String::new()
}

/// Go `decisionToStatus`.
#[must_use]
pub fn decision_to_status(decision: &Decision) -> ExecutionStatus {
    if decision.resolution == DecisionResolution::Allow {
        return ExecutionStatus::Pending;
    }
    if decision.selection_outcome == Some(BackendSelectionOutcome::Unsupported) {
        return ExecutionStatus::Unsupported;
    }
    ExecutionStatus::Denied
}

/// Go `decisionErrorClass`.
#[must_use]
pub fn decision_error_class(decision: &Decision) -> ErrorClass {
    if decision.matched_rules.iter().any(|rule| rule == "backend:mismatch") {
        return ErrorClass::BackendMismatch;
    }
    if decision.matched_rules.iter().any(|rule| rule == "enforcement:unsupported" || rule == "backend:unsupported") {
        return ErrorClass::BackendMissing;
    }
    if decision.approval_status == DecisionApprovalStatus::Rejected {
        return ErrorClass::ApprovalRejected;
    }
    if decision.approval_required {
        return ErrorClass::ApprovalRequired;
    }
    ErrorClass::PolicyDenied
}

/// Go `decisionErrorCode`.
#[must_use]
pub fn decision_error_code(decision: &Decision) -> String {
    if decision.matched_rules.iter().any(|rule| rule == "backend:mismatch") {
        return "sandbox_backend_mismatch".to_string();
    }
    if decision.matched_rules.iter().any(|rule| rule == "enforcement:unsupported" || rule == "backend:unsupported") {
        return "sandbox_backend_unsupported".to_string();
    }
    if decision.approval_status == DecisionApprovalStatus::Rejected {
        return "sandbox_approval_rejected".to_string();
    }
    if decision.approval_required {
        return "sandbox_approval_required".to_string();
    }
    "sandbox_policy_denied".to_string()
}

/// Go `synchronizeExecutionConsumerState`: propagates the execution status
/// into the consumer policy record and mirrors it into the result/decision
/// consumer views.
pub(crate) fn synchronize_execution_consumer_state(execution: &mut Execution) {
    let Some(consumer) = &mut execution.consumer else { return };
    let Some(record) = &mut consumer.policy_record else { return };
    record.sandbox_execution_id = execution.execution_id.clone();
    record.decision = execution.decision.resolution;
    record.approval_status = execution.decision.approval_status;
    record.secret_resolution = secret_resolution_from_consumer(consumer);
    if record.enforcement_strength.is_empty() {
        if let Some(declaration) = &consumer.declaration {
            record.enforcement_strength = declaration.required_enforcement_strength.trim().to_string();
        }
    }
    if record.enforcement_strength.is_empty() {
        record.enforcement_strength = "declared_only".to_string();
    }
    match execution.status {
        ExecutionStatus::Running => {
            record.status = PolicyRecordStatus::Running;
        }
        ExecutionStatus::Completed => {
            record.status = PolicyRecordStatus::Completed;
            record.completed_at = Some(Utc::now());
        }
        ExecutionStatus::Failed => {
            record.status = PolicyRecordStatus::Failed;
            record.failure_class = execution.result.error_class.clone();
            record.completed_at = Some(Utc::now());
        }
        ExecutionStatus::Cancelled => {
            record.status = PolicyRecordStatus::Cancelled;
            record.failure_class = execution.result.error_class.clone();
            record.completed_at = Some(Utc::now());
        }
        ExecutionStatus::Unsupported => {
            record.status = PolicyRecordStatus::Unsupported;
            record.failure_class = execution.result.error_class.clone();
            record.completed_at = Some(Utc::now());
        }
        ExecutionStatus::Denied => {
            if record.status != PolicyRecordStatus::Unsupported {
                record.status = if execution.decision.approval_status == DecisionApprovalStatus::Pending {
                    PolicyRecordStatus::ApprovalPending
                } else {
                    PolicyRecordStatus::Denied
                };
            }
            record.failure_class = execution.result.error_class.clone();
            record.completed_at = Some(Utc::now());
        }
        _ => {
            record.status = PolicyRecordStatus::PreflightAllowed;
        }
    }
    let mirror = execution.consumer.clone();
    if execution.result.consumer.is_none() {
        execution.result.consumer = mirror.clone();
    } else if let Some(result_consumer) = &mut execution.result.consumer {
        result_consumer.policy_record = mirror.as_ref().and_then(|consumer| consumer.policy_record.clone());
    }
    if execution.decision.consumer.is_none() {
        execution.decision.consumer = mirror;
    } else if let Some(decision_consumer) = &mut execution.decision.consumer {
        decision_consumer.policy_record = execution
            .consumer
            .as_ref()
            .and_then(|consumer| consumer.policy_record.clone());
    }
}

/// Go `secretResolutionFromConsumer`.
#[must_use]
pub fn secret_resolution_from_consumer(view: &ConsumerContractView) -> SecretResolution {
    if view.secret_scope.is_empty() {
        return SecretResolution::NotApplicable;
    }
    let mut resolution = SecretResolution::Resolved;
    for item in &view.secret_scope {
        match item.resolution {
            SecretResolution::Unavailable => return SecretResolution::Unavailable,
            SecretResolution::Denied => {
                resolution = SecretResolution::Denied;
            }
            SecretResolution::Resolved => {}
            other => {
                if resolution == SecretResolution::Resolved {
                    resolution = other;
                }
            }
        }
    }
    resolution
}

/// Go `secretScopeBindingRecordID`.
fn secret_scope_binding_record_id(item: &SecretScopeOutcome) -> String {
    let base = first_non_empty(&[item.default_rule_id.as_str(), &new_id("secret_binding")]);
    let mut parts: Vec<String> = vec![base];
    let scope = item.environment_scope.as_str().trim();
    if !scope.is_empty() {
        parts.push(scope.to_string());
    }
    let secret_ref = item.secret_ref.trim();
    if !secret_ref.is_empty() {
        parts.push(secret_ref.to_string());
    }
    parts.join(":")
}

/// Go `commandApprovalRule`.
#[must_use]
pub fn command_approval_rule(profile: &Profile, command: &str) -> String {
    let base = std::path::Path::new(command.trim())
        .file_name()
        .map(|name| name.to_string_lossy().to_string())
        .unwrap_or_default();
    for required in &profile.approval_policy.required_for_commands {
        let required = required.trim();
        if required.eq_ignore_ascii_case(command.trim()) || required.eq_ignore_ascii_case(&base) {
            return "command:approval_required".to_string();
        }
    }
    String::new()
}

/// Go `evaluateFilesystem` (the error return is dead in the Go source; the
/// decision + matched rule are returned).
#[must_use]
pub fn evaluate_filesystem(profile: &Profile, cwd: &str, access: &AccessRequest) -> (DecisionResolution, String) {
    match profile.filesystem_policy.mode {
        FilesystemMode::Full => return (DecisionResolution::Allow, "filesystem:full_access".to_string()),
        FilesystemMode::None => {
            if !cwd.is_empty() || !access.read_roots.is_empty() || !access.write_roots.is_empty() {
                return (DecisionResolution::Deny, "filesystem:none".to_string());
            }
            return (DecisionResolution::Allow, "filesystem:none".to_string());
        }
        FilesystemMode::Scoped => {}
    }

    let read_roots = effective_read_roots(profile);
    let write_roots = effective_write_roots(profile);
    let mut all_roots = read_roots.clone();
    all_roots.extend(write_roots.iter().cloned());
    if !cwd.is_empty() && !within_any(cwd, &all_roots) {
        return (DecisionResolution::Deny, "filesystem:cwd_outside_scoped_roots".to_string());
    }
    for root in &access.read_roots {
        if !within_any(root, &read_roots) && !within_any(root, &write_roots) {
            return (DecisionResolution::Deny, "filesystem:read_outside_scoped_roots".to_string());
        }
    }
    for root in &access.write_roots {
        if !within_any(root, &write_roots) {
            if profile.approval_policy.required_for_writes_outside_roots {
                return (DecisionResolution::Ask, "filesystem:write_outside_roots_requires_approval".to_string());
            }
            return (DecisionResolution::Deny, "filesystem:write_outside_scoped_roots".to_string());
        }
    }
    if !cwd.is_empty() && !within_any(cwd, &write_roots) && profile.approval_policy.required_for_writes_outside_roots {
        return (DecisionResolution::Ask, "filesystem:cwd_write_scope_requires_approval".to_string());
    }
    (DecisionResolution::Allow, "filesystem:scoped".to_string())
}

/// Go `evaluateNetwork`.
#[must_use]
pub fn evaluate_network(profile: &Profile, access: &AccessRequest) -> (DecisionResolution, String) {
    if access.network_mode.is_none() || access.network_mode == Some(NetworkMode::Deny) {
        return (DecisionResolution::Allow, "network:none".to_string());
    }
    match profile.network_policy.mode {
        NetworkMode::Full => (DecisionResolution::Allow, "network:full".to_string()),
        NetworkMode::Deny => {
            if profile.approval_policy.required_for_network {
                (DecisionResolution::Ask, "network:approval_required".to_string())
            } else {
                (DecisionResolution::Deny, "network:denied".to_string())
            }
        }
        NetworkMode::AllowList => {
            if access.network_mode == Some(NetworkMode::Full) {
                if profile.approval_policy.required_for_network {
                    return (DecisionResolution::Ask, "network:approval_required".to_string());
                }
                return (DecisionResolution::Deny, "network:mode_exceeds_profile".to_string());
            }
            if access.allow_loopback && !profile.network_policy.allow_loopback {
                if profile.approval_policy.required_for_network {
                    return (DecisionResolution::Ask, "network:loopback_requires_approval".to_string());
                }
                return (DecisionResolution::Deny, "network:loopback_denied".to_string());
            }
            if !subset_strings(&access.allowed_hosts, &profile.network_policy.allowed_hosts)
                || !subset_ints(&access.allowed_ports, &profile.network_policy.allowed_ports)
            {
                if profile.approval_policy.required_for_network {
                    return (DecisionResolution::Ask, "network:allow_list_requires_approval".to_string());
                }
                return (DecisionResolution::Deny, "network:allow_list_denied".to_string());
            }
            (DecisionResolution::Allow, "network:allow_list".to_string())
        }
        _ => (DecisionResolution::Deny, "network:unknown_mode".to_string()),
    }
}
