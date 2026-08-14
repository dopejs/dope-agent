//! Telegram channel connector (port of `daemon/internal/connectors/telegram`).
//!
//! The crate keeps Telegram-specific setup, allowment, routing, diagnostic,
//! smoke, and fake-transport behavior behind the shared connector contracts:
//! - [`transport`] — Bot API REST long-poll transport (`getMe` /
//!   `getUpdates` / `sendMessage`), update normalization, fake transport.
//! - [`allowment`] — allowlist evaluation (who may message the bot) and route
//!   decisions (direct / group / mention / command gating).
//! - [`readiness`] — hosted-setup evaluation into terminal states.
//! - [`diagnostics`] — error classification and diagnostic-state building.
//! - [`smoke`] — structured smoke-evidence building.
//! - [`runtime`] — the connector runtime driving the shared message loop.
//!
//! Binding conventions per rs/MIGRATION.md: serde camelCase, thiserror enums,
//! `chrono::DateTime<Utc>`, snake_case methods, no unwrap/expect outside
//! tests. The Go `tenantctx` runtime tenant source is not ported; the tenant
//! id is passed explicitly through [`Config::tenant_id`] (with the Go
//! store-default fallback preserved in the runtime).

mod allowment;
mod diagnostics;
mod readiness;
mod runtime;
mod smoke;
mod transport;

pub use allowment::*;
pub use diagnostics::*;
pub use readiness::*;
pub use runtime::*;
pub use smoke::*;
pub use transport::*;

/// Errors surfaced by the Telegram connector. Display strings match the Go
/// sentinel errors exactly.
#[derive(Debug, thiserror::Error, Clone, PartialEq, Eq)]
pub enum TelegramError {
    #[error("telegram connector id is required")]
    ConnectorIdRequired,
    #[error("telegram display name is required")]
    DisplayNameRequired,
    #[error("telegram bot token is required")]
    BotTokenRequired,
    #[error("connector id is required")]
    DiagnosticConnectorRequired,
    #[error("diagnostic reason code is required")]
    DiagnosticReasonRequired,
    #[error("{0}")]
    Store(String),
    #[error("{0}")]
    Supervisor(String),
    #[error("{0}")]
    Transport(String),
}

impl From<dope_connectors::ConnectorsError> for TelegramError {
    fn from(err: dope_connectors::ConnectorsError) -> Self {
        TelegramError::Supervisor(err.to_string())
    }
}

/// String enum with explicit per-variant wire literals plus an explicit
/// default variant (the Go zero-value equivalent): serde representation is
/// exactly the literal, and `as_str`/`Display` agree with it.
macro_rules! wire_enum {
    ($name:ident, default $default:ident; $($variant:ident => $lit:literal),+ $(,)?) => {
        #[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, serde::Serialize, serde::Deserialize)]
        pub enum $name {
            $(
                #[serde(rename = $lit)]
                $variant
            ),+
        }
        impl Default for $name {
            fn default() -> Self {
                $name::$default
            }
        }
        impl $name {
            #[must_use]
            pub fn as_str(self) -> &'static str {
                match self {
                    $( $name::$variant => $lit ),+
                }
            }
        }
        impl std::fmt::Display for $name {
            fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                f.write_str(self.as_str())
            }
        }
    };
}
pub(crate) use wire_enum;

/// Reports whether a timestamp is the Go zero `time.Time` (Unix epoch), the
/// value Rust's `DateTime<Utc>::default()` produces.
#[must_use]
pub(crate) fn is_unset_time(dt: &chrono::DateTime<chrono::Utc>) -> bool {
    dt.timestamp() == 0 && dt.timestamp_subsec_nanos() == 0
}
