//! Behavioral tests for the Slack route decision logic (port of
//! destinations_test.go TestDecideRouteValidatesSlackPolicyAndSurfaces).

use dope_connectors::DiagnosticReasonCode;
use dope_slack::destinations::{
    ConversationRoute, ConversationType, RoutePolicy, RouteValidationState, SelectedChannelState,
};
use dope_slack::route::{InboundEvent, RouteOutcome, decide_route};

fn policy() -> RoutePolicy {
    RoutePolicy {
        validation_state: RouteValidationState::Valid,
        allowed_dm_users: vec!["user_allowed".to_string()],
        allowed_dm_user_groups: vec!["group_allowed".to_string()],
        selected_channels: vec![ConversationRoute {
            conversation_id: "channel_selected".to_string(),
            conversation_type: ConversationType::Channel,
            selected_channel_state: SelectedChannelState::Selected,
            validation_state: RouteValidationState::Valid,
            ..ConversationRoute::default()
        }],
        ..RoutePolicy::default()
    }
}

#[test]
fn decide_route_validates_slack_policy_and_surfaces() {
    let policy = policy();
    let cases: Vec<(&str, InboundEvent, RouteOutcome, &str)> = vec![
        (
            "allowed dm user",
            InboundEvent {
                workspace_id: "workspace".to_string(),
                conversation_id: "dm_1".to_string(),
                conversation_type: ConversationType::DirectMessage,
                message_id: "m1".to_string(),
                sender_id: "user_allowed".to_string(),
                ..InboundEvent::default()
            },
            RouteOutcome::Accepted,
            "accepted",
        ),
        (
            "allowed dm group",
            InboundEvent {
                workspace_id: "workspace".to_string(),
                conversation_id: "dm_2".to_string(),
                conversation_type: ConversationType::DirectMessage,
                message_id: "m2".to_string(),
                sender_id: "user_other".to_string(),
                sender_user_group_ids: vec!["group_allowed".to_string()],
                ..InboundEvent::default()
            },
            RouteOutcome::Accepted,
            "accepted",
        ),
        (
            "blocked dm",
            InboundEvent {
                workspace_id: "workspace".to_string(),
                conversation_id: "dm_3".to_string(),
                conversation_type: ConversationType::DirectMessage,
                message_id: "m3".to_string(),
                sender_id: "user_other".to_string(),
                ..InboundEvent::default()
            },
            RouteOutcome::Blocked,
            DiagnosticReasonCode::BlockedRoute.as_str(),
        ),
        (
            "selected channel mention",
            InboundEvent {
                workspace_id: "workspace".to_string(),
                conversation_id: "channel_selected".to_string(),
                conversation_type: ConversationType::Channel,
                message_id: "m4".to_string(),
                mentioned: true,
                ..InboundEvent::default()
            },
            RouteOutcome::Accepted,
            "accepted",
        ),
        (
            "selected channel no mention",
            InboundEvent {
                workspace_id: "workspace".to_string(),
                conversation_id: "channel_selected".to_string(),
                conversation_type: ConversationType::Channel,
                message_id: "m5".to_string(),
                ..InboundEvent::default()
            },
            RouteOutcome::Ignored,
            "mention_required",
        ),
        (
            "unselected channel",
            InboundEvent {
                workspace_id: "workspace".to_string(),
                conversation_id: "channel_other".to_string(),
                conversation_type: ConversationType::Channel,
                message_id: "m6".to_string(),
                mentioned: true,
                ..InboundEvent::default()
            },
            RouteOutcome::Blocked,
            DiagnosticReasonCode::BlockedRoute.as_str(),
        ),
        (
            "wrong workspace",
            InboundEvent {
                workspace_id: "workspace_other".to_string(),
                conversation_id: "channel_selected".to_string(),
                conversation_type: ConversationType::Channel,
                message_id: "m7".to_string(),
                mentioned: true,
                ..InboundEvent::default()
            },
            RouteOutcome::Blocked,
            DiagnosticReasonCode::BlockedRoute.as_str(),
        ),
        (
            "unsupported",
            InboundEvent {
                workspace_id: "workspace".to_string(),
                conversation_id: "channel_selected".to_string(),
                conversation_type: ConversationType::Channel,
                message_id: "m8".to_string(),
                surface: "huddle".to_string(),
                ..InboundEvent::default()
            },
            RouteOutcome::Unsupported,
            DiagnosticReasonCode::UnsupportedCapability.as_str(),
        ),
        (
            "missing identity",
            InboundEvent {
                workspace_id: "workspace".to_string(),
                conversation_type: ConversationType::Channel,
                message_id: "m9".to_string(),
                ..InboundEvent::default()
            },
            RouteOutcome::Failed,
            DiagnosticReasonCode::UnknownConnectorFailure.as_str(),
        ),
    ];
    for (name, event, want_outcome, want_reason) in cases {
        let decision = decide_route(&event, &policy, "workspace", "bot_1");
        assert_eq!(decision.outcome, want_outcome, "{name}: outcome");
        assert_eq!(decision.reason_code, want_reason, "{name}: reason");
    }
}

#[test]
fn normalize_mention_text_strips_bot_mention() {
    assert_eq!(
        dope_slack::transport::normalize_mention_text("<@bot_redacted> hello", "bot_redacted"),
        "hello"
    );
    assert_eq!(
        dope_slack::transport::normalize_mention_text("hello", "bot_redacted"),
        "hello"
    );
    assert_eq!(
        dope_slack::transport::normalize_mention_text("<@bot_redacted>", " bot_redacted "),
        ""
    );
}

#[test]
fn slack_identity_key_joins_trimmed_components() {
    let key = dope_slack::runtime::slack_message_identity_key(&[
        "ten_slack",
        " slack-main ",
        "workspace_redacted",
        "channel_selected",
        "message_1",
    ]);
    assert_eq!(
        key,
        "ten_slack\0slack-main\0workspace_redacted\0channel_selected\0message_1"
    );
}

#[test]
fn unsupported_surface_error_keeps_go_message() {
    let err = dope_slack::transport::unsupported_surface_error(" huddle ");
    assert_eq!(err.to_string(), "unsupported slack surface: huddle");
}
