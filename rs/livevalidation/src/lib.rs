//! Port of `daemon/internal/livevalidation`: Roadmap 40 live replay safety
//! policy. See rs/MIGRATION.md for conventions.
//!
//! The crate keeps live validation separate from non-live evaluation replay.
//! It coordinates support-matrix readiness, permission and quota gates, fresh
//! approvals, kill switches, side-effect ledger evidence, retry/abort
//! decisions, ambiguous commit handling, reconciliation, retention, and
//! outcome comparison.
//!
//! Go threads `context.Context` through every call; in Rust the resolved
//! tenant context travels in the [`dope_identity::tenantctx`] task-local, so
//! async manager methods read it back at gate time the same way.

macro_rules! define_string_enum {
    ($(#[$meta:meta])* $name:ident { $( $const_name:ident => $value:literal ),+ $(,)? }) => {
        $(#[$meta])*
        ///
        /// Open string enum (Go `type X string`): known values are exposed as
        /// associated constants, but arbitrary persisted values round-trip
        /// unchanged.
        #[derive(Debug, Clone, Default, PartialEq, Eq, Hash, PartialOrd, Ord, serde::Serialize, serde::Deserialize)]
        #[serde(transparent)]
        pub struct $name(pub String);

        impl $name {
            $( pub const $const_name: &'static str = $value; )+

            #[must_use]
            pub fn new(value: impl Into<String>) -> Self {
                Self(value.into())
            }

            #[must_use]
            pub fn as_str(&self) -> &str {
                &self.0
            }

            #[must_use]
            pub fn is_empty(&self) -> bool {
                self.0.is_empty()
            }
        }

        impl From<&str> for $name {
            fn from(value: &str) -> Self {
                Self(value.to_string())
            }
        }

        impl From<String> for $name {
            fn from(value: String) -> Self {
                Self(value)
            }
        }

        impl std::fmt::Display for $name {
            fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                f.write_str(&self.0)
            }
        }

        impl PartialEq<&str> for $name {
            fn eq(&self, other: &&str) -> bool {
                self.0 == *other
            }
        }

        impl PartialEq<$name> for &str {
            fn eq(&self, other: &$name) -> bool {
                *self == other.0
            }
        }
    };
}

mod ambiguous_commit;
mod approval;
mod attempts;
mod comparison;
mod error;
mod events;
mod executor;
mod fake_outcome;
mod idempotency;
mod kill_switch;
mod ledger;
mod manager;
mod matrix;
mod matrix_connector_smoke;
mod plan_executor;
mod readiness;
mod reconciliation;
mod retention;
mod store;
mod types;

pub use error::LiveValidationError;
pub use error::MatrixError;
pub use error::StartFailure;
pub use events::LEDGER_EVENT_OPERATOR_ACTION_NEEDED;
pub use events::LEDGER_EVENT_SIDE_EFFECT_RECORDED;
pub use events::LedgerEventSink;
pub use executor::SideEffectExecutionInput;
pub use fake_outcome::FakeOutcomeResult;
pub use fake_outcome::fake_outcome_result_for;
pub use idempotency::correlation_key;
pub use ledger::LedgerOutcome;
pub use ledger::is_terminal_ledger_outcome;
pub use ledger::known_ledger_outcome;
pub use ledger::validate_ledger_transition;
pub use manager::CandidateToolClassResolver;
pub use manager::Clock;
pub use manager::Denial;
pub use manager::Dependencies;
pub use manager::Manager;
pub use manager::StartInput;
pub use manager::StartResult;
pub use matrix::CompensationKind;
pub use matrix::Matrix;
pub use matrix::MatrixApproval;
pub use matrix::MatrixRow;
pub use matrix::RetryPolicy;
pub use matrix::SafetyClass;
pub use matrix::ToolClass;
pub use matrix::default_matrix_row;
pub use matrix::default_matrix_rows;
pub use matrix_connector_smoke::MatrixConnectorSmokeEvidence;
pub use matrix_connector_smoke::MatrixConnectorSmokeInput;
pub use matrix_connector_smoke::build_matrix_connector_smoke_evidence;
pub use plan_executor::SideEffectPlanResult;
pub use plan_executor::SideEffectPlanStep;
pub use readiness::CandidateReadinessInput;
pub use readiness::CandidateReadinessResult;
pub use readiness::ReadinessStatus;
pub use readiness::ToolClassReadiness;
pub use readiness::evaluate_candidate_readiness;
pub use store::AttemptFilter;
pub use store::BoxFuture;
pub use store::ComparisonFilter;
pub use store::KillSwitchFilter;
pub use store::LedgerFilter;
pub use store::Store;
pub use types::*;

#[cfg(test)]
mod testutil;
