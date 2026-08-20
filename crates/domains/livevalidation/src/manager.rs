//! Manager and the start-gate pipeline (port of `manager.go`).

use std::sync::Arc;

use chrono::DateTime;
use chrono::Utc;
use kura_billing::DenialPayload;
use kura_billing::ResolveInput;
use kura_billing::UsageReservation;
use kura_billing::live_validation_operation_key;
use kura_billing::reserve_live_validation_preflight;
use kura_identity::Permission;
use kura_identity::TenantContext;
use kura_identity::evaluate_permission;
use kura_identity::tenantctx;
use serde::Deserialize;
use serde::Serialize;

use crate::approval::normalize_fresh_approvals;
use crate::error::LiveValidationError;
use crate::error::MatrixError;
use crate::error::StartFailure;
use crate::events::LedgerEventSink;
use crate::matrix::Matrix;
use crate::matrix::ToolClass;
use crate::matrix::default_matrix_rows;
use crate::readiness::CandidateReadinessInput;
use crate::readiness::ReadinessStatus;
use crate::readiness::evaluate_candidate_readiness;
use crate::store::Store;
use crate::types::Attempt;
use crate::types::AttemptStatus;
use crate::types::FreshApproval;
use crate::types::GateDecision;
use crate::types::SideEffectScope;

/// Clock override for deterministic tests (Go `Dependencies.Clock`).
pub type Clock = Arc<dyn Fn() -> DateTime<Utc> + Send + Sync>;

/// Resolves the tool classes a candidate can reach (Go
/// `CandidateToolClassResolver`).
pub type CandidateToolClassResolver = Arc<
    dyn Fn(String) -> crate::store::BoxFuture<'static, Result<Vec<ToolClass>, LiveValidationError>>
        + Send
        + Sync,
>;

/// Port of `Dependencies`.
pub struct Dependencies {
    pub environment_scope: String,
    pub store: Option<Arc<dyn Store>>,
    pub enabled: bool,
    pub billing: Option<Arc<kura_billing::Manager>>,
    pub hosted_billing: bool,
    pub clock: Option<Clock>,
    pub ledger_event_sink: Option<LedgerEventSink>,
    pub candidate_tool_class_resolver: Option<CandidateToolClassResolver>,
}

/// Port of `Manager`.
pub struct Manager {
    environment_scope: String,
    store: Option<Arc<dyn Store>>,
    enabled: bool,
    billing: Option<Arc<kura_billing::Manager>>,
    hosted_billing: bool,
    clock: Clock,
    ledger_event_sink: Option<LedgerEventSink>,
    candidate_tool_class_resolver: Option<CandidateToolClassResolver>,
}

impl Manager {
    /// Port of `NewManager`.
    #[must_use]
    pub fn new(deps: Dependencies) -> Self {
        let clock = deps.clock.unwrap_or_else(|| Arc::new(Utc::now));
        let environment_scope = if deps.environment_scope.is_empty() {
            "test".to_string()
        } else {
            deps.environment_scope
        };
        Manager {
            environment_scope,
            store: deps.store,
            enabled: deps.enabled,
            billing: deps.billing,
            hosted_billing: deps.hosted_billing,
            clock,
            ledger_event_sink: deps.ledger_event_sink,
            candidate_tool_class_resolver: deps.candidate_tool_class_resolver,
        }
    }

    pub(crate) fn now(&self) -> DateTime<Utc> {
        (self.clock)()
    }

    pub(crate) fn store(&self) -> Option<&Arc<dyn Store>> {
        self.store.as_ref()
    }

    pub(crate) fn emit_ledger_event(
        &self,
        event_name: &str,
        entry: &crate::types::SideEffectLedgerEntry,
    ) {
        if let Some(sink) = &self.ledger_event_sink {
            sink(event_name, entry);
        }
    }

    /// Port of `Manager.Enabled`.
    #[must_use]
    pub fn enabled(&self) -> bool {
        self.enabled
    }

    /// Port of `Manager.EnvironmentScope`.
    #[must_use]
    pub fn environment_scope(&self) -> &str {
        if self.environment_scope.is_empty() {
            "test"
        } else {
            &self.environment_scope
        }
    }

    /// Port of `Manager.SupportMatrix`.
    pub fn support_matrix(&self) -> Result<Matrix, MatrixError> {
        Matrix::new(default_matrix_rows())
    }

