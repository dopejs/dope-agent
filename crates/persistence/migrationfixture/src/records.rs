//! JSON document shapes for the r49-r51 channel-connector records.
//!
//! These mirror the Go record structs in daemon/internal/store/{discord,telegram,slack}_setup.go
//! byte-for-byte (field order, camelCase renames, omitempty behavior) so the
//! document_json written by the fixture matches what the Go Save* accessors
//! would have stored. The kura-store crate does not yet port those accessors,
//! so the fixture writes the rows directly and reuses these shapes.

use std::collections::BTreeMap;

use serde::Serialize;

/// Go DiscordHostedSetupRecord (json tags: tenantId omitempty ... destinations omitempty).
#[derive(Debug, Clone, Default, Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct DiscordHostedSetupDocument {
    #[serde(skip_serializing_if = "String::is_empty")]
    pub tenant_id: String,
    pub connector_id: String,
    pub connector_kind: String,
    pub display_name: String,
    pub status: String,
    pub readiness_state: String,
    pub hosted_ready: bool,
    pub credential_state: String,
    #[serde(rename = "respondInDM")]
    pub respond_in_dm: bool,
    pub require_mention: bool,
    pub delivery_mode: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    pub reason_code: String,
    pub redaction_status: String,
    pub created_at: String,
    pub updated_at: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    pub validated_at: String,
    pub retention_expires_at: String,
    #[serde(skip_serializing_if = "Vec::is_empty")]
    pub destinations: Vec<DiscordDestinationValidationDocument>,
}

/// Go DiscordDestinationValidationRecord.
#[derive(Debug, Clone, Default, Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct DiscordDestinationValidationDocument {
    #[serde(skip_serializing_if = "String::is_empty")]
    pub tenant_id: String,
    pub connector_id: String,
    pub destination_id: String,
    pub destination_type: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    pub provider_label: String,
    pub selected: bool,
    pub validation_state: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    pub reason_code: String,
    pub validated_at: String,
    pub redaction_status: String,
    #[serde(skip_serializing_if = "BTreeMap::is_empty")]
    pub safe_evidence: BTreeMap<String, String>,
}

/// Go TelegramHostedSetupRecord.
#[derive(Debug, Clone, Default, Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct TelegramHostedSetupDocument {
    #[serde(skip_serializing_if = "String::is_empty")]
    pub tenant_id: String,
    pub connector_id: String,
    pub connector_kind: String,
    pub display_name: String,
    pub status: String,
    pub terminal_state: String,
    pub hosted_ready: bool,
    pub credential_state: String,
    pub allowment_state: String,
    pub group_behavior: String,
    pub delivery_eligible: bool,
    #[serde(skip_serializing_if = "String::is_empty")]
    pub reason_code: String,
    pub redaction_status: String,
    pub created_at: String,
    pub updated_at: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    pub validated_at: String,
    pub retention_expires_at: String,
}

/// Go TelegramAllowmentRecord (scope fields keep their telegram* json names).
#[derive(Debug, Clone, Default, Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct TelegramAllowmentDocument {
    #[serde(skip_serializing_if = "String::is_empty")]
    pub tenant_id: String,
    pub connector_id: String,
    pub allowment_id: String,
    #[serde(rename = "telegramScopeType")]
    pub scope_type: String,
    #[serde(rename = "telegramScopeId")]
    pub scope_id: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    pub provider_label: String,
    pub enabled: bool,
    pub group_gate: String,
    pub validation_state: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    pub reason_code: String,
    pub validated_at: String,
    pub redaction_status: String,
    #[serde(skip_serializing_if = "BTreeMap::is_empty")]
    pub safe_evidence: BTreeMap<String, String>,
}

/// Go TelegramSmokeEvidenceRecord.
#[derive(Debug, Clone, Default, Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct TelegramSmokeEvidenceDocument {
    pub smoke_evidence_id: String,
    pub tenant_id: String,
    pub connector_id: String,
    pub status: String,
    pub credential_mode: String,
    pub owner: String,
    pub reason: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    pub remaining_risk: String,
    pub validated_at: String,
    pub retention_expires_at: String,
    pub redaction_status: String,
    #[serde(skip_serializing_if = "BTreeMap::is_empty")]
    pub safe_evidence: BTreeMap<String, String>,
}

