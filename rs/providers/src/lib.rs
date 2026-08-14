//! Port of `daemon/internal/providers`: the LLM provider registry, profile
//! projection, managed-auth lifecycle, and dispatch resolution (Roadmap 9/10).

mod manager;
mod types;

pub use manager::{
    new_manager, Check, CheckInput, Manager, new_check_id, ResolvedDispatch, SyncResult,
};
pub use types::*;
