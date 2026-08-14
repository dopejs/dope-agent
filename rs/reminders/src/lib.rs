//! dope-reminders: port of daemon/internal/reminders — the reminder ledger manager
//! (create/acknowledge/complete/dismiss/cancel/snooze/reschedule, tick-based due
//! detection, delivery linkage, workflow-launch behavior), follow-up link staleness
//! refresh, and the live-validation support-matrix row for reminder lifecycle
//! mutations.
//!
//! The manager is a synchronous port (like dope-delivery / dope-scheduler):
//! context.Context is dropped, the catch-up + tick loop runs in a detached std thread
//! when `start` is called, and the store is shared as Arc<parking_lot::Mutex<SQLiteStore>>
//! so the manager is Send + Sync for axum AppState.

pub mod follow_up;
pub mod live_validation;
mod manager;
mod types;

pub use follow_up::{clone_follow_up_link, refresh_follow_up_link, resolve_tenant_id};
pub use live_validation::live_validation_matrix_rows;
pub use manager::{Clock, Dependencies, Manager, RealClock};
pub use types::*;
