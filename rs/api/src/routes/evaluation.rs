//! evaluation2 route family — port of daemon/internal/api/evaluation.go,
//! the store-backed half of evaluation_product.go, and live_validation.go.
//!
//! Routes under `/v1/evaluation/*`:
//! - `GET/POST /v1/evaluation/replay-candidates` — list / upsert a replay
//!   candidate (201); fixture-kind candidates and missing candidateIds are
//!   rejected with 400
//! - `GET /v1/evaluation/replay-candidates/{candidateId}` — one candidate (404
//!   when absent)
//! - `POST /v1/evaluation/replay-candidates/{candidateId}/attempts` — create a
//!   replay attempt (202); live-validation mode is rejected (400); billing
//!   reservation denials surface as the Go `writeBillingDenial` (429/503 with
//!   the stable denial payload)
//! - `POST /v1/evaluation/replay-candidates/{candidateId}/live-validations` —
//!   hand off a candidate to the live-validation manager (202; 409 blocked;
//!   503 disabled)
//! - `GET /v1/evaluation/replay-attempts`, `GET .../{attemptId}`,
//!   `POST .../{attemptId}/compare` (201)
//! - `GET /v1/evaluation/comparisons`, `GET .../{comparisonId}`
//! - `GET /v1/evaluation/fixtures`
//!
//! Routes under `/v1/live-validations/*`:
//! - `GET/POST /v1/live-validations` — list attempts / start one (202; 409
//!   when a gate blocks with the `StartResult` body; 503 when disabled)
//! - `GET .../{validationId}`, `GET .../{validationId}/ledger`,
//!   `POST .../{validationId}/abort`, `GET .../{validationId}/retention`,
//!   `POST .../{validationId}/compare` (202),
//!   `POST .../{validationId}/reconciliations/{ambiguousCommitId}/resolve`
//! - `GET /v1/live-validations/support-matrix`,
//!   `GET/POST /v1/live-validations/kill-switches`
//! - connector smoke/conformance evidence:
//!   `GET .../{discord|telegram|slack|matrix}-smoke`,
//!   `POST .../matrix-smoke` (non-safe-live records only),
//!   `GET .../{discord|telegram|slack|matrix}-conformance`
//!
//! NOT PORTED (manager/store method missing — reported, not duplicated):
//! - the tenant-scoped evaluation product family (discovery policies/runs,
//!   discovered candidates, product fixtures + revisions + review/suppress,
//!   suppressions, replay campaigns, dashboard projections, tool-call
//!   inspections, retention/apply): dope-store's SQLiteStore has the tables
//!   (migration r41) but none of the product DAOs
//!   (ListDiscoveryPolicies/UpsertDiscoveryPolicy/SaveDiscoveryRun/...), and no
//!   crate implements `dope_evaluation::product_store::ProductStore`.
//! - `POST /v1/live-validations/{telegram|slack}-smoke`: the Go api layer
//!   delegates the evidence build to the connectors packages; dope-api does not
//!   depend on dope-telegram/dope-slack, so the recorders are not ported.
//! - matrix safe-live smoke: Go runs a provider probe through a
//!   `matrixSmokeExecutor`; no Rust equivalent exists (non-safe-live records
//!   are built inline exactly like Go's `matrixSmokeRecordFromRequest`).
//! - `recordLiveValidationAudit`: dope-audit has no live-validation event
//!   builder yet (best-effort in Go; errors are ignored there).
//! - the store-persistence half of Go `publishEvent`: the Rust bus is
//!   in-memory; events are published to `state.event_bus` only.
//!
//! Middleware note: the Go registrations wrap these routes with
//! `protected()` only (no by-id tenant guard); the outer app-wiring wave
//! applies the middleware. Handlers read the `TenantContext` extension when
//! present and behave like the Go nil-auth path otherwise.
//!
//! Tenant scoping: the replay-ledger routes are environment-scoped (the
//! manager fills the scope); the live-validation manager reads the resolved
//! tenant from the `dope_identity::tenantctx` task-local; the connector
//! smoke/conformance routes require a resolved tenant context plus
//! credential-inspection authority (403 credential denial otherwise).

use std::collections::HashMap;
use std::sync::Arc;

use axum::body::Bytes;
use axum::extract::{Extension, Path, Query, State};
use axum::http::StatusCode;
use axum::response::{IntoResponse, Response};
use axum::routing::{get, post};
use axum::Router;
use axum::Json as AxumJson;
use chrono::{DateTime, Utc};
use dope_evaluation::{
    CandidateFilter, ComparisonFilter, ComparisonResult, CreateComparisonInput,
    CreateReplayAttemptInput, EvaluationError, FixtureFilter, ReplayAttempt, ReplayAttemptStatus,
    ReplayCandidate, ReplayMode, RegressionFixture,
};
use dope_livevalidation::{
    ApprovalMode, ApprovalStatus, ApprovalTarget, Attempt, AttemptFilter, AttemptStatus, Comparison,
    FreshApproval, KillSwitch, KillSwitchFilter, KillSwitchScope, LiveValidationError, MatrixRow,
    ReconciliationResolution, ReconciliationResolutionValue, RetentionPolicy, SafetyClass,
    SideEffectLedgerEntry, SideEffectScope, StartFailure, StartInput, StartResult, ToolClass,
};
use serde::{Deserialize, Serialize};
use serde_json::json;

use crate::error::ApiError;
use crate::middleware::{TenantContext, environment_scope_from_config};
use crate::response::Json;
use crate::state::AppState;

// ---------------------------------------------------------------------------
// Handler error
// ---------------------------------------------------------------------------

/// Handler error carrying either the canonical ApiError mapping or the two Go
/// bodies that escape that mapping: the stable credential denial
/// (writeCredentialDenial, 403 {error, reasonCode}) and the billing denial
/// (writeBillingDenial, 429/503 with the DenialPayload).
#[derive(Debug)]
enum EvaluationApiError {
    Api(ApiError),
    ServiceUnavailable(String),
    CredentialDenial { reason_code: String },
    BillingDenial { status: StatusCode, body: serde_json::Value },
}

impl From<ApiError> for EvaluationApiError {
    fn from(err: ApiError) -> Self {
        Self::Api(err)
    }
}

impl IntoResponse for EvaluationApiError {
    fn into_response(self) -> Response {
        match self {
            Self::Api(err) => err.into_response(),
            Self::ServiceUnavailable(message) => (
                StatusCode::SERVICE_UNAVAILABLE,
                AxumJson(serde_json::json!({
                    "code": "internal",
                    "message": message,
                    "error": message,
                })),
            )
                .into_response(),
            Self::CredentialDenial { reason_code } => (
                StatusCode::FORBIDDEN,
                AxumJson(serde_json::json!({
                    "error": "credential_access_denied",
                    "reasonCode": reason_code,
                })),
            )
                .into_response(),
            Self::BillingDenial { status, body } => (status, AxumJson(body)).into_response(),
        }
    }
}

fn bad_request(message: impl Into<String>) -> EvaluationApiError {
    EvaluationApiError::Api(ApiError::BadRequest(message.into()))
}

