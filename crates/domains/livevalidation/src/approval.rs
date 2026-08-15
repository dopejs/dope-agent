//! Fresh-approval requirements and rollup (port of `approval.go`).

use crate::manager::Manager;
use crate::matrix::MatrixApproval;
use crate::matrix::SafetyClass;
use crate::matrix::ToolClass;
use crate::types::ApprovalStatus;
use crate::types::ApprovalSummary;
use crate::types::ApprovalTarget;
use crate::types::Attempt;
use crate::types::FreshApproval;
use crate::types::SideEffectScope;

/// One required fresh approval (Go `approvalRequirement`).
#[derive(Clone)]
struct ApprovalRequirement {
    validation_id: String,
    tenant_id: String,
    scope_id: String,
    target: ApprovalTarget,
    tool_class: ToolClass,
    safety_class: SafetyClass,
    action_ref: String,
}

impl Manager {
    /// Port of `Manager.approvalSummary`.
    pub(crate) fn approval_summary(
        &self,
        attempt: &Attempt,
        scope: &SideEffectScope,
        approvals: &[FreshApproval],
    ) -> ApprovalSummary {
        let requirements = self.approval_requirements(attempt, scope);
        let required = requirements.len() as i64;
        let mut approved = 0i64;
        let mut denied = 0i64;
        let mut expired = 0i64;
        for requirement in &requirements {
            let mut status = ApprovalStatus::from(ApprovalStatus::PENDING);
            for approval in approvals {
                if approval_matches(requirement, approval) {
                    status = approval.status.clone();
                    break;
                }
            }
            if status == ApprovalStatus::APPROVED {
                approved += 1;
            } else if status == ApprovalStatus::DENIED {
                denied += 1;
            } else if status == ApprovalStatus::EXPIRED {
                expired += 1;
            }
        }
        let pending = (required - approved - denied - expired).max(0);
        ApprovalSummary {
            required,
            approved,
            denied,
            expired,
            pending,
        }
    }

    /// Port of `Manager.approvalRequirements`.
    fn approval_requirements(
        &self,
        attempt: &Attempt,
        scope: &SideEffectScope,
    ) -> Vec<ApprovalRequirement> {
        let Ok(matrix) = self.support_matrix() else {
            return Vec::new();
        };
        let mut requirements = Vec::new();
        for tool_class in &scope.included_tool_classes {
            let Ok(row) = matrix.lookup(tool_class) else {
                continue;
            };
            let base = ApprovalRequirement {
                validation_id: attempt.validation_id.clone(),
                tenant_id: attempt.tenant_id.clone(),
                scope_id: scope.scope_id.clone(),
                target: ApprovalTarget::default(),
                tool_class: tool_class.clone(),
                safety_class: row.safety_class.clone(),
                action_ref: String::new(),
            };
            let per_action = row.approval == MatrixApproval::PER_ACTION
                || row.safety_class == SafetyClass::NON_IDEMPOTENT_MUTATION;
            if per_action {
                let actions = matching_actions(&scope.included_actions, tool_class);
                if actions.is_empty() {
                    let mut req = base;
                    req.target = ApprovalTarget::from(ApprovalTarget::ACTION);
                    requirements.push(req);
                    continue;
                }
                for action in actions {
                    let mut req = base.clone();
                    req.target = ApprovalTarget::from(ApprovalTarget::ACTION);
                    req.action_ref = action;
                    requirements.push(req);
                }
            } else if row.approval == MatrixApproval::SCOPE_LEVEL {
                let mut req = base;
                req.target = ApprovalTarget::from(ApprovalTarget::SCOPE);
                requirements.push(req);
            }
        }
        requirements
    }
}

fn approval_matches(requirement: &ApprovalRequirement, approval: &FreshApproval) -> bool {
    if approval.approval_target != requirement.target
        || approval.tool_class != requirement.tool_class
    {
        return false;
    }
    if approval.validation_id != requirement.validation_id {
        return false;
    }
    if approval.tenant_id != requirement.tenant_id {
        return false;
    }
    if approval.safety_class != requirement.safety_class {
        return false;
    }
    if requirement.target == ApprovalTarget::SCOPE
        && approval.approved_scope != requirement.scope_id
    {
        return false;
    }
    if requirement.target == ApprovalTarget::ACTION
        && !requirement.action_ref.is_empty()
        && approval.action_ref != requirement.action_ref
    {
        return false;
    }
    true
}

fn matching_actions(actions: &[String], _tool_class: &ToolClass) -> Vec<String> {
    actions.to_vec()
}

/// Port of `normalizeFreshApprovals`: back-fills the tenant id on approvals
/// that were submitted without one.
pub(crate) fn normalize_fresh_approvals(
    mut approvals: Vec<FreshApproval>,
    attempt: &Attempt,
) -> Vec<FreshApproval> {
    if approvals.is_empty() {
        return approvals;
    }
    for approval in &mut approvals {
        if approval.tenant_id.is_empty() {
            approval.tenant_id = attempt.tenant_id.clone();
        }
    }
    approvals
}