/// Go TelegramUpdateEvidenceRecord (chat/message/update keep their telegram* json names).
#[derive(Debug, Clone, Default, Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct TelegramUpdateEvidenceDocument {
    pub tenant_id: String,
    pub connector_id: String,
    #[serde(rename = "telegramChatId")]
    pub chat_id: String,
    #[serde(rename = "telegramMessageId")]
    pub message_id: String,
    #[serde(rename = "telegramUpdateId")]
    pub update_id: String,
    pub route_outcome: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    pub reason_code: String,
    pub received_at: String,
    pub retention_expires_at: String,
    pub redaction_status: String,
    #[serde(skip_serializing_if = "BTreeMap::is_empty")]
    pub safe_evidence: BTreeMap<String, String>,
}

/// Go SlackHostedSetupRecord.
#[derive(Debug, Clone, Default, Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct SlackHostedSetupDocument {
    #[serde(skip_serializing_if = "String::is_empty")]
    pub tenant_id: String,
    pub connector_id: String,
    pub connector_kind: String,
    pub display_name: String,
    pub status: String,
    pub terminal_state: String,
    pub oauth_state: String,
    pub route_policy_state: String,
    pub delivery_eligible: bool,
    pub workspace_binding_id: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    pub reason_code: String,
    pub redaction_status: String,
    pub created_at: String,
    pub updated_at: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    pub validated_at: String,
    pub retention_expires_at: String,
}

/// Go SlackConversationRouteRecord.
#[derive(Debug, Clone, Default, Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct SlackConversationRouteDocument {
    pub conversation_id: String,
    pub conversation_type: String,
    pub selected_channel_state: String,
    pub validation_state: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    pub reason_code: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    pub redaction_status: String,
    #[serde(skip_serializing_if = "BTreeMap::is_empty")]
    pub safe_evidence: BTreeMap<String, String>,
}

/// Go SlackRoutePolicyRecord (selectedChannels/allowedDMUsers/allowedDMUserGroups
/// have no omitempty and are always serialized).
#[derive(Debug, Clone, Default, Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct SlackRoutePolicyDocument {
    #[serde(skip_serializing_if = "String::is_empty")]
    pub tenant_id: String,
    pub connector_id: String,
    pub workspace_binding_id: String,
    pub selected_channels: Vec<SlackConversationRouteDocument>,
    #[serde(rename = "allowedDMUsers")]
    pub allowed_dm_users: Vec<String>,
    #[serde(rename = "allowedDMUserGroups")]
    pub allowed_dm_user_groups: Vec<String>,
    pub mention_gate: String,
    pub thread_reply_mode: String,
    pub validation_state: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    pub reason_code: String,
    pub validated_at: String,
    pub redaction_status: String,
    #[serde(skip_serializing_if = "BTreeMap::is_empty")]
    pub safe_evidence: BTreeMap<String, String>,
}

/// Go SlackSmokeEvidenceRecord.
#[derive(Debug, Clone, Default, Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct SlackSmokeEvidenceDocument {
    pub smoke_evidence_id: String,
    pub tenant_id: String,
    pub connector_id: String,
    pub workspace_binding_id: String,
    pub status: String,
    pub authorization_mode: String,
    pub owner: String,
    pub reason: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    pub remaining_risk: String,
    pub validated_at: String,
    pub retention_expires_at: String,
    pub redaction_status: String,
    #[serde(skip_serializing_if = "BTreeMap::is_empty")]
    pub safe_evidence: BTreeMap<String, String>,
}

/// Go SlackEventEvidenceRecord.
#[derive(Debug, Clone, Default, Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct SlackEventEvidenceDocument {
    pub tenant_id: String,
    pub connector_id: String,
    pub workspace_id: String,
    pub conversation_id: String,
    pub message_id: String,
    pub event_id: String,
    pub route_outcome: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    pub reason_code: String,
    pub received_at: String,
    pub retention_expires_at: String,
    pub redaction_status: String,
    #[serde(skip_serializing_if = "BTreeMap::is_empty")]
    pub safe_evidence: BTreeMap<String, String>,
}

