//! Behavioral tests for diagnostics classification and redaction (port of
//! diagnostics_test.go).

use std::collections::HashMap;

use chrono::{TimeZone, Utc};
use dope_connectors::{DiagnosticReasonCode, RedactionStatus};
use dope_slack::diagnostics::{
    build_diagnostic_state, diagnostic_reason_for_error, diagnostic_reason_for_message,
};
use dope_slack::transport_webapi::WebApiError;

#[test]
fn diagnostic_reason_for_error_maps_slack_failures() {
    let cases = [
        (
            "invalid_auth oauth grant revoked",
            DiagnosticReasonCode::AuthMissing,
        ),
        (
            "missing_scope not_in_channel permission",
            DiagnosticReasonCode::PermissionMissing,
        ),
        (
            "slack web api returned 429 rate limited",
            DiagnosticReasonCode::RateLimited,
        ),
        (
            "slack provider unavailable 5xx",
            DiagnosticReasonCode::ProviderUnavailable,
        ),
        (
            "event delivery timeout network failed",
            DiagnosticReasonCode::NetworkFailed,
        ),
        (
            "unsupported slack huddle interactive block",
            DiagnosticReasonCode::UnsupportedCapability,
        ),
    ];
    for (message, want) in cases {
        let err = std::io::Error::other(message);
        assert_eq!(
            diagnostic_reason_for_error(&err),
            want,
            "message: {message}"
        );
        assert_eq!(
            diagnostic_reason_for_message(message),
            want,
            "message path: {message}"
        );
    }
}

#[test]
fn web_api_error_class_maps_codes_and_statuses() {
    let cases = [
        (
            WebApiError {
                status_code: 0,
                code: "invalid_auth".to_string(),
                message: String::new(),
            },
            DiagnosticReasonCode::AuthMissing,
        ),
        (
            WebApiError {
                status_code: 0,
                code: "token_revoked".to_string(),
                message: String::new(),
            },
            DiagnosticReasonCode::AuthMissing,
        ),
        (
            WebApiError {
                status_code: 0,
                code: "missing_scope".to_string(),
                message: String::new(),
            },
            DiagnosticReasonCode::PermissionMissing,
        ),
        (
            WebApiError {
                status_code: 0,
                code: "not_in_channel".to_string(),
                message: String::new(),
            },
            DiagnosticReasonCode::PermissionMissing,
        ),
        (
            WebApiError {
                status_code: 0,
                code: "is_archived".to_string(),
                message: String::new(),
            },
            DiagnosticReasonCode::PermissionMissing,
        ),
        (
            WebApiError {
                status_code: 0,
                code: "ratelimited".to_string(),
                message: String::new(),
            },
            DiagnosticReasonCode::RateLimited,
        ),
        (
            WebApiError {
                status_code: 429,
                code: String::new(),
                message: String::new(),
            },
            DiagnosticReasonCode::RateLimited,
        ),
        (
            WebApiError {
                status_code: 500,
                code: String::new(),
                message: String::new(),
            },
            DiagnosticReasonCode::ProviderUnavailable,
        ),
        (
            WebApiError {
                status_code: 403,
                code: String::new(),
                message: String::new(),
            },
            DiagnosticReasonCode::ProviderUnavailable,
        ),
        (
            WebApiError {
                status_code: 0,
                code: "unknown_code".to_string(),
                message: String::new(),
            },
            DiagnosticReasonCode::UnknownConnectorFailure,
        ),
    ];
    for (err, want) in cases {
        assert_eq!(err.error_class(), want, "error_class for {err:?}");
    }
}

#[test]
fn build_diagnostic_state_redacts_slack_unsafe_evidence() {
    let now = Utc
        .with_ymd_and_hms(2026, 5, 8, 10, 0, 0)
        .single()
        .expect("valid timestamp");
    let state = build_diagnostic_state(
        "ten_slack",
        "slack-main",
        "workspace_binding_redacted",
        DiagnosticReasonCode::AuthMissing,
        &HashMap::from([
            ("botToken".to_string(), "xoxb-secret".to_string()),
            ("hint".to_string(), "workspace redacted".to_string()),
        ]),
        now,
    )
    .expect("build diagnostic state");
    assert_eq!(state.redaction_status, RedactionStatus::Suppressed);
    assert!(
        !state.redaction_failure_id.is_empty(),
        "expected a redaction failure id"
    );
    assert_eq!(state.redaction_failure_id, "redaction_failed_slack-main");
    for value in state.safe_evidence.values() {
        assert!(
            !value.contains("xoxb-secret"),
            "unsafe evidence leaked: {value}"
        );
    }
}

#[test]
fn build_diagnostic_state_keeps_safe_evidence() {
    let now = Utc
        .with_ymd_and_hms(2026, 5, 8, 10, 0, 0)
        .single()
        .expect("valid timestamp");
    let evidence = HashMap::from([
        ("workspaceId".to_string(), "workspace_redacted".to_string()),
        ("stage".to_string(), "message_loop".to_string()),
    ]);
    let state = build_diagnostic_state(
        "ten_slack",
        "slack-main",
        "workspace_binding_redacted",
        DiagnosticReasonCode::ProviderUnavailable,
        &evidence,
        now,
    )
    .expect("build diagnostic state");
    assert_eq!(state.redaction_status, RedactionStatus::Redacted);
    assert_eq!(state.safe_evidence, evidence);
    assert_eq!(state.status.as_str(), "degraded");
    assert_eq!(state.user_visible_severity, "error");
}