    /// Port of `Manager.Start`: runs the gate pipeline — tenant context,
    /// permission, quota reservation, kill switch, support matrix, fresh
    /// approvals — then persists the attempt as awaiting-approval or running.
    pub async fn start(&self, input: StartInput) -> Result<StartResult, StartFailure> {
        if !self.enabled() {
            return Err(StartFailure::Disabled);
        }
        let now = self.now();
        let tenant_context = tenantctx::from_context().unwrap_or_default();
        if tenant_context.tenant_id.is_empty() || tenant_context.principal_id.is_empty() {
            let attempt = self.new_attempt(&input, &TenantContext::default(), now);
            return Err(self
                .block(
                    attempt,
                    "permission",
                    "tenant_context_missing",
                    "Tenant context is required.",
                    "",
                )
                .await?);
        }
        let mut attempt = self.new_attempt(&input, &tenant_context, now);

        let permission = evaluate_permission(&tenant_context, Permission::LiveValidationExecute);
        attempt.permission_decision = GateDecision {
            allowed: permission.allowed,
            reason_code: permission.reason_code.clone(),
            ..GateDecision::default()
        };
        attempt.permission_decision.checked_at = now;
        if !permission.allowed {
            return Err(self
                .block(
                    attempt,
                    "permission",
                    &first_non_empty([permission.reason_code.as_str(), "permission_missing"]),
                    "Missing live validation permission.",
                    "live_validation.execute",
                )
                .await?);
        }

        let (quota_decision, quota_denial, reservation) = self
            .evaluate_quota(
                &tenant_context.tenant_id,
                &attempt.validation_id,
                &input.client_key,
                now,
            )
            .await;
        attempt.quota_decision = quota_decision.clone();
        if !quota_decision.allowed {
            return Err(self
                .block(
                    attempt,
                    "quota",
                    &first_non_empty([quota_decision.reason_code.as_str(), "quota_denied"]),
                    &first_non_empty([
                        quota_denial.message.as_str(),
                        "Live validation quota denied.",
                    ]),
                    &quota_denial.operation_key,
                )
                .await?);
        }

        let (kill_switch_decision, kill_denial) = self
            .evaluate_kill_switch(&tenant_context.tenant_id, now)
            .await?;
        // NOTE: the quota release on the error path must happen before return.
        // Re-structure: evaluate_kill_switch returns Result; release on error.
        attempt.kill_switch_decision = kill_switch_decision.clone();
        if !kill_switch_decision.allowed {
            self.release_quota(
                &reservation,
                "live validation blocked by kill switch before start",
            )
            .await;
            return Err(self
                .block(
                    attempt,
                    "kill_switch",
                    &kill_switch_decision.reason_code,
                    &kill_denial.message,
                    &kill_denial.reference,
                )
                .await?);
        }

        let input = match self.resolve_candidate_tool_classes(input).await {
            Ok(input) => input,
            Err(err) => {
                self.release_quota(
                    &reservation,
                    "live validation candidate tool class resolution failed before start",
                )
                .await;
                return Err(err.into());
            }
        };
        if let Some(denial) = self.evaluate_support(&input) {
            self.release_quota(
                &reservation,
                "live validation support check blocked before start",
            )
            .await;
            return Err(self
                .block(
                    attempt,
                    &denial.gate,
                    &denial.reason_code,
                    &denial.message,
                    &denial.reference,
                )
                .await?);
        }

        let fresh_approvals = normalize_fresh_approvals(input.fresh_approvals, &attempt);
        let approval_summary =
            self.approval_summary(&attempt, &input.requested_scope, &fresh_approvals);
        attempt.approval_summary = approval_summary.clone();
        if approval_summary.denied > 0 {
            self.release_quota(
                &reservation,
                "live validation approval denied before live start",
            )
            .await;
            return Err(self
                .block(
                    attempt,
                    "approval",
                    "live_validation.approval_denied",
                    "A required fresh approval was denied.",
                    "",
                )
                .await?);
        }
        if approval_summary.expired > 0 {
            self.release_quota(
                &reservation,
                "live validation approval expired before live start",
            )
            .await;
            return Err(self
                .block(
                    attempt,
                    "approval",
                    "live_validation.approval_expired",
                    "A required fresh approval is expired.",
                    "",
                )
                .await?);
        }
        if approval_summary.pending > 0 {
            attempt.status = AttemptStatus::from(AttemptStatus::AWAITING_APPROVAL);
            if let Err(err) = self.persist_attempt(&attempt).await {
                self.release_quota(
                    &reservation,
                    "live validation awaiting-approval attempt failed to persist",
                )
                .await;
                return Err(err.into());
            }
            self.release_quota(
                &reservation,
                "live validation awaits approval before live start",
            )
            .await;
            return Ok(StartResult {
                attempt,
                denials: Vec::new(),
            });
        }

        attempt.status = AttemptStatus::from(AttemptStatus::RUNNING);
        attempt.started_at = Some(now);
        if let Err(err) = self.persist_attempt(&attempt).await {
            self.release_quota(
                &reservation,
                "live validation running attempt failed to persist",
            )
            .await;
            return Err(err.into());
        }
        self.commit_quota(
            &reservation,
            "live_validation.started",
            "live validation started after gates passed",
        )
        .await?;
        Ok(StartResult {
            attempt,
            denials: Vec::new(),
        })
    }

