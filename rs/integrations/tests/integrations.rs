
use chrono::Utc;
use dope_integrations::{
    backend_kind_supports_domain, classify_provider_evidence,
    diagnostic_failure_for_operation_failure, BackendBinding, BackendKind, CreateInput,
    DiagnosticReasonCode, FreshnessState, Manager, ProbeKind, ProviderDiagnosticEvidence,
    ReadinessStatus, UpdateReadinessInput,
};
use serde_json::Map;

fn manager() -> Manager {
    Manager::new("test")
}

fn create_input(integration_id: &str, domain_kind: &str) -> CreateInput {
    CreateInput {
        tenant_id: "ten_1".to_string(),
        integration_id: integration_id.to_string(),
        domain_kind: domain_kind.to_string(),
        display_name: "Test".to_string(),
        backend_binding: BackendBinding {
            backend_kind: BackendKind::FakeLocal,
            supports_probe_read: true,
            ..BackendBinding::default()
        },
        ..CreateInput::default()
    }
}

#[test]
fn create_sets_not_configured() {
    let manager = manager();
    let resource = manager.create(create_input("cal-1", "calendar")).expect("create");
    assert_eq!(resource.readiness_status, ReadinessStatus::NotConfigured);
    assert_eq!(resource.integration_id, "cal-1");
    assert_eq!(resource.domain_kind, "calendar");
}

#[test]
fn update_readiness_to_healthy() {
    let manager = manager();
    let _ = manager.create(create_input("cal-1", "calendar")).expect("create");
    let resource = manager
        .update_readiness(
            "cal-1",
            UpdateReadinessInput {
                readiness_status: ReadinessStatus::Healthy,
                ..UpdateReadinessInput::default()
            },
        )
        .expect("update");
    assert_eq!(resource.readiness_status, ReadinessStatus::Healthy);
    assert!(resource.last_ready_at.is_some());
}

#[test]
fn create_rejects_missing_fields() {
    let manager = manager();
    assert!(manager.create(create_input("", "calendar")).is_err());
    assert!(manager.create(create_input("cal-1", "")).is_err());
}

#[test]
fn fake_backend_supports_calendar_and_mail() {
    assert!(backend_kind_supports_domain(BackendKind::FakeLocal, "calendar"));
    assert!(backend_kind_supports_domain(BackendKind::FakeLocal, "mail"));
    assert!(!backend_kind_supports_domain(BackendKind::FakeLocal, "reminders"));
    assert!(!backend_kind_supports_domain(BackendKind::AdapterRpc, "calendar"));
}

#[test]
fn run_probe_returns_completed() {
    let manager = manager();
    let _ = manager.create(create_input("cal-1", "calendar")).expect("create");
    let input = Map::new();
    let (_resource, result, summary) = manager.run_probe("cal-1", ProbeKind::Inspect, &input).expect("probe");
    assert_eq!(result.status, "completed");
    assert_eq!(summary.integration_id, "cal-1");
}

#[test]
fn classify_provider_evidence_maps_token_expired() {
    let evidence = ProviderDiagnosticEvidence {
        provider_error_class: "token_expired".to_string(),
        ..ProviderDiagnosticEvidence::default()
    };
    let classification = classify_provider_evidence(&evidence);
    assert_eq!(classification.reason_code, DiagnosticReasonCode::TokenExpired);
    assert!(!classification.ambiguous);
}

#[test]
fn classify_provider_evidence_feishu_scope() {
    let evidence = ProviderDiagnosticEvidence {
        provider_kind: "feishu_lark".to_string(),
        provider_error_class: "99991669".to_string(),
        ..ProviderDiagnosticEvidence::default()
    };
    let classification = classify_provider_evidence(&evidence);
    assert_eq!(classification.reason_code, DiagnosticReasonCode::ScopeMissing);
}

#[test]
fn operation_failure_projection_is_fresh() {
    let projection = diagnostic_failure_for_operation_failure(
        "calendar",
        "feishu_lark",
        "cal-1",
        "create_event",
        "scope_not_granted",
        "missing scope",
        true,
        Utc::now(),
    );
    assert_eq!(projection.reason_code, DiagnosticReasonCode::ScopeMissing);
    assert_eq!(projection.freshness_state, FreshnessState::Fresh);
}
