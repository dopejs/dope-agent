//! Destination validation types (port of destinations.go): the destination
//! kind, validation state, and the per-destination validation record with
//! redacted evidence.

use std::collections::HashMap;

use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};

/// Go `DestinationType`.
#[derive(Debug, Clone, Copy, Default, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum DestinationType {
    #[default]
    Guild,
    Channel,
    DirectMessage,
}

impl DestinationType {
    #[must_use]
    pub fn as_str(self) -> &'static str {
        match self {
            DestinationType::Guild => "guild",
            DestinationType::Channel => "channel",
            DestinationType::DirectMessage => "direct_message",
        }
    }
}

impl std::fmt::Display for DestinationType {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_str(self.as_str())
    }
}

/// Go `DestinationValidationState`.
#[derive(Debug, Clone, Copy, Default, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum DestinationValidationState {
    /// Default matches Go's empty-string zero value, which normalizes to
    /// `invalid` (readiness.normalizeDestinationEvidence).
    #[default]
    Invalid,
    Valid,
    MissingPermission,
    MessageContentMissing,
    BotNotMember,
    NotFound,
    DmRestricted,
    Stale,
}

impl DestinationValidationState {
    #[must_use]
    pub fn as_str(self) -> &'static str {
        match self {
            DestinationValidationState::Invalid => "invalid",
            DestinationValidationState::Valid => "valid",
            DestinationValidationState::MissingPermission => "missing_permission",
            DestinationValidationState::MessageContentMissing => "message_content_missing",
            DestinationValidationState::BotNotMember => "bot_not_member",
            DestinationValidationState::NotFound => "not_found",
            DestinationValidationState::DmRestricted => "dm_restricted",
            DestinationValidationState::Stale => "stale",
        }
    }
}

impl std::fmt::Display for DestinationValidationState {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_str(self.as_str())
    }
}

/// Go `DestinationValidation`. Wire shape matches the Go json tags exactly.
#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct DestinationValidation {
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub tenant_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub connector_id: String,
    pub destination_id: String,
    pub destination_type: DestinationType,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub provider_label: String,
    pub selected: bool,
    pub validation_state: DestinationValidationState,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub reason_code: String,
    pub validated_at: DateTime<Utc>,
    pub redaction_status: crate::RedactionStatus,
    #[serde(default, skip_serializing_if = "HashMap::is_empty")]
    pub safe_evidence: HashMap<String, String>,
}

/// Go `hasExplicitHostedDestination`: any selected guild/channel destination
/// with a non-empty id counts as explicit.
#[must_use]
pub fn has_explicit_hosted_destination(destinations: &[DestinationValidation]) -> bool {
    for destination in destinations {
        if !destination.selected {
            continue;
        }
        match destination.destination_type {
            DestinationType::Guild | DestinationType::Channel => {
                if !destination.destination_id.trim().is_empty() {
                    return true;
                }
            }
            DestinationType::DirectMessage => {}
        }
    }
    false
}

/// Go `selectedDestinationsValid`: every selected destination must be valid;
/// an empty destination list is never valid.
#[must_use]
pub fn selected_destinations_valid(destinations: &[DestinationValidation]) -> bool {
    if destinations.is_empty() {
        return false;
    }
    for destination in destinations {
        if !destination.selected {
            continue;
        }
        if destination.validation_state != DestinationValidationState::Valid {
            return false;
        }
    }
    true
}
