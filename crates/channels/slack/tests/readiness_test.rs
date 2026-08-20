//! Behavioral tests for the hosted-setup readiness state machine (port of
//! readiness_test.go).

use chrono::{Duration, TimeZone, Utc};
use kura_connectors::{DiagnosticReasonCode, LifecycleState};
use kura_slack::destinations::{
    ConversationRoute, ConversationType, RoutePolicy, RouteValidationState, SelectedChannelState,
};
use kura_slack::readiness::{
    HostedSetupInput, OAuthState, RoutePolicyState, TerminalState, WorkspaceBinding,
    evaluate_hosted_setup,
};

fn ts(y: i32, mo: u32, d: u32, h: u32, mi: u32) -> chrono::DateTime<Utc> {
    Utc.with_ymd_and_hms(y, mo, d, h, mi, 0)
        .single()
        .expect("valid timestamp")
}

fn ready_binding() -> WorkspaceBinding {
    WorkspaceBinding {
        workspace_id: "workspace_redacted".to_string(),
        installation_id: "installation_redacted".to_string(),
        oauth_grant_state: "valid".to_string(),
        required_scope_state: "valid".to_string(),
        ..WorkspaceBinding::default()
    }
}

fn ready_route_policy() -> RoutePolicy {
    RoutePolicy {
        validation_state: RouteValidationState::Valid,
        selected_channels: vec![ConversationRoute {
            conversation_id: "channel_redacted".to_string(),
            conversation_type: ConversationType::Channel,
            selected_channel_state: SelectedChannelState::Selected,
            validation_state: RouteValidationState::Valid,
            ..ConversationRoute::default()
        }],
        ..RoutePolicy::default()
    }
}

#[test]
fn evaluate_hosted_setup_ready_with_valid_workspace_and_route_policy() {
    let now = ts(2026, 5, 8, 10, 0);
    let setup = evaluate_hosted_setup(HostedSetupInput {
        tenant_id: "ten_slack".to_string(),
        connector_id: "slack-main".to_string(),
        display_name: "Slack Main".to_string(),
        oauth_state: OAuthState::GrantValid,
        provider_available: true,
        network_available: true,
        validated_at: now,
        workspace_binding: ready_binding(),
        route_policy: ready_route_policy(),
        ..HostedSetupInput::default()
    });
    assert_eq!(
        setup.terminal_state,
        TerminalState::Ready,
        "expected ready hosted setup"
    );
    assert_eq!(setup.status, LifecycleState::Healthy);
    assert!(setup.delivery_eligible);
    assert_eq!(
        setup
            .workspace_binding
            .as_ref()
            .map(|b| b.workspace_binding_id.as_str()),
        Some("slack_workspace_slack-main"),
        "expected normalized workspace binding"
    );
    assert_eq!(setup.route_policy_state, RoutePolicyState::Valid);
}

#[test]
fn evaluate_hosted_setup_maps_oauth_terminal_states() {
    let now = ts(2026, 5, 8, 10, 0);
    let cases: Vec<(&str, HostedSetupInput, TerminalState, OAuthState, &str)> = vec![
        (
            "missing grant",
            HostedSetupInput {
                oauth_state: OAuthState::GrantMissing,
                ..HostedSetupInput::default()
            },
            TerminalState::ActionRequired,
            OAuthState::GrantMissing,
            DiagnosticReasonCode::AuthMissing.as_str(),
        ),
        (
            "revoked",
            HostedSetupInput {
                oauth_state: OAuthState::Revoked,
                ..HostedSetupInput::default()
            },
            TerminalState::ActionRequired,
            OAuthState::Revoked,
            DiagnosticReasonCode::AuthMissing.as_str(),
        ),
        (
            "scope missing",
            HostedSetupInput {
                oauth_state: OAuthState::ScopeMissing,
                ..HostedSetupInput::default()
            },
            TerminalState::ActionRequired,
            OAuthState::ScopeMissing,
            DiagnosticReasonCode::PermissionMissing.as_str(),
        ),
        (
            "approval required",
            HostedSetupInput {
                oauth_state: OAuthState::ApprovalRequired,
                ..HostedSetupInput::default()
            },
            TerminalState::ActionRequired,
            OAuthState::ApprovalRequired,
            DiagnosticReasonCode::PermissionMissing.as_str(),
        ),
        (
            "cancelled",
            HostedSetupInput {
                oauth_state: OAuthState::Started,
                cancelled: true,
                ..HostedSetupInput::default()
            },
            TerminalState::Cancelled,
            OAuthState::Started,
            "user_cancelled",
        ),
        (
            "provider unavailable",
            HostedSetupInput {
                oauth_state: OAuthState::GrantValid,
                provider_available: false,
                network_available: true,
                ..HostedSetupInput::default()
            },
            TerminalState::Unavailable,
            OAuthState::GrantValid,
            DiagnosticReasonCode::ProviderUnavailable.as_str(),
        ),
        (
            "network failed",
            HostedSetupInput {
                oauth_state: OAuthState::GrantValid,
                provider_available: true,
                network_available: false,
                ..HostedSetupInput::default()
            },
            TerminalState::Unavailable,
            OAuthState::GrantValid,
            DiagnosticReasonCode::NetworkFailed.as_str(),
        ),
    ];
    for (name, mut input, want_state, want_oauth, want_reason) in cases {
        input.tenant_id = "ten_slack".to_string();
        input.connector_id = "slack-main".to_string();
        input.display_name = "Slack Main".to_string();
        input.validated_at = now;
        let setup = evaluate_hosted_setup(input);
        assert_eq!(setup.terminal_state, want_state, "{name}: terminal state");
        assert_eq!(setup.oauth_state, want_oauth, "{name}: oauth state");
        assert_eq!(setup.reason_code, want_reason, "{name}: reason");
        assert!(
            !setup.delivery_eligible,
            "{name}: must be delivery-ineligible"
        );
    }
}