// ---------------------------------------------------------------------------
// Response DTOs (port of the Go api-package response types)
// ---------------------------------------------------------------------------

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
struct ReplayCandidateListResponse {
    environment_scope: String,
    items: Vec<ReplayCandidate>,
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
struct ReplayAttemptListResponse {
    environment_scope: String,
    items: Vec<ReplayAttempt>,
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
struct ReplayComparisonListResponse {
    environment_scope: String,
    items: Vec<ComparisonResult>,
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
struct ReplayFixtureListResponse {
    environment_scope: String,
    items: Vec<RegressionFixture>,
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
struct LiveValidationAttemptListResponse {
    #[serde(default, skip_serializing_if = "String::is_empty")]
    tenant_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    environment_scope: String,
    items: Vec<Attempt>,
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
struct LiveValidationSupportMatrixResponse {
    #[serde(default, skip_serializing_if = "String::is_empty")]
    environment_scope: String,
    version: String,
    items: Vec<MatrixRow>,
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
struct LiveValidationDiscordConformanceResponse {
    tenant_id: String,
    connector_id: String,
    items: Vec<dope_connectors::ConformanceResult>,
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
struct LiveValidationLedgerResponse {
    validation_id: String,
    tenant_id: String,
    items: Vec<SideEffectLedgerEntry>,
}

/// Go slackSmokeEvidenceResource (setupwizard.go): the tenant-safe projection
/// of a Slack smoke evidence record.
#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
struct SlackSmokeEvidenceResource {
    smoke_evidence_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    tenant_id: String,
    connector_id: String,
    workspace_binding_id: String,
    status: String,
    authorization_mode: String,
    owner: String,
    reason: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    remaining_risk: String,
    validated_at: DateTime<Utc>,
    retention_expires_at: DateTime<Utc>,
    redaction_status: String,
    #[serde(default, skip_serializing_if = "HashMap::is_empty")]
    safe_evidence: HashMap<String, String>,
}
// ---------------------------------------------------------------------------
// Request DTOs (ports of the Go api-package request types)
// ---------------------------------------------------------------------------

/// Go CreateLiveValidationRequest = livevalidation.StartInput.
#[derive(Debug, Clone, Default, Deserialize)]
#[serde(rename_all = "camelCase")]
struct CreateLiveValidationRequest {
    #[serde(default, skip_serializing_if = "String::is_empty")]
    validation_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    candidate_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    source_attempt_id: String,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    candidate_tool_classes: Vec<String>,
    #[serde(default)]
    requested_scope: Option<CreateLiveValidationScope>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    fresh_approvals: Vec<CreateLiveValidationApproval>,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    client_key: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    change_window_label: String,
}

#[derive(Debug, Clone, Default, Deserialize)]
#[serde(rename_all = "camelCase")]
struct CreateLiveValidationScope {
    #[serde(default, skip_serializing_if = "String::is_empty")]
    scope_id: String,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    included_tool_classes: Vec<String>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    excluded_tool_classes: Vec<String>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    included_actions: Vec<String>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    excluded_actions: Vec<String>,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    approval_mode: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    declared_by: String,
    #[serde(default)]
    declared_at: Option<DateTime<Utc>>,
}

#[derive(Debug, Clone, Default, Deserialize)]
#[serde(rename_all = "camelCase")]
struct CreateLiveValidationApproval {
    #[serde(default, skip_serializing_if = "String::is_empty")]
    approval_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    validation_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    tenant_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    approval_target: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    tool_class: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    safety_class: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    action_ref: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    approved_scope: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    status: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    requested_by: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    resolved_by: String,
    #[serde(default)]
    requested_at: Option<DateTime<Utc>>,
    #[serde(default)]
    resolved_at: Option<DateTime<Utc>>,
}

impl CreateLiveValidationRequest {
    fn into_start_input(self) -> StartInput {
        let scope = self
            .requested_scope
            .map(|scope| SideEffectScope {
                scope_id: scope.scope_id,
                validation_id: self.validation_id.clone(),
                included_tool_classes: scope
                    .included_tool_classes
                    .into_iter()
                    .map(ToolClass::new)
                    .collect(),
                excluded_tool_classes: scope
                    .excluded_tool_classes
                    .into_iter()
                    .map(ToolClass::new)
                    .collect(),
                included_actions: scope.included_actions,
                excluded_actions: scope.excluded_actions,
                approval_mode: ApprovalMode::new(scope.approval_mode),
                declared_by: scope.declared_by,
                declared_at: scope.declared_at.unwrap_or_default(),
            })
            .unwrap_or_default();
        StartInput {
            validation_id: self.validation_id,
            candidate_id: self.candidate_id,
            source_attempt_id: self.source_attempt_id,
            candidate_tool_classes: self
                .candidate_tool_classes
                .into_iter()
                .map(ToolClass::new)
                .collect(),
            requested_scope: scope,
            fresh_approvals: self
                .fresh_approvals
                .into_iter()
                .map(|approval| FreshApproval {
                    approval_id: approval.approval_id,
                    validation_id: approval.validation_id,
                    tenant_id: approval.tenant_id,
                    approval_target: ApprovalTarget::new(approval.approval_target),
                    tool_class: ToolClass::new(approval.tool_class),
                    safety_class: SafetyClass::new(approval.safety_class),
                    action_ref: approval.action_ref,
                    approved_scope: approval.approved_scope,
                    status: ApprovalStatus::new(approval.status),
                    requested_by: approval.requested_by,
                    resolved_by: approval.resolved_by,
                    requested_at: approval.requested_at.unwrap_or_default(),
                    resolved_at: approval.resolved_at,
                })
                .collect(),
            client_key: self.client_key,
            change_window_label: self.change_window_label,
        }
    }
}

/// Go ResolveLiveValidationReconciliationRequest.
#[derive(Debug, Clone, Default, Deserialize)]
#[serde(rename_all = "camelCase")]
struct ResolveLiveValidationReconciliationRequest {
    resolution: String,
    reason: String,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    evidence_refs: Vec<String>,
}

/// Go UpdateLiveValidationKillSwitchRequest.
#[derive(Debug, Clone, Default, Deserialize)]
#[serde(rename_all = "camelCase")]
struct UpdateLiveValidationKillSwitchRequest {
    #[serde(default, skip_serializing_if = "String::is_empty")]
    scope: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    tenant_id: String,
    enabled: bool,
    reason: String,
    #[serde(default)]
    expires_at: Option<DateTime<Utc>>,
}

/// Go RecordMatrixSmokeRequest.
#[derive(Debug, Clone, Default, Deserialize)]
#[serde(rename_all = "camelCase")]
struct RecordMatrixSmokeRequest {
    #[serde(default, skip_serializing_if = "String::is_empty")]
    connector_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    homeserver_binding_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    status: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    authorization_mode: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    owner: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    reason: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    remaining_risk: String,
    #[serde(default)]
    validated_at: Option<DateTime<Utc>>,
    #[serde(default)]
    safe_evidence: HashMap<String, String>,
}

// ---------------------------------------------------------------------------
// Manager accessors and shared helpers
// ---------------------------------------------------------------------------

/// Go manager == nil check for the evaluation manager (500).
fn evaluation_manager(state: &AppState) -> Result<Arc<dope_evaluation::Manager>, ApiError> {
    state.evaluation.clone().ok_or_else(|| {
        ApiError::Internal("evaluation manager is not configured".to_string())
    })
}

/// Go manager == nil check for the live-validation manager (500).
fn live_validation_manager(state: &AppState) -> Result<Arc<dope_livevalidation::Manager>, ApiError> {
    state.live_validation.clone().ok_or_else(|| {
        ApiError::Internal("live validation manager is not configured".to_string())
    })
}

/// Go queryInt: an absent or unparseable value is 0.
fn query_int(params: &HashMap<String, String>, name: &str) -> i64 {
    match params.get(name) {
        Some(raw) if !raw.trim().is_empty() => raw.trim().parse().unwrap_or(0),
        _ => 0,
    }
}

/// Tolerant closed-enum parse for query filters. Go casts query strings onto
/// open string enums (unknown values filter nothing); the Rust closed enums
/// cannot represent unknown values, so an unparseable value degrades to the
/// default (no filter).
fn parse_enum<T: serde::de::DeserializeOwned + Default>(raw: &str) -> T {
    let raw = raw.trim();
    if raw.is_empty() {
        return T::default();
    }
    serde_json::from_value(serde_json::Value::String(raw.to_string())).unwrap_or_default()
}

/// Go firstNonEmptyString.
fn first_non_empty_string<'a>(primary: &'a str, fallback: &'a str) -> String {
    if primary.trim().is_empty() {
        fallback.to_string()
    } else {
        primary.trim().to_string()
    }
}

/// Go liveValidationToolClasses: trims, dedupes, and drops empties.
fn live_validation_tool_classes(items: &[String]) -> Vec<ToolClass> {
    let mut classes: Vec<ToolClass> = Vec::new();
    for item in items {
        let tool_class = ToolClass::new(item.trim());
        if tool_class.is_empty() || classes.contains(&tool_class) {
            continue;
        }
        classes.push(tool_class);
    }
    classes
}

/// Go requireHostedCredentialReadAny: a resolved tenant context plus
/// credential-inspection authority (or one of the manage permissions).
fn require_hosted_credential_read_any(
    tenant: Option<&TenantContext>,
    manage_permissions: &[dope_identity::Permission],
) -> Result<(), EvaluationApiError> {
    let Some(tc) = tenant else {
        return Err(EvaluationApiError::CredentialDenial {
            reason_code: "credential_denied:missing_tenant".to_string(),
        });
    };
    if tc.0.tenant_id.trim().is_empty() {
        return Err(EvaluationApiError::CredentialDenial {
            reason_code: "credential_denied:missing_tenant".to_string(),
        });
    }
    if !dope_identity::can_inspect_credentials(&tc.0, manage_permissions) {
        return Err(EvaluationApiError::CredentialDenial {
            reason_code: "credential_denied:missing_permission".to_string(),
        });
    }
    Ok(())
}

/// Go smoke-recorder permission gate: identity.HasPermission(...,
/// PermissionLiveValidationExecute).
fn require_live_validation_execute(
    tenant: Option<&TenantContext>,
) -> Result<(), EvaluationApiError> {
    let Some(tc) = tenant else {
        return Err(EvaluationApiError::CredentialDenial {
            reason_code: "credential_denied:missing_tenant".to_string(),
        });
    };
    if tc.0.tenant_id.trim().is_empty() {
        return Err(EvaluationApiError::CredentialDenial {
            reason_code: "credential_denied:missing_tenant".to_string(),
        });
    }
    if !dope_identity::has_permission(
        &tc.0.permissions,
        dope_identity::Permission::LiveValidationExecute,
    ) {
        return Err(EvaluationApiError::CredentialDenial {
            reason_code: "live_validation_execute_required".to_string(),
        });
    }
    Ok(())
}

/// Resolved tenant id for response/filter fields (Go tenantctx.FromContext).
/// Prefers the request's TenantContext extension (installed by protected())
/// and falls back to the dope_identity tenantctx task-local.
fn resolved_tenant_id(tenant: Option<&TenantContext>) -> String {
    if let Some(tc) = tenant {
        let id = tc.0.tenant_id.trim().to_string();
        if !id.is_empty() {
            return id;
        }
    }
    dope_identity::tenantctx::from_context()
        .map(|tc| tc.tenant_id)
        .unwrap_or_default()
}

/// Runs the future with the resolved tenant installed in the tenantctx
/// task-local (which the live-validation manager reads). Prefers an existing
/// task-local; falls back to the request's TenantContext extension.
async fn with_tenant_context<T, F>(tenant: Option<&TenantContext>, fut: F) -> T
where
    F: std::future::Future<Output = T>,
{
    if dope_identity::tenantctx::from_context().is_some() {
        fut.await
    } else if let Some(tc) = tenant {
        dope_identity::tenantctx::scope(tc.0.clone(), fut).await
    } else {
        fut.await
    }
}

/// Go writeBillingDenial: 503 unless the cause is a quota denial (429); the
/// stable DenialPayload body when present, otherwise writeError.
fn billing_reservation_error(
    reservation: dope_evaluation::BillingReservationError,
) -> EvaluationApiError {
    let status = if matches!(reservation.error, dope_billing::BillingError::QuotaDenied) {
        StatusCode::TOO_MANY_REQUESTS
    } else {
        StatusCode::SERVICE_UNAVAILABLE
    };
    let body = match &reservation.result.denial {
        Some(denial) => serde_json::to_value(denial).unwrap_or(serde_json::Value::Null),
        None => serde_json::json!({ "error": reservation.error.to_string() }),
    };
    EvaluationApiError::BillingDenial { status, body }
}

// ---------------------------------------------------------------------------
// Event publishing (Go publishEvaluationReplayEvent /
// publishEvaluationComparisonEvent)
// ---------------------------------------------------------------------------

fn publish_evaluation_replay_event(state: &AppState, name: &str, attempt: &ReplayAttempt) {
    let mut payload = serde_json::Map::new();
    payload.insert("candidateId".to_string(), json!(attempt.candidate_id));
    payload.insert("attemptId".to_string(), json!(attempt.attempt_id));
    payload.insert("mode".to_string(), json!(attempt.mode.as_str()));
    payload.insert("status".to_string(), json!(attempt.status.as_str()));
    payload.insert("environmentScope".to_string(), json!(attempt.environment_scope));
    payload.insert("resultRunId".to_string(), json!(attempt.result_run_id));
    payload.insert("resultWorkflowId".to_string(), json!(attempt.result_workflow_id));
    payload.insert("blockedReasons".to_string(), json!(attempt.blocked_reasons));
    let event = dope_events::Event {
        category: "evaluation".to_string(),
        name: name.to_string(),
        scope: dope_events::Scope {
            run_id: attempt.result_run_id.clone(),
            workflow_id: attempt.result_workflow_id.clone(),
            ..dope_events::Scope::default()
        },
        resource: dope_events::Resource {
            kind: "replay_attempt".to_string(),
            id: attempt.attempt_id.clone(),
        },
        payload,
        ..dope_events::Event::default()
    };
    state.event_bus.publish(event);
}

fn publish_evaluation_comparison_event(state: &AppState, comparison: &ComparisonResult) {
    let planes: Vec<String> = comparison
        .drift_findings
        .iter()
        .map(|finding| finding.plane.as_str().to_string())
        .collect();
    let mut payload = serde_json::Map::new();
    payload.insert("candidateId".to_string(), json!(comparison.candidate_id));
    payload.insert("attemptId".to_string(), json!(comparison.attempt_id));
    payload.insert("comparisonId".to_string(), json!(comparison.comparison_id));
    payload.insert("terminalStatus".to_string(), json!(comparison.terminal_status.as_str()));
    payload.insert("environmentScope".to_string(), json!(comparison.environment_scope));
    payload.insert("driftPlanes".to_string(), json!(planes));
    let event = dope_events::Event {
        category: "evaluation".to_string(),
        name: "evaluation.comparison_completed".to_string(),
        resource: dope_events::Resource {
            kind: "replay_comparison".to_string(),
            id: comparison.comparison_id.clone(),
        },
        payload,
        ..dope_events::Event::default()
    };
    state.event_bus.publish(event);
}

/// Go publishLiveValidationStartEvent: event name follows the attempt status
/// (blocked / awaiting-approval / started).
fn publish_live_validation_start_event(state: &AppState, result: &StartResult) {
    let name = match result.attempt.status.as_str() {
        AttemptStatus::BLOCKED => dope_events::LIVE_VALIDATION_BLOCKED_NAME,
        AttemptStatus::AWAITING_APPROVAL => dope_events::LIVE_VALIDATION_AWAITING_APPROVAL_NAME,
        _ => dope_events::LIVE_VALIDATION_STARTED_NAME,
    };
    let event = dope_events::live_validation_attempt_event(
        name,
        result.attempt.clone(),
        &result.denials,
    );
    state.event_bus.publish(event);
}

// ---------------------------------------------------------------------------
// Live-validation start (shared by the collection and the candidate handoff)
// ---------------------------------------------------------------------------

async fn run_live_validation_start(
    state: &AppState,
    manager: &dope_livevalidation::Manager,
    input: StartInput,
) -> Result<(StatusCode, StartResult), EvaluationApiError> {
    match manager.start(input).await {
        Ok(result) => {
            publish_live_validation_start_event(state, &result);
            // Go recordLiveValidationAudit — no dope_audit live-validation
            // builder yet (best-effort in Go; errors ignored).
            Ok((StatusCode::ACCEPTED, result))
        }
        Err(StartFailure::Disabled) => Err(EvaluationApiError::ServiceUnavailable(
            "live validation is disabled".to_string(),
        )),
        Err(StartFailure::Blocked(result)) => {
            publish_live_validation_start_event(state, &result);
            // Go recordLiveValidationAudit — deferred (see above).
            Ok((StatusCode::CONFLICT, result))
        }
        Err(StartFailure::Internal(err)) => Err(bad_request(err.to_string())),
    }
}

// ---------------------------------------------------------------------------
// Evaluation: replay candidates
// ---------------------------------------------------------------------------

/// GET /v1/evaluation/replay-candidates — list (Go handleEvaluationReplayCandidates GET).
#[allow(clippy::unused_async)]
async fn list_replay_candidates(
    State(state): State<AppState>,
    Query(params): Query<HashMap<String, String>>,
) -> Result<Json<ReplayCandidateListResponse>, ApiError> {
    let manager = evaluation_manager(&state)?;
    let filter = CandidateFilter {
        candidate_kind: parse_enum(params.get("candidateKind").map(String::as_str).unwrap_or("")),
        source_kind: parse_enum(params.get("sourceKind").map(String::as_str).unwrap_or("")),
        readiness_status: parse_enum(
            params.get("readinessStatus").map(String::as_str).unwrap_or(""),
        ),
        limit: query_int(&params, "limit"),
        ..CandidateFilter::default()
    };
    let items = manager
        .list_replay_candidates(&filter)
        .map_err(ApiError::internal)?;
    Ok(Json(ReplayCandidateListResponse {
        environment_scope: environment_scope_from_config(&state.config),
        items,
    }))
}

/// POST /v1/evaluation/replay-candidates — upsert a curated replay candidate (201).
#[allow(clippy::unused_async)]
async fn create_replay_candidate(
    State(state): State<AppState>,
    body: Bytes,
) -> Result<(StatusCode, AxumJson<ReplayCandidate>), ApiError> {
    let manager = evaluation_manager(&state)?;
    let input: ReplayCandidate = if body.is_empty() {
        ReplayCandidate::default()
    } else {
        // Go decodes the body straight into the resource and the manager fills
        // zero timestamps; the Rust serde shape requires them, so inject the
        // manager's effective "now" when the client omitted them.
        let mut value: serde_json::Value = serde_json::from_slice(&body)
            .map_err(|err| ApiError::BadRequest(err.to_string()))?;
        if value.get("createdAt").is_none() {
            value["createdAt"] = serde_json::json!(Utc::now());
        }
        if value.get("updatedAt").is_none() {
            value["updatedAt"] = serde_json::json!(Utc::now());
        }
        serde_json::from_value(value).map_err(|err| ApiError::BadRequest(err.to_string()))?
    };
    // Go: fixture replay candidates are managed by repo fixtures.
    if input.candidate_kind == dope_evaluation::CandidateKind::Fixture {
        return Err(ApiError::BadRequest(
            "fixture replay candidates are managed by repo fixtures".to_string(),
        ));
    }
    if input.candidate_id.trim().is_empty() {
        return Err(ApiError::BadRequest("candidateId is required".to_string()));
    }
    manager
        .upsert_replay_candidate(input.clone())
        .map_err(|err| ApiError::BadRequest(err.to_string()))?;
    let created = manager
        .get_replay_candidate(&input.candidate_id)
        .map_err(ApiError::internal)?
        .ok_or_else(|| ApiError::internal("created replay candidate not found"))?;
    Ok((StatusCode::CREATED, AxumJson(created)))
}

/// GET /v1/evaluation/replay-candidates/{candidateId} — one candidate.
#[allow(clippy::unused_async)]
async fn get_replay_candidate(
    State(state): State<AppState>,
    Path(candidate_id): Path<String>,
) -> Result<Json<ReplayCandidate>, ApiError> {
    let manager = evaluation_manager(&state)?;
    let item = manager
        .get_replay_candidate(&candidate_id)
        .map_err(ApiError::internal)?;
    match item {
        Some(item) => Ok(Json(item)),
        None => Err(ApiError::NotFound("replay candidate not found".to_string())),
    }
}

/// POST /v1/evaluation/replay-candidates/{candidateId}/attempts — create a
/// replay attempt (202; Go handleEvaluationReplayCandidateRoutes attempts).
async fn replay_candidate_attempts(
    State(state): State<AppState>,
    Path(candidate_id): Path<String>,
    body: Bytes,
) -> Result<(StatusCode, AxumJson<ReplayAttempt>), EvaluationApiError> {
    let manager = evaluation_manager(&state)?;
    let input: CreateReplayAttemptInput = if body.is_empty() {
        CreateReplayAttemptInput::default()
    } else {
        serde_json::from_slice(&body)
            .map_err(|err| EvaluationApiError::Api(ApiError::BadRequest(err.to_string())))?
    };
    if input.mode == Some(ReplayMode::LiveValidation) {
        return Err(bad_request(
            "live validation attempts must use /v1/live-validations",
        ));
    }
    let attempt = match manager.create_replay_attempt(&candidate_id, input).await {
        Ok(attempt) => attempt,
        Err(EvaluationError::BillingReservation(reservation)) => {
            return Err(billing_reservation_error(reservation));
        }
        Err(err) => return Err(bad_request(err.to_string())),
    };
    publish_evaluation_replay_event(&state, "evaluation.replay_started", &attempt);
    match attempt.status {
        ReplayAttemptStatus::Completed => {
            publish_evaluation_replay_event(&state, "evaluation.replay_completed", &attempt);
        }
        ReplayAttemptStatus::Blocked => {
            publish_evaluation_replay_event(&state, "evaluation.replay_blocked", &attempt);
        }
        ReplayAttemptStatus::Unreplayable => {
            publish_evaluation_replay_event(&state, "evaluation.replay_unreplayable", &attempt);
        }
        ReplayAttemptStatus::Failed => {
            publish_evaluation_replay_event(&state, "evaluation.replay_failed", &attempt);
        }
        _ => {}
    }
    Ok((StatusCode::ACCEPTED, AxumJson(attempt)))
}

/// POST /v1/evaluation/replay-candidates/{candidateId}/live-validations — hand
/// off a replay candidate to the live-validation manager (202 / 409 / 503).
async fn replay_candidate_live_validations(
    State(state): State<AppState>,
    Path(candidate_id): Path<String>,
    tenant: Option<Extension<TenantContext>>,
    body: Bytes,
) -> Result<(StatusCode, AxumJson<StartResult>), EvaluationApiError> {
    let evaluation = evaluation_manager(&state)?;
    let live_validation = live_validation_manager(&state)?;
    let candidate = evaluation
        .prepare_live_validation_handoff(&candidate_id)
        .map_err(|err| bad_request(err.to_string()))?;
    let mut input: CreateLiveValidationRequest = if body.is_empty() {
        CreateLiveValidationRequest::default()
    } else {
        serde_json::from_slice(&body)
            .map_err(|err| EvaluationApiError::Api(ApiError::BadRequest(err.to_string())))?
    };
    if !input.candidate_id.is_empty() && input.candidate_id != candidate_id {
        return Err(bad_request("candidateId must match the replay candidate route"));
    }
    input.candidate_id = candidate_id;
    if input.candidate_tool_classes.is_empty() {
        input.candidate_tool_classes = live_validation_tool_classes(&candidate.tool_classes)
            .into_iter()
            .map(|tool_class| tool_class.to_string())
            .collect();
    }
    let input = input.into_start_input();
    let (status, result) = with_tenant_context(
        tenant.as_ref().map(|t| &t.0),
        run_live_validation_start(&state, &live_validation, input),
    )
    .await?;
    Ok((status, AxumJson(result)))
}

// ---------------------------------------------------------------------------
// Evaluation: replay attempts / comparisons / fixtures
// ---------------------------------------------------------------------------

/// GET /v1/evaluation/replay-attempts — list.
#[allow(clippy::unused_async)]
async fn list_replay_attempts(
    State(state): State<AppState>,
    Query(params): Query<HashMap<String, String>>,
) -> Result<Json<ReplayAttemptListResponse>, ApiError> {
    let manager = evaluation_manager(&state)?;
    let filter = dope_evaluation::AttemptFilter {
        candidate_id: params.get("candidateId").cloned().unwrap_or_default(),
        status: parse_enum(params.get("status").map(String::as_str).unwrap_or("")),
        limit: query_int(&params, "limit"),
        ..dope_evaluation::AttemptFilter::default()
    };
    let items = manager
        .list_replay_attempts(&filter)
        .map_err(ApiError::internal)?;
    Ok(Json(ReplayAttemptListResponse {
        environment_scope: environment_scope_from_config(&state.config),
        items,
    }))
}

/// GET /v1/evaluation/replay-attempts/{attemptId} — one attempt.
#[allow(clippy::unused_async)]
async fn get_replay_attempt(
    State(state): State<AppState>,
    Path(attempt_id): Path<String>,
) -> Result<Json<ReplayAttempt>, ApiError> {
    let manager = evaluation_manager(&state)?;
    let item = manager
        .get_replay_attempt(&attempt_id)
        .map_err(ApiError::internal)?;
    match item {
        Some(item) => Ok(Json(item)),
        None => Err(ApiError::NotFound("replay attempt not found".to_string())),
    }
}

/// POST /v1/evaluation/replay-attempts/{attemptId}/compare — generate a
/// plane-level comparison (201).
#[allow(clippy::unused_async)]
async fn replay_attempt_compare(
    State(state): State<AppState>,
    Path(attempt_id): Path<String>,
    body: Bytes,
) -> Result<(StatusCode, AxumJson<ComparisonResult>), ApiError> {
    let manager = evaluation_manager(&state)?;
    let input: CreateComparisonInput = if body.is_empty() {
        CreateComparisonInput::default()
    } else {
        serde_json::from_slice(&body).map_err(|err| ApiError::BadRequest(err.to_string()))?
    };
    let comparison = manager
        .create_comparison(&attempt_id, input)
        .map_err(|err| ApiError::BadRequest(err.to_string()))?;
    publish_evaluation_comparison_event(&state, &comparison);
    Ok((StatusCode::CREATED, AxumJson(comparison)))
}

/// GET /v1/evaluation/comparisons — list.
#[allow(clippy::unused_async)]
async fn list_comparisons(
    State(state): State<AppState>,
    Query(params): Query<HashMap<String, String>>,
) -> Result<Json<ReplayComparisonListResponse>, ApiError> {
    let manager = evaluation_manager(&state)?;
    let filter = ComparisonFilter {
        candidate_id: params.get("candidateId").cloned().unwrap_or_default(),
        attempt_id: params.get("attemptId").cloned().unwrap_or_default(),
        terminal_status: parse_enum(
            params.get("terminalStatus").map(String::as_str).unwrap_or(""),
        ),
        limit: query_int(&params, "limit"),
        ..ComparisonFilter::default()
    };
    let items = manager
        .list_comparisons(&filter)
        .map_err(ApiError::internal)?;
    Ok(Json(ReplayComparisonListResponse {
        environment_scope: environment_scope_from_config(&state.config),
        items,
    }))
}

/// GET /v1/evaluation/comparisons/{comparisonId} — one comparison.
#[allow(clippy::unused_async)]
async fn get_comparison(
    State(state): State<AppState>,
    Path(comparison_id): Path<String>,
) -> Result<Json<ComparisonResult>, ApiError> {
    let manager = evaluation_manager(&state)?;
    let item = manager
        .get_comparison(&comparison_id)
        .map_err(ApiError::internal)?;
    match item {
        Some(item) => Ok(Json(item)),
        None => Err(ApiError::NotFound("comparison not found".to_string())),
    }
}

/// GET /v1/evaluation/fixtures — list regression fixtures.
#[allow(clippy::unused_async)]
async fn list_fixtures(
    State(state): State<AppState>,
    Query(params): Query<HashMap<String, String>>,
) -> Result<Json<ReplayFixtureListResponse>, ApiError> {
    let manager = evaluation_manager(&state)?;
    let filter = FixtureFilter {
        domain_class: parse_enum(params.get("domainClass").map(String::as_str).unwrap_or("")),
        limit: query_int(&params, "limit"),
        ..FixtureFilter::default()
    };
    let items = manager.list_fixtures(&filter).map_err(ApiError::internal)?;
    Ok(Json(ReplayFixtureListResponse {
        environment_scope: environment_scope_from_config(&state.config),
        items,
    }))
}

// ---------------------------------------------------------------------------
// Live validations: collection + item routes
// ---------------------------------------------------------------------------

/// GET /v1/live-validations — list attempts (Go handleLiveValidationCollection GET).
async fn list_live_validations(
    State(state): State<AppState>,
    Query(params): Query<HashMap<String, String>>,
    tenant: Option<Extension<TenantContext>>,
) -> Result<Json<LiveValidationAttemptListResponse>, ApiError> {
    let manager = live_validation_manager(&state)?;
    let tenant_id = resolved_tenant_id(tenant.as_ref().map(|t| &t.0));
    let filter = AttemptFilter {
        tenant_id: tenant_id.clone(),
        candidate_id: params.get("candidateId").cloned().unwrap_or_default(),
        status: AttemptStatus::new(params.get("status").cloned().unwrap_or_default()),
        limit: query_int(&params, "limit"),
        ..AttemptFilter::default()
    };
    let items = manager.list_attempts(filter).await.map_err(ApiError::internal)?;
    Ok(Json(LiveValidationAttemptListResponse {
        tenant_id,
        environment_scope: manager.environment_scope().to_string(),
        items,
    }))
}

/// POST /v1/live-validations — start an attempt (202 / 409 / 503).
async fn start_live_validation(
    State(state): State<AppState>,
    tenant: Option<Extension<TenantContext>>,
    body: Bytes,
) -> Result<(StatusCode, AxumJson<StartResult>), EvaluationApiError> {
    let manager = live_validation_manager(&state)?;
    let input: CreateLiveValidationRequest = if body.is_empty() {
        CreateLiveValidationRequest::default()
    } else {
        serde_json::from_slice(&body)
            .map_err(|err| EvaluationApiError::Api(ApiError::BadRequest(err.to_string())))?
    };
    let (status, result) = with_tenant_context(
        tenant.as_ref().map(|t| &t.0),
        run_live_validation_start(&state, &manager, input.into_start_input()),
    )
    .await?;
    Ok((status, AxumJson(result)))
}

/// GET /v1/live-validations/{validationId} — one attempt (404 when absent).
async fn get_live_validation(
    State(state): State<AppState>,
    Path(validation_id): Path<String>,
    tenant: Option<Extension<TenantContext>>,
) -> Result<Json<Attempt>, ApiError> {
    let manager = live_validation_manager(&state)?;
    let item = with_tenant_context(tenant.as_ref().map(|t| &t.0), manager.get_attempt(&validation_id))
        .await
        .map_err(ApiError::internal)?;
    match item {
        Some(item) => Ok(Json(item)),
        None => Err(ApiError::NotFound("live validation not found".to_string())),
    }
}

/// GET /v1/live-validations/{validationId}/ledger — side-effect ledger entries.
async fn live_validation_ledger(
    State(state): State<AppState>,
    Path(validation_id): Path<String>,
    Query(params): Query<HashMap<String, String>>,
    tenant: Option<Extension<TenantContext>>,
) -> Result<Json<LiveValidationLedgerResponse>, ApiError> {
    let manager = live_validation_manager(&state)?;
    let tenant_id = resolved_tenant_id(tenant.as_ref().map(|t| &t.0));
    let filter = dope_livevalidation::LedgerFilter {
        tenant_id: tenant_id.clone(),
        validation_id: validation_id.clone(),
        tool_class: ToolClass::new(params.get("toolClass").cloned().unwrap_or_default()),
        // The outcome query filter is not ported: LedgerOutcome is defined in
        // the private ledger module and not re-exported by dope-livevalidation.
        limit: query_int(&params, "limit"),
        ..dope_livevalidation::LedgerFilter::default()
    };
    let items = manager.list_ledger_entries(filter).await.map_err(ApiError::internal)?;
    Ok(Json(LiveValidationLedgerResponse {
        validation_id,
        tenant_id,
        items,
    }))
}

/// POST /v1/live-validations/{validationId}/abort — abort an attempt.
async fn live_validation_abort(
    State(state): State<AppState>,
    Path(validation_id): Path<String>,
    tenant: Option<Extension<TenantContext>>,
) -> Result<Json<Attempt>, ApiError> {
    let manager = live_validation_manager(&state)?;
    let item = with_tenant_context(tenant.as_ref().map(|t| &t.0), manager.abort(&validation_id))
        .await
        .map_err(|err| ApiError::BadRequest(err.to_string()))?;
    let event = dope_events::live_validation_attempt_event(
        dope_events::LIVE_VALIDATION_ABORTED_NAME,
        item.clone(),
        &[],
    );
    state.event_bus.publish(event);
    Ok(Json(item))
}

/// GET /v1/live-validations/{validationId}/retention — default retention policy.
#[allow(clippy::unused_async)]
async fn live_validation_retention(
    State(state): State<AppState>,
    Path(_validation_id): Path<String>,
) -> Result<Json<RetentionPolicy>, ApiError> {
    let manager = live_validation_manager(&state)?;
    Ok(Json(manager.default_retention_policy()))
}

/// POST /v1/live-validations/{validationId}/compare — outcome comparison (202).
async fn live_validation_compare(
    State(state): State<AppState>,
    Path(validation_id): Path<String>,
    tenant: Option<Extension<TenantContext>>,
) -> Result<(StatusCode, AxumJson<Comparison>), ApiError> {
    let manager = live_validation_manager(&state)?;
    let comparison = with_tenant_context(tenant.as_ref().map(|t| &t.0), manager.create_comparison(&validation_id))
        .await
        .map_err(|err| ApiError::BadRequest(err.to_string()))?;
    let event = dope_events::live_validation_comparison_event(comparison.clone());
    state.event_bus.publish(event);
    Ok((StatusCode::ACCEPTED, AxumJson(comparison)))
}

/// POST /v1/live-validations/{validationId}/reconciliations/{ambiguousCommitId}/resolve
/// — operator resolution of an ambiguous commit (403 without authority).
async fn live_validation_reconcile(
    State(state): State<AppState>,
    Path((_validation_id, ambiguous_commit_id)): Path<(String, String)>,
    tenant: Option<Extension<TenantContext>>,
    body: Bytes,
) -> Result<Json<ReconciliationResolution>, ApiError> {
    let manager = live_validation_manager(&state)?;
    let request: ResolveLiveValidationReconciliationRequest = if body.is_empty() {
        ResolveLiveValidationReconciliationRequest::default()
    } else {
        serde_json::from_slice(&body).map_err(|err| ApiError::BadRequest(err.to_string()))?
    };
    let resolution = with_tenant_context(
        tenant.as_ref().map(|t| &t.0),
        manager.resolve_reconciliation(ReconciliationResolution {
            ambiguous_commit_id,
            resolution: ReconciliationResolutionValue::new(request.resolution),
            reason: request.reason,
            evidence_refs: request.evidence_refs,
            ..ReconciliationResolution::default()
        }),
    )
    .await
    .map_err(|err| match err {
            LiveValidationError::ReconciliationPermissionDenied => {
                ApiError::Forbidden(err.to_string())
            }
            other => ApiError::BadRequest(other.to_string()),
        })?;
    let event = dope_events::live_validation_reconciliation_event(resolution.clone());
    state.event_bus.publish(event);
    Ok(Json(resolution))
}

// ---------------------------------------------------------------------------
// Live validations: kill switches + support matrix
// ---------------------------------------------------------------------------

/// GET /v1/live-validations/kill-switches — list (Go handleLiveValidationKillSwitches GET).
async fn list_kill_switches(
    State(state): State<AppState>,
    Query(params): Query<HashMap<String, String>>,
    tenant: Option<Extension<TenantContext>>,
) -> Result<Json<serde_json::Value>, ApiError> {
    let manager = live_validation_manager(&state)?;
    let tenant_id = params
        .get("tenantId")
        .cloned()
        .filter(|value| !value.trim().is_empty())
        .or_else(|| {
            let resolved = resolved_tenant_id(tenant.as_ref().map(|t| &t.0));
            (!resolved.is_empty()).then_some(resolved)
        })
        .unwrap_or_default();
    let filter = KillSwitchFilter {
        tenant_id: tenant_id.clone(),
        scope: KillSwitchScope::new(params.get("scope").cloned().unwrap_or_default()),
        limit: query_int(&params, "limit"),
        ..KillSwitchFilter::default()
    };
    let items = manager.list_kill_switches(filter).await.map_err(ApiError::internal)?;
    Ok(Json(json!({ "tenantId": tenant_id, "items": items })))
}

/// POST /v1/live-validations/kill-switches — set a kill switch (403 without
/// reconciliation authority).
async fn set_kill_switch(
    State(state): State<AppState>,
    tenant: Option<Extension<TenantContext>>,
    body: Bytes,
) -> Result<Json<KillSwitch>, ApiError> {
    let manager = live_validation_manager(&state)?;
    let request: UpdateLiveValidationKillSwitchRequest = if body.is_empty() {
        UpdateLiveValidationKillSwitchRequest::default()
    } else {
        serde_json::from_slice(&body).map_err(|err| ApiError::BadRequest(err.to_string()))?
    };
    let item = with_tenant_context(
        tenant.as_ref().map(|t| &t.0),
        manager.set_kill_switch(KillSwitch {
            scope: KillSwitchScope::new(request.scope),
            tenant_id: request.tenant_id,
            enabled: request.enabled,
            reason: request.reason,
            expires_at: request.expires_at,
            ..KillSwitch::default()
        }),
    )
    .await
    .map_err(|err| match err {
            LiveValidationError::KillSwitchPermissionDenied => {
                ApiError::Forbidden(err.to_string())
            }
            other => ApiError::BadRequest(other.to_string()),
        })?;
    let event = dope_events::live_validation_attempt_event(
        dope_events::LIVE_VALIDATION_KILL_SWITCH_CHANGED_NAME,
        Attempt {
            tenant_id: item.tenant_id.clone(),
            validation_id: item.kill_switch_id.clone(),
            status: AttemptStatus::from(AttemptStatus::ABORTED),
            updated_at: item.changed_at,
            ..Attempt::default()
        },
        &[],
    );
    state.event_bus.publish(event);
    Ok(Json(item))
}

/// GET /v1/live-validations/support-matrix — the v1 support matrix.
#[allow(clippy::unused_async)]
async fn live_validation_support_matrix(
    State(state): State<AppState>,
) -> Result<Json<LiveValidationSupportMatrixResponse>, ApiError> {
    let manager = live_validation_manager(&state)?;
    let matrix = manager.support_matrix().map_err(ApiError::internal)?;
    Ok(Json(LiveValidationSupportMatrixResponse {
        environment_scope: manager.environment_scope().to_string(),
        version: "v1".to_string(),
        items: matrix.rows(),
    }))
}

// ---------------------------------------------------------------------------
// Live validations: connector conformance + smoke evidence
// ---------------------------------------------------------------------------

/// GET /v1/live-validations/{discord|telegram|slack|matrix}-conformance — the
/// tenant's connector conformance results (404 when none).
async fn live_validation_connector_conformance(
    State(state): State<AppState>,
    tenant: Option<Extension<TenantContext>>,
    Query(params): Query<HashMap<String, String>>,
) -> Result<Json<LiveValidationDiscordConformanceResponse>, EvaluationApiError> {
    let _manager = live_validation_manager(&state)?;
    require_hosted_credential_read_any(
        tenant.as_ref().map(|t| &t.0),
        &[dope_identity::Permission::ConnectorsManage],
    )?;
    let tenant_id = tenant.as_ref().map(|t| t.0.0.tenant_id.clone()).unwrap_or_default();
    let connector_id = params
        .get("connectorId")
        .map(|value| value.trim().to_string())
        .unwrap_or_default();
    if connector_id.is_empty() {
        return Err(bad_request("connectorId is required"));
    }
    let items = state
        .store
        .lock()
        .list_connector_conformance_results(&tenant_id, &connector_id, Utc::now())
        .map_err(ApiError::from_store)?;
    if items.is_empty() {
        // Go: http.NotFound (plain-text 404); the JSON body is the crate's
        // standard not-found shape.
        return Err(EvaluationApiError::Api(ApiError::NotFound("not found".to_string())));
    }
    Ok(Json(LiveValidationDiscordConformanceResponse {
        tenant_id,
        connector_id,
        items,
    }))
}

/// GET /v1/live-validations/discord-smoke — latest Discord smoke evidence.
async fn live_validation_discord_smoke(
    State(state): State<AppState>,
    tenant: Option<Extension<TenantContext>>,
    Query(params): Query<HashMap<String, String>>,
) -> Result<Json<dope_store::DiscordSmokeEvidenceRecord>, EvaluationApiError> {
    let _manager = live_validation_manager(&state)?;
    require_hosted_credential_read_any(
        tenant.as_ref().map(|t| &t.0),
        &[dope_identity::Permission::ConnectorsManage],
    )?;
    let tenant_id = tenant.as_ref().map(|t| t.0.0.tenant_id.clone()).unwrap_or_default();
    let connector_id = params
        .get("connectorId")
        .map(|value| value.trim().to_string())
        .unwrap_or_default();
    if connector_id.is_empty() {
        return Err(bad_request("connectorId is required"));
    }
    let evidence = state
        .store
        .lock()
        .latest_discord_smoke_evidence(&tenant_id, &connector_id, Utc::now())
        .map_err(ApiError::from_store)?;
    match evidence {
        Some(evidence) => Ok(Json(evidence)),
        None => Err(EvaluationApiError::Api(ApiError::NotFound("not found".to_string()))),
    }
}

/// GET /v1/live-validations/telegram-smoke — latest Telegram smoke evidence.
async fn live_validation_telegram_smoke(
    State(state): State<AppState>,
    tenant: Option<Extension<TenantContext>>,
    Query(params): Query<HashMap<String, String>>,
) -> Result<Json<dope_store::TelegramSmokeEvidenceRecord>, EvaluationApiError> {
    let _manager = live_validation_manager(&state)?;
    require_hosted_credential_read_any(
        tenant.as_ref().map(|t| &t.0),
        &[dope_identity::Permission::ConnectorsManage],
    )?;
    let tenant_id = tenant.as_ref().map(|t| t.0.0.tenant_id.clone()).unwrap_or_default();
    let connector_id = params
        .get("connectorId")
        .map(|value| value.trim().to_string())
        .unwrap_or_default();
    if connector_id.is_empty() {
        return Err(bad_request("connectorId is required"));
    }
    let evidence = state
        .store
        .lock()
        .latest_telegram_smoke_evidence(&tenant_id, &connector_id, Utc::now())
        .map_err(ApiError::from_store)?;
    match evidence {
        Some(evidence) => Ok(Json(evidence)),
        None => Err(EvaluationApiError::Api(ApiError::NotFound("not found".to_string()))),
    }
}

/// GET /v1/live-validations/slack-smoke — latest Slack smoke evidence, projected
/// to the tenant-safe resource shape.
async fn live_validation_slack_smoke(
    State(state): State<AppState>,
    tenant: Option<Extension<TenantContext>>,
    Query(params): Query<HashMap<String, String>>,
) -> Result<Json<SlackSmokeEvidenceResource>, EvaluationApiError> {
    let _manager = live_validation_manager(&state)?;
    require_hosted_credential_read_any(
        tenant.as_ref().map(|t| &t.0),
        &[dope_identity::Permission::ConnectorsManage],
    )?;
    let tenant_id = tenant.as_ref().map(|t| t.0.0.tenant_id.clone()).unwrap_or_default();
    let connector_id = params
        .get("connectorId")
        .map(|value| value.trim().to_string())
        .unwrap_or_default();
    if connector_id.is_empty() {
        return Err(bad_request("connectorId is required"));
    }
    let evidence = state
        .store
        .lock()
        .latest_slack_smoke_evidence(&tenant_id, &connector_id, Utc::now())
        .map_err(ApiError::from_store)?;
    match evidence {
        Some(evidence) => Ok(Json(project_slack_smoke_evidence_resource(&evidence))),
        None => Err(EvaluationApiError::Api(ApiError::NotFound("not found".to_string()))),
    }
}

/// GET /v1/live-validations/matrix-smoke — latest Matrix smoke evidence.
async fn live_validation_matrix_smoke(
    State(state): State<AppState>,
    tenant: Option<Extension<TenantContext>>,
    Query(params): Query<HashMap<String, String>>,
) -> Result<Json<dope_store::MatrixSmokeEvidenceRecord>, EvaluationApiError> {
    let _manager = live_validation_manager(&state)?;
    require_hosted_credential_read_any(
        tenant.as_ref().map(|t| &t.0),
        &[dope_identity::Permission::ConnectorsManage],
    )?;
    let tenant_id = tenant.as_ref().map(|t| t.0.0.tenant_id.clone()).unwrap_or_default();
    let connector_id = params
        .get("connectorId")
        .map(|value| value.trim().to_string())
        .unwrap_or_default();
    if connector_id.is_empty() {
        return Err(bad_request("connectorId is required"));
    }
    let evidence = state
        .store
        .lock()
        .latest_matrix_smoke_evidence(&tenant_id, &connector_id, Utc::now())
        .map_err(ApiError::from_store)?;
    match evidence {
        Some(evidence) => Ok(Json(evidence)),
        None => Err(EvaluationApiError::Api(ApiError::NotFound("not found".to_string()))),
    }
}

/// POST /v1/live-validations/matrix-smoke — record structured smoke evidence.
/// Safe-live records require a provider-probe executor that has no Rust
/// equivalent yet (Go: 400 "matrix safe-live smoke executor is not configured").
async fn record_matrix_smoke(
    State(state): State<AppState>,
    tenant: Option<Extension<TenantContext>>,
    Query(params): Query<HashMap<String, String>>,
    body: Bytes,
) -> Result<(StatusCode, AxumJson<dope_store::MatrixSmokeEvidenceRecord>), EvaluationApiError> {
    let _manager = live_validation_manager(&state)?;
    require_live_validation_execute(tenant.as_ref().map(|t| &t.0))?;
    let tenant_id = tenant.as_ref().map(|t| t.0.0.tenant_id.clone()).unwrap_or_default();
    let mut request: RecordMatrixSmokeRequest = if body.is_empty() {
        RecordMatrixSmokeRequest::default()
    } else {
        serde_json::from_slice(&body)
            .map_err(|err| EvaluationApiError::Api(ApiError::BadRequest(err.to_string())))?
    };
    if request.connector_id.trim().is_empty() {
        request.connector_id = params.get("connectorId").cloned().unwrap_or_default();
    }
    if request.connector_id.trim().is_empty() {
        return Err(bad_request("connectorId is required"));
    }
    if request.authorization_mode.trim() == "safe_live" {
        return Err(bad_request("matrix safe-live smoke executor is not configured"));
    }
    let record = matrix_smoke_record_from_request(&tenant_id, &request)?;
    state
        .store
        .lock()
        .save_matrix_smoke_evidence(&record)
        .map_err(ApiError::from_store)?;
    Ok((StatusCode::CREATED, AxumJson(record)))
}

/// Go matrixSmokeRecordFromRequest (live_validation.go): validates the
/// status/authorization-mode combination and builds the store record.
fn matrix_smoke_record_from_request(
    tenant_id: &str,
    input: &RecordMatrixSmokeRequest,
) -> Result<dope_store::MatrixSmokeEvidenceRecord, EvaluationApiError> {
    let mode = input.authorization_mode.trim();
    let status = input.status.trim();
    let mode = if mode.is_empty() { "unavailable" } else { mode };
    let status = if status.is_empty() { "skipped" } else { status };
    match status {
        "skipped" => {
            if mode != "unavailable" {
                return Err(bad_request(
                    "skipped Matrix smoke must use unavailable authorization mode",
                ));
            }
        }
        "passed" | "failed" => {
            if mode != "fake_matrix" && mode != "safe_live" {
                return Err(bad_request(
                    "passed or failed Matrix smoke requires fake_matrix or safe_live authorization mode",
                ));
            }
        }
        _ => return Err(bad_request("status must be passed, failed, or skipped")),
    }
    let validated_at = input.validated_at.unwrap_or_else(Utc::now).with_timezone(&Utc);
    let connector_id = input.connector_id.trim().to_string();
    let binding_id = first_non_empty_string(
        &input.homeserver_binding_id,
        &format!("matrix_homeserver_{connector_id}"),
    );
    let owner = first_non_empty_string(&input.owner, "operator");
    let reason = first_non_empty_string(&input.reason, "safe_matrix_authorization_unavailable");
    let mut remaining_risk = input.remaining_risk.clone();
    if remaining_risk.trim().is_empty() && status == "skipped" {
        remaining_risk =
            "No live Matrix hosted smoke was run; release review must consume this structured skip."
                .to_string();
    }
    Ok(dope_store::MatrixSmokeEvidenceRecord {
        smoke_evidence_id: format!("matrix_smoke_{connector_id}"),
        tenant_id: tenant_id.to_string(),
        connector_id,
        homeserver_binding_id: binding_id,
        status: status.to_string(),
        authorization_mode: mode.to_string(),
        owner,
        reason,
        remaining_risk,
        validated_at,
        retention_expires_at: validated_at + chrono::Duration::days(90),
        redaction_status: "redacted".to_string(),
        safe_evidence: input.safe_evidence.clone(),
    })
}

/// Go projectSlackSmokeEvidenceResource (setupwizard.go).
fn project_slack_smoke_evidence_resource(
    record: &dope_store::SlackSmokeEvidenceRecord,
) -> SlackSmokeEvidenceResource {
    SlackSmokeEvidenceResource {
        smoke_evidence_id: record.smoke_evidence_id.clone(),
        tenant_id: record.tenant_id.clone(),
        connector_id: record.connector_id.clone(),
        workspace_binding_id: record.workspace_binding_id.clone(),
        status: record.status.clone(),
        authorization_mode: record.authorization_mode.clone(),
        owner: record.owner.clone(),
        reason: record.reason.clone(),
        remaining_risk: record.remaining_risk.clone(),
        validated_at: record.validated_at,
        retention_expires_at: record.retention_expires_at,
        redaction_status: record.redaction_status.clone(),
        safe_evidence: record.safe_evidence.clone(),
    }
}

// ---------------------------------------------------------------------------
// Router assembly
// ---------------------------------------------------------------------------

/// Route family router.
///
/// The tenant-scoped evaluation product routes (discovery-policies,
/// discovery-runs, discovered-candidates, product-fixtures, suppressions,
/// campaigns, dashboard, tool-call-inspections, retention/apply) are not
/// registered: their SQLiteStore DAOs do not exist in dope-store (see the
/// module doc).
#[must_use]
pub fn router() -> Router<AppState> {
    Router::new()
        // Evaluation replay ledger.
        .route(
            "/v1/evaluation/replay-candidates",
            get(list_replay_candidates).post(create_replay_candidate),
        )
        .route(
            "/v1/evaluation/replay-candidates/{candidate_id}",
            get(get_replay_candidate),
        )
        .route(
            "/v1/evaluation/replay-candidates/{candidate_id}/attempts",
            post(replay_candidate_attempts),
        )
        .route(
            "/v1/evaluation/replay-candidates/{candidate_id}/live-validations",
            post(replay_candidate_live_validations),
        )
        .route("/v1/evaluation/replay-attempts", get(list_replay_attempts))
        .route(
            "/v1/evaluation/replay-attempts/{attempt_id}",
            get(get_replay_attempt),
        )
        .route(
            "/v1/evaluation/replay-attempts/{attempt_id}/compare",
            post(replay_attempt_compare),
        )
        .route("/v1/evaluation/comparisons", get(list_comparisons))
        .route(
            "/v1/evaluation/comparisons/{comparison_id}",
            get(get_comparison),
        )
        .route("/v1/evaluation/fixtures", get(list_fixtures))
        // Live validation collection + items.
        .route(
            "/v1/live-validations",
            get(list_live_validations).post(start_live_validation),
        )
        .route(
            "/v1/live-validations/support-matrix",
            get(live_validation_support_matrix),
        )
        .route(
            "/v1/live-validations/kill-switches",
            get(list_kill_switches).post(set_kill_switch),
        )
        .route(
            "/v1/live-validations/discord-smoke",
            get(live_validation_discord_smoke),
        )
        .route(
            "/v1/live-validations/discord-conformance",
            get(live_validation_connector_conformance),
        )
        .route(
            "/v1/live-validations/telegram-smoke",
            get(live_validation_telegram_smoke),
        )
        .route(
            "/v1/live-validations/telegram-conformance",
            get(live_validation_connector_conformance),
        )
        .route(
            "/v1/live-validations/slack-smoke",
            get(live_validation_slack_smoke),
        )
        .route(
            "/v1/live-validations/slack-conformance",
            get(live_validation_connector_conformance),
        )
        .route(
            "/v1/live-validations/matrix-smoke",
            get(live_validation_matrix_smoke).post(record_matrix_smoke),
        )
        .route(
            "/v1/live-validations/matrix-conformance",
            get(live_validation_connector_conformance),
        )
        .route(
            "/v1/live-validations/{validation_id}",
            get(get_live_validation),
        )
        .route(
            "/v1/live-validations/{validation_id}/ledger",
            get(live_validation_ledger),
        )
        .route(
            "/v1/live-validations/{validation_id}/abort",
            post(live_validation_abort),
        )
        .route(
            "/v1/live-validations/{validation_id}/retention",
            get(live_validation_retention),
        )
        .route(
            "/v1/live-validations/{validation_id}/compare",
            post(live_validation_compare),
        )
        .route(
            "/v1/live-validations/{validation_id}/reconciliations/{ambiguous_commit_id}/resolve",
            post(live_validation_reconcile),
        )
}

#[cfg(test)]
mod tests {
    use super::*;

