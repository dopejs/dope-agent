//! Shared helpers across the evaluation modules (Go free functions that live in
//! `manager.go` / `fixtures.go` and are reused by other files).

use chrono::{DateTime, NaiveDate, Utc};

/// Go's zero `time.Time` (`0001-01-01T00:00:00Z`), used for `IsZero` checks.
#[must_use]
pub fn go_zero_time() -> DateTime<Utc> {
    DateTime::from_naive_utc_and_offset(
        NaiveDate::from_ymd_opt(1, 1, 1)
            .and_then(|date| date.and_hms_opt(0, 0, 0))
            .unwrap_or(DateTime::UNIX_EPOCH.naive_utc()),
        Utc,
    )
}

/// Go `firstNonEmpty`: first value whose trimmed form is non-empty, else "".
#[must_use]
pub fn first_non_empty(values: &[&str]) -> String {
    values
        .iter()
        .find(|value| !value.trim().is_empty())
        .map(|value| (*value).to_string())
        .unwrap_or_default()
}

/// Go `zeroTimeDefault`: fallback when the value is the zero time.
#[must_use]
pub fn zero_time_default(value: DateTime<Utc>, fallback: DateTime<Utc>) -> DateTime<Utc> {
    if value == go_zero_time() || value == DateTime::UNIX_EPOCH {
        fallback
    } else {
        value
    }
}

/// Go `replayModeDefault`: non-live when unset. With the closed enum the
/// default already is `NonLive`, so this is the identity on the value.
#[must_use]
pub fn replay_mode_default(mode: Option<crate::types::ReplayMode>) -> crate::types::ReplayMode {
    mode.unwrap_or_default()
}

/// Go `appendReasons` with the "cannot proceed safely" fallback.
#[must_use]
pub fn append_reasons(primary: &[String], secondary: &[String]) -> Vec<String> {
    let mut items = Vec::with_capacity(primary.len() + secondary.len());
    items.extend_from_slice(primary);
    items.extend_from_slice(secondary);
    if items.is_empty() {
        items.push("replay cannot proceed safely with available evidence".to_string());
    }
    items
}

/// Go `newID`: `prefix + "_" + 16 hex chars` (Go uses 8 random bytes).
#[must_use]
pub fn new_id(prefix: &str) -> String {
    let hex = uuid::Uuid::new_v4().simple().to_string();
    format!("{prefix}_{}", &hex[..16])
}
