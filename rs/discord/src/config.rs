//! Discord connector configuration (port of runtime.go's Config) plus the
//! config-derived credential/destination evidence helpers.

use std::collections::HashMap;

use chrono::{DateTime, Utc};

use crate::destinations::{DestinationType, DestinationValidation, DestinationValidationState};
use crate::readiness::CredentialState;

/// Go `Config` from runtime.go. Plain in-memory struct (no JSON tags in Go).
#[derive(Debug, Clone, Default, PartialEq)]
pub struct Config {
    pub enabled: bool,
    pub connector_id: String,
    pub display_name: String,
    /// Delivery mode: empty defaults to "gateway"; only "gateway" is accepted.
    pub delivery_mode: String,
    pub bot_token: String,
    pub require_mention: bool,
    pub respond_in_dm: bool,
    pub allowed_guild_ids: Vec<String>,
    pub allowed_channel_ids: Vec<String>,
    /// Explicit tenant binding replacing Go's tenantctx-based context lookup
    /// (tenantctx.RuntimeTenantID is not ported; see rs/MIGRATION.md). Empty
    /// falls back to the store's default personal tenant at Start time.
    pub tenant_id: String,
}

/// Go `credentialStateForConfig`: a non-empty (trimmed) bot token is
/// considered a valid credential.
#[must_use]
pub fn credential_state_for_config(cfg: &Config) -> CredentialState {
    if cfg.bot_token.trim().is_empty() {
        CredentialState::Missing
    } else {
        CredentialState::Valid
    }
}

/// Go `destinationEvidenceForConfig`: projects the configured guild/channel
/// allow-lists into stale destination validations awaiting transport
/// validation.
#[must_use]
pub fn destination_evidence_for_config(
    cfg: &Config,
    now: DateTime<Utc>,
) -> Vec<DestinationValidation> {
    let mut destinations =
        Vec::with_capacity(cfg.allowed_guild_ids.len() + cfg.allowed_channel_ids.len());
    for guild_id in &cfg.allowed_guild_ids {
        let guild_id = guild_id.trim();
        if guild_id.is_empty() {
            continue;
        }
        destinations.push(DestinationValidation {
            connector_id: cfg.connector_id.clone(),
            destination_id: guild_id.to_string(),
            destination_type: DestinationType::Guild,
            selected: true,
            validation_state: DestinationValidationState::Stale,
            reason_code: "destination_validation_required".to_string(),
            validated_at: now,
            redaction_status: crate::redaction_status_redacted(),
            safe_evidence: HashMap::from([
                ("source".to_string(), "local_config_projection".to_string()),
                ("validation".to_string(), "required".to_string()),
            ]),
            ..DestinationValidation::default()
        });
    }
    for channel_id in &cfg.allowed_channel_ids {
        let channel_id = channel_id.trim();
        if channel_id.is_empty() {
            continue;
        }
        destinations.push(DestinationValidation {
            connector_id: cfg.connector_id.clone(),
            destination_id: channel_id.to_string(),
            destination_type: DestinationType::Channel,
            selected: true,
            validation_state: DestinationValidationState::Stale,
            reason_code: "destination_validation_required".to_string(),
            validated_at: now,
            redaction_status: crate::redaction_status_redacted(),
            safe_evidence: HashMap::from([
                ("source".to_string(), "local_config_projection".to_string()),
                ("validation".to_string(), "required".to_string()),
            ]),
            ..DestinationValidation::default()
        });
    }
    destinations
}
