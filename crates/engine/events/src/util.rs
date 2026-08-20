//! Internal helpers shared by the event constructors: Go zero-time
//! semantics and the payload-map builder macro.

use chrono::{DateTime, NaiveDate, Utc};

/// Go's zero `time.Time` (Jan 1, year 1) — the sentinel the ported domain
/// crates use to represent "no time set". Mirrors `kura_billing::go_zero_time`.
#[must_use]
pub(crate) fn go_zero_time() -> DateTime<Utc> {
    DateTime::from_naive_utc_and_offset(
        NaiveDate::from_ymd_opt(1, 1, 1)
            .and_then(|date| date.and_hms_opt(0, 0, 0))
            .unwrap_or(DateTime::UNIX_EPOCH.naive_utc()),
        Utc,
    )
}

/// True when `t` is an "unset" sentinel: Go's zero `time.Time` (year 1),
/// chrono's `DateTime::<Utc>::MIN_UTC`, or chrono's `Default` (`UNIX_EPOCH` —
/// what `..Default::default()` yields on derived-`Default` structs). All three
/// appear across the ported domain crates for unset times.
#[must_use]
pub(crate) fn is_go_zero_time(t: DateTime<Utc>) -> bool {
    t == go_zero_time() || t == DateTime::<Utc>::MIN_UTC || t == DateTime::<Utc>::UNIX_EPOCH
}

/// `time.Now().UTC()` equivalent used when a Go constructor falls back to
/// "now" for a zero `OccurredAt`.
#[must_use]
pub(crate) fn now_utc() -> DateTime<Utc> {
    Utc::now()
}

/// Builds a `serde_json::Map` payload from key/value pairs, mirroring the Go
/// `map[string]any{...}` literals in the event builders. Values are serialized
/// with `serde_json::to_value` (infallible for the scalar/string/time types the
/// builders emit; a failure collapses to `Null` rather than panicking).
macro_rules! payload {
    ($($key:expr => $value:expr),+ $(,)?) => {{
        let mut map = serde_json::Map::new();
        $(
            map.insert(
                $key.to_string(),
                serde_json::to_value(&$value).unwrap_or(serde_json::Value::Null),
            );
        )+
        map
    }};
}
pub(crate) use payload;
