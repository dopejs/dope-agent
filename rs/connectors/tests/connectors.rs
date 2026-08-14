//! Serde round-trip tests for the dope-connectors wire types: serialize each
//! struct to JSON, assert the camelCase field names and snake_case enum values
//! match the Go json tags, then deserialize back and compare.

use std::collections::HashMap;

use chrono::{DateTime, Utc};
use dope_connectors::{
    Connector, ConnectorDiagnosticState, ConformanceResult, ConformanceResultStatus,
    DiagnosticReasonCode, FreshnessState, LifecycleState, RedactionStatus, RemediationOwner,
    RetrySafety, Status,
};

fn ts(s: &str) -> DateTime<Utc> {
    DateTime::parse_from_rfc3339(s).unwrap().with_timezone(&Utc)
}

#[test]
fn connector_roundtrips_camel_case_fields_and_enum_values() {
    let connector = Connector {
        tenant_id: "tenant-a".to_string(),
        connector_id: "conn-1".to_string(),
        kind: "matrix".to_string(),
        display_name: "Matrix".to_string(),
        status: Status::Healthy,
        disabled_reason: String::new(),
        secret_refs: vec!["ref/matrix/token".to_string()],
        secret_summary: vec![],
        failure_count: 3,
        restart_count: 1,
        backoff_seconds: 30,
        next_restart_at: None,
        last_restart_at: None,
        last_heartbeat_at: None,
        last_failure_reason: "timeout".to_string(),
        capability_profile: Default::default(),
        diagnostic_state: Default::default(),
        conformance_result: Default::default(),
        account_binding: Default::default(),
        created_at: ts("2025-01-02T03:04:05Z"),
        updated_at: ts("2025-01-02T04:05:06Z"),
    };

    let json = serde_json::to_value(&connector).unwrap();
    let obj = json.as_object().unwrap();

    // camelCase field names per supervisor.go json tags.
    assert!(obj.contains_key("tenantId"));
    assert!(obj.contains_key("connectorId"));
    assert!(obj.contains_key("displayName"));
    assert!(obj.contains_key("secretRefs"));
    assert!(obj.contains_key("failureCount"));
    assert!(obj.contains_key("restartCount"));
    assert!(obj.contains_key("backoffSeconds"));
    assert!(obj.contains_key("lastFailureReason"));
    assert!(obj.contains_key("createdAt"));
    assert!(obj.contains_key("updatedAt"));

    // snake_case enum wire value.
    assert_eq!(obj["status"], serde_json::json!("healthy"));
    // Go's omitempty: empty optional fields are absent.
    assert!(!obj.contains_key("disabledReason"));
    assert!(!obj.contains_key("secretSummary"));
    assert!(!obj.contains_key("nextRestartAt"));
    assert!(!obj.contains_key("lastRestartAt"));
    assert!(!obj.contains_key("lastHeartbeatAt"));
    assert!(!obj.contains_key("capabilityProfile"));
    assert!(!obj.contains_key("diagnosticState"));
    assert!(!obj.contains_key("conformanceResult"));
    assert!(!obj.contains_key("accountBinding"));

    let back: Connector = serde_json::from_value(json).unwrap();
    assert_eq!(back, connector);
}

#[test]
fn conformance_result_roundtrips_camel_case_fields_and_enum_values() {
    let result = ConformanceResult {
        conformance_result_id: "conf_scenario-1_redaction".to_string(),
        tenant_id: "tenant-a".to_string(),
        connector_kind: "matrix".to_string(),
        connector_id: "conn-1".to_string(),
        scenario_id: "scenario-1".to_string(),
        area: "redaction".to_string(),
        result: ConformanceResultStatus::Pass,
        reason_code: String::new(),
        redaction_status: RedactionStatus::Redacted,
        evidence_timestamp: ts("2025-01-02T03:04:05Z"),
        retention_expires_at: ts("2025-04-02T03:04:05Z"),
    };

    let json = serde_json::to_value(&result).unwrap();
    let obj = json.as_object().unwrap();

    assert!(obj.contains_key("conformanceResultId"));
    assert!(obj.contains_key("tenantId"));
    assert!(obj.contains_key("connectorKind"));
    assert!(obj.contains_key("connectorId"));
    assert!(obj.contains_key("scenarioId"));
    assert!(obj.contains_key("area"));
    assert!(obj.contains_key("result"));
    assert!(obj.contains_key("redactionStatus"));
    assert!(obj.contains_key("evidenceTimestamp"));
    assert!(obj.contains_key("retentionExpiresAt"));
    // snake_case enum wire values.
    assert_eq!(obj["result"], serde_json::json!("pass"));
    assert_eq!(obj["redactionStatus"], serde_json::json!("redacted"));
    // omitempty
    assert!(!obj.contains_key("reasonCode"));

    let back: ConformanceResult = serde_json::from_value(json).unwrap();
    assert_eq!(back, result);
}

