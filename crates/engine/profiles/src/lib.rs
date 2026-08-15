//! Port of daemon/internal/profiles. See rs/MIGRATION.md for conventions.
//!
//! Agent profile domain: profiles, versions, active selections, overlay
//! references, runtime projections and audit events, plus the mutation policy
//! validation and redaction rules that keep raw persona text out of persisted
//! and API surfaces.
//!
//! Wire notes (persisted as `document_json`, returned by the API):
//! - All string-kind types mirror Go `type X string`: unknown values
//!   round-trip through JSON so that policy validation (not serde) rejects
//!   them with stable reason codes.
//! - ID fields stay plain `String`. Go profile IDs are `prof_`-prefixed store
//!   IDs (not UUIDs), so `dope_protocol::ProfileId` cannot be used here
//!   without breaking deserialization of existing documents.

mod policy;
mod projection;
mod redaction;
mod types;

pub use policy::{
    ProfilesError, can_activate, invalid_profile_reason, rollback_eligibility_for,
    validate_mutation, validation_reason_code,
};
pub use projection::{
    APPLIED_BINDING_CLASSIFICATION_MARKER, DEFERRED_BINDING_CLASSIFICATION_MARKER,
    RuntimeProjectionInput, build_runtime_projection, default_legacy_mapping_evidence,
    default_legacy_overlay_reference_inputs,
};
pub use redaction::{normalize_overlay, redact_profile, safe_profile_summary};
pub use types::*;
