//! Domain types for the bindings package (port of binding.go).
//!
//! Go models these vocabularies as typed strings with package-level constants;
//! validation must be able to *represent* unknown values in order to reject them
//! with safe reason codes (FR-009). We therefore keep the same shape in Rust:
//! transparent string newtypes with associated constants, rather than closed enums.

use std::borrow::Cow;

use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};

/// Defines a transparent string-backed vocabulary type with associated constants,
/// mirroring Go's `type X string` + `const (...)` blocks. Unknown values remain
/// representable so policy validation can reject them.
macro_rules! string_enum {
    ($(#[$meta:meta])* $name:ident { $($(#[$vmeta:meta])* $const:ident = $val:literal),* $(,)? }) => {
        $(#[$meta])*
        #[derive(Debug, Clone, PartialEq, Eq, Hash, Default, Serialize, Deserialize)]
        #[serde(transparent)]
        pub struct $name(pub Cow<'static, str>);

        impl $name {
            $($(#[$vmeta])* pub const $const: Self = Self(Cow::Borrowed($val));)*

            /// Builds a value from an arbitrary (possibly unknown) string.
            pub fn new(value: impl Into<String>) -> Self {
                Self(Cow::Owned(value.into()))
            }

            pub fn as_str(&self) -> &str {
                &self.0
            }

            pub fn is_empty(&self) -> bool {
                self.0.is_empty()
            }
        }

        impl From<&str> for $name {
            fn from(value: &str) -> Self {
                Self::new(value)
            }
        }

        impl From<String> for $name {
            fn from(value: String) -> Self {
                Self::new(value)
            }
        }

        impl std::fmt::Display for $name {
            fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                f.write_str(&self.0)
            }
        }
    };
}

string_enum! {
    /// ScopeKind identifies what a binding rule applies to.
    ScopeKind {
        CHANNEL = "channel",
        INTEGRATION_ACCOUNT = "integration_account",
    }
}

string_enum! {
    /// WorkspaceStatus is the lifecycle state of a workspace record.
    WorkspaceStatus {
        ACTIVE = "active",
        ARCHIVED = "archived",
        DISABLED = "disabled",
    }
}

string_enum! {
    /// BindingStatus is the lifecycle state of a binding rule.
    BindingStatus {
        ACTIVE = "active",
        DISABLED = "disabled",
    }
}

string_enum! {
    /// RepairStatus is the safe, user-facing health of a workspace or binding rule.
    RepairStatus {
        HEALTHY = "healthy",
        DISABLED = "disabled",
        INVALID = "invalid",
        STALE = "stale",
        UNSUPPORTED = "unsupported",
        NEEDS_REPAIR = "needs_repair",
    }
}

string_enum! {
    /// ValidationStatus marks whether stored state passed validation.
    ValidationStatus {
        VALID = "valid",
        INVALID = "invalid",
    }
}

string_enum! {
    /// RedactionStatus mirrors the profile domain redaction vocabulary.
    RedactionStatus {
        REDACTED = "redacted",
        NOT_REQUIRED = "not_required",
        SUPPRESSED = "suppressed",
        FAILED = "redaction_failed",
    }
}

string_enum! {
    /// Visibility is the per-capability, per-scope policy value an authorized user sets.
    Visibility {
        VISIBLE = "visible",
        HIDDEN = "hidden",
        DISABLED = "disabled",
        DEFAULT_ENABLED = "default_enabled",
    }
}

string_enum! {
    /// VisibilityScopeKind is the scope a capability visibility policy is attached to.
    /// In this phase only profile and workspace scopes are user-editable; tenant and
    /// connector limits are enforced as higher-level constraints (see visibility.rs).
    VisibilityScopeKind {
        PROFILE = "profile",
        WORKSPACE = "workspace",
    }
}

string_enum! {
    /// ResolutionOutcome is the result of resolving an effective binding selection at
    /// work-start.
    ResolutionOutcome {
        /// An explicit binding produced a valid selection.
        RESOLVED = "resolved",
        /// No explicit binding applied; tenant defaults were used.
        DEFAULT = "default",
        /// The selected profile/workspace is invalid and the system must fail closed
        /// (FR-031): new work is blocked, no silent substitution.
        REPAIR_REQUIRED = "repair_required",
    }
}

string_enum! {
    /// Classification labels runtime binding evidence so legacy/default behavior is never
    /// presented as explicit user-configured binding (FR-026). It supersedes the planted
    /// profile-projection marker "roadmap_58_deferred_binding_unapplied".
    Classification {
        APPLIED = "applied_binding",
        DEFAULT = "default_binding",
        LEGACY = "legacy_default",
    }
}

string_enum! {
    /// BindingRuntimeScope identifies which binding precedence level influenced a run.
    BindingRuntimeScope {
        CHANNEL = "channel",
        INTEGRATION_ACCOUNT = "integration_account",
        TENANT_DEFAULT = "tenant_default",
    }
}

string_enum! {
    /// EffectiveVisibility is the resolved visibility of a capability after combining all
    /// applicable scopes. It is distinct from Visibility (the per-scope user input) because
    /// "default_enabled" collapses to visible+offered and a higher-level prohibition
    /// surfaces as "blocked".
    EffectiveVisibility {
        VISIBLE = "visible",
        HIDDEN = "hidden",
        DISABLED = "disabled",
        /// A higher-level tenant/connector limit prohibits the capability regardless
        /// of profile/workspace policy.
        BLOCKED = "blocked",
    }
}

/// Workspace is a tenant-scoped persisted product record used for binding identity,
/// safe display, status, audit, and repair. It grants no storage or filesystem access
/// by itself (FR-002, FR-020).
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Workspace {
    pub workspace_id: String,
    pub tenant_id: String,
    pub display_name: String,
    pub status: WorkspaceStatus,
    pub is_default: bool,
    pub owner_principal_id: String,
    pub repair_status: RepairStatus,
    pub redaction_status: RedactionStatus,
    pub created_at: DateTime<Utc>,
    pub updated_at: DateTime<Utc>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub archived_at: Option<DateTime<Utc>>,
}

/// BindingRule connects a binding scope (channel or integration account) to a selected
/// profile and/or workspace.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct BindingRule {
    pub binding_id: String,
    pub tenant_id: String,
    pub scope_kind: ScopeKind,
    pub scope_ref: String,
    pub selected_profile_id: String,
    pub selected_profile_version_id: String,
    pub selected_workspace_id: String,
    pub status: BindingStatus,
    pub repair_status: RepairStatus,
    pub validation_status: ValidationStatus,
    pub actor_principal_id: String,
    pub audit_event_id: String,
    pub previous_selection_summary: String,
    pub resulting_selection_summary: String,
    pub redaction_status: RedactionStatus,
    pub created_at: DateTime<Utc>,
    pub updated_at: DateTime<Utc>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub disabled_at: Option<DateTime<Utc>>,
}

/// CapabilityVisibilityPolicy is tenant-owned policy describing a capability's
/// visibility for a profile- or workspace-scoped binding.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct CapabilityVisibilityPolicy {
    pub policy_id: String,
    pub tenant_id: String,
    pub scope_kind: VisibilityScopeKind,
    pub scope_ref: String,
    pub capability_id: String,
    pub visibility: Visibility,
    pub actor_principal_id: String,
    pub validation_status: ValidationStatus,
    pub redaction_status: RedactionStatus,
    pub created_at: DateTime<Utc>,
    pub updated_at: DateTime<Utc>,
}

/// EffectiveBindingSelection is the resolved profile, workspace, binding scope, and
/// capability visibility set that applies when new work starts. It is materialized into
/// RuntimeBindingEvidence; it is not persisted as its own table.
#[derive(Debug, Clone, PartialEq, Eq, Default, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct EffectiveBindingSelection {
    pub outcome: ResolutionOutcome,
    pub binding_scope: BindingRuntimeScope,
    pub binding_id: String,
    pub selected_profile_id: String,
    pub selected_profile_version_id: String,
    pub selected_workspace_id: String,
    /// Set when `outcome == ResolutionOutcome::REPAIR_REQUIRED`.
    pub repair_status: RepairStatus,
    pub repair_reason: String,
    /// Summarizes the effective per-capability decisions.
    #[serde(default)]
    pub capability_visibility: Vec<CapabilityDecision>,
}

/// CapabilityDecision is the resolved, safe-to-surface visibility decision for one
/// capability under the active binding (FR-013, FR-014, SC-012).
#[derive(Debug, Clone, PartialEq, Eq, Default, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct CapabilityDecision {
    pub capability_id: String,
    pub effective: EffectiveVisibility,
    pub default_enabled: bool,
    pub offered: bool,
    pub executable: bool,
    /// A safe machine-readable reason code explaining the decision.
    pub reason: String,
    /// The scope that produced the strictest (winning) constraint.
    pub scope: String,
}

