//! Port of daemon/internal/contracts. See rs/MIGRATION.md for conventions.
//!
//! JSON schema/fixture loading and validation helpers that walk the
//! repository's `schemas/` directory.

mod validator;

pub use validator::{ContractError, Validator};