pub(crate) fn discord_hosted_setup_document(
    tenant_id: &str,
    connector_id: &str,
    connector_kind: &str,
    display_name: &str,
    status: &str,
    readiness_state: &str,
    credential_state: &str,
    respond_in_dm: bool,
    require_mention: bool,
    delivery_mode: &str,
    reason_code: &str,
    redaction_status: &str,
    created_at: &str,
    updated_at: &str,
    validated_at: &str,
    retention_expires_at: &str,
) -> Result<String, String> {
    // Go normalizeDiscordHostedSetupRecord derives HostedReady from ReadinessState.
    let doc = DiscordHostedSetupDocument {
        tenant_id: tenant_id.to_string(),
        connector_id: connector_id.to_string(),
        connector_kind: connector_kind.to_string(),
        display_name: display_name.to_string(),
        status: status.to_string(),
        readiness_state: readiness_state.to_string(),
        hosted_ready: readiness_state == "hosted_ready",
        credential_state: credential_state.to_string(),
        respond_in_dm,
        require_mention,
        delivery_mode: delivery_mode.to_string(),
        reason_code: reason_code.to_string(),
        redaction_status: redaction_status.to_string(),
        created_at: created_at.to_string(),
        updated_at: updated_at.to_string(),
        validated_at: validated_at.to_string(),
        retention_expires_at: retention_expires_at.to_string(),
        destinations: Vec::new(),
    };
    serde_json::to_string(&doc).map_err(|e| format!("marshal discord hosted setup: {e}"))
}

pub(crate) fn discord_destination_validation_document(
    tenant_id: &str,
    connector_id: &str,
    destination_id: &str,
    destination_type: &str,
    selected: bool,
    validation_state: &str,
    reason_code: &str,
    validated_at: &str,
    redaction_status: &str,
    safe_evidence: &[(&str, &str)],
) -> Result<String, String> {
    let mut evidence = BTreeMap::new();
    for (key, value) in safe_evidence {
        evidence.insert((*key).to_string(), (*value).to_string());
    }
    let doc = DiscordDestinationValidationDocument {
        tenant_id: tenant_id.to_string(),
        connector_id: connector_id.to_string(),
        destination_id: destination_id.to_string(),
        destination_type: destination_type.to_string(),
        provider_label: String::new(),
        selected,
        validation_state: validation_state.to_string(),
        reason_code: reason_code.to_string(),
        validated_at: validated_at.to_string(),
        redaction_status: redaction_status.to_string(),
        safe_evidence: evidence,
    };
    serde_json::to_string(&doc).map_err(|e| format!("marshal discord destination validation: {e}"))
}

pub(crate) fn telegram_hosted_setup_document(
    tenant_id: &str,
    connector_id: &str,
    connector_kind: &str,
    display_name: &str,
    status: &str,
    terminal_state: &str,
    credential_state: &str,
    allowment_state: &str,
    group_behavior: &str,
    reason_code: &str,
    redaction_status: &str,
    created_at: &str,
    updated_at: &str,
    validated_at: &str,
    retention_expires_at: &str,
) -> Result<String, String> {
    // Go normalizeTelegramHostedSetupRecord derives HostedReady from TerminalState.
    let doc = TelegramHostedSetupDocument {
        tenant_id: tenant_id.to_string(),
        connector_id: connector_id.to_string(),
        connector_kind: connector_kind.to_string(),
        display_name: display_name.to_string(),
        status: status.to_string(),
        terminal_state: terminal_state.to_string(),
        hosted_ready: terminal_state == "ready",
        credential_state: credential_state.to_string(),
        allowment_state: allowment_state.to_string(),
        group_behavior: group_behavior.to_string(),
        delivery_eligible: false,
        reason_code: reason_code.to_string(),
        redaction_status: redaction_status.to_string(),
        created_at: created_at.to_string(),
        updated_at: updated_at.to_string(),
        validated_at: validated_at.to_string(),
        retention_expires_at: retention_expires_at.to_string(),
    };
    serde_json::to_string(&doc).map_err(|e| format!("marshal telegram hosted setup: {e}"))
}

