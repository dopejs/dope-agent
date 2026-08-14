//! Secret-value redaction for sandbox result surfaces.
//!
//! Ported from daemon/internal/sandbox/manager.go: collectSecretRedactionValues,
//! collectSecretRedactionValuesFromProcessEnv, redactSubprocessResult,
//! redactSecretText, derivedSecretVariants.

use std::collections::{HashMap, HashSet};

use base64::Engine as _;
use base64::engine::general_purpose::{STANDARD, STANDARD_NO_PAD, URL_SAFE, URL_SAFE_NO_PAD};
use md5::Md5;
use sha1::Sha1;
use sha2::{Digest, Sha256, Sha512};

use crate::{ConsumerContractView, SecretResolution, SubprocessResult};

/// The literal replacement for redacted secret material.
pub const REDACTED: &str = "[REDACTED]";

/// Go `collectSecretRedactionValues`: gathers the resolved secret values (and
/// their derived encodings) from the request environment, for redaction of
/// stdout/stderr/error surfaces.
#[must_use]
pub fn collect_secret_redaction_values(
    env: &HashMap<String, String>,
    consumer: &ConsumerContractView,
) -> Vec<String> {
    if env.is_empty() || consumer.secret_scope.is_empty() {
        return Vec::new();
    }
    let mut values: Vec<String> = Vec::with_capacity(consumer.secret_scope.len());
    let mut seen: HashSet<String> = HashSet::new();
    for item in &consumer.secret_scope {
        if item.resolution != SecretResolution::Resolved {
            continue;
        }
        let value = env
            .get(&item.secret_ref)
            .map(|v| v.trim().to_string())
            .unwrap_or_default();
        if value.is_empty() {
            continue;
        }
        if seen.contains(&value) {
            continue;
        }
        seen.insert(value.clone());
        values.push(value.clone());
        for derived in derived_secret_variants(&value) {
            if seen.insert(derived.clone()) {
                values.push(derived);
            }
        }
    }
    values
}

/// Go `collectSecretRedactionValuesFromProcessEnv`: resolves secret refs from
/// the daemon process environment (used by attached executions where the
/// request environment is not available to the runner thread).
#[must_use]
pub fn collect_secret_redaction_values_from_process_env(
    consumer: &ConsumerContractView,
) -> Vec<String> {
    if consumer.secret_scope.is_empty() {
        return Vec::new();
    }
    let mut env: HashMap<String, String> = HashMap::new();
    for item in &consumer.secret_scope {
        if let Ok(value) = std::env::var(&item.secret_ref) {
            env.insert(item.secret_ref.clone(), value);
        }
    }
    collect_secret_redaction_values(&env, consumer)
}

/// Go `redactSubprocessResult`: applies secret-value redaction to the
/// stdout/stderr/error surfaces of a subprocess result.
#[must_use]
pub fn redact_subprocess_result(
    mut result: SubprocessResult,
    secret_values: &[String],
) -> SubprocessResult {
    if secret_values.is_empty() {
        return result;
    }
    result.stdout = redact_secret_text(&result.stdout, secret_values);
    result.stderr = redact_secret_text(&result.stderr, secret_values);
    result.error = redact_secret_text(&result.error, secret_values);
    result
}

/// Go `redactSecretText`: replaces every known secret value (and derived
/// variant) with the redaction marker.
#[must_use]
pub fn redact_secret_text(text: &str, secret_values: &[String]) -> String {
    if text.trim().is_empty() || secret_values.is_empty() {
        return text.to_string();
    }
    let mut redacted = text.to_string();
    for secret_value in secret_values {
        if secret_value.is_empty() {
            continue;
        }
        redacted = redacted.replace(secret_value, REDACTED);
    }
    redacted
}

/// Go `derivedSecretVariants`: hex/base64 encodings and standard digests of a
/// secret value, so derived material is redacted too.
#[must_use]
pub fn derived_secret_variants(value: &str) -> Vec<String> {
    let trimmed = value.trim();
    if trimmed.is_empty() {
        return Vec::new();
    }
    let items: Vec<String> = vec![
        hex_encode(trimmed.as_bytes()),
        STANDARD.encode(trimmed.as_bytes()),
        STANDARD_NO_PAD.encode(trimmed.as_bytes()),
        URL_SAFE.encode(trimmed.as_bytes()),
        URL_SAFE_NO_PAD.encode(trimmed.as_bytes()),
        format!("{:x}", Md5::digest(trimmed.as_bytes())),
        format!("{:x}", Sha1::digest(trimmed.as_bytes())),
        format!("{:x}", Sha256::digest(trimmed.as_bytes())),
        format!("{:x}", Sha256::digest(trimmed.as_bytes())),
        format!("{:x}", Sha512::digest(trimmed.as_bytes())),
    ];
    let mut out: Vec<String> = Vec::with_capacity(items.len());
    for item in items {
        if !item.trim().is_empty() && item != trimmed {
            out.push(item);
        }
    }
    out
}

/// Go `hex.EncodeToString` (no hex crate in the workspace dependency set).
#[must_use]
pub fn hex_encode(bytes: &[u8]) -> String {
    let mut out = String::with_capacity(bytes.len() * 2);
    for byte in bytes {
        out.push_str(&format!("{byte:02x}"));
    }
    out
}
