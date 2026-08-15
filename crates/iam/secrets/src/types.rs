//! Secret reference model and I/O types. Serde uses camelCase to match the
//! Go JSON tags; `omitempty` maps to `skip_serializing_if`.

use std::fmt;

use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};

use crate::redaction::redact_secret_value;

/// Free-form metadata document attached to a secret (Go `map[string]any`).
pub type Document = serde_json::Map<String, serde_json::Value>;

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum SecretStatus {
    Active,
    Disabled,
    PendingRemediation,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum SecretVersionStatus {
    Active,
    Superseded,
    Disabled,
    PendingRemediation,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum ResolutionStatus {
    Resolved,
    Unavailable,
    Denied,
    NotApplicable,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum AuditAction {
    #[serde(rename = "secret.create")]
    SecretCreate,
    #[serde(rename = "secret.update_metadata")]
    SecretUpdate,
    #[serde(rename = "secret.rotate")]
    SecretRotate,
    #[serde(rename = "secret.disable")]
    SecretDisable,
    #[serde(rename = "secret.use")]
    SecretUse,
    #[serde(rename = "credential.denied")]
    CredentialDenied,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum ResourceKind {
    TenantSecret,
    /// Go value is `"tenant_secret_version"`, not `"secret_version"`.
    #[serde(rename = "tenant_secret_version")]
    SecretVersion,
    Integration,
    ProviderAuthState,
    Connector,
    McpServer,
    McpTool,
    SandboxPolicy,
    DisabledCredential,
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct TenantSecret {
    pub secret_id: String,
    pub tenant_id: String,
    pub secret_ref: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub display_name: String,
    pub status: SecretStatus,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub active_version_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub disabled_reason: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub remediation_reason: String,
    pub created_at: DateTime<Utc>,
    pub updated_at: DateTime<Utc>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub rotated_at: Option<DateTime<Utc>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub disabled_at: Option<DateTime<Utc>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub document: Option<Document>,
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct SecretVersion {
    pub secret_version_id: String,
    pub secret_id: String,
    pub tenant_id: String,
    pub secret_ref: String,
    pub version_number: i64,
    pub status: SecretVersionStatus,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub value_backend_ref: String,
    pub created_at: DateTime<Utc>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub activated_at: Option<DateTime<Utc>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub superseded_at: Option<DateTime<Utc>>,
}

/// Output of `Manager::resolve`. The secret `value` is never serialized (Go
/// `json:"-"`) and never appears in `Debug` output.
#[derive(Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ResolvedSecret {
    pub tenant_id: String,
    pub secret_id: String,
    pub secret_ref: String,
    pub secret_version_id: String,
    pub resolution: ResolutionStatus,
    /// Go: `Value string \`json:"-"\``. Skipped in both directions.
    #[serde(skip)]
    pub value: String,
    pub resolved_at: DateTime<Utc>,
}

impl fmt::Debug for ResolvedSecret {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.debug_struct("ResolvedSecret")
            .field("tenant_id", &self.tenant_id)
            .field("secret_id", &self.secret_id)
            .field("secret_ref", &self.secret_ref)
            .field("secret_version_id", &self.secret_version_id)
            .field("resolution", &self.resolution)
            .field("value", &redact_secret_value(&self.value))
            .field("resolved_at", &self.resolved_at)
            .finish()
    }
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct DisabledResource {
    pub tenant_id: String,
    pub resource_kind: ResourceKind,
    pub resource_id: String,
    pub status: SecretStatus,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub disabled_reason: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub remediation_reason: String,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub secret_refs: Vec<String>,
    pub updated_at: DateTime<Utc>,
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct BridgedCredentialResource {
    pub tenant_id: String,
    pub resource_kind: ResourceKind,
    pub resource_id: String,
    /// Plain free-form status string in Go (not `SecretStatus`).
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub status: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub disabled_reason: String,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub secret_refs: Vec<String>,
    pub updated_at: DateTime<Utc>,
}

/// Input to [`crate::LegacyCredentialResourceStore`] (no JSON tags in Go).
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct LegacyCredentialResourceBridgeInput {
    pub tenant_id: String,
    pub active_secret_refs: Vec<String>,
    pub disabled_secret_refs: Vec<String>,
}

#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct LegacyCredentialResourceBridgeResult {
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub bridged: Vec<BridgedCredentialResource>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub disabled: Vec<BridgedCredentialResource>,
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct AuditRecord {
    pub tenant_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub principal_id: String,
    pub resource_kind: ResourceKind,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub resource_id: String,
    pub action: AuditAction,
    pub outcome: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub reason_code: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub secret_ref: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub secret_version_id: String,
    #[serde(default, skip_serializing_if = "is_zero")]
    pub secret_ref_count: i64,
    pub created_at: DateTime<Utc>,
}

fn is_zero(value: &i64) -> bool {
    *value == 0
}

/// `Manager::create` input. Carries the raw secret `value`; `Debug` redacts it.
#[derive(Clone, Default)]
pub struct CreateInput {
    pub tenant_id: String,
    pub secret_ref: String,
    pub display_name: String,
    pub value: String,
    pub document: Option<Document>,
}

impl fmt::Debug for CreateInput {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.debug_struct("CreateInput")
            .field("tenant_id", &self.tenant_id)
            .field("secret_ref", &self.secret_ref)
            .field("display_name", &self.display_name)
            .field("value", &redact_secret_value(&self.value))
            .field("document", &self.document)
            .finish()
    }
}

/// `Manager::update_metadata` input. `None` fields are left unchanged.
#[derive(Debug, Clone, Default)]
pub struct UpdateMetadataInput {
    pub tenant_id: String,
    pub secret_ref: String,
    pub display_name: Option<String>,
    pub document: Option<Document>,
}

/// `Manager::rotate` input. Carries the raw secret `value`; `Debug` redacts it.
#[derive(Clone, Default)]
pub struct RotateInput {
    pub tenant_id: String,
    pub secret_ref: String,
    pub value: String,
}

impl fmt::Debug for RotateInput {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.debug_struct("RotateInput")
            .field("tenant_id", &self.tenant_id)
            .field("secret_ref", &self.secret_ref)
            .field("value", &redact_secret_value(&self.value))
            .finish()
    }
}

#[derive(Debug, Clone, Default)]
pub struct DisableInput {
    pub tenant_id: String,
    pub secret_ref: String,
    pub disabled_reason: String,
}

#[derive(Debug, Clone, Default)]
pub struct CreateDisabledMetadataInput {
    pub tenant_id: String,
    pub secret_ref: String,
    pub display_name: String,
    pub disabled_reason: String,
    pub remediation_reason: String,
    pub document: Option<Document>,
}

#[derive(Debug, Clone, Default)]
pub struct ResolveInput {
    pub tenant_id: String,
    pub secret_ref: String,
}