pub(crate) fn telegram_allowment_document(
    tenant_id: &str,
    connector_id: &str,
    allowment_id: &str,
    scope_type: &str,
    scope_id: &str,
    enabled: bool,
    group_gate: &str,
    validation_state: &str,
    reason_code: &str,
    validated_at: &str,
    redaction_status: &str,
    safe_evidence: &[(&str, &str)],
) -> Result<String, String> {
    let mut evidence = BTreeMap::new();
    for (key, value) in safe_evidence {
        evidence.insert((*key).to_string(), (*value).to_string());
    }
    let doc = TelegramAllowmentDocument {
        tenant_id: tenant_id.to_string(),
        connector_id: connector_id.to_string(),
        allowment_id: allowment_id.to_string(),
        scope_type: scope_type.to_string(),
        scope_id: scope_id.to_string(),
        provider_label: String::new(),
        enabled,
        group_gate: group_gate.to_string(),
        validation_state: validation_state.to_string(),
        reason_code: reason_code.to_string(),
        validated_at: validated_at.to_string(),
        redaction_status: redaction_status.to_string(),
        safe_evidence: evidence,
    };
    serde_json::to_string(&doc).map_err(|e| format!("marshal telegram allowment: {e}"))
}

pub(crate) fn telegram_smoke_evidence_document(
    smoke_evidence_id: &str,
    tenant_id: &str,
    connector_id: &str,
    status: &str,
    credential_mode: &str,
    owner: &str,
    reason: &str,
    remaining_risk: &str,
    validated_at: &str,
    retention_expires_at: &str,
    redaction_status: &str,
    safe_evidence: &[(&str, &str)],
) -> Result<String, String> {
    let mut evidence = BTreeMap::new();
    for (key, value) in safe_evidence {
        evidence.insert((*key).to_string(), (*value).to_string());
    }
    let doc = TelegramSmokeEvidenceDocument {
        smoke_evidence_id: smoke_evidence_id.to_string(),
        tenant_id: tenant_id.to_string(),
        connector_id: connector_id.to_string(),
        status: status.to_string(),
        credential_mode: credential_mode.to_string(),
        owner: owner.to_string(),
        reason: reason.to_string(),
        remaining_risk: remaining_risk.to_string(),
        validated_at: validated_at.to_string(),
        retention_expires_at: retention_expires_at.to_string(),
        redaction_status: redaction_status.to_string(),
        safe_evidence: evidence,
    };
    serde_json::to_string(&doc).map_err(|e| format!("marshal telegram smoke evidence: {e}"))
}

pub(crate) fn telegram_update_evidence_document(
    tenant_id: &str,
    connector_id: &str,
    chat_id: &str,
    message_id: &str,
    update_id: &str,
    route_outcome: &str,
    reason_code: &str,
    received_at: &str,
    retention_expires_at: &str,
    redaction_status: &str,
    safe_evidence: &[(&str, &str)],
) -> Result<String, String> {
    let mut evidence = BTreeMap::new();
    for (key, value) in safe_evidence {
        evidence.insert((*key).to_string(), (*value).to_string());
    }
    let doc = TelegramUpdateEvidenceDocument {
        tenant_id: tenant_id.to_string(),
        connector_id: connector_id.to_string(),
        chat_id: chat_id.to_string(),
        message_id: message_id.to_string(),
        update_id: update_id.to_string(),
        route_outcome: route_outcome.to_string(),
        reason_code: reason_code.to_string(),
        received_at: received_at.to_string(),
        retention_expires_at: retention_expires_at.to_string(),
        redaction_status: redaction_status.to_string(),
        safe_evidence: evidence,
    };
    serde_json::to_string(&doc).map_err(|e| format!("marshal telegram update evidence: {e}"))
}

pub(crate) fn slack_hosted_setup_document(
    tenant_id: &str,
    connector_id: &str,
    connector_kind: &str,
    display_name: &str,
    status: &str,
    terminal_state: &str,
    oauth_state: &str,
    route_policy_state: &str,
    workspace_binding_id: &str,
    reason_code: &str,
    redaction_status: &str,
    created_at: &str,
    updated_at: &str,
    validated_at: &str,
    retention_expires_at: &str,
) -> Result<String, String> {
    let doc = SlackHostedSetupDocument {
        tenant_id: tenant_id.to_string(),
        connector_id: connector_id.to_string(),
        connector_kind: connector_kind.to_string(),
        display_name: display_name.to_string(),
        status: status.to_string(),
        terminal_state: terminal_state.to_string(),
        oauth_state: oauth_state.to_string(),
        route_policy_state: route_policy_state.to_string(),
        delivery_eligible: false,
        workspace_binding_id: workspace_binding_id.to_string(),
        reason_code: reason_code.to_string(),
        redaction_status: redaction_status.to_string(),
        created_at: created_at.to_string(),
        updated_at: updated_at.to_string(),
        validated_at: validated_at.to_string(),
        retention_expires_at: retention_expires_at.to_string(),
    };
    serde_json::to_string(&doc).map_err(|e| format!("marshal slack hosted setup: {e}"))
}