    use std::sync::Arc;

    use axum::body::to_bytes;
    use axum::http::Request as HttpRequest;
    use dope_events::Bus;
    use dope_identity::{
        LifecycleStatus, Role, TenantContext as IdentityTenantContext, permissions_for_role,
    };
    use dope_store::SQLiteStore;
    use parking_lot::Mutex;
    use tower::ServiceExt;
    use uuid::Uuid;

    /// Fixed timestamp used by the live-validation crate tests: 2026-04-29T10:00:00Z.
    fn fixed_now() -> DateTime<Utc> {
        DateTime::<Utc>::from_timestamp_secs(1_777_456_800).expect("fixed clock")
    }

    fn test_config() -> dope_config::Config {
        dope_config::Config {
            environment: dope_config::Environment::Test,
            bind_addr: "127.0.0.1:19192".to_string(),
            data_dir: "/tmp/dope-api-test".to_string(),
            log_level: "info".to_string(),
            version: "0.1.0".to_string(),
            llm: dope_config::LlmConfig::default(),
            connectors: dope_config::ConnectorConfig {
                discord: dope_config::DiscordConnectorConfig {
                    enabled: false,
                    ..Default::default()
                },
                telegram: dope_config::TelegramConnectorConfig {
                    enabled: false,
                    ..Default::default()
                },
                slack: dope_config::SlackConnectorConfig {
                    enabled: false,
                    ..Default::default()
                },
                matrix: dope_config::MatrixConnectorConfig {
                    enabled: false,
                    ..Default::default()
                },
            },
        }
    }

