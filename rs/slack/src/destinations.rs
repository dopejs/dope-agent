//! Slack route-policy destination types (port of destinations.go): conversation
//! routes, the route policy record, policy normalization, and readiness
//! evaluation.

use std::collections::HashMap;

use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};

use dope_connectors::RedactionStatus;

use crate::util::{first_non_empty, is_unset_time};

/// Slack conversation kind (Go `ConversationType`). The `Channel` default
/// matches Go's policy normalization (empty type -> channel).
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Default, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum ConversationType {
    #[default]
    Channel,
    DirectMessage,
}

impl ConversationType {
    #[must_use]
    pub fn as_str(self) -> &'static str {
        match self {
            ConversationType::DirectMessage => "direct_message",
            ConversationType::Channel => "channel",
        }
    }
}

impl std::fmt::Display for ConversationType {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_str(self.as_str())
    }
}

/// Selected-channel state for a conversation route (Go `SelectedChannelState`).
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Default, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum SelectedChannelState {
    Selected,
    #[default]
    NotSelected,
    Stale,
    Archived,
    MissingMembership,
    NotApplicable,
}

impl SelectedChannelState {
    #[must_use]
    pub fn as_str(self) -> &'static str {
        match self {
            SelectedChannelState::Selected => "selected",
            SelectedChannelState::NotSelected => "not_selected",
            SelectedChannelState::Stale => "stale",
            SelectedChannelState::Archived => "archived",
            SelectedChannelState::MissingMembership => "missing_membership",
            SelectedChannelState::NotApplicable => "not_applicable",
        }
    }
}

impl std::fmt::Display for SelectedChannelState {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_str(self.as_str())
    }
}

/// Route-policy validation state (Go `RouteValidationState`). The `Blocked`
/// default matches Go's policy normalization (empty state -> blocked).
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Default, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum RouteValidationState {
    Valid,
    Partial,
    Stale,
    #[default]
    Blocked,
    MissingPermission,
}

impl RouteValidationState {
    #[must_use]
    pub fn as_str(self) -> &'static str {
        match self {
            RouteValidationState::Valid => "valid",
            RouteValidationState::Partial => "partial",
            RouteValidationState::Stale => "stale",
            RouteValidationState::Blocked => "blocked",
            RouteValidationState::MissingPermission => "missing_permission",
        }
    }
}

impl std::fmt::Display for RouteValidationState {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_str(self.as_str())
    }
}

/// One selected-channel route inside a route policy (Go `ConversationRoute`).
#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ConversationRoute {
    pub conversation_id: String,
    pub conversation_type: ConversationType,
    pub selected_channel_state: SelectedChannelState,
    pub validation_state: RouteValidationState,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub reason_code: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub redaction_status: Option<RedactionStatus>,
    #[serde(default, skip_serializing_if = "HashMap::is_empty")]
    pub safe_evidence: HashMap<String, String>,
}

/// Slack route policy (Go `RoutePolicy`): DM allowment + selected channels
/// with their validation states.
#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct RoutePolicy {
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub tenant_id: String,
    pub connector_id: String,
    pub workspace_binding_id: String,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub selected_channels: Vec<ConversationRoute>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub allowed_dm_users: Vec<String>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub allowed_dm_user_groups: Vec<String>,
    pub mention_gate: String,
    pub thread_reply_mode: String,
    pub validation_state: RouteValidationState,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub reason_code: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub created_at: Option<DateTime<Utc>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub updated_at: Option<DateTime<Utc>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub validated_at: Option<DateTime<Utc>>,
    pub redaction_status: RedactionStatus,
    #[serde(default, skip_serializing_if = "HashMap::is_empty")]
    pub safe_evidence: HashMap<String, String>,
}

/// Go `NormalizeRoutePolicy`: trims identity fields, fills defaults, and
/// stamps timestamps with the given now.
#[must_use]
pub fn normalize_route_policy(mut policy: RoutePolicy, now: DateTime<Utc>) -> RoutePolicy {
    let now = if is_unset_time(&now) { Utc::now() } else { now };
    policy.tenant_id = policy.tenant_id.trim().to_string();
    policy.connector_id = policy.connector_id.trim().to_string();
    policy.workspace_binding_id = policy.workspace_binding_id.trim().to_string();
    policy.mention_gate = first_non_empty(&[&policy.mention_gate, "agent_mention_required"]);
    policy.thread_reply_mode =
        first_non_empty(&[&policy.thread_reply_mode, "channel_mentions_thread_rooted"]);
    if policy.validation_state == RouteValidationState::default() {
        policy.validation_state = RouteValidationState::Blocked;
    }
    if policy.redaction_status == RedactionStatus::default() {
        policy.redaction_status = RedactionStatus::Redacted;
    }
    if policy.created_at.is_none() {
        policy.created_at = Some(now);
    }
    if policy.updated_at.is_none() {
        policy.updated_at = Some(now);
    }
    if policy.validated_at.is_none() {
        policy.validated_at = Some(now);
    }
    for route in &mut policy.selected_channels {
        route.conversation_id = route.conversation_id.trim().to_string();
        if route.conversation_type == ConversationType::default() {
            route.conversation_type = ConversationType::Channel;
        }
        if route.selected_channel_state == SelectedChannelState::default() {
            route.selected_channel_state = SelectedChannelState::NotSelected;
        }
        if route.validation_state == RouteValidationState::default() {
            route.validation_state = RouteValidationState::Blocked;
        }
        if route.redaction_status.is_none() {
            route.redaction_status = Some(RedactionStatus::Redacted);
        }
    }
    policy
}

/// Go `HasReadyRoutePolicy`: a valid policy with any DM allowment or a
/// selected, validated channel route is delivery-ready.
#[must_use]
pub fn has_ready_route_policy(policy: &RoutePolicy) -> bool {
    let policy = normalize_route_policy(policy.clone(), DateTime::<Utc>::default());
    if policy.validation_state != RouteValidationState::Valid {
        return false;
    }
    if has_allowed_dm(&policy) {
        return true;
    }
    policy.selected_channels.iter().any(|channel| {
        channel.conversation_type == ConversationType::Channel
            && !channel.conversation_id.is_empty()
            && channel.selected_channel_state == SelectedChannelState::Selected
            && channel.validation_state == RouteValidationState::Valid
    })
}

/// Go `hasAllowedDM`.
#[must_use]
fn has_allowed_dm(policy: &RoutePolicy) -> bool {
    policy
        .allowed_dm_users
        .iter()
        .any(|id| !id.trim().is_empty())
        || policy
            .allowed_dm_user_groups
            .iter()
            .any(|id| !id.trim().is_empty())
}