/// RuntimeBindingEvidence is durable evidence attached to run/thread/session/workflow/
/// handoff/channel inspection showing which binding selections influenced execution.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct RuntimeBindingEvidence {
    pub projection_id: String,
    pub tenant_id: String,
    pub resource_kind: String,
    pub resource_id: String,
    pub selected_profile_id: String,
    pub selected_profile_version_id: String,
    pub selected_workspace_id: String,
    pub binding_scope: BindingRuntimeScope,
    pub binding_id: String,
    pub classification: Classification,
    pub selection_reason: String,
    #[serde(default)]
    pub capability_visibility: Vec<CapabilityDecision>,
    pub occurred_at: DateTime<Utc>,
    pub redaction_status: RedactionStatus,
}

/// The placeholder value the profile runtime projection recorded before Roadmap 58.
/// When an explicit binding influences a run, evidence records
/// `Classification::APPLIED` instead of this marker (FR-026, B22).
pub const DEFERRED_BINDING_CLASSIFICATION_MARKER: &str = "roadmap_58_deferred_binding_unapplied";

// Go's zero `time.Time` maps to the Unix epoch here; Default impls exist so tests and
// builders can construct records field-by-field like Go's zero-value structs.
impl Default for Workspace {
    fn default() -> Self {
        Self {
            workspace_id: String::new(),
            tenant_id: String::new(),
            display_name: String::new(),
            status: WorkspaceStatus::default(),
            is_default: false,
            owner_principal_id: String::new(),
            repair_status: RepairStatus::default(),
            redaction_status: RedactionStatus::default(),
            created_at: DateTime::<Utc>::UNIX_EPOCH,
            updated_at: DateTime::<Utc>::UNIX_EPOCH,
            archived_at: None,
        }
    }
}