#[test]
fn connector_diagnostic_state_roundtrips_camel_case_fields_and_enum_values() {
    let state = ConnectorDiagnosticState {
        diagnostic_state_id: "diag_conn-1_rate_limited".to_string(),
        tenant_id: "tenant-a".to_string(),
        connector_id: "conn-1".to_string(),
        connector_account_id: "acct-9".to_string(),
        status: LifecycleState::RateLimited,
        reason_code: DiagnosticReasonCode::RateLimited,
        remediation_owner: RemediationOwner::Provider,
        user_visible_severity: "warning".to_string(),
        retry_safety: RetrySafety::RetryAfter,
        evidence_timestamp: ts("2025-01-02T03:04:05Z"),
        freshness_state: FreshnessState::Fresh,
        redaction_status: RedactionStatus::Redacted,
        retention_expires_at: ts("2025-04-02T03:04:05Z"),
        safe_evidence: HashMap::from([("scope".to_string(), "rate_limited".to_string())]),
        redaction_failure_id: String::new(),
    };

    let json = serde_json::to_value(&state).unwrap();
    let obj = json.as_object().unwrap();

    assert!(obj.contains_key("diagnosticStateId"));
    assert!(obj.contains_key("tenantId"));
    assert!(obj.contains_key("connectorId"));
    assert!(obj.contains_key("connectorAccountId"));
    assert!(obj.contains_key("userVisibleSeverity"));
    assert!(obj.contains_key("evidenceTimestamp"));
    assert!(obj.contains_key("freshnessState"));
    assert!(obj.contains_key("retentionExpiresAt"));
    assert!(obj.contains_key("safeEvidence"));

    // snake_case enum wire values.
    assert_eq!(obj["status"], serde_json::json!("rate_limited"));
    assert_eq!(obj["reasonCode"], serde_json::json!("rate_limited"));
    assert_eq!(obj["remediationOwner"], serde_json::json!("provider"));
    assert_eq!(obj["retrySafety"], serde_json::json!("retry_after"));
    assert_eq!(obj["freshnessState"], serde_json::json!("fresh"));
    assert_eq!(obj["redactionStatus"], serde_json::json!("redacted"));
    // omitempty
    assert!(!obj.contains_key("redactionFailureId"));

    let back: ConnectorDiagnosticState = serde_json::from_value(json).unwrap();
    assert_eq!(back, state);
}

#[test]
fn enum_wire_values_match_go_constants() {
    // Spot-check every enum against the Go string constants.
    assert_eq!(Status::Registered.as_str(), "registered");
    assert_eq!(Status::BackingOff.as_str(), "backing_off");
    assert_eq!(Status::Disabled.as_str(), "disabled");

    assert_eq!(LifecycleState::Configured.as_str(), "configured");
    assert_eq!(LifecycleState::PermissionBlocked.as_str(), "permission_blocked");
    assert_eq!(LifecycleState::UnsupportedCapability.as_str(), "unsupported_capability");

    assert_eq!(ConformanceResultStatus::Fail.as_str(), "fail");
    assert_eq!(ConformanceResultStatus::Limited.as_str(), "limited");

    assert_eq!(RedactionStatus::Redacted.as_str(), "redacted");
    assert_eq!(RedactionStatus::Suppressed.as_str(), "suppressed");
    assert_eq!(RedactionStatus::Failed.as_str(), "redaction_failed");

    assert_eq!(DiagnosticReasonCode::AuthMissing.as_str(), "auth_missing");
    assert_eq!(DiagnosticReasonCode::UnknownConnectorFailure.as_str(), "unknown_connector_failure");

    assert_eq!(RemediationOwner::User.as_str(), "product_user");
    assert_eq!(RemediationOwner::NoneRequired.as_str(), "none_required");

    assert_eq!(RetrySafety::NoActionNeeded.as_str(), "no_action_needed");
    assert_eq!(RetrySafety::Unsafe.as_str(), "unsafe");

    assert_eq!(FreshnessState::Fresh.as_str(), "fresh");
    assert_eq!(FreshnessState::Stale.as_str(), "stale");
}