pub(crate) fn slack_route_policy_document(
    tenant_id: &str,
    connector_id: &str,
    workspace_binding_id: &str,
    selected_channels: Vec<SlackConversationRouteDocument>,
    allowed_dm_users: Vec<String>,
    allowed_dm_user_groups: Vec<String>,
    mention_gate: &str,
    thread_reply_mode: &str,
    validation_state: &str,
    reason_code: &str,
    validated_at: &str,
    redaction_status: &str,
    safe_evidence: &[(&str, &str)],
) -> Result<String, String> {
    let mut evidence = BTreeMap::new();
    for (key, value) in safe_evidence {
        evidence.insert((*key).to_string(), (*value).to_string());
    }
    let doc = SlackRoutePolicyDocument {
        tenant_id: tenant_id.to_string(),
        connector_id: connector_id.to_string(),
        workspace_binding_id: workspace_binding_id.to_string(),
        selected_channels,
        allowed_dm_users,
        allowed_dm_user_groups,
        mention_gate: mention_gate.to_string(),
        thread_reply_mode: thread_reply_mode.to_string(),
        validation_state: validation_state.to_string(),
        reason_code: reason_code.to_string(),
        validated_at: validated_at.to_string(),
        redaction_status: redaction_status.to_string(),
        safe_evidence: evidence,
    };
    serde_json::to_string(&doc).map_err(|e| format!("marshal slack route policy: {e}"))
}

pub(crate) fn slack_smoke_evidence_document(
    smoke_evidence_id: &str,
    tenant_id: &str,
    connector_id: &str,
    workspace_binding_id: &str,
    status: &str,
    authorization_mode: &str,
    owner: &str,
    reason: &str,
    remaining_risk: &str,
    validated_at: &str,
    retention_expires_at: &str,
    redaction_status: &str,
    safe_evidence: &[(&str, &str)],
) -> Result<String, String> {
    let mut evidence = BTreeMap::new();
    for (key, value) in safe_evidence {
        evidence.insert((*key).to_string(), (*value).to_string());
    }
    let doc = SlackSmokeEvidenceDocument {
        smoke_evidence_id: smoke_evidence_id.to_string(),
        tenant_id: tenant_id.to_string(),
        connector_id: connector_id.to_string(),
        workspace_binding_id: workspace_binding_id.to_string(),
        status: status.to_string(),
        authorization_mode: authorization_mode.to_string(),
        owner: owner.to_string(),
        reason: reason.to_string(),
        remaining_risk: remaining_risk.to_string(),
        validated_at: validated_at.to_string(),
        retention_expires_at: retention_expires_at.to_string(),
        redaction_status: redaction_status.to_string(),
        safe_evidence: evidence,
    };
    serde_json::to_string(&doc).map_err(|e| format!("marshal slack smoke evidence: {e}"))
}

pub(crate) fn slack_event_evidence_document(
    tenant_id: &str,
    connector_id: &str,
    workspace_id: &str,
    conversation_id: &str,
    message_id: &str,
    event_id: &str,
    route_outcome: &str,
    reason_code: &str,
    received_at: &str,
    retention_expires_at: &str,
    redaction_status: &str,
    safe_evidence: &[(&str, &str)],
) -> Result<String, String> {
    let mut evidence = BTreeMap::new();
    for (key, value) in safe_evidence {
        evidence.insert((*key).to_string(), (*value).to_string());
    }
    let doc = SlackEventEvidenceDocument {
        tenant_id: tenant_id.to_string(),
        connector_id: connector_id.to_string(),
        workspace_id: workspace_id.to_string(),
        conversation_id: conversation_id.to_string(),
        message_id: message_id.to_string(),
        event_id: event_id.to_string(),
        route_outcome: route_outcome.to_string(),
        reason_code: reason_code.to_string(),
        received_at: received_at.to_string(),
        retention_expires_at: retention_expires_at.to_string(),
        redaction_status: redaction_status.to_string(),
        safe_evidence: evidence,
    };
    serde_json::to_string(&doc).map_err(|e| format!("marshal slack event evidence: {e}"))
}