impl Default for BindingRule {
    fn default() -> Self {
        Self {
            binding_id: String::new(),
            tenant_id: String::new(),
            scope_kind: ScopeKind::default(),
            scope_ref: String::new(),
            selected_profile_id: String::new(),
            selected_profile_version_id: String::new(),
            selected_workspace_id: String::new(),
            status: BindingStatus::default(),
            repair_status: RepairStatus::default(),
            validation_status: ValidationStatus::default(),
            actor_principal_id: String::new(),
            audit_event_id: String::new(),
            previous_selection_summary: String::new(),
            resulting_selection_summary: String::new(),
            redaction_status: RedactionStatus::default(),
            created_at: DateTime::<Utc>::UNIX_EPOCH,
            updated_at: DateTime::<Utc>::UNIX_EPOCH,
            disabled_at: None,
        }
    }
}

impl Default for CapabilityVisibilityPolicy {
    fn default() -> Self {
        Self {
            policy_id: String::new(),
            tenant_id: String::new(),
            scope_kind: VisibilityScopeKind::default(),
            scope_ref: String::new(),
            capability_id: String::new(),
            visibility: Visibility::default(),
            actor_principal_id: String::new(),
            validation_status: ValidationStatus::default(),
            redaction_status: RedactionStatus::default(),
            created_at: DateTime::<Utc>::UNIX_EPOCH,
            updated_at: DateTime::<Utc>::UNIX_EPOCH,
        }
    }
}

impl Default for RuntimeBindingEvidence {
    fn default() -> Self {
        Self {
            projection_id: String::new(),
            tenant_id: String::new(),
            resource_kind: String::new(),
            resource_id: String::new(),
            selected_profile_id: String::new(),
            selected_profile_version_id: String::new(),
            selected_workspace_id: String::new(),
            binding_scope: BindingRuntimeScope::default(),
            binding_id: String::new(),
            classification: Classification::default(),
            selection_reason: String::new(),
            capability_visibility: Vec::new(),
            occurred_at: DateTime::<Utc>::UNIX_EPOCH,
            redaction_status: RedactionStatus::default(),
        }
    }
}