    fn fresh_store() -> Arc<Mutex<SQLiteStore>> {
        let dir = std::env::temp_dir().join(format!("dope-api-evaluation2-{}", Uuid::now_v7()));
        std::fs::create_dir_all(&dir).expect("mkdir");
        Arc::new(Mutex::new(
            SQLiteStore::new(dir.to_str().expect("path")).expect("store"),
        ))
    }

    struct Harness {
        state: AppState,
        store: Arc<Mutex<SQLiteStore>>,
        bus: Arc<Bus>,
    }

    fn harness(
        evaluation: Option<Arc<dope_evaluation::Manager>>,
        live_validation: Option<Arc<dope_livevalidation::Manager>>,
    ) -> Harness {
        let store = fresh_store();
        let bus = Arc::new(Bus::new());
        let mut state = AppState::new(test_config(), bus.clone(), Arc::clone(&store));
        state.evaluation = evaluation;
        state.live_validation = live_validation;
        Harness { state, store, bus }
    }

    /// Adapter mapping the evaluation manager's Store trait onto the
    /// SQLiteStore replay-ledger DAOs. The production adapter lives in the
    /// app-wiring layer; this one exists so handler tests can run.
    #[derive(Clone)]
    struct SqliteEvaluationStore {
        store: Arc<Mutex<SQLiteStore>>,
    }