    /// Port of `Manager.evaluateQuota`. A failure or denial both surface as
    /// `allowed: false`; the reservation is returned so later gates can
    /// release it.
    async fn evaluate_quota(
        &self,
        tenant_id: &str,
        validation_id: &str,
        client_key: &str,
        now: DateTime<Utc>,
    ) -> (GateDecision, DenialPayload, Option<UsageReservation>) {
        let operation_key = live_validation_operation_key(tenant_id, validation_id, client_key);
        match reserve_live_validation_preflight(
            self.billing.as_deref(),
            tenant_id,
            validation_id,
            client_key,
            self.hosted_billing,
        )
        .await
        {
            Ok(result) => {
                if result.failure.is_some() || !result.allowed {
                    let denial = result.denial.clone().unwrap_or_default();
                    let reason_code = first_non_empty([
                        denial.reason_code.as_str(),
                        result
                            .failure
                            .as_ref()
                            .map_or_else(String::new, ToString::to_string)
                            .as_str(),
                    ]);
                    (
                        GateDecision {
                            allowed: false,
                            reason_code,
                            reference: operation_key,
                            checked_at: now,
                        },
                        denial,
                        result.reservation,
                    )
                } else {
                    (
                        GateDecision {
                            allowed: result.allowed,
                            reference: operation_key,
                            ..GateDecision::default()
                        },
                        DenialPayload::default(),
                        result.reservation,
                    )
                }
            }
            Err(err) => (
                GateDecision {
                    allowed: false,
                    reason_code: err.to_string(),
                    reference: operation_key,
                    checked_at: now,
                },
                DenialPayload::default(),
                None,
            ),
        }
    }

    /// Port of `Manager.resolveCandidateToolClasses`.
    async fn resolve_candidate_tool_classes(
        &self,
        mut input: StartInput,
    ) -> Result<StartInput, LiveValidationError> {
        if !input.candidate_tool_classes.is_empty()
            || self.candidate_tool_class_resolver.is_none()
            || input.candidate_id.is_empty()
        {
            return Ok(input);
        }
        let resolver = self
            .candidate_tool_class_resolver
            .as_ref()
            .expect("checked above");
        let classes = resolver(input.candidate_id.clone()).await?;
        input.candidate_tool_classes = dedupe_tool_classes(classes);
        Ok(input)
    }

    /// Port of `Manager.releaseQuota`: best-effort, errors swallowed.
    pub(crate) async fn release_quota(&self, reservation: &Option<UsageReservation>, reason: &str) {
        let (Some(billing), Some(reservation)) = (self.billing.as_ref(), reservation) else {
            return;
        };
        if reservation.reservation_id.is_empty() {
            return;
        }
        let _ = billing
            .release(ResolveInput {
                tenant_id: reservation.tenant_id.clone(),
                category: reservation.category.clone(),
                operation_key: reservation.operation_key.clone(),
                amount: reservation.amount_reserved,
                reason_code: "live_validation.preflight_released".to_string(),
                reason: reason.to_string(),
                ..ResolveInput::default()
            })
            .await;
    }

    /// Port of `Manager.commitQuota`.
    async fn commit_quota(
        &self,
        reservation: &Option<UsageReservation>,
        reason_code: &str,
        reason: &str,
    ) -> Result<(), LiveValidationError> {
        let (Some(billing), Some(reservation)) = (self.billing.as_ref(), reservation) else {
            return Ok(());
        };
        if reservation.reservation_id.is_empty() {
            return Ok(());
        }
        billing
            .commit(ResolveInput {
                tenant_id: reservation.tenant_id.clone(),
                category: reservation.category.clone(),
                operation_key: reservation.operation_key.clone(),
                amount: reservation.amount_reserved,
                reason_code: reason_code.to_string(),
                reason: reason.to_string(),
                ..ResolveInput::default()
            })
            .await?;
        Ok(())
    }

