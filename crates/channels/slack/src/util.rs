//! Shared package helpers (Go package-level helpers used across the package).

use chrono::{DateTime, Utc};

/// Go firstNonEmpty: the first trimmed non-empty value, else "".
#[must_use]
pub fn first_non_empty(values: &[&str]) -> String {
    for value in values {
        let trimmed = value.trim();
        if !trimmed.is_empty() {
            return trimmed.to_string();
        }
    }
    String::new()
}

/// A chrono-defaulted timestamp (UNIX epoch) stands in for Go's zero
/// time.Time.
#[must_use]
pub fn is_unset_time(dt: &DateTime<Utc>) -> bool {
    dt.timestamp() == 0 && dt.timestamp_subsec_nanos() == 0
}

/// Converts a JSON value into a string-keyed object map (empty when the value
/// is not an object).
#[must_use]
pub fn json_object(value: serde_json::Value) -> serde_json::Map<String, serde_json::Value> {
    value.as_object().cloned().unwrap_or_default()
}