    fn store_err(err: String) -> EvaluationError {
        EvaluationError::Store(err)
    }

    impl dope_evaluation::manager::Store for SqliteEvaluationStore {
        fn upsert_replay_candidate(&self, item: ReplayCandidate) -> Result<(), EvaluationError> {
            self.store.lock().upsert_replay_candidate(&item).map_err(store_err)
        }
        fn list_replay_candidates(
            &self,
            filter: &CandidateFilter,
        ) -> Result<Vec<ReplayCandidate>, EvaluationError> {
            self.store.lock().list_replay_candidates(filter).map_err(store_err)
        }
        fn get_replay_candidate(
            &self,
            environment_scope: &str,
            candidate_id: &str,
        ) -> Result<Option<ReplayCandidate>, EvaluationError> {
            self.store
                .lock()
                .get_replay_candidate(environment_scope, candidate_id)
                .map_err(store_err)
        }
        fn upsert_replay_attempt(&self, item: ReplayAttempt) -> Result<(), EvaluationError> {
            self.store.lock().upsert_replay_attempt(&item).map_err(store_err)
        }
        fn list_replay_attempts(
            &self,
            filter: &dope_evaluation::AttemptFilter,
        ) -> Result<Vec<ReplayAttempt>, EvaluationError> {
            self.store.lock().list_replay_attempts(filter).map_err(store_err)
        }
        fn get_replay_attempt(
            &self,
            environment_scope: &str,
            attempt_id: &str,
        ) -> Result<Option<ReplayAttempt>, EvaluationError> {
            self.store
                .lock()
                .get_replay_attempt(environment_scope, attempt_id)
                .map_err(store_err)
        }
        fn upsert_comparison_result(&self, item: ComparisonResult) -> Result<(), EvaluationError> {
            self.store.lock().upsert_comparison_result(&item).map_err(store_err)
        }
        fn list_comparison_results(
            &self,
            filter: &ComparisonFilter,
        ) -> Result<Vec<ComparisonResult>, EvaluationError> {
            self.store.lock().list_comparison_results(filter).map_err(store_err)
        }
        fn get_comparison_result(
            &self,
            environment_scope: &str,
            comparison_id: &str,
        ) -> Result<Option<ComparisonResult>, EvaluationError> {
            self.store
                .lock()
                .get_comparison_result(environment_scope, comparison_id)
                .map_err(store_err)
        }
        fn upsert_regression_fixture(&self, item: RegressionFixture) -> Result<(), EvaluationError> {
            self.store
                .lock()
                .upsert_regression_fixture(&item)
                .map_err(store_err)
        }
        fn list_regression_fixtures(
            &self,
            filter: &FixtureFilter,
        ) -> Result<Vec<RegressionFixture>, EvaluationError> {
            self.store.lock().list_regression_fixtures(filter).map_err(store_err)
        }
    }