    /// Port of `Manager.evaluateSupport`.
    fn evaluate_support(&self, input: &StartInput) -> Option<Denial> {
        let matrix = match self.support_matrix() {
            Ok(matrix) => matrix,
            Err(err) => {
                return Some(Denial {
                    gate: "support_matrix".to_string(),
                    reason_code: "live_validation.support_matrix_invalid".to_string(),
                    message: err.to_string(),
                    reference: String::new(),
                });
            }
        };
        let scope = &input.requested_scope;
        let reachable_classes = &input.candidate_tool_classes;
        if reachable_classes.is_empty() {
            return Some(Denial {
                gate: "support_matrix".to_string(),
                reason_code: "live_validation.candidate_tool_classes_required".to_string(),
                message: "Live validation requires explicit candidate tool classes.".to_string(),
                reference: String::new(),
            });
        }
        let readiness = evaluate_candidate_readiness(
            &matrix,
            CandidateReadinessInput {
                candidate_id: input.candidate_id.clone(),
                reachable_tool_classes: reachable_classes.clone(),
                requested_scope: scope.clone(),
            },
        );
        if readiness.status == ReadinessStatus::BLOCKED {
            let reference = readiness
                .unsupported_classes
                .first()
                .map_or(String::new(), ToString::to_string);
            return Some(Denial {
                gate: "support_matrix".to_string(),
                reason_code: "live_validation.unsupported_tool_class".to_string(),
                message:
                    "Unsupported candidate tool classes must be explicitly excluded from live validation scope."
                        .to_string(),
                reference,
            });
        }
        for tool_class in &scope.included_tool_classes {
            if matrix.lookup(tool_class).is_err() {
                return Some(Denial {
                    gate: "support_matrix".to_string(),
                    reason_code: "live_validation.unsupported_tool_class".to_string(),
                    message: format!("Tool class {tool_class} is unsupported for live validation."),
                    reference: tool_class.to_string(),
                });
            }
        }
        None
    }
}

/// Port of `StartInput`.
#[derive(Debug, Clone, Default)]
pub struct StartInput {
    pub validation_id: String,
    pub candidate_id: String,
    pub source_attempt_id: String,
    pub candidate_tool_classes: Vec<ToolClass>,
    pub requested_scope: SideEffectScope,
    pub fresh_approvals: Vec<FreshApproval>,
    pub client_key: String,
    pub change_window_label: String,
}

/// A single gate denial (Go `Denial`).
#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Denial {
    pub gate: String,
    pub reason_code: String,
    pub message: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub reference: String,
}

/// Port of `StartResult`.
#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct StartResult {
    pub attempt: Attempt,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub denials: Vec<Denial>,
}

/// Port of `firstNonEmpty`.
pub(crate) fn first_non_empty<'a>(values: impl IntoIterator<Item = &'a str>) -> String {
    for value in values {
        if !value.is_empty() {
            return value.to_string();
        }
    }
    String::new()
}

/// Port of `dedupeToolClasses`.
pub(crate) fn dedupe_tool_classes(items: Vec<ToolClass>) -> Vec<ToolClass> {
    let mut deduped = Vec::with_capacity(items.len());
    for item in items {
        if item.is_empty() || deduped.contains(&item) {
            continue;
        }
        deduped.push(item);
    }
    deduped
}

