//! Behavioral tests for the smoke evidence builder (port of smoke_test.go).

use std::collections::HashMap;

use chrono::{TimeZone, Utc};
use kura_slack::smoke::{
    AuthorizationMode, SmokeEvidence, SmokeInput, SmokeStatus, build_smoke_evidence,
};

fn now() -> chrono::DateTime<Utc> {
    Utc.with_ymd_and_hms(2026, 5, 8, 13, 0, 0)
        .single()
        .expect("valid timestamp")
}

#[test]
fn build_smoke_evidence_structured_skip_and_fake_pass() {
    let skip = build_smoke_evidence(SmokeInput {
        tenant_id: "ten_slack".to_string(),
        connector_id: "slack-main".to_string(),
        workspace_binding_id: "workspace_binding_redacted".to_string(),
        validated_at: now(),
        ..SmokeInput::default()
    });
    assert_eq!(skip.status, SmokeStatus::Skipped);
    assert_eq!(skip.authorization_mode, AuthorizationMode::Unavailable);
    assert!(
        !skip.remaining_risk.is_empty(),
        "structured skip must carry remaining risk"
    );

    let fake_pass = build_smoke_evidence(SmokeInput {
        tenant_id: "ten_slack".to_string(),
        connector_id: "slack-main".to_string(),
        workspace_binding_id: "workspace_binding_redacted".to_string(),
        fake_oauth: true,
        passed: true,
        validated_at: now(),
        safe_evidence: HashMap::from([("transport".to_string(), "fake".to_string())]),
        remaining_risk: "live provider not exercised".to_string(),
        ..SmokeInput::default()
    });
    assert_eq!(fake_pass.status, SmokeStatus::Passed);
    assert_eq!(fake_pass.authorization_mode, AuthorizationMode::FakeOAuth);
    assert_eq!(fake_pass.reason, "healthy");
}

#[test]
fn build_smoke_evidence_suppresses_unsafe_evidence() {
    let smoke = build_smoke_evidence(SmokeInput {
        tenant_id: "ten_slack".to_string(),
        connector_id: "slack-main".to_string(),
        workspace_binding_id: "workspace_binding_redacted".to_string(),
        safe_live_approved: true,
        safe_evidence: HashMap::from([(
            "authorization".to_string(),
            "Bearer xoxb-secret".to_string(),
        )]),
        ..SmokeInput::default()
    });
    assert_eq!(smoke.redaction_status.as_str(), "suppressed");
    assert!(
        smoke.safe_evidence.is_empty(),
        "unsafe smoke evidence must be suppressed"
    );
    let _: SmokeEvidence = smoke;
}
