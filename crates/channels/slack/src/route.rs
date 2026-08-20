//! Slack routing decision logic (port of route.go): the inbound event shape,
//! route outcome enum, and DecideRoute policy evaluation (identity, surface,
//! workspace, DM/channel gates).

use std::collections::HashSet;

use chrono::{DateTime, Utc};

use kura_connectors::DiagnosticReasonCode;

use crate::destinations::{
    ConversationType, RoutePolicy, RouteValidationState, SelectedChannelState,
    normalize_route_policy,
};
use crate::util::first_non_empty;

/// Outcome of a route decision (Go `RouteOutcome`).
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default)]
pub enum RouteOutcome {
    #[default]
    Accepted,
    Ignored,
    Blocked,
    Duplicate,
    Unsupported,
    Failed,
}

impl RouteOutcome {
    #[must_use]
    pub fn as_str(self) -> &'static str {
        match self {
            RouteOutcome::Accepted => "accepted",
            RouteOutcome::Ignored => "ignored",
            RouteOutcome::Blocked => "blocked",
            RouteOutcome::Duplicate => "duplicate",
            RouteOutcome::Unsupported => "unsupported",
            RouteOutcome::Failed => "failed",
        }
    }
}

impl std::fmt::Display for RouteOutcome {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_str(self.as_str())
    }
}

/// One raw Slack inbound event delivered by the transport (Go `InboundEvent`).
#[derive(Debug, Clone, Default, PartialEq)]
pub struct InboundEvent {
    pub tenant_id: String,
    pub connector_id: String,
    pub workspace_id: String,
    pub conversation_id: String,
    pub conversation_type: ConversationType,
    pub message_id: String,
    pub thread_root_message_id: String,
    pub event_id: String,
    pub sender_id: String,
    pub sender_user_group_ids: Vec<String>,
    pub text: String,
    pub mentioned: bool,
    pub surface: String,
    pub received_at: DateTime<Utc>,
}

/// Result of evaluating one inbound event against the route policy
/// (Go `RouteDecision`).
#[derive(Debug, Clone, Default, PartialEq)]
pub struct RouteDecision {
    pub outcome: RouteOutcome,
    pub reason_code: String,
    pub surface: String,
    pub normalized_text: String,
}

/// Go `DecideRoute`: applies the Slack routing gates in order.
pub fn decide_route(
    event: &InboundEvent,
    policy: &RoutePolicy,
    workspace_id: &str,
    bot_user_id: &str,
) -> RouteDecision {
    let surface = first_non_empty(&[&event.surface, event.conversation_type.as_str()]);
    if missing_slack_identity(event) {
        return RouteDecision {
            outcome: RouteOutcome::Failed,
            reason_code: DiagnosticReasonCode::UnknownConnectorFailure
                .as_str()
                .to_string(),
            surface,
            normalized_text: String::new(),
        };
    }
    if is_unsupported_surface(&surface) {
        return RouteDecision {
            outcome: RouteOutcome::Unsupported,
            reason_code: DiagnosticReasonCode::UnsupportedCapability
                .as_str()
                .to_string(),
            surface,
            normalized_text: String::new(),
        };
    }
    if !workspace_id.trim().is_empty() && event.workspace_id.trim() != workspace_id.trim() {
        return RouteDecision {
            outcome: RouteOutcome::Blocked,
            reason_code: DiagnosticReasonCode::BlockedRoute.as_str().to_string(),
            surface,
            normalized_text: String::new(),
        };
    }
    let policy = normalize_route_policy(policy.clone(), event.received_at);
    match event.conversation_type {
        ConversationType::DirectMessage => {
            if allowed_dm_sender(event, &policy) {
                return RouteDecision {
                    outcome: RouteOutcome::Accepted,
                    reason_code: "accepted".to_string(),
                    surface,
                    normalized_text: event.text.trim().to_string(),
                };
            }
            RouteDecision {
                outcome: RouteOutcome::Blocked,
                reason_code: DiagnosticReasonCode::BlockedRoute.as_str().to_string(),
                surface,
                normalized_text: String::new(),
            }
        }
        ConversationType::Channel => {
            if !selected_channel(&event.conversation_id, &policy) {
                return RouteDecision {
                    outcome: RouteOutcome::Blocked,
                    reason_code: DiagnosticReasonCode::BlockedRoute.as_str().to_string(),
                    surface,
                    normalized_text: String::new(),
                };
            }
            let mut text = event.text.clone();
            let mut mentioned = event.mentioned;
            if !bot_user_id.trim().is_empty() {
                let normalized = crate::transport::normalize_mention_text(&text, bot_user_id);
                mentioned = mentioned || normalized != text.trim();
                text = normalized;
            }
            if !mentioned {
                return RouteDecision {
                    outcome: RouteOutcome::Ignored,
                    reason_code: "mention_required".to_string(),
                    surface,
                    normalized_text: String::new(),
                };
            }
            RouteDecision {
                outcome: RouteOutcome::Accepted,
                reason_code: "accepted".to_string(),
                surface,
                normalized_text: text.trim().to_string(),
            }
        }
    }
}

/// Go `missingSlackIdentity`.
#[must_use]
pub fn missing_slack_identity(event: &InboundEvent) -> bool {
    event.workspace_id.trim().is_empty()
        || event.conversation_id.trim().is_empty()
        || event.message_id.trim().is_empty()
}

/// Go `allowedDMSender`: direct messages require the sender (or one of the
/// sender's user groups) on the policy allowlist.
#[must_use]
pub fn allowed_dm_sender(event: &InboundEvent, policy: &RoutePolicy) -> bool {
    for id in &policy.allowed_dm_users {
        if !id.trim().is_empty() && id.trim() == event.sender_id.trim() {
            return true;
        }
    }
    let allowed_groups: HashSet<String> = policy
        .allowed_dm_user_groups
        .iter()
        .filter_map(|id| {
            let trimmed = id.trim();
            if trimmed.is_empty() {
                None
            } else {
                Some(trimmed.to_string())
            }
        })
        .collect();
    event
        .sender_user_group_ids
        .iter()
        .any(|id| allowed_groups.contains(id.trim()))
}

/// Go `selectedChannel`: the conversation is routable only when the policy
/// holds a selected, validated channel route for it.
#[must_use]
pub fn selected_channel(conversation_id: &str, policy: &RoutePolicy) -> bool {
    policy.selected_channels.iter().any(|channel| {
        channel.conversation_type == ConversationType::Channel
            && channel.conversation_id == conversation_id.trim()
            && channel.selected_channel_state == SelectedChannelState::Selected
            && channel.validation_state == RouteValidationState::Valid
    })
}

/// Go `isUnsupportedSurface`.
#[must_use]
pub fn is_unsupported_surface(surface: &str) -> bool {
    matches!(
        surface.trim(),
        "file"
            | "voice_clip"
            | "huddle"
            | "canvas"
            | "workflow_button"
            | "interactive_block"
            | "rich_media"
            | "thinking"
            | "incremental_update"
    )
}
