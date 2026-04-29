package livevalidation

func (m *Manager) approvalSummary(attempt Attempt, scope SideEffectScope, approvals []FreshApproval) ApprovalSummary {
	requirements := m.approvalRequirements(attempt, scope)
	required := len(requirements)
	approved := 0
	denied := 0
	expired := 0
	for _, requirement := range requirements {
		status := ApprovalStatusPending
		for _, approval := range approvals {
			if approvalMatches(requirement, approval) {
				status = approval.Status
				break
			}
		}
		switch status {
		case ApprovalStatusApproved:
			approved++
		case ApprovalStatusDenied:
			denied++
		case ApprovalStatusExpired:
			expired++
		}
	}
	pending := required - approved - denied - expired
	if pending < 0 {
		pending = 0
	}
	return ApprovalSummary{Required: required, Approved: approved, Denied: denied, Expired: expired, Pending: pending}
}

type approvalRequirement struct {
	validationID string
	tenantID     string
	scopeID      string
	target       ApprovalTarget
	toolClass    ToolClass
	safetyClass  SafetyClass
	actionRef    string
}

func (m *Manager) approvalRequirements(attempt Attempt, scope SideEffectScope) []approvalRequirement {
	matrix, err := m.SupportMatrix()
	if err != nil {
		return nil
	}
	requirements := []approvalRequirement{}
	for _, toolClass := range scope.IncludedToolClasses {
		row, err := matrix.Lookup(toolClass)
		if err != nil {
			continue
		}
		base := approvalRequirement{
			validationID: attempt.ValidationID,
			tenantID:     attempt.TenantID,
			scopeID:      scope.ScopeID,
			toolClass:    toolClass,
			safetyClass:  row.SafetyClass,
		}
		switch {
		case row.Approval == MatrixApprovalPerAction || row.SafetyClass == SafetyClassNonIdempotentMutation:
			actions := matchingActions(scope.IncludedActions, toolClass)
			if len(actions) == 0 {
				req := base
				req.target = ApprovalTargetAction
				requirements = append(requirements, req)
				continue
			}
			for _, action := range actions {
				req := base
				req.target = ApprovalTargetAction
				req.actionRef = action
				requirements = append(requirements, req)
			}
		case row.Approval == MatrixApprovalScopeLevel:
			req := base
			req.target = ApprovalTargetScope
			requirements = append(requirements, req)
		}
	}
	return requirements
}

func approvalMatches(requirement approvalRequirement, approval FreshApproval) bool {
	if approval.Target != requirement.target || approval.ToolClass != requirement.toolClass {
		return false
	}
	if approval.ValidationID != requirement.validationID {
		return false
	}
	if approval.TenantID != requirement.tenantID {
		return false
	}
	if approval.SafetyClass != requirement.safetyClass {
		return false
	}
	if requirement.target == ApprovalTargetScope && approval.ApprovedScope != requirement.scopeID {
		return false
	}
	if requirement.target == ApprovalTargetAction && requirement.actionRef != "" && approval.ActionRef != requirement.actionRef {
		return false
	}
	return true
}

func matchingActions(actions []string, _ ToolClass) []string {
	return actions
}

func normalizeFreshApprovals(approvals []FreshApproval, attempt Attempt) []FreshApproval {
	if len(approvals) == 0 {
		return nil
	}
	normalized := make([]FreshApproval, len(approvals))
	copy(normalized, approvals)
	for idx := range normalized {
		if normalized[idx].TenantID == "" {
			normalized[idx].TenantID = attempt.TenantID
		}
	}
	return normalized
}
