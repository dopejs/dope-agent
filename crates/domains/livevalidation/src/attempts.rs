//! Attempt lifecycle (port of `attempts.go`): construction, blocking, and
//! the list/get/abort operations.

use chrono::DateTime;
use chrono::Utc;

use kura_identity::TenantContext;
use kura_identity::tenantctx;

use crate::error::LiveValidationError;
use crate::error::StartFailure;
use crate::manager::Denial;
use crate::manager::Manager;
use crate::manager::StartInput;
use crate::manager::StartResult;
use crate::manager::first_non_empty;
use crate::store::AttemptFilter;
use crate::types::Attempt;
use crate::types::AttemptStatus;
use crate::types::GateDecision;

impl Manager {
    /// Port of `Manager.newAttempt`.
    pub(crate) fn new_attempt(
        &self,
        input: &StartInput,
        tenant_context: &TenantContext,
        now: DateTime<Utc>,
    ) -> Attempt {
        let mut validation_id = input.validation_id.clone();
        if validation_id.is_empty() {
            validation_id = crate::manager::new_id("live_validation");
        }
        let mut scope = input.requested_scope.clone();
        scope.validation_id =
            first_non_empty([scope.validation_id.as_str(), validation_id.as_str()]);
        Attempt {
            validation_id,
            tenant_id: tenant_context.tenant_id.clone(),
            candidate_id: input.candidate_id.clone(),
            source_attempt_id: input.source_attempt_id.clone(),
            requested_by: tenant_context.principal_id.clone(),
            environment_scope: self.environment_scope().to_string(),
            requested_scope: scope,
            status: AttemptStatus::from(AttemptStatus::QUEUED),
            permission_decision: GateDecision {
                allowed: true,
                checked_at: now,
                ..GateDecision::default()
            },
            quota_decision: GateDecision {
                allowed: true,
                checked_at: now,
                ..GateDecision::default()
            },
            kill_switch_decision: GateDecision {
                allowed: true,
                checked_at: now,
                ..GateDecision::default()
            },
            created_at: now,
            updated_at: now,
            ..Attempt::default()
        }
    }

    /// Port of `Manager.block`: persists the blocked attempt and returns the
    /// `StartFailure::Blocked` verdict carrying it.
    pub(crate) async fn block(
        &self,
        mut attempt: Attempt,
        gate: &str,
        reason_code: &str,
        message: &str,
        reference: &str,
    ) -> Result<StartFailure, LiveValidationError> {
        attempt.status = AttemptStatus::from(AttemptStatus::BLOCKED);
        attempt.updated_at = self.now();
        self.persist_attempt(&attempt).await?;
        let denial = Denial {
            gate: gate.to_string(),
            reason_code: reason_code.to_string(),
            message: message.to_string(),
            reference: reference.to_string(),
        };
        Ok(StartFailure::Blocked(StartResult {
            attempt,
            denials: vec![denial],
        }))
    }

    /// Port of `Manager.persistAttempt`.
    pub(crate) async fn persist_attempt(
        &self,
        attempt: &Attempt,
    ) -> Result<(), LiveValidationError> {
        if let Some(store) = self.store() {
            store.upsert_attempt(attempt.clone()).await?;
        }
        Ok(())
    }

    /// Port of `Manager.ListAttempts`.
    pub async fn list_attempts(
        &self,
        mut filter: AttemptFilter,
    ) -> Result<Vec<Attempt>, LiveValidationError> {
        let Some(store) = self.store() else {
            return Ok(Vec::new());
        };
        filter.environment_scope =
            first_non_empty([filter.environment_scope.as_str(), self.environment_scope()]);
        store.list_attempts(filter).await
    }

    /// Port of `Manager.GetAttempt`.
    pub async fn get_attempt(
        &self,
        validation_id: &str,
    ) -> Result<Option<Attempt>, LiveValidationError> {
        let Some(store) = self.store() else {
            return Ok(None);
        };
        let tenant_id = tenantctx::from_context()
            .map(|ctx| ctx.tenant_id)
            .unwrap_or_default();
        store.get_attempt(&tenant_id, validation_id).await
    }

    /// Port of `Manager.Abort`.
    pub async fn abort(&self, validation_id: &str) -> Result<Attempt, LiveValidationError> {
        let Some(mut attempt) = self.get_attempt(validation_id).await? else {
            return Err(LiveValidationError::NotFound(validation_id.to_string()));
        };
        let terminal = [
            AttemptStatus::from(AttemptStatus::COMPLETED),
            AttemptStatus::from(AttemptStatus::ABORTED),
            AttemptStatus::from(AttemptStatus::FAILED),
        ];
        if terminal.contains(&attempt.status) {
            return Ok(attempt);
        }
        let now = self.now();
        attempt.status = AttemptStatus::from(AttemptStatus::ABORTED);
        attempt.completed_at = Some(now);
        attempt.updated_at = now;
        self.persist_attempt(&attempt).await?;
        Ok(attempt)
    }
}