#[test]
fn evaluate_hosted_setup_requires_validated_route_policy() {
    let now = ts(2026, 5, 8, 10, 0);
    let base = HostedSetupInput {
        tenant_id: "ten_slack".to_string(),
        connector_id: "slack-main".to_string(),
        display_name: "Slack Main".to_string(),
        oauth_state: OAuthState::GrantValid,
        provider_available: true,
        network_available: true,
        validated_at: now,
        workspace_binding: ready_binding(),
        ..HostedSetupInput::default()
    };
    let setup = evaluate_hosted_setup(base.clone());
    assert_eq!(setup.terminal_state, TerminalState::ActionRequired);
    assert_eq!(setup.route_policy_state, RoutePolicyState::None);
    assert!(
        !setup.delivery_eligible,
        "valid OAuth without selected route must stay action-required"
    );

    let blocked = HostedSetupInput {
        route_policy: RoutePolicy {
            validation_state: RouteValidationState::Blocked,
            allowed_dm_users: vec!["user_hash_1".to_string()],
            ..RoutePolicy::default()
        },
        ..base
    };
    let setup = evaluate_hosted_setup(blocked);
    assert_eq!(setup.terminal_state, TerminalState::ActionRequired);
    assert!(
        !setup.delivery_eligible,
        "blocked route policy with DM allowment must not be ready"
    );
}

#[test]
fn evaluate_hosted_setup_workspace_binding_cardinality_and_timeout() {
    let now = ts(2026, 5, 8, 10, 6);
    let timeout_setup = evaluate_hosted_setup(HostedSetupInput {
        tenant_id: "ten_slack".to_string(),
        connector_id: "slack-main".to_string(),
        display_name: "Slack Main".to_string(),
        oauth_state: OAuthState::GrantValid,
        provider_available: true,
        network_available: true,
        expected_workspace_id: "workspace_expected".to_string(),
        started_at: now - Duration::minutes(6),
        validated_at: now,
        workspace_binding: WorkspaceBinding {
            workspace_id: "workspace_other".to_string(),
            installation_id: "installation_redacted".to_string(),
            oauth_grant_state: "valid".to_string(),
            required_scope_state: "valid".to_string(),
            ..WorkspaceBinding::default()
        },
        route_policy: RoutePolicy {
            validation_state: RouteValidationState::Valid,
            allowed_dm_users: vec!["user_hash_1".to_string()],
            ..RoutePolicy::default()
        },
        ..HostedSetupInput::default()
    });
    assert_eq!(timeout_setup.terminal_state, TerminalState::Unavailable);
    assert_eq!(timeout_setup.reason_code, "setup_timeout");

    let first = evaluate_hosted_setup(HostedSetupInput {
        tenant_id: "ten_slack".to_string(),
        connector_id: "slack-east".to_string(),
        validated_at: now,
        ..HostedSetupInput::default()
    });
    let second = evaluate_hosted_setup(HostedSetupInput {
        tenant_id: "ten_slack".to_string(),
        connector_id: "slack-west".to_string(),
        validated_at: now,
        ..HostedSetupInput::default()
    });
    assert_ne!(
        first.workspace_binding_id, second.workspace_binding_id,
        "multiple connectors for one tenant need distinct workspace bindings"
    );

    let mismatch = evaluate_hosted_setup(HostedSetupInput {
        tenant_id: "ten_slack".to_string(),
        connector_id: "slack-main".to_string(),
        oauth_state: OAuthState::GrantValid,
        provider_available: true,
        network_available: true,
        expected_workspace_id: "workspace_expected".to_string(),
        validated_at: now,
        workspace_binding: WorkspaceBinding {
            workspace_id: "workspace_other".to_string(),
            installation_id: "installation_redacted".to_string(),
            oauth_grant_state: "valid".to_string(),
            required_scope_state: "valid".to_string(),
            ..WorkspaceBinding::default()
        },
        route_policy: RoutePolicy {
            validation_state: RouteValidationState::Valid,
            allowed_dm_users: vec!["user_hash_1".to_string()],
            ..RoutePolicy::default()
        },
        ..HostedSetupInput::default()
    });
    assert_eq!(mismatch.terminal_state, TerminalState::ActionRequired);
    assert_eq!(mismatch.reason_code, "workspace_mismatch");
    assert!(
        !mismatch.delivery_eligible,
        "expected workspace mismatch to fail closed"
    );
}