    fn evaluation_manager(store: Arc<Mutex<SQLiteStore>>) -> Arc<dope_evaluation::Manager> {
        Arc::new(dope_evaluation::Manager::new(dope_evaluation::Dependencies {
            environment_scope: "test".to_string(),
            store: Some(Arc::new(SqliteEvaluationStore { store })),
            fixtures_dir: String::new(),
            runtime_recorder: None,
            billing: None,
            hosted_billing: false,
            clock: Some(Arc::new(fixed_now) as Arc<dyn Fn() -> DateTime<Utc> + Send + Sync>),
        }))
    }

    fn live_validation_manager(hosted_billing: bool) -> Arc<dope_livevalidation::Manager> {
        Arc::new(dope_livevalidation::Manager::new(
            dope_livevalidation::Dependencies {
                environment_scope: "test".to_string(),
                // NOTE: the store stays None in these tests — dope-livevalidation
                // does not re-export LedgerOutcome, so its async Store trait is
                // not implementable outside the crate (reported).
                store: None,
                enabled: true,
                billing: None,
                hosted_billing,
                clock: Some(Arc::new(fixed_now) as dope_livevalidation::Clock),
                ledger_event_sink: None,
                candidate_tool_class_resolver: None,
            },
        ))
    }

    fn request(method: &str, uri: &str, body: Option<&str>) -> HttpRequest<axum::body::Body> {
        HttpRequest::builder()
            .method(method)
            .uri(uri)
            .body(match body {
                Some(body) => axum::body::Body::from(body.to_string()),
                None => axum::body::Body::empty(),
            })
            .expect("request")
    }

    async fn send(
        app: &axum::Router,
        req: HttpRequest<axum::body::Body>,
    ) -> (StatusCode, serde_json::Value) {
        let response = app.clone().oneshot(req).await.expect("oneshot");
        let status = response.status();
        let bytes = to_bytes(response.into_body(), usize::MAX).await.expect("body");
        let json = serde_json::from_slice(&bytes).unwrap_or(serde_json::Value::Null);
        (status, json)
    }

    /// Runs the request with a resolved tenant context installed in the
    /// dope_identity tenantctx task-local (the live-validation manager reads it).
    async fn send_with_tenant(
        app: &axum::Router,
        req: HttpRequest<axum::body::Body>,
        tenant: IdentityTenantContext,
    ) -> (StatusCode, serde_json::Value) {
        let response = dope_identity::tenantctx::scope(tenant, app.clone().oneshot(req))
            .await
            .expect("oneshot");
        let status = response.status();
        let bytes = to_bytes(response.into_body(), usize::MAX).await.expect("body");
        let json = serde_json::from_slice(&bytes).unwrap_or(serde_json::Value::Null);
        (status, json)
    }

    /// Attaches the TenantContext extension the smoke/conformance handlers read.
    fn with_tenant_extension(
        mut req: HttpRequest<axum::body::Body>,
        tenant: IdentityTenantContext,
    ) -> HttpRequest<axum::body::Body> {
        req.extensions_mut().insert(crate::middleware::TenantContext(tenant));
        req
    }

    fn operator_context() -> IdentityTenantContext {
        IdentityTenantContext {
            tenant_id: "ten_1".to_string(),
            principal_id: "prn_operator".to_string(),
            role: Some(Role::Operator),
            permissions: permissions_for_role(Role::Operator, LifecycleStatus::Active),
            ..IdentityTenantContext::default()
        }
    }

    fn admin_context() -> IdentityTenantContext {
        IdentityTenantContext {
            tenant_id: "ten_1".to_string(),
            principal_id: "prn_admin".to_string(),
            role: Some(Role::Admin),
            permissions: permissions_for_role(Role::Admin, LifecycleStatus::Active),
            ..IdentityTenantContext::default()
        }
    }

    fn viewer_context() -> IdentityTenantContext {
        IdentityTenantContext {
            tenant_id: "ten_1".to_string(),
            principal_id: "prn_viewer".to_string(),
            role: Some(Role::Viewer),
            permissions: permissions_for_role(Role::Viewer, LifecycleStatus::Active),
            ..IdentityTenantContext::default()
        }
    }

    /// Tenant context carrying the smoke-recorder permissions the Go tests use.
    fn smoke_tenant_context(tenant_id: &str, principal_id: &str) -> IdentityTenantContext {
        IdentityTenantContext {
            tenant_id: tenant_id.to_string(),
            principal_id: principal_id.to_string(),
            permissions: vec![
                dope_identity::Permission::LiveValidationExecute,
                dope_identity::Permission::ConnectorsManage,
                dope_identity::Permission::CredentialsInspect,
            ],
            ..IdentityTenantContext::default()
        }
    }

    fn seed_candidate(store: &Arc<Mutex<SQLiteStore>>, candidate: &ReplayCandidate) {
        store.lock().upsert_replay_candidate(candidate).expect("seed candidate");
    }

    fn curated_candidate(candidate_id: &str, tool_classes: Vec<String>) -> ReplayCandidate {
        ReplayCandidate {
            candidate_id: candidate_id.to_string(),
            candidate_kind: dope_evaluation::CandidateKind::CuratedWork,
            display_name: "candidate".to_string(),
            source_kind: dope_evaluation::SourceKind::Run,
            source_id: "run_1".to_string(),
            source_refs: vec![dope_evaluation::SourceRef {
                kind: dope_evaluation::SourceKind::Run,
                id: "run_1".to_string(),
                route: String::new(),
            }],
            tool_classes,
            environment_scope: "test".to_string(),
            readiness_status: dope_evaluation::ReadinessStatus::FullyReplayable,
            default_replay_mode: ReplayMode::NonLive,
            created_at: fixed_now(),
            updated_at: fixed_now(),
            ..ReplayCandidate::default()
        }
    }

