//! Ledger event sink and event-name constants (port of `events.go`).

use std::sync::Arc;

use crate::types::SideEffectLedgerEntry;

/// Emits structured evidence when a side-effect ledger entry changes. Go
/// threads `context.Context`; in Rust the tenant context lives in the
/// `dope_identity::tenantctx` task-local, so the sink only receives the event
/// name and the entry.
pub type LedgerEventSink = Arc<dyn Fn(&str, &SideEffectLedgerEntry) + Send + Sync>;

pub const LEDGER_EVENT_SIDE_EFFECT_RECORDED: &str = "live_validation.side_effect_recorded";
pub const LEDGER_EVENT_OPERATOR_ACTION_NEEDED: &str = "live_validation.operator_action_needed";