/// Port of `newID`: `<prefix>_<16 hex chars>`.
pub(crate) fn new_id(prefix: &str) -> String {
    let uuid = uuid::Uuid::new_v4().simple().to_string();
    format!("{prefix}_{}", &uuid[..16])
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::sync::Arc;

    use kura_identity::LifecycleStatus;
    use kura_identity::Role;
    use kura_identity::TenantContext;
    use kura_identity::permissions_for_role;
    use kura_identity::tenantctx;

    use crate::error::LiveValidationError;
    use crate::error::StartFailure;
    use crate::matrix::ToolClass;
    use crate::testutil::MemStore;
    use crate::testutil::fixed_clock;
    use crate::testutil::operator_context;
    use crate::types::ApprovalMode;
    use crate::types::ApprovalStatus;
    use crate::types::ApprovalTarget;
    use crate::types::AttemptStatus;
    use crate::types::FreshApproval;
    use crate::types::SideEffectScope;

    fn make_scope(included: Vec<ToolClass>, excluded: Vec<ToolClass>) -> SideEffectScope {
        SideEffectScope {
            scope_id: "scope_1".to_string(),
            included_tool_classes: included,
            excluded_tool_classes: excluded,
            approval_mode: ApprovalMode::from(ApprovalMode::SCOPE_LEVEL),
            declared_by: "prn_operator".to_string(),
            declared_at: fixed_clock(),
            ..SideEffectScope::default()
        }
    }

    fn hosted_manager() -> Manager {
        let clock: Clock = Arc::new(fixed_clock);
        Manager::new(Dependencies {
            environment_scope: "test".to_string(),
            store: None,
            enabled: true,
            billing: None,
            hosted_billing: true,
            clock: Some(clock),
            ledger_event_sink: None,
            candidate_tool_class_resolver: None,
        })
    }

    fn viewer() -> TenantContext {
        TenantContext {
            tenant_id: "ten_1".to_string(),
            principal_id: "prn_viewer".to_string(),
            role: Some(Role::Viewer),
            permissions: permissions_for_role(Role::Viewer, LifecycleStatus::Active),
            ..TenantContext::default()
        }
    }

    #[tokio::test]
    async fn start_denies_missing_execute_permission_before_quota() {
        let manager = hosted_manager();
        let result = tenantctx::scope(viewer(), async {
            manager
                .start(StartInput {
                    validation_id: "lv_permission".to_string(),
                    candidate_id: "candidate_1".to_string(),
                    candidate_tool_classes: vec![ToolClass::from(
                        ToolClass::DAEMON_INSPECTION_READ,
                    )],
                    requested_scope: make_scope(
                        vec![ToolClass::from(ToolClass::DAEMON_INSPECTION_READ)],
                        vec![],
                    ),
                    ..StartInput::default()
                })
                .await
        })
        .await;

        let Err(StartFailure::Blocked(start_result)) = result else {
            panic!("expected permission block");
        };
        assert_eq!(
            start_result.attempt.status,
            AttemptStatus::from(AttemptStatus::BLOCKED)
        );
        assert_eq!(start_result.denials.len(), 1);
        assert_eq!(start_result.denials[0].gate, "permission");
        assert!(start_result.attempt.quota_decision.allowed);
    }

    #[tokio::test]
    async fn start_requires_unsupported_candidate_classes_to_be_explicitly_excluded() {
        let store = Arc::new(MemStore::default());
        let manager = Manager::new(Dependencies {
            environment_scope: "test".to_string(),
            store: Some(store),
            enabled: true,
            billing: None,
            hosted_billing: false,
            clock: Some(Arc::new(fixed_clock)),
            ledger_event_sink: None,
            candidate_tool_class_resolver: None,
        });

        let included = vec![ToolClass::from(ToolClass::DAEMON_INSPECTION_READ)];
        let candidate_classes = vec![
            ToolClass::from(ToolClass::DAEMON_INSPECTION_READ),
            ToolClass::from(ToolClass::MCP_TOOL_CALL),
        ];

        let blocked = tenantctx::scope(operator_context(), async {
            manager
                .start(StartInput {
                    validation_id: "lv_mixed_blocked".to_string(),
                    candidate_id: "candidate_mixed".to_string(),
                    candidate_tool_classes: candidate_classes.clone(),
                    requested_scope: make_scope(included.clone(), vec![]),
                    fresh_approvals: vec![FreshApproval {
                        approval_id: "approval_scope".to_string(),
                        validation_id: "lv_mixed_blocked".to_string(),
                        tenant_id: "ten_1".to_string(),
                        approval_target: ApprovalTarget::from(ApprovalTarget::SCOPE),
                        tool_class: ToolClass::from(ToolClass::DAEMON_INSPECTION_READ),
                        safety_class: crate::matrix::SafetyClass::from(
                            crate::matrix::SafetyClass::READ_ONLY,
                        ),
                        approved_scope: "scope_1".to_string(),
                        status: ApprovalStatus::from(ApprovalStatus::APPROVED),
                        ..FreshApproval::default()
                    }],
                    ..StartInput::default()
                })
                .await
        })
        .await;

        let Err(StartFailure::Blocked(blocked_result)) = blocked else {
            panic!("expected support_matrix block for mixed candidate");
        };
        assert_eq!(blocked_result.denials[0].gate, "support_matrix");

        let running = tenantctx::scope(operator_context(), async {
            manager
                .start(StartInput {
                    validation_id: "lv_mixed_running".to_string(),
                    candidate_id: "candidate_mixed".to_string(),
                    candidate_tool_classes: candidate_classes,
                    requested_scope: make_scope(
                        included,
                        vec![ToolClass::from(ToolClass::MCP_TOOL_CALL)],
                    ),
                    fresh_approvals: vec![FreshApproval {
                        approval_id: "approval_scope".to_string(),
                        validation_id: "lv_mixed_running".to_string(),
                        tenant_id: "ten_1".to_string(),
                        approval_target: ApprovalTarget::from(ApprovalTarget::SCOPE),
                        tool_class: ToolClass::from(ToolClass::DAEMON_INSPECTION_READ),
                        safety_class: crate::matrix::SafetyClass::from(
                            crate::matrix::SafetyClass::READ_ONLY,
                        ),
                        approved_scope: "scope_1".to_string(),
                        status: ApprovalStatus::from(ApprovalStatus::APPROVED),
                        ..FreshApproval::default()
                    }],
                    ..StartInput::default()
                })
                .await
        })
        .await;

        let Ok(running_result) = running else {
            panic!("expected mixed candidate with exclusion to run");
        };
        assert_eq!(
            running_result.attempt.status,
            AttemptStatus::from(AttemptStatus::RUNNING)
        );
    }

    #[tokio::test]
    async fn start_resolves_candidate_tool_classes_before_support_gate() {
        let store = Arc::new(MemStore::default());
        let resolver: CandidateToolClassResolver = Arc::new(move |_candidate_id: String| {
            Box::pin(async move {
                Ok::<Vec<ToolClass>, LiveValidationError>(vec![
                    ToolClass::from(ToolClass::DAEMON_INSPECTION_READ),
                    ToolClass::from(ToolClass::MCP_TOOL_CALL),
                ])
            })
        });
        let manager = Manager::new(Dependencies {
            environment_scope: "test".to_string(),
            store: Some(store),
            enabled: true,
            billing: None,
            hosted_billing: false,
            clock: Some(Arc::new(fixed_clock)),
            ledger_event_sink: None,
            candidate_tool_class_resolver: Some(resolver),
        });

        let blocked = tenantctx::scope(operator_context(), async {
            manager
                .start(StartInput {
                    validation_id: "lv_resolved_mixed".to_string(),
                    candidate_id: "candidate_mixed".to_string(),
                    requested_scope: make_scope(
                        vec![ToolClass::from(ToolClass::DAEMON_INSPECTION_READ)],
                        vec![],
                    ),
                    fresh_approvals: vec![FreshApproval {
                        approval_id: "approval_scope".to_string(),
                        validation_id: "lv_resolved_mixed".to_string(),
                        tenant_id: "ten_1".to_string(),
                        approval_target: ApprovalTarget::from(ApprovalTarget::SCOPE),
                        tool_class: ToolClass::from(ToolClass::DAEMON_INSPECTION_READ),
                        safety_class: crate::matrix::SafetyClass::from(
                            crate::matrix::SafetyClass::READ_ONLY,
                        ),
                        approved_scope: "scope_1".to_string(),
                        status: ApprovalStatus::from(ApprovalStatus::APPROVED),
                        ..FreshApproval::default()
                    }],
                    ..StartInput::default()
                })
                .await
        })
        .await;

        let Err(StartFailure::Blocked(blocked_result)) = blocked else {
            panic!("expected support_matrix block for resolved candidate");
        };
        assert_eq!(blocked_result.denials[0].gate, "support_matrix");
        assert_eq!(blocked_result.denials[0].reference, "mcp.tool_call");
    }

    #[tokio::test]
    async fn start_hosted_quota_unavailable_fails_closed() {
        let manager = hosted_manager();
        let result = tenantctx::scope(operator_context(), async {
            manager
                .start(StartInput {
                    validation_id: "lv_quota".to_string(),
                    candidate_id: "candidate_1".to_string(),
                    candidate_tool_classes: vec![ToolClass::from(
                        ToolClass::DAEMON_INSPECTION_READ,
                    )],
                    requested_scope: make_scope(
                        vec![ToolClass::from(ToolClass::DAEMON_INSPECTION_READ)],
                        vec![],
                    ),
                    ..StartInput::default()
                })
                .await
        })
        .await;

        let Err(StartFailure::Blocked(start_result)) = result else {
            panic!("expected quota block");
        };
        assert_eq!(start_result.denials[0].gate, "quota");
        assert!(!start_result.attempt.quota_decision.allowed);
        assert_eq!(
            start_result.attempt.quota_decision.reason_code,
            "quota_denied:quota_state_unavailable"
        );
    }
}