    fn live_validation_start_body(validation_id: &str, tool_class: &str) -> String {
        serde_json::json!({
            "validationId": validation_id,
            "candidateId": "candidate_1",
            "candidateToolClasses": [tool_class],
            "requestedScope": {
                "scopeId": format!("scope_{validation_id}"),
                "includedToolClasses": [tool_class],
                "approvalMode": "scope_level",
                "declaredBy": "prn_operator",
                "declaredAt": "2026-04-29T10:00:00Z",
            }
        })
        .to_string()
    }

    // Port of Go TestEvaluationRoutesLaunchReplayAndCompare (without the
    // runtime-recorder run/workflow assertions — no recorder is wired here).
    #[tokio::test]
    async fn replay_candidates_crud_attempt_compare_and_events() {
        let store = fresh_store();
        seed_candidate(&store, &curated_candidate("candidate_curated", vec![]));
        store
            .lock()
            .upsert_regression_fixture(&RegressionFixture {
                fixture_id: "fixture_schedule_1".to_string(),
                display_name: "Schedule fixture".to_string(),
                domain_class: dope_evaluation::FixtureDomainClass::Schedule,
                source_refs: vec![],
                captured_evidence_refs: vec![],
                assumptions: vec![],
                limitations: vec![],
                expected_replay_mode: ReplayMode::NonLive,
                expected_comparison_summary: dope_evaluation::PlaneSummaries::default(),
                candidate_id: "candidate_fixture".to_string(),
                environment_scope: "test".to_string(),
                created_at: fixed_now(),
                updated_at: fixed_now(),
                ..RegressionFixture::default()
            })
            .expect("seed fixture");
        let h = harness(Some(evaluation_manager(Arc::clone(&store))), None);
        let app = crate::routes::router(h.state.clone());

        // GET list -> the seeded candidate, with the environment scope.
        let (status, json) = send(&app, request("GET", "/v1/evaluation/replay-candidates", None)).await;
        assert_eq!(status, StatusCode::OK, "body: {json}");
        assert_eq!(json["environmentScope"], "test");
        assert_eq!(json["items"].as_array().map(|a| a.len()).unwrap_or(0), 1);
        assert_eq!(json["items"][0]["candidateId"], "candidate_curated");

        // POST curated candidate -> 201.
        let (status, json) = send(
            &app,
            request(
                "POST",
                "/v1/evaluation/replay-candidates",
                Some(
                    r#"{"candidateId":"candidate_api_1","candidateKind":"curated_work","displayName":"Curated Run","sourceKind":"run","sourceId":"run_a","sourceRefs":[{"kind":"run","id":"run_a","route":"/v1/runs/run_a"}],"environmentScope":"test","readinessStatus":"partially_replayable","readinessReasons":["curated run has captured summaries"],"limitations":["evidence-only replay"],"defaultReplayMode":"non_live","expectedComparisonSummary":{"runtime":"runtime captured","policy":"policy captured","evidence":"evidence captured"}}"#,
                ),
            ),
        )
        .await;
        assert_eq!(status, StatusCode::CREATED, "create body: {json}");
        assert_eq!(json["candidateId"], "candidate_api_1");
        assert_eq!(json["candidateKind"], "curated_work");
        assert_eq!(json["environmentScope"], "test");

        // POST missing source refs -> 400 (manager validation).
        let (status, json) = send(
            &app,
            request(
                "POST",
                "/v1/evaluation/replay-candidates",
                Some(r#"{"candidateId":"candidate_missing","candidateKind":"curated_work","displayName":"Missing Source"}"#),
            ),
        )
        .await;
        assert_eq!(status, StatusCode::BAD_REQUEST, "body: {json}");
        assert_eq!(json["code"], "bad_request");

        // POST fixture-kind candidate -> 400 (managed by repo fixtures).
        let (status, json) = send(
            &app,
            request(
                "POST",
                "/v1/evaluation/replay-candidates",
                Some(
                    r#"{"candidateId":"candidate_api_fixture","candidateKind":"fixture","displayName":"API Fixture","sourceKind":"fixture","sourceId":"fixture_api","sourceRefs":[{"kind":"fixture","id":"fixture_api"}],"environmentScope":"test","readinessStatus":"fully_replayable","readinessReasons":[],"limitations":[],"defaultReplayMode":"non_live"}"#,
                ),
            ),
        )
        .await;
        assert_eq!(status, StatusCode::BAD_REQUEST, "body: {json}");
        assert_eq!(json["message"], "fixture replay candidates are managed by repo fixtures");

        // GET fixtures -> the seeded fixture.
        let (status, json) = send(&app, request("GET", "/v1/evaluation/fixtures", None)).await;
        assert_eq!(status, StatusCode::OK, "body: {json}");
        assert_eq!(json["environmentScope"], "test");
        assert_eq!(json["items"].as_array().map(|a| a.len()).unwrap_or(0), 1);
        assert_eq!(json["items"][0]["fixtureId"], "fixture_schedule_1");

        // GET candidate detail + 404 for a missing one.
        let (status, json) = send(
            &app,
            request("GET", "/v1/evaluation/replay-candidates/candidate_curated", None),
        )
        .await;
        assert_eq!(status, StatusCode::OK, "body: {json}");
        assert_eq!(json["candidateId"], "candidate_curated");
        let (status, json) = send(
            &app,
            request("GET", "/v1/evaluation/replay-candidates/does-not-exist", None),
        )
        .await;
        assert_eq!(status, StatusCode::NOT_FOUND, "body: {json}");
        assert_eq!(json["message"], "replay candidate not found");

        // POST attempt (empty body) -> 202 completed non-live.
        let (status, json) = send(
            &app,
            request("POST", "/v1/evaluation/replay-candidates/candidate_curated/attempts", None),
        )
        .await;
        assert_eq!(status, StatusCode::ACCEPTED, "attempt body: {json}");
        assert_eq!(json["mode"], "non_live");
        assert_eq!(json["status"], "completed");
        let attempt_id = json["attemptId"].as_str().expect("attemptId").to_string();

        // POST attempt with live-validation mode -> 400 bypass rejection.
        let (status, json) = send(
            &app,
            request(
                "POST",
                "/v1/evaluation/replay-candidates/candidate_curated/attempts",
                Some(r#"{"mode":"live_validation"}"#),
            ),
        )
        .await;
        assert_eq!(status, StatusCode::BAD_REQUEST, "body: {json}");
        assert_eq!(json["message"], "live validation attempts must use /v1/live-validations");

        // GET attempts list + 404 for a missing attempt.
        let (status, json) = send(&app, request("GET", "/v1/evaluation/replay-attempts", None)).await;
        assert_eq!(status, StatusCode::OK, "body: {json}");
        assert_eq!(json["items"].as_array().map(|a| a.len()).unwrap_or(0), 1);
        let (status, json) = send(
            &app,
            request("GET", "/v1/evaluation/replay-attempts/does-not-exist", None),
        )
        .await;
        assert_eq!(status, StatusCode::NOT_FOUND, "body: {json}");
        assert_eq!(json["message"], "replay attempt not found");

        // POST compare -> 201 matched.
        let (status, json) = send(
            &app,
            request("POST", &format!("/v1/evaluation/replay-attempts/{attempt_id}/compare"), None),
        )
        .await;
        assert_eq!(status, StatusCode::CREATED, "compare body: {json}");
        assert_eq!(json["terminalStatus"], "matched");
        let comparison_id = json["comparisonId"].as_str().expect("comparisonId").to_string();

        // GET comparisons list + detail.
        let (status, json) = send(&app, request("GET", "/v1/evaluation/comparisons", None)).await;
        assert_eq!(status, StatusCode::OK, "body: {json}");
        assert_eq!(json["environmentScope"], "test");
        assert_eq!(json["items"].as_array().map(|a| a.len()).unwrap_or(0), 1);
        let (status, json) = send(
            &app,
            request("GET", &format!("/v1/evaluation/comparisons/{comparison_id}"), None),
        )
        .await;
        assert_eq!(status, StatusCode::OK, "body: {json}");
        assert_eq!(json["comparisonId"], comparison_id);
        let (status, json) = send(
            &app,
            request("GET", "/v1/evaluation/comparisons/does-not-exist", None),
        )
        .await;
        assert_eq!(status, StatusCode::NOT_FOUND, "body: {json}");
        assert_eq!(json["message"], "comparison not found");

        // The replay_started / replay_completed / comparison_completed events fired.
        let events = h
            .bus
            .list(&dope_events::Filter {
                category: "evaluation".to_string(),
                ..Default::default()
            });
        let names: Vec<String> = events.iter().map(|event| event.name.clone()).collect();
        for expected in [
            "evaluation.replay_started",
            "evaluation.replay_completed",
            "evaluation.comparison_completed",
        ] {
            assert!(
                names.iter().any(|name| name == expected),
                "expected {expected} in {names:?}"
            );
        }
    }

    #[tokio::test]
    async fn unconfigured_managers_return_500() {
        let h = harness(None, None);
        let app = crate::routes::router(h.state.clone());
        let (status, json) = send(&app, request("GET", "/v1/evaluation/replay-candidates", None)).await;
        assert_eq!(status, StatusCode::INTERNAL_SERVER_ERROR, "body: {json}");
        assert_eq!(json["message"], "evaluation manager is not configured");
        let (status, json) = send(&app, request("GET", "/v1/live-validations", None)).await;
        assert_eq!(status, StatusCode::INTERNAL_SERVER_ERROR, "body: {json}");
        assert_eq!(json["message"], "live validation manager is not configured");
    }

    // Port of Go TestLiveValidationRouteStartDenialsAndAwaitingApproval.
    #[tokio::test]
    async fn live_validation_start_denials_and_awaiting_approval() {
        let cases: Vec<(
            &str,
            IdentityTenantContext,
            Arc<dope_livevalidation::Manager>,
            String,
            StatusCode,
            Option<&str>,
            Option<&str>,
        )> = vec![
            (
                "permission",
                viewer_context(),
                live_validation_manager(false),
                live_validation_start_body("lv_permission", "daemon.inspection.read"),
                StatusCode::CONFLICT,
                Some("permission"),
                None,
            ),
            (
                "quota unavailable",
                operator_context(),
                live_validation_manager(true),
                live_validation_start_body("lv_quota", "daemon.inspection.read"),
                StatusCode::CONFLICT,
                Some("quota"),
                None,
            ),
            (
                "support matrix",
                operator_context(),
                live_validation_manager(false),
                live_validation_start_body("lv_support", "mcp.tool_call"),
                StatusCode::CONFLICT,
                Some("support_matrix"),
                None,
            ),
            (
                "awaiting approval",
                operator_context(),
                live_validation_manager(false),
                live_validation_start_body("lv_approval", "daemon.inspection.read"),
                StatusCode::ACCEPTED,
                None,
                Some("awaiting_approval"),
            ),
        ];
        for (name, tenant, manager, body, want_status, want_gate, want_state) in cases {
            let h = harness(None, Some(manager));
            let (status, json) = send_with_tenant(
                &crate::routes::router(h.state.clone()),
                request("POST", "/v1/live-validations", Some(&body)),
                tenant,
            )
            .await;
            assert_eq!(status, want_status, "{name} body: {json}");
            if let Some(gate) = want_gate {
                assert_eq!(json["denials"][0]["gate"], gate, "{name}");
            }
            if let Some(state) = want_state {
                assert_eq!(json["attempt"]["status"], state, "{name}");
            }
        }
    }

    #[tokio::test]
    async fn live_validation_support_matrix_route() {
        let h = harness(None, Some(live_validation_manager(false)));
        let (status, json) = send(
            &crate::routes::router(h.state.clone()),
            request("GET", "/v1/live-validations/support-matrix", None),
        )
        .await;
        assert_eq!(status, StatusCode::OK, "body: {json}");
        assert_eq!(json["version"], "v1");
        assert_eq!(json["environmentScope"], "test");
        let items = json["items"].as_array().expect("items");
        assert!(!items.is_empty());
        assert!(
            items.iter().any(|row| {
                row["toolClass"] == "mcp.tool_call" && row["safetyClass"] == "unsupported"
            }),
            "expected unsupported MCP row in {items:?}"
        );
    }

    // Port of Go TestLiveValidationKillSwitchSetListAndBlocksStart (the set +
    // list legs; the block-start leg needs a store-backed manager).
    #[tokio::test]
    async fn live_validation_kill_switches_set_and_list() {
        let h = harness(None, Some(live_validation_manager(false)));
        let app = crate::routes::router(h.state.clone());
        let (status, json) = send_with_tenant(
            &app,
            request(
                "POST",
                "/v1/live-validations/kill-switches",
                Some(r#"{"scope":"tenant","enabled":true,"reason":"containment"}"#),
            ),
            admin_context(),
        )
        .await;
        assert_eq!(status, StatusCode::OK, "body: {json}");
        assert_eq!(json["enabled"], true);
        assert_eq!(json["tenantId"], "ten_1");
        assert_eq!(json["scope"], "tenant");
        let (status, json) = send_with_tenant(
            &app,
            request("GET", "/v1/live-validations/kill-switches", None),
            admin_context(),
        )
        .await;
        assert_eq!(status, StatusCode::OK, "body: {json}");
        assert_eq!(json["tenantId"], "ten_1");
    }

    // Port of the reconcile legs of Go TestLiveValidationLedgerReconciliationRetentionAndComparisonRoutes.
    #[tokio::test]
    async fn live_validation_reconcile_requires_authority() {
        let h = harness(None, Some(live_validation_manager(false)));
        let app = crate::routes::router(h.state.clone());
        let body = r#"{"resolution":"confirmed_committed","reason":"provider checked"}"#;
        let (status, json) = send_with_tenant(
            &app,
            request(
                "POST",
                "/v1/live-validations/lv_1/reconciliations/amb_1/resolve",
                Some(body),
            ),
            viewer_context(),
        )
        .await;
        assert_eq!(status, StatusCode::FORBIDDEN, "viewer body: {json}");
        assert_eq!(json["code"], "forbidden");
        let (status, json) = send_with_tenant(
            &app,
            request(
                "POST",
                "/v1/live-validations/lv_1/reconciliations/amb_1/resolve",
                Some(body),
            ),
            admin_context(),
        )
        .await;
        assert_eq!(status, StatusCode::OK, "admin body: {json}");
        assert_eq!(json["ambiguousCommitId"], "amb_1");
        assert_eq!(json["resolution"], "confirmed_committed");
        assert_eq!(json["resolvedBy"], "prn_admin");
    }

    #[tokio::test]
    async fn live_validation_list_carries_resolved_tenant() {
        let h = harness(None, Some(live_validation_manager(false)));
        let (status, json) = send_with_tenant(
            &crate::routes::router(h.state.clone()),
            request("GET", "/v1/live-validations", None),
            operator_context(),
        )
        .await;
        assert_eq!(status, StatusCode::OK, "body: {json}");
        assert_eq!(json["tenantId"], "ten_1");
        assert_eq!(json["environmentScope"], "test");
        assert_eq!(json["items"].as_array().map(|a| a.len()).unwrap_or(0), 0);
    }

    // Port of Go TestReplayCandidateLiveValidationRouteHandsOffToLiveValidationManager.
    #[tokio::test]
    async fn replay_candidate_live_validation_hands_off() {
        let store = fresh_store();
        seed_candidate(
            &store,
            &curated_candidate("candidate_live", vec!["daemon.inspection.read".to_string()]),
        );
        let h = harness(
            Some(evaluation_manager(Arc::clone(&store))),
            Some(live_validation_manager(false)),
        );
        let body = serde_json::json!({
            "validationId": "lv_nested",
            "candidateId": "candidate_live",
            "candidateToolClasses": ["daemon.inspection.read"],
            "requestedScope": {
                "scopeId": "scope_lv_nested",
                "includedToolClasses": ["daemon.inspection.read"],
                "approvalMode": "scope_level",
                "declaredBy": "prn_operator",
                "declaredAt": "2026-04-29T09:59:00Z",
            },
            "freshApprovals": [{
                "approvalId": "approval_1",
                "validationId": "lv_nested",
                "approvalTarget": "scope",
                "toolClass": "daemon.inspection.read",
                "safetyClass": "read_only",
                "approvedScope": "scope_lv_nested",
                "status": "approved",
                "requestedBy": "prn_operator",
                "requestedAt": "2026-04-29T09:59:00Z",
                "resolvedAt": "2026-04-29T09:59:30Z",
            }],
        })
        .to_string();
        let (status, json) = send_with_tenant(
            &crate::routes::router(h.state.clone()),
            request(
                "POST",
                "/v1/evaluation/replay-candidates/candidate_live/live-validations",
                Some(&body),
            ),
            operator_context(),
        )
        .await;
        assert_eq!(status, StatusCode::ACCEPTED, "body: {json}");
        assert_eq!(json["attempt"]["candidateId"], "candidate_live");
        assert_eq!(json["attempt"]["status"], "running");
    }

    // Port of Go TestReplayCandidateLiveValidationRouteDerivesCandidateToolClasses.
    #[tokio::test]
    async fn replay_candidate_live_validation_derives_candidate_tool_classes() {
        let store = fresh_store();
        seed_candidate(
            &store,
            &curated_candidate(
                "candidate_mixed",
                vec![
                    "daemon.inspection.read".to_string(),
                    "mcp.tool_call".to_string(),
                ],
            ),
        );
        let h = harness(
            Some(evaluation_manager(Arc::clone(&store))),
            Some(live_validation_manager(false)),
        );
        // No candidateToolClasses in the request: the route derives them from
        // the candidate, and the unsupported mcp.tool_call class blocks.
        let body = serde_json::json!({
            "validationId": "lv_mixed",
            "candidateId": "candidate_mixed",
            "requestedScope": {
                "scopeId": "scope_lv_mixed",
                "includedToolClasses": ["daemon.inspection.read"],
                "approvalMode": "scope_level",
                "declaredBy": "prn_operator",
                "declaredAt": "2026-04-29T10:00:00Z",
            },
        })
        .to_string();
        let (status, json) = send_with_tenant(
            &crate::routes::router(h.state.clone()),
            request(
                "POST",
                "/v1/evaluation/replay-candidates/candidate_mixed/live-validations",
                Some(&body),
            ),
            operator_context(),
        )
        .await;
        assert_eq!(status, StatusCode::CONFLICT, "body: {json}");
        assert_eq!(json["denials"][0]["gate"], "support_matrix");
        assert_eq!(json["denials"][0]["reference"], "mcp.tool_call");
    }

    #[tokio::test]
    async fn live_validation_smoke_requires_tenant_and_permission() {
        let h = harness(None, Some(live_validation_manager(false)));
        let app = crate::routes::router(h.state.clone());
        // No tenant context -> 403 missing_tenant.
        let (status, json) = send(
            &app,
            request("GET", "/v1/live-validations/discord-smoke?connectorId=discord-main", None),
        )
        .await;
        assert_eq!(status, StatusCode::FORBIDDEN, "body: {json}");
        assert_eq!(json["reasonCode"], "credential_denied:missing_tenant");
        assert_eq!(json["error"], "credential_access_denied");
        // Viewer without credential-inspection authority -> 403 missing_permission.
        let (status, json) = send(
            &app,
            with_tenant_extension(
                request("GET", "/v1/live-validations/discord-smoke?connectorId=discord-main", None),
                viewer_context(),
            ),
        )
        .await;
        assert_eq!(status, StatusCode::FORBIDDEN, "body: {json}");
        assert_eq!(json["reasonCode"], "credential_denied:missing_permission");
        // Authorized but no evidence -> 404.
        let (status, json) = send(
            &app,
            with_tenant_extension(
                request("GET", "/v1/live-validations/discord-smoke?connectorId=discord-main", None),
                smoke_tenant_context("ten_discord", "prn_operator"),
            ),
        )
        .await;
        assert_eq!(status, StatusCode::NOT_FOUND, "body: {json}");
    }

    #[tokio::test]
    async fn live_validation_matrix_smoke_records_structured_skip_evidence() {
        let h = harness(None, Some(live_validation_manager(false)));
        let app = crate::routes::router(h.state.clone());
        let tenant = smoke_tenant_context("ten_matrix", "prn_operator");
        // validatedAt must keep the 90-day retention window in the future
        // relative to the test clock.
        let post_body = r#"{"connectorId":"matrix-main","homeserverBindingId":"matrix_hs_1","status":"skipped","authorizationMode":"unavailable","owner":"operator","reason":"safe Matrix credentials unavailable","validatedAt":"2026-09-01T14:00:00Z","safeEvidence":{"policy":"structured_skip"}}"#;
        let (status, json) = send(
            &app,
            with_tenant_extension(request("POST", "/v1/live-validations/matrix-smoke", Some(post_body)), tenant.clone()),
        )
        .await;
        assert_eq!(status, StatusCode::CREATED, "post body: {json}");
        assert_eq!(json["status"], "skipped");
        assert_eq!(json["authorizationMode"], "unavailable");
        assert_eq!(json["smokeEvidenceId"], "matrix_smoke_matrix-main");
        let (status, json) = send(
            &app,
            with_tenant_extension(
                request("GET", "/v1/live-validations/matrix-smoke?connectorId=matrix-main", None),
                tenant,
            ),
        )
        .await;
        assert_eq!(status, StatusCode::OK, "get body: {json}");
        assert_eq!(json["status"], "skipped");
        assert_eq!(json["authorizationMode"], "unavailable");
        assert!(!json.to_string().contains("accessToken"));
    }

    #[tokio::test]
    async fn live_validation_slack_smoke_projects_tenant_safe_evidence() {
        let h = harness(None, Some(live_validation_manager(false)));
        let app = crate::routes::router(h.state.clone());
        let tenant = smoke_tenant_context("ten_slack", "prn_operator");
        // The POST recorder is not ported (dope-api has no dope-slack dep), so
        // seed the evidence through the store and verify the GET projection.
        {
            let store = h.store.lock();
            store
                .save_slack_smoke_evidence(&dope_store::SlackSmokeEvidenceRecord {
                    smoke_evidence_id: "slack_smoke_slack-main".to_string(),
                    tenant_id: "ten_slack".to_string(),
                    connector_id: "slack-main".to_string(),
                    workspace_binding_id: "workspace_binding_redacted".to_string(),
                    status: "passed".to_string(),
                    authorization_mode: "fake_oauth".to_string(),
                    owner: "operator".to_string(),
                    reason: "healthy".to_string(),
                    remaining_risk: String::new(),
                    validated_at: Utc::now() + chrono::Duration::days(30),
                    retention_expires_at: Utc::now() + chrono::Duration::days(120),
                    redaction_status: "redacted".to_string(),
                    safe_evidence: HashMap::from([("mode".to_string(), "fake".to_string())]),
                })
                .expect("seed slack smoke evidence");
        }
        let (status, json) = send(
            &app,
            with_tenant_extension(
                request("GET", "/v1/live-validations/slack-smoke?connectorId=slack-main", None),
                tenant,
            ),
        )
        .await;
        assert_eq!(status, StatusCode::OK, "body: {json}");
        assert_eq!(json["status"], "passed");
        assert_eq!(json["authorizationMode"], "fake_oauth");
        let raw = json.to_string();
        assert!(!raw.contains("xoxb-"), "leaked credential evidence: {raw}");
        assert!(!raw.contains("secret"), "leaked secret evidence: {raw}");
    }

    #[tokio::test]
    async fn unported_product_routes_return_404() {
        let h = harness(None, None);
        let app = crate::routes::router(h.state.clone());
        for uri in [
            "/v1/evaluation/campaigns",
            "/v1/evaluation/campaigns/campaign_1",
            "/v1/evaluation/dashboard",
            "/v1/evaluation/discovery-policies",
            "/v1/evaluation/discovery-runs",
            "/v1/evaluation/discovered-candidates",
            "/v1/evaluation/product-fixtures",
            "/v1/evaluation/suppressions",
            "/v1/evaluation/tool-call-inspections/inspection_1",
            "/v1/evaluation/retention/apply",
        ] {
            let (status, _) = send(&app, request("GET", uri, None)).await;
            assert_eq!(status, StatusCode::NOT_FOUND, "uri: {uri}");
        }
    }
}
