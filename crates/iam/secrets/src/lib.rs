//! Port of `daemon/internal/secrets`. See rs/MIGRATION.md for conventions.
//!
//! Tenant secret lifecycle: reference model (`TenantSecret` / `SecretVersion`),
//! a `Manager` enforcing create/rotate/disable/resolve transitions, a local
//! filesystem [`LocalBackend`] value store, redaction guarantees (secret
//! values are never serialized and never appear in `Debug` output), and the
//! legacy local-credential bridge migration.
//!
//! Persistence inversion: Go's `internal/store` implements the `Store` /
//! `BridgeProgressStore` / `LegacyCredentialResourceStore` interfaces. Here
//! those are object-safe traits owned by this crate; `kura-store` (wave 5)
//! will implement them over SQLite. This crate stays persistence-free.
//!
//! IDs are prefixed random-hex strings (`sec_*`, `secver_*`), not UUIDs, so
//! all ID fields are plain `String`.

mod backend;
mod bridge;
mod error;
mod manager;
mod redaction;
mod types;

#[cfg(test)]
mod testutil;

pub use backend::{LocalBackend, ValueBackend, random_hex, safe_path_segment};
pub use bridge::{
    BridgeProgressStore, LegacyCredentialResourceStore, LocalCredentialBridgeInput,
    LocalCredentialBridgeResult, bridge_local_credential_files,
};
pub use error::{Result, SecretsError};
pub use manager::{BoxFuture, Manager, Store};
pub use redaction::{
    REDACTED_VALUE, RedactedSecretSummary, contains_any_leak_sentinel,
    json_contains_any_leak_sentinel, redact_secret_refs, redact_secret_value,
};
pub use types::*;
